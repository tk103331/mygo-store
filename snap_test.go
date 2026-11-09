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
		Message:  "正在下载安装包…",
	})
	shot("04b-detail-downloading")
	installer := filepath.Join(t.TempDir(), "Denova-1.2.0-arm64.dmg")
	if err := os.WriteFile(installer, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.releases().set("denova", acquireState{
		Phase:   phaseReady,
		Path:    installer,
		Asset:   "Denova-1.2.0-arm64.dmg",
		Tag:     "v1.2.0",
		Source:  "https://github.com/alfredxw/denova/releases",
		Message: "安装包已下载",
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

	// The empty favorites state.
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
}
