#!/usr/bin/env python3
"""Sync the MyGo Store catalog from awesomemygo.dev.

The site is built from yldm-tech/awesome-mygo: one JSON file per app in
packages/catalog/data/apps, plus packages/catalog/data/scores.json. This
script downloads that data, computes the same 0-100 score the site shows,
reads each app's bundle identifier from its repository, downloads each
app's icon and screenshot, and writes data/apps.json.

Usage:
    python3 scripts/sync.py                 # sync everything
    python3 scripts/sync.py --no-assets     # data only
    python3 scripts/sync.py --no-identifiers  # skip the identifiers too
    python3 scripts/sync.py --force-assets  # download images again
    python3 scripts/sync.py --catalog DIR   # use an unpacked catalog/data

Requires only Python 3.11+ (standard library).
"""

from __future__ import annotations

import argparse
import calendar
import concurrent.futures
import io
import json
import math
import os
import re
import shutil
import sys
import tarfile
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DATA_FILE = ROOT / "data" / "apps.json"
ICON_DIR = ROOT / "assets" / "icons"
SHOT_DIR = ROOT / "assets" / "shots"

TARBALL = "https://codeload.github.com/yldm-tech/awesome-mygo/tar.gz/refs/heads/main"
DEFAULT_ICON = "https://awesomemygo.dev/icons/{file}"
AILOGO_ICON = "https://ailogo.yldm.ai/avatar/{id}.webp"
SHOT = "https://awesomemygo.dev/shots/{file}"

UA = "MyGoStore/1.0 (+https://awesomemygo.dev/showcase)"

# The files of an app's repository that name its bundle identifier, which
# tells an installed app apart from the others, whatever its name.
IDENTIFIER_FILES = ("mygo.json", "mygo.config.ts", "mygo.config.js", "mygo.config.mjs")
IDENTIFIER_TS_RE = re.compile(r"""\bidentifier\s*:\s*["']([^"']+)["']""")
IDENTIFIER_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")

GROUPS = ["agents", "dev", "ai", "network", "notes", "system", "devices", "media", "game", "chat"]
PLATFORMS = ["win", "linux", "mac"]

# ---------------------------------------------------------------------------
# Score: a Python port of packages/catalog/src/score.ts, so the app shows the
# same number as the site.
# ---------------------------------------------------------------------------

NEUTRAL = 0.5
STAR_ANCHOR = 1000
LIKE_ANCHOR = 50
ACTIVITY_WEEKS = 12
ACTIVITY_FULL = 8
ACTIVITY_MIN = 4
RELEASE_ANCHOR = 12
WEIGHTS = {"stars": 30, "activity": 25, "openness": 20, "likes": 15, "release": 10}


def _clamp01(n: float) -> float:
    return min(1.0, max(0.0, n))


def _log_share(n: float, anchor: float) -> float:
    return _clamp01(math.log1p(max(0.0, n)) / math.log1p(anchor))


def _round1(n: float) -> float:
    # Java/JS Math.round for non-negative values (Python's round is banker's).
    return math.floor(n * 10 + 0.5) / 10


def _activity_share(weeks: int, active: int) -> float:
    target = max(ACTIVITY_MIN, min(ACTIVITY_FULL, weeks))
    missing = max(0, ACTIVITY_MIN - weeks)
    return _clamp01((active + NEUTRAL * missing) / target)


def _openness(signals: dict) -> float:
    if not signals.get("repo"):
        return 0.0
    lic = signals.get("license")
    if lic is None:
        return 0.3
    return 1.0 if signals.get("osi") else 0.6


def _release_days(signals: dict, as_of: str) -> int:
    if "releaseDays" in signals:
        return int(signals["releaseDays"])
    released = signals.get("released")
    if not released:
        return 0
    try:
        days = (
            time.mktime(time.strptime(as_of, "%Y-%m-%d"))
            - time.mktime(time.strptime(released, "%Y-%m-%d"))
        ) / 86400
    except ValueError:
        return 0
    return 1 if 1 <= days <= ACTIVITY_WEEKS * 7 else 0


