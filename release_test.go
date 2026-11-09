package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func asset(name string) releaseAsset {
	return releaseAsset{Name: name, BrowserDownloadURL: "https://example.com/" + name}
}

func TestPickAssetMacArm64(t *testing.T) {
	assets := []releaseAsset{
		asset("Denova-1.2.0-x64.dmg"),
		asset("Denova-1.2.0-arm64.dmg"),
		asset("Denova-1.2.0-universal.dmg"),
	}
	got, ok := pickAsset(assets, "darwin", "arm64")
	if !ok || got.Name != "Denova-1.2.0-arm64.dmg" {
		t.Fatalf("picked %q (%v), want the arm64 dmg", got.Name, ok)
	}
}

func TestPickAssetMacIntel(t *testing.T) {
	assets := []releaseAsset{
		asset("Denova-1.2.0-arm64.dmg"),
		asset("Denova-1.2.0-x64.dmg"),
	}
	got, ok := pickAsset(assets, "darwin", "amd64")
	if !ok || got.Name != "Denova-1.2.0-x64.dmg" {
		t.Fatalf("picked %q (%v), want the x64 dmg", got.Name, ok)
	}
}

func TestPickAssetRejectsOtherArchitecture(t *testing.T) {
	for _, tc := range []struct {
		goos, goarch string
		assets       []releaseAsset
	}{
		{"darwin", "amd64", []releaseAsset{asset("App-arm64.dmg")}},
		{"darwin", "arm64", []releaseAsset{asset("App-x86_64.dmg")}},
		{"windows", "arm64", []releaseAsset{asset("App-win-x64.exe")}},
		{"linux", "amd64", []releaseAsset{asset("app_arm64.AppImage")}},
	} {
		if got, ok := pickAsset(tc.assets, tc.goos, tc.goarch); ok {
			t.Errorf("%s/%s picked %q, want no match", tc.goos, tc.goarch, got.Name)
		}
	}
}

func TestPickAssetPrefersInstallers(t *testing.T) {
	tests := []struct {
		goos, goarch string
		assets       []releaseAsset
		want         string
	}{
		{
			"windows", "amd64",
			[]releaseAsset{asset("App-win-x64.zip"), asset("App-win-x64.exe"), asset("App-win-arm64.exe")},
			"App-win-x64.exe",
		},
		{
			"windows", "amd64",
			[]releaseAsset{asset("App-x64.exe"), asset("App-x64.msi")},
			"App-x64.exe",
		},
		{
			"linux", "amd64",
			[]releaseAsset{asset("app_amd64.deb"), asset("app_amd64.AppImage")},
			"app_amd64.AppImage",
		},
		{
			"linux", "amd64",
			[]releaseAsset{asset("app_amd64.deb"), asset("app_x64.rpm")},
			"app_amd64.deb",
		},
		{
			"darwin", "arm64",
			[]releaseAsset{asset("App-mac-arm64.zip"), asset("App-mac-arm64.pkg"), asset("App-mac-universal.dmg")},
			"App-mac-universal.dmg",
		},
	}
	for _, tc := range tests {
		got, ok := pickAsset(tc.assets, tc.goos, tc.goarch)
		if !ok || got.Name != tc.want {
			t.Errorf("%s/%s picked %q (%v), want %q", tc.goos, tc.goarch, got.Name, ok, tc.want)
		}
	}
}

func TestPickAssetIgnoresAuxiliaryFiles(t *testing.T) {
	assets := []releaseAsset{
		asset("App-1.0.dmg.sha256"),
		asset("App-1.0.dmg.sha512"),
		asset("checksums.txt"),
		asset("App-1.0-symbols.zip"),
		asset("App-1.0-debug.zip"),
		asset("App-1.0.dmg"),
	}
	got, ok := pickAsset(assets, "darwin", "arm64")
	if !ok || got.Name != "App-1.0.dmg" {
		t.Fatalf("picked %q (%v), want App-1.0.dmg", got.Name, ok)
	}
}

