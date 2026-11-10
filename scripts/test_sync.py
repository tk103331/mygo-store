#!/usr/bin/env python3
"""Tests of the catalog sync, runnable without network access."""

from __future__ import annotations

import contextlib
import importlib.util
import io
import pathlib
import tempfile
import unittest
import urllib.error
from unittest import mock

HERE = pathlib.Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("sync", HERE / "sync.py")
sync = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(sync)


class RepoSlugTest(unittest.TestCase):
    def test_forms(self):
        cases = {
            "owner/name": "owner/name",
            "https://github.com/owner/name": "owner/name",
            "http://github.com/owner/name": "owner/name",
            "github.com/owner/name": "owner/name",
            "https://github.com/owner/name.git": "owner/name",
            "https://github.com/owner/name.git/": "owner/name",
            "git@github.com:owner/name.git": "owner/name",
            "  owner/name  ": "owner/name",
        }
        for given, want in cases.items():
            with self.subTest(given=given):
                self.assertEqual(sync.repo_slug(given), want)

    def test_rejects(self):
        for given in ("", "owner", "owner/name/extra", "owner/na me", "../name", "owner/"):
            with self.subTest(given=given):
                self.assertEqual(sync.repo_slug(given), "")


class ParseIdentifierTest(unittest.TestCase):
    def test_json(self):
        text = '{"name": "App", "identifier": "com.example.app"}'
        self.assertEqual(sync.parse_identifier(text, "mygo.json"), "com.example.app")

    def test_typescript(self):
        text = 'export default defineConfig({ name: "App", identifier: "dev.acme.app" })'
        self.assertEqual(sync.parse_identifier(text, "mygo.config.ts"), "dev.acme.app")

    def test_missing_or_invalid(self):
        self.assertEqual(sync.parse_identifier('{"name": "App"}', "mygo.json"), "")
        self.assertEqual(sync.parse_identifier("not json", "mygo.json"), "")
        self.assertEqual(sync.parse_identifier('{"identifier": "a b"}', "mygo.json"), "")


class AddIdentifiersTest(unittest.TestCase):
    def test_identifier_follows_repo(self):
        apps = [{"slug": "a", "repo": "o/a", "name": "A"}, {"slug": "b", "repo": "o/b", "name": "B"}]
        out = sync.add_identifiers(apps, {"a": "com.a"})
        self.assertEqual(list(out[0]), ["slug", "repo", "identifier", "name"])
        self.assertEqual(out[0]["identifier"], "com.a")
        self.assertNotIn("identifier", out[1])


class SyncIdentifiersTest(unittest.TestCase):
    def test_keeps_previous_and_reads_new_apps(self):
        apps = [{"slug": "a", "repo": "o/a"}, {"slug": "b", "repo": "o/b"}]
        previous = {"apps": [{"slug": "a", "repo": "o/a", "identifier": "com.a"}]}
        with mock.patch.object(sync, "fetch_identifier", return_value="com.b") as fetch:
            identifiers = sync.sync_identifiers(apps, previous, workers=2)
        self.assertEqual(identifiers, {"a": "com.a", "b": "com.b"})
        fetch.assert_called_once_with("o/b")

    def test_reads_one_repository_once(self):
        apps = [{"slug": "a", "repo": "o/x"}, {"slug": "b", "repo": "https://github.com/o/x.git"}]
        with mock.patch.object(sync, "fetch_identifier", return_value="com.x") as fetch:
            identifiers = sync.sync_identifiers(apps, {}, workers=2)
        self.assertEqual(identifiers, {"a": "com.x", "b": "com.x"})
        fetch.assert_called_once_with("o/x")

    def test_refresh_reads_again_and_keeps_old_on_failure(self):
        apps = [{"slug": "a", "repo": "o/a"}]
        previous = {"apps": [{"slug": "a", "repo": "o/a", "identifier": "com.a"}]}
        with mock.patch.object(sync, "fetch_identifier", return_value="") as fetch:
            identifiers = sync.sync_identifiers(apps, previous, refresh=True, workers=1)
        self.assertEqual(identifiers, {"a": "com.a"})
        fetch.assert_called_once_with("o/a")

    def test_a_new_repository_that_fails_has_no_identifier(self):
        apps = [{"slug": "a", "repo": "o/a"}]
        with mock.patch.object(sync, "fetch_identifier", return_value=""):
            identifiers = sync.sync_identifiers(apps, {}, workers=1)
        self.assertEqual(identifiers, {})