def score_of(signals: dict, as_of: str) -> float:
    """The score the site shows for an entry, 0-100, rounded."""
    open_source = bool(signals.get("repo"))
    shares: dict[str, float | None] = {
        "stars": (
            _log_share(signals["stars"], STAR_ANCHOR)
            if open_source and not signals.get("path") and "stars" in signals
            else None
        ),
        "activity": (
            _activity_share(signals["weeks"], signals["activeWeeks"])
            if open_source
            and signals.get("weeks", 0) > 0
            and "activeWeeks" in signals
            else None
        ),
        "openness": _openness(signals),
        "likes": _log_share(signals.get("likes", 0), LIKE_ANCHOR),
        "release": (
            _log_share(_release_days(signals, as_of), RELEASE_ANCHOR) if open_source else None
        ),
    }
    tenths = 0
    for signal, weight in WEIGHTS.items():
        share = shares[signal]
        points = _round1(weight * (NEUTRAL if share is None else share))
        tenths += math.floor(points * 10 + 0.5)
    exact = tenths / 10
    if signals.get("archived") is True:
        exact *= 0.5
    # The site shows the exact score to one decimal, not just the rounded total.
    return _round1(exact)


# ---------------------------------------------------------------------------
# Catalog
# ---------------------------------------------------------------------------

def read_local_catalog(path: Path) -> tuple[list[dict], dict]:
    apps = []
    for file in sorted((path / "apps").glob("*.json")):
        apps.append(json.loads(file.read_text()))
    scores = json.loads((path / "scores.json").read_text())
    return apps, scores


def read_remote_catalog() -> tuple[list[dict], dict]:
    print(f"↓ {TARBALL}", flush=True)
    request = urllib.request.Request(TARBALL, headers={"User-Agent": UA})
    with urllib.request.urlopen(request, timeout=180) as response:
        archive = response.read()
    print(f"  {len(archive) / 1e6:.1f} MB", flush=True)
    apps: list[dict] = []
    scores: dict = {}
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as tar:
        for member in tar.getmembers():
            marker = "packages/catalog/data/apps/"
            if member.isfile() and marker in member.name and member.name.endswith(".json"):
                handle = tar.extractfile(member)
                if handle:
                    apps.append(json.loads(handle.read()))
            elif member.isfile() and member.name.endswith("packages/catalog/data/scores.json"):
                handle = tar.extractfile(member)
                if handle:
                    scores = json.loads(handle.read())
    apps.sort(key=lambda app: app["slug"])
    return apps, scores


def build_catalog(apps: list[dict], scores: dict) -> dict:
    as_of = scores.get("asOf", "")
    entries = scores.get("entries", {})
    out = []
    for app in apps:
        signals = dict(entries.get(f"app:{app['slug']}", {}))
        signals.setdefault("repo", app.get("repo"))
        platforms = app.get("platforms") or PLATFORMS
        summary = app.get("summary", {})
        description = app.get("description", {})
        out.append(
            {
                "slug": app["slug"],
                "name": app["name"],
                "author": app["author"],
                "kind": app["kind"],
                "platforms": platforms,
                "group": app.get("group", "other"),
                "category": app.get("category"),
                "url": app["url"],
                "repo": app.get("repo"),
                "summary": summary.get("zh-CN") or summary.get("en", ""),
                "description": description.get("zh-CN") or description.get("en", ""),
                "added": app.get("added"),
                "score": score_of(signals, as_of),
                "likes": signals.get("likes", 0),
                "stars": signals.get("stars"),
                "license": signals.get("license"),
                "icon": None,
                "shot": None,
            }
        )
    return {
        "source": "https://awesomemygo.dev/showcase",
        "asOf": as_of,
        "apps": out,
    }


# ---------------------------------------------------------------------------
# Assets
# ---------------------------------------------------------------------------

def icon_url(app: dict) -> str | None:
    icon = app.get("icon")
    if not icon:
        return None
    if "ailogo" in icon:
        return AILOGO_ICON.format(id=icon["ailogo"])
    if "file" in icon:
        return DEFAULT_ICON.format(file=icon["file"])
    return None


def shot_url(app: dict) -> str | None:
    shot = app.get("screenshot")
    return SHOT.format(file=shot) if shot else None


