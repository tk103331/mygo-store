package main

import (
	"os"

	"github.com/egoist/mygo"
)

// releases returns the release manager, creating it on first use so the
// tests and the window share the same entry point.
func (s *store) releases() *releaseManager {
	if s.releaseMgr == nil {
		s.releaseMgr = newReleaseManager(s.releaseChanged)
	}
	return s.releaseMgr
}

// seedDownloads remembers installers from earlier sessions and forgets the
// ones the user has deleted.
func (s *store) seedDownloads() {
	m := s.releases()
	changed := false
	for slug, path := range s.prefs.Downloads {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			m.seed(slug, path)
			continue
		}
		delete(s.prefs.Downloads, slug)
		changed = true
	}
	if changed {
		savePrefs(s.prefs)
	}
}

// acquire is the 获取 action. It opens an installer already downloaded,
// retries a failed lookup, or asks GitHub Releases for one that matches
// this system; without a matching release it opens the app's page.
func (s *store) acquire(app *App) {
	m := s.releases()
	if path, ok := m.readyPath(app.Slug); ok {
		s.markAcquired(app)
		openPath(path)
		return
	}

	state := m.state(app.Slug)
	switch state.Phase {
	case phaseSearching, phaseDownloading:
		return
	case phaseNoAsset:
		s.markAcquired(app)
		openExternal(app.URL)
		return
	case phaseReady:
		// The file disappeared since the last frame; look it up again.
		m.reset(app.Slug)
	}

	if app.Repo == "" {
		s.markAcquired(app)
		openExternal(app.URL)
		return
	}
	s.startAcquire(app)
}

// acquireState reads an app's installer state for the view.
func (s *store) acquireState(slug string) acquireState {
	if s.releaseMgr == nil {
		return acquireState{}
	}
	return s.releaseMgr.state(slug)
}

// markAcquired remembers that the user asked for this app.
func (s *store) markAcquired(app *App) {
	if s.prefs.Acquired[app.Slug] {
		return
	}
	s.prefs.Acquired[app.Slug] = true
	savePrefs(s.prefs)
}

// releaseChanged runs from the download goroutine as the state moves on.
// Preferences, which the view reads, change on the main thread.
func (s *store) releaseChanged(slug string, state acquireState) {
	switch state.Phase {
	case phaseReady:
		s.onMain(func() {
			if s.prefs.Downloads == nil {
				s.prefs.Downloads = map[string]string{}
			}
			s.prefs.Downloads[slug] = state.Path
			s.prefs.Acquired[slug] = true
			savePrefs(s.prefs)
			s.invalidate()
		})
	case phaseNoAsset:
		s.onMain(func() {
			s.prefs.Acquired[slug] = true
			savePrefs(s.prefs)
			s.invalidate()
		})
		if app := s.appBySlug(slug); app != nil {
			openExternal(app.URL)
		}
	default:
		s.invalidate()
	}
}

// onMain runs fn on the UI thread where the view reads preferences.
func (s *store) onMain(fn func()) {
	if s.runOnMain != nil {
		s.runOnMain(fn)
		return
	}
	mygo.RunOnMain(fn)
}

func (s *store) invalidate() {
	if s.win != nil {
		s.win.Invalidate()
	}
}

// openPath and revealPath are variables so tests can record them.
var openPath = func(path string) {
	go func() { _ = mygo.Shell.OpenPath(path) }()
}

var revealPath = func(path string) {
	mygo.Shell.ShowItemInFolder(path)
}