class SharedIdentifiersTest(unittest.TestCase):
    def test_reports_shared_identifiers(self):
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr):
            sync.report_shared_identifiers({"a": "dev.x.app", "b": "dev.x.app", "c": "com.c"})
        self.assertIn("dev.x.app: a, b", stderr.getvalue())


class FakeResponse:
    def __init__(self, data: bytes, headers: dict[str, str]):
        self._body = io.BytesIO(data)
        self.headers = headers
        self.status = 200

    def read(self, size: int = -1) -> bytes:
        return self._body.read(size)

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False


class DownloadTest(unittest.TestCase):
    def test_fetches_and_keeps_validators(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = pathlib.Path(tmp) / "a.webp"
            response = FakeResponse(b"image", {"ETag": '"e1"', "Content-Length": "5"})
            with mock.patch.object(sync.urllib.request, "urlopen", return_value=response):
                ok, entry, fetched = sync.download("https://x/a.webp", dest, None)
            self.assertTrue(ok)
            self.assertTrue(fetched)
            self.assertEqual(dest.read_bytes(), b"image")
            self.assertEqual(entry, {"url": "https://x/a.webp", "etag": '"e1"', "size": 5})

    def test_asks_with_the_validator_and_keeps_the_file_on_304(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = pathlib.Path(tmp) / "a.webp"
            dest.write_bytes(b"old")
            entry = {"url": "https://x/a.webp", "etag": '"e1"'}
            error = urllib.error.HTTPError("https://x/a.webp", 304, "Not Modified", {}, None)
            with mock.patch.object(sync.urllib.request, "urlopen", side_effect=error) as open_:
                ok, kept, fetched = sync.download("https://x/a.webp", dest, entry)
            self.assertTrue(ok)
            self.assertFalse(fetched)
            self.assertEqual(kept, entry)
            request = open_.call_args.args[0]
            self.assertEqual(request.get_header("If-none-match"), '"e1"')
            self.assertEqual(dest.read_bytes(), b"old")

    def test_a_new_source_replaces_the_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = pathlib.Path(tmp) / "a.webp"
            dest.write_bytes(b"old")
            entry = {"url": "https://x/old.webp", "etag": '"e1"'}
            response = FakeResponse(b"new", {"ETag": '"e2"'})
            with mock.patch.object(sync.urllib.request, "urlopen", return_value=response):
                ok, updated, fetched = sync.download("https://x/new.webp", dest, entry)
            self.assertTrue(ok)
            self.assertTrue(fetched)
            self.assertEqual(dest.read_bytes(), b"new")
            self.assertEqual(updated["url"], "https://x/new.webp")
            self.assertEqual(updated["etag"], '"e2"')

    def test_identical_content_keeps_the_entry(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = pathlib.Path(tmp) / "a.webp"
            dest.write_bytes(b"same")
            entry = {"url": "https://x/a.webp", "etag": '"old"'}
            response = FakeResponse(b"same", {"ETag": '"new"'})
            with mock.patch.object(sync.urllib.request, "urlopen", return_value=response):
                ok, kept, fetched = sync.download("https://x/a.webp", dest, entry)
        self.assertTrue(ok)
        self.assertFalse(fetched)
        self.assertEqual(kept, entry)

    def test_a_missing_image_is_not_retried(self):
        error = urllib.error.HTTPError("https://x/a.webp", 404, "Not Found", {}, None)
        with tempfile.TemporaryDirectory() as tmp:
            dest = pathlib.Path(tmp) / "a.webp"
            with mock.patch.object(sync.urllib.request, "urlopen", side_effect=error) as open_:
                ok, _, fetched = sync.download("https://x/a.webp", dest, None)
        self.assertFalse(ok)
        self.assertFalse(fetched)
        self.assertEqual(open_.call_count, 1)

    def test_skips_a_file_the_host_gives_no_validator_for(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = pathlib.Path(tmp) / "a.webp"
            dest.write_bytes(b"old")
            entry = {"url": "https://x/a.webp"}
            with mock.patch.object(sync.urllib.request, "urlopen") as open_:
                ok, kept, fetched = sync.download("https://x/a.webp", dest, entry)
            self.assertTrue(ok)
            self.assertFalse(fetched)
            self.assertEqual(kept, entry)
            open_.assert_not_called()


if __name__ == "__main__":
    unittest.main()