def download(url: str, dest: Path, force: bool = False, retries: int = 3) -> bool:
    if dest.exists() and dest.stat().st_size > 0 and not force:
        return True
    dest.parent.mkdir(parents=True, exist_ok=True)
    request = urllib.request.Request(url, headers={"User-Agent": UA})
    for attempt in range(retries):
        try:
            with urllib.request.urlopen(request, timeout=45) as response:
                data = response.read()
            if not data:
                raise ValueError("empty response")
            temp = dest.with_suffix(dest.suffix + ".part")
            temp.write_bytes(data)
            temp.replace(dest)
            return True
        except Exception as error:  # timeouts, TLS resets, partial bodies
            if attempt + 1 == retries:
                print(f"  ! {url}: {error}", file=sys.stderr)
                return False
            time.sleep(1.5 * (attempt + 1))
    return False


def repo_slug(repo: str) -> str:
    """The owner/name of a repository, as "owner/name" URLs give it."""
    repo = (repo or "").strip()
    for prefix in ("https://github.com/", "http://github.com/", "github.com/"):
        if repo.startswith(prefix):
            repo = repo[len(prefix) :]
            break
    repo = repo.removesuffix(".git").strip("/")
    parts = repo.split("/")
    if len(parts) < 2 or not parts[0] or not parts[1]:
        return ""
    return f"{parts[0]}/{parts[1]}"


def parse_identifier(text: str, filename: str) -> str:
    """The identifier a mygo.json or mygo.config.ts names, or ""."""
    identifier = ""
    if filename.endswith(".json"):
        try:
            data = json.loads(text)
        except ValueError:
            return ""
        if isinstance(data, dict):
            identifier = data.get("identifier", "")
    else:
        found = IDENTIFIER_TS_RE.search(text)
        if found:
            identifier = found.group(1)
    identifier = identifier.strip() if isinstance(identifier, str) else ""
    return identifier if IDENTIFIER_RE.match(identifier) else ""


def fetch_identifier(repo: str) -> str:
    """The identifier of an app, from the configuration of its repository."""
    repo = repo_slug(repo)
    if not repo:
        return ""
    for name in IDENTIFIER_FILES:
        url = f"https://raw.githubusercontent.com/{repo}/HEAD/{name}"
        request = urllib.request.Request(url, headers={"User-Agent": UA})
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                text = response.read(1 << 20).decode("utf-8", "replace")
        except Exception:
            continue
        if identifier := parse_identifier(text, name):
            return identifier
    return ""


def sync_identifiers(apps: list[dict], previous: dict) -> dict[str, str]:
    """Fetch the bundle identifiers of the apps, keeping the ones a failed
    fetch loses from the previous catalog."""
    known = {
        app["slug"]: app["identifier"]
        for app in previous.get("apps", [])
        if isinstance(app, dict) and app.get("identifier")
    }
    jobs = [(app["slug"], app.get("repo", "")) for app in apps if app.get("repo")]
    print(f"↓ identifiers of {len(jobs)} apps", flush=True)
    found: dict[str, str] = {}
    done = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=16) as pool:
        futures = {pool.submit(fetch_identifier, repo): (slug, repo) for slug, repo in jobs}
        for future in concurrent.futures.as_completed(futures):
            slug, _ = futures[future]
            if identifier := future.result() or known.get(slug, ""):
                found[slug] = identifier
            done += 1
            if done % 50 == 0 or done == len(jobs):
                print(f"  {done}/{len(jobs)}", flush=True)
    print(f"  {len(found)} identifiers", flush=True)
    return found


def add_identifiers(apps: list[dict], identifiers: dict[str, str]) -> list[dict]:
    """Add the identifier of each app after its repository, where it sits in
    the catalog a reader browses."""
    out = []
    for app in apps:
        item = {}
        for key, value in app.items():
            item[key] = value
            if key == "repo" and app["slug"] in identifiers:
                item["identifier"] = identifiers[app["slug"]]
        out.append(item)
    return out


def report_shared_identifiers(identifiers: dict[str, str]) -> None:
    """Tell which identifiers two apps share, as a fork keeps the one of the
    app it was copied from; the app matches those by name instead."""
    by_identifier: dict[str, list[str]] = {}
    for slug, identifier in identifiers.items():
        by_identifier.setdefault(identifier.lower(), []).append(slug)
    shared = {identifier: slugs for identifier, slugs in by_identifier.items() if len(slugs) > 1}
    if not shared:
        return
    print(f"  ! {len(shared)} identifiers are shared; their apps match by name:", file=sys.stderr)
    for identifier, slugs in sorted(shared.items()):
        print(f"    {identifier}: {', '.join(sorted(slugs))}", file=sys.stderr)


