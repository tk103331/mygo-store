package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSnapshots renders the app's views offscreen, as a window would draw
// them, and writes PNGs for visual inspection:
//
//	SNAP_DIR=/tmp/shots go test -run TestSnapshots .
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("SNAP_DIR")
	if dir == "" {
		t.Skip("set SNAP_DIR to write snapshots")
	}
	t.Setenv("MYGO_STORE_CONFIG", t.TempDir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &store{
		catalog:   loadCatalog(),
		prefs:     newPrefs(),
		images:    newImageCache(),
		nav:       "discover",
		sort:      sortRecommended,
		runOnMain: func(fn func()) { fn() },
	}
	tt := ui.NewTester(s.view, 1180, 780)
	tt.SetTitleBar(ui.TitleBar{Height: 52, Left: 78, Right: 14})
	tt.SetDark(true)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	shot := func(name string) {
		t.Helper()
		tt.Frame()
		file, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}

	shot("01-discover")

	// A category page.
	s.selectNav("group:dev")
	shot("02-group-dev")

	// A search.
	s.query = "git"
	tt.Frame()
	shot("03-search-git")

	// An app detail.
	s.query = ""
	s.selectNav("discover")
	s.openDetail("denova")
	shot("04-detail-denova")

	// A download in flight, then the downloaded installer.
	s.releases().set("denova", acquireState{
		Phase:    phaseDownloading,
		Progress: 0.63,
		Asset:    "Denova-1.2.0-arm64.dmg",
		Tag:      "v1.2.0",
	})
	shot("04b-detail-downloading")
	installer := filepath.Join(t.TempDir(), "Denova-1.2.0-arm64.dmg")
	if err := os.WriteFile(installer, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.releases().set("denova", acquireState{
		Phase:  phaseReady,
		Path:   installer,
		Asset:  "Denova-1.2.0-arm64.dmg",
		Tag:    "v1.2.0",
		Source: "https://github.com/alfredxw/denova/releases",
	})
	shot("04c-detail-ready")
	s.releases().reset("denova")

	// A detail page without a screenshot asset.
	if app := s.appBySlug("2048"); app != nil {
		s.openDetail("2048")
		shot("05-detail-2048")
	}

	// Favorites with a few entries.
	s.prefs.Favorites["denova"] = true
	s.prefs.Favorites["kero"] = true
	s.prefs.Acquired["denova"] = true
	s.selectNav("favorites")
	shot("06-favorites")

	// The acquired page with an app that is installed on this computer.
	s.prefs.Acquired = map[string]bool{}
	s.setInstalled([]installedApp{{
		Name:       "Denova",
		Version:    "1.2.0",
		Identifier: "com.example.denova",
		Path:       "/Applications/Denova.app",
	}})
	s.selectNav("acquired")
	shot("07-acquired-installed")
	s.openDetail("denova")
	shot("07b-detail-installed")
	s.setInstalled(nil)

	// The empty acquired state.
	s.prefs.Favorites = map[string]bool{}
	s.selectNav("acquired")
	s.prefs.Acquired = map[string]bool{}
	shot("07-empty")

	// Light mode.
	tt.SetDark(false)
	s.selectNav("discover")
	shot("08-light")

	// The smallest window.
	s.selectNav("discover")
	small := ui.NewTester(s.view, 900, 600)
	small.SetTitleBar(ui.TitleBar{Height: 52, Left: 78, Right: 14})
	small.SetDark(true)
	small.Frame()
	smallShot := func(name string) {
		t.Helper()
		small.Frame()
		file, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, small.Image()); err != nil {
			t.Fatal(err)
		}
	}
	smallShot("09-small-discover")
	s.openDetail("denova")
	smallShot("10-small-detail")

	// The English interface.
	en := ui.NewTester(s.view, 1180, 780)
	en.SetTitleBar(ui.TitleBar{Height: 52, Left: 78, Right: 14})
	en.SetDark(true)
	en.SetLocale("en")
	en.Frame()
	enShot := func(name string) {
		t.Helper()
		en.Frame()
		file, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, en.Image()); err != nil {
			t.Fatal(err)
		}
	}
	s.detail = ""
	s.selectNav("discover")
	enShot("11-discover-en")
	s.selectNav("group:dev")
	enShot("12-group-dev-en")
	s.openDetail("denova")
	enShot("13-detail-denova-en")

	// The language menu in the window, for the platforms whose title bar
	// leaves no menu bar, as Windows does.
	s.detail = ""
	s.selectNav("discover")
	menu := ui.NewTester(s.view, 1180, 780)
	menu.SetTitleBar(ui.TitleBar{Height: 52, Left: 78, Right: 14})
	menu.SetDark(true)
	menu.SetLocale("zh-Hans")
	menu.Frame()
	if err := menu.Click("语言"); err != nil {
		t.Fatal(err)
	}
	menu.Frame()
	file, err := os.Create(filepath.Join(dir, "14-language-menu.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, menu.Image()); err != nil {
		t.Fatal(err)
	}
}
