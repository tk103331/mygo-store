package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchInstalled(t *testing.T) {
	catalog := &Catalog{Apps: []App{
		{Slug: "denova", Name: "Denova", Identifier: "com.example.denova"},
		{Slug: "mygo-translate", Name: "MyGo 翻译"},
		{Slug: "2048", Name: "2048", Identifier: "com.example.2048"},
	}}
	apps := matchInstalled(catalog, []installedApp{
		// The identifier knows the app although the system names it otherwise.
		{Name: "某个名字", Identifier: "com.example.denova", Path: `C:\Programs\Other\Other.exe`},
		// The name matches a catalog app whose app has no identifier on disk.
		{Name: "MyGo 翻译", Path: "/Applications/MyGo 翻译.app"},
		// The file the app runs from matches, as 2048.exe does.
		{Name: "Some App", Path: `C:\Users\me\AppData\Local\Programs\2048\2048.exe`},
		// An app the catalog does not know is not an acquired app.
		{Name: "Not in the catalog", Path: "/Applications/Other.app"},
		// A second copy of the same app appears once.
		{Name: "Denova", Identifier: "com.example.denova", Path: `D:\Denova\Denova.exe`},
	})
	if len(apps) != 3 {
		t.Fatalf("matched %d apps, want 3: %+v", len(apps), apps)
	}
	got := make(map[string]bool, len(apps))
	for i := range apps {
		got[apps[i].Slug] = true
	}
	for _, want := range []string{"2048", "denova", "mygo-translate"} {
		if !got[want] {
			t.Errorf("the match lacks %q: %+v", want, apps)
		}
	}
}

func TestDefaultIdentifierMatchesCatalog(t *testing.T) {
	// A catalog synced with identifiers matches an installed app exactly.
	catalog := &Catalog{Apps: []App{
		{Slug: "some-app", Name: "Some App", Identifier: "com.mygo.someapp"},
	}}
	apps := matchInstalled(catalog, []installedApp{
		{Name: "Something Else", Identifier: "com.mygo.someapp"},
	})
	if len(apps) != 1 || apps[0].Slug != "some-app" {
		t.Fatalf("matched %+v, want some-app by identifier", apps)
	}
}

func TestInstalledAppsJoinAcquired(t *testing.T) {
	s, tt := newTestStore(t)
	s.setInstalled([]installedApp{{
		Name:       "Denova",
		Version:    "1.2.0",
		Identifier: "com.example.denova",
		Path:       "/Applications/Denova.app",
	}})

	if got := s.acquiredCount(); got != 1 {
		t.Fatalf("acquired count = %d, want 1", got)
	}
	if !s.isAcquired("denova") {
		t.Fatal("the installed app is not an acquired app")
	}

	s.selectNav("acquired")
	tt.Frame()
	if !tt.HasText("Denova") || !tt.HasText("已安装") {
		t.Fatalf("the acquired page does not show the installed app: %q", tt.Texts())
	}

	if err := tt.Click("Denova"); err != nil {
		t.Fatalf("open the installed app's detail: %v", err)
	}
	tt.Frame()
	if !tt.HasText("已安装 1.2.0") {
		t.Errorf("the detail page does not show the installed version: %q", tt.Texts())
	}

	var launched []installedApp
	previous := launchInstalledApp
	launchInstalledApp = func(app installedApp) { launched = append(launched, app) }
	t.Cleanup(func() { launchInstalledApp = previous })

	if err := tt.Click("打开"); err != nil {
		t.Fatalf("click open: %v", err)
	}
	if len(launched) != 1 || launched[0].Slug != "denova" {
		t.Fatalf("launched %+v, want denova", launched)
	}
	if s.prefs.Acquired["denova"] {
		t.Error("detection should not write the app to the preferences")
	}
}

func TestDetectButton(t *testing.T) {
	s, tt := newTestStore(t)
	previous := installedScanner
	installedScanner = func(func(string) bool) ([]installedApp, error) {
		return []installedApp{{
			Name:    "Denova",
			Version: "1.0",
			Path:    "/Applications/Denova.app",
		}}, nil
	}
	t.Cleanup(func() { installedScanner = previous })

	s.selectNav("acquired")
	tt.Frame()
	if err := tt.Click("检测已安装应用"); err != nil {
		t.Fatalf("click detect installed apps: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Denova") {
		t.Fatalf("the scan did not show the installed app: %q", tt.Texts())
	}
}

func TestRefreshInstalled(t *testing.T) {
	s, _ := newTestStore(t)
	previous := installedScanner
	installedScanner = func(known func(string) bool) ([]installedApp, error) {
		if !known("Denova") {
			t.Error("the scanner does not know the catalog's apps")
		}
		return []installedApp{{
			Name:    "Denova",
			Version: "2.0",
			Path:    "/Applications/Denova.app",
		}}, nil
	}
	t.Cleanup(func() { installedScanner = previous })

	s.refreshInstalled()
	if s.installedBusy {
		t.Error("the scan is still marked running")
	}
	app, ok := s.installedApp("denova")
	if !ok {
		t.Fatal("the scan did not find Denova")
	}
	if app.Version != "2.0" {
		t.Errorf("version = %q, want 2.0", app.Version)
	}
}

func TestFileStem(t *testing.T) {
	cases := map[string]string{
		"/Applications/Denova.app":               "Denova",
		`C:\Program Files\Denova\Denova.exe`:     "Denova",
		"/usr/share/applications/denova.desktop": "denova",
		"/opt/Denova-1.2.AppImage":               "Denova-1.2",
	}
	for path, want := range cases {
		if got := fileStem(path); got != want {
			t.Errorf("fileStem(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestInstalledMarkerInThisBinary(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("no executable of the test")
	}
	// The test binary links the app's code, so it holds the marker that the
	// macOS and Linux scans look for.
	if !isMyGoExecutable(exe) {
		t.Errorf("%s does not look like a MyGo executable", filepath.Base(exe))
	}
}