def sync_assets(apps: list[dict], force: bool) -> None:
    ICON_DIR.mkdir(parents=True, exist_ok=True)
    SHOT_DIR.mkdir(parents=True, exist_ok=True)
    jobs = []
    for app in apps:
        icon = icon_url(app)
        shot = shot_url(app)
        if icon:
            jobs.append((icon, ICON_DIR / f"{app['slug']}.webp"))
        if shot:
            jobs.append((shot, SHOT_DIR / f"{app['slug']}.webp"))
    total = len(jobs)
    done = 0
    failed = 0
    print(f"↓ {total} images", flush=True)
    with concurrent.futures.ThreadPoolExecutor(max_workers=16) as pool:
        futures = {pool.submit(download, url, dest, force): (url, dest) for url, dest in jobs}
        for future in concurrent.futures.as_completed(futures):
            done += 1
            if not future.result():
                failed += 1
            if done % 50 == 0 or done == total:
                print(f"  {done}/{total}", flush=True)
    print(f"  {total - failed} downloaded, {failed} failed", flush=True)


def next_synced_at(previous: str) -> str:
    """A timestamp after the previous one, so a same-second change still wins."""
    try:
        before = calendar.timegm(time.strptime(previous, "%Y-%m-%dT%H:%M:%SZ"))
    except (TypeError, ValueError):
        before = 0
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(max(time.time(), before + 1)))


def read_previous() -> dict:
    try:
        data = json.loads(DATA_FILE.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}
    return data if isinstance(data, dict) else {}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--catalog", type=Path, help="unpacked packages/catalog/data directory")
    parser.add_argument("--no-assets", action="store_true", help="skip icons and screenshots")
    parser.add_argument("--no-identifiers", action="store_true", help="skip reading the apps' bundle identifiers")
    parser.add_argument("--force-assets", action="store_true", help="download images even if present")
    parser.add_argument("--keep-missing", action="store_true", help="keep old images for apps that left")
    args = parser.parse_args()

    if args.catalog:
        apps, scores = read_local_catalog(args.catalog)
    else:
        apps, scores = read_remote_catalog()

    previous = read_previous()
    catalog = build_catalog(apps, scores)
    if not args.no_identifiers:
        identifiers = sync_identifiers(catalog["apps"], previous)
        report_shared_identifiers(identifiers)
        catalog["apps"] = add_identifiers(catalog["apps"], identifiers)
    print(f"✓ {len(catalog['apps'])} apps · scores as of {catalog['asOf']}")

    if not args.no_assets:
        sync_assets(apps, args.force_assets)

    # Only point at files that exist.
    for app in catalog["apps"]:
        if (ICON_DIR / f"{app['slug']}.webp").exists():
            app["icon"] = f"assets/icons/{app['slug']}.webp"
        if (SHOT_DIR / f"{app['slug']}.webp").exists():
            app["shot"] = f"assets/shots/{app['slug']}.webp"

    # When the upstream data did not change, keep the previous syncedAt so
    # the daily action does not commit or publish a new catalog every day.
    if (
        previous.get("syncedAt")
        and previous.get("source") == catalog["source"]
        and previous.get("asOf") == catalog["asOf"]
        and previous.get("apps") == catalog["apps"]
    ):
        catalog["syncedAt"] = previous["syncedAt"]
        print(f"✓ catalog unchanged since {catalog['syncedAt']}")
    else:
        catalog["syncedAt"] = next_synced_at(previous.get("syncedAt", ""))
        print(f"✓ catalog synced at {catalog['syncedAt']}")

    catalog = {
        "source": catalog["source"],
        "asOf": catalog["asOf"],
        "syncedAt": catalog["syncedAt"],
        "apps": catalog["apps"],
    }

    DATA_FILE.parent.mkdir(parents=True, exist_ok=True)
    DATA_FILE.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n")
    print(f"✓ wrote {DATA_FILE.relative_to(ROOT)} ({DATA_FILE.stat().st_size / 1024:.0f} KB)")

    if not args.keep_missing:
        slugs = {app["slug"] for app in apps}
        for directory in (ICON_DIR, SHOT_DIR):
            for file in directory.glob("*.webp"):
                if file.stem not in slugs:
                    file.unlink()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