func TestPickReleaseSkipsPrereleasesAndFallsBack(t *testing.T) {
	releases := []release{
		{TagName: "v3.0", Assets: []releaseAsset{asset("notes.txt")}},
		{TagName: "v2.9", Prerelease: true, Assets: []releaseAsset{asset("App-mac-arm64.dmg")}},
		{TagName: "v2.8", Draft: true, Assets: []releaseAsset{asset("App-mac-arm64.dmg")}},
		{TagName: "v2.7", Assets: []releaseAsset{asset("App-mac-arm64.zip")}},
	}
	rel, got, ok := pickRelease(releases, "darwin", "arm64")
	if !ok || rel.TagName != "v2.7" || got.Name != "App-mac-arm64.zip" {
		t.Fatalf("picked %q from %q (%v), want the v2.7 zip", got.Name, rel.TagName, ok)
	}
}

func TestRepoPath(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"egoist/mygo", "egoist/mygo", true},
		{"https://github.com/egoist/mygo", "egoist/mygo", true},
		{"https://github.com/egoist/mygo.git", "egoist/mygo", true},
		{"github.com/egoist/mygo/releases", "egoist/mygo", true},
		{"egoist", "", false},
		{"", "", false},
	} {
		got, ok := repoPath(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("repoPath(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestDownloadSavesInstaller(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MYGO_STORE_DOWNLOADS", dir)

	payload := []byte("installer bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("the download has no User-Agent")
		}
		w.Header().Set("Content-Length", "15")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	m := newReleaseManager(nil)
	base := acquireState{Phase: phaseDownloading, Progress: -1, Asset: "Demo 1.0.dmg"}
	path, err := m.download("demo", base, releaseAsset{
		Name:               "Demo 1.0.dmg",
		BrowserDownloadURL: srv.URL + "/Demo.dmg",
		Size:               int64(len(payload)),
	})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if filepath.Dir(path) != dir || filepath.Base(path) != "Demo 1.0.dmg" {
		t.Errorf("path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Errorf("downloaded %q, want %q", data, payload)
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Errorf(".part file remains: %v", err)
	}
}

func TestDownloadDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MYGO_STORE_DOWNLOADS", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("second"))
	}))
	defer srv.Close()

	if err := os.WriteFile(filepath.Join(dir, "Demo.dmg"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newReleaseManager(nil)
	path, err := m.download("demo", acquireState{}, releaseAsset{
		Name:               "Demo.dmg",
		BrowserDownloadURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "Demo (1).dmg" {
		t.Errorf("path = %q, want Demo (1).dmg", path)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "Demo.dmg")); string(data) != "first" {
		t.Errorf("the existing file changed to %q", data)
	}
}

func TestDownloadRemovesPartOnFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MYGO_STORE_DOWNLOADS", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := newReleaseManager(nil)
	if _, err := m.download("demo", acquireState{}, releaseAsset{
		Name:               "Demo.dmg",
		BrowserDownloadURL: srv.URL,
	}); err == nil {
		t.Fatal("download succeeded, want an error")
	}
	if _, err := os.Stat(filepath.Join(dir, "Demo.dmg.part")); !os.IsNotExist(err) {
		t.Errorf(".part file remains: %v", err)
	}
}

func TestSafeFileName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"../../evil.dmg", "evil.dmg"},
		{`C:\Users\me\evil.exe`, "evil.exe"},
		{"App: 1.0?.dmg", "App- 1.0-.dmg"},
		{"..", ""},
	} {
		if got := safeFileName(tc.in); got != tc.want {
			t.Errorf("safeFileName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSeedDownloads(t *testing.T) {
	t.Setenv("MYGO_STORE_CONFIG", t.TempDir())
	dir := t.TempDir()
	t.Setenv("MYGO_STORE_DOWNLOADS", dir)

	kept := filepath.Join(dir, "Kept.dmg")
	if err := os.WriteFile(kept, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &store{prefs: newPrefs()}
	s.prefs.Downloads["kept"] = kept
	s.prefs.Downloads["gone"] = filepath.Join(dir, "gone.dmg")
	s.seedDownloads()

	state := s.acquireState("kept")
	if state.Phase != phaseReady || state.Path != kept {
		t.Errorf("kept state = %+v", state)
	}
	if _, ok := s.prefs.Downloads["gone"]; ok {
		t.Error("a missing installer was not pruned")
	}
	if _, ok := s.prefs.Downloads["kept"]; !ok {
		t.Error("an existing installer was pruned")
	}
}

func waitAcquire(t *testing.T, s *store, slug string) acquireState {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		state := s.acquireState(slug)
		switch state.Phase {
		case phaseReady, phaseFailed, phaseNoAsset:
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("acquisition of %s did not finish: %+v", slug, s.acquireState(slug))
	return acquireState{}
}

func TestStartAcquireDownloadsMatchingAsset(t *testing.T) {
	t.Setenv("MYGO_STORE_CONFIG", t.TempDir())
	t.Setenv("MYGO_STORE_DOWNLOADS", t.TempDir())

	assetName := "App-" + runtime.GOOS + "-" + runtime.GOARCH + ".dmg"
	switch runtime.GOOS {
	case "windows":
		assetName = "App-" + runtime.GOARCH + ".exe"
	case "linux":
		assetName = "App-" + runtime.GOARCH + ".AppImage"
	}

	payload := []byte("installer")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/app/releases":
			_ = json.NewEncoder(w).Encode([]release{{
				TagName: "v1.0.0",
				HTMLURL: srv.URL + "/releases/v1.0.0",
				Assets: []releaseAsset{
					{Name: assetName, BrowserDownloadURL: srv.URL + "/App.bin", Size: int64(len(payload))},
					{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
				},
			}})
		case "/App.bin":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	previousBase := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = previousBase })

	app := &App{Slug: "demo", Name: "Demo", Repo: "owner/app", URL: "https://example.com/demo"}
	s := &store{
		catalog:   &Catalog{Apps: []App{*app}},
		prefs:     newPrefs(),
		runOnMain: func(fn func()) { fn() },
	}
	s.startAcquire(app)

	state := waitAcquire(t, s, "demo")
	if state.Phase != phaseReady {
		t.Fatalf("state = %+v, want ready", state)
	}
	if filepath.Base(state.Path) != assetName {
		t.Errorf("path = %q, want %q", state.Path, assetName)
	}
	if data, err := os.ReadFile(state.Path); err != nil || string(data) != string(payload) {
		t.Errorf("download = %q, %v; want %q", data, err, payload)
	}
	if !s.prefs.Acquired["demo"] || s.prefs.Downloads["demo"] != state.Path {
		t.Errorf("prefs = %+v", s.prefs)
	}
}

func TestStartAcquireFallsBackWithoutAsset(t *testing.T) {
	t.Setenv("MYGO_STORE_CONFIG", t.TempDir())
	t.Setenv("MYGO_STORE_DOWNLOADS", t.TempDir())

	opposite := map[string]string{
		"darwin":  "App-linux-arm64.AppImage",
		"windows": "App-linux-arm64.AppImage",
		"linux":   "App-macos-arm64.dmg",
	}[runtime.GOOS]

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/app/releases" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]release{{
			TagName: "v1.0.0",
			Assets: []releaseAsset{
				{Name: opposite, BrowserDownloadURL: srv.URL + "/App.bin"},
			},
		}})
	}))
	defer srv.Close()

	previousBase := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = previousBase })

	opened := make(chan string, 1)
	previousOpen := openExternal
	openExternal = func(url string) { opened <- url }
	t.Cleanup(func() { openExternal = previousOpen })

	app := &App{Slug: "demo", Name: "Demo", Repo: "owner/app", URL: "https://example.com/demo"}
	s := &store{
		catalog:   &Catalog{Apps: []App{*app}},
		prefs:     newPrefs(),
		runOnMain: func(fn func()) { fn() },
	}
	s.startAcquire(app)

	state := waitAcquire(t, s, "demo")
	if state.Phase != phaseNoAsset {
		t.Fatalf("state = %+v, want no asset", state)
	}
	select {
	case url := <-opened:
		if url != app.URL {
			t.Errorf("opened %q, want %q", url, app.URL)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the official page was not opened")
	}
	if !s.prefs.Acquired["demo"] {
		t.Error("the app was not marked acquired")
	}
}

func TestFetchReleasesReportNoReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	previous := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = previous })

	if _, err := fetchReleases("owner/repo"); err == nil || !strings.Contains(err.Error(), "GitHub releases") {
		t.Errorf("fetchReleases error = %v, want errNoReleases", err)
	}
}
