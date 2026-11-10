package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/egoist/mygo"
)

// installedApp is a MyGo app that is installed on this computer, whether
// the user got it from the store or by other means.
type installedApp struct {
	Slug       string // the catalog app it is
	Name       string // the name the system shows for it
	Version    string // the installed version, "" when unknown
	Identifier string // its bundle identifier, "" when the system has none
	Path       string // what launches it: an executable, .app bundle or desktop entry
}

// installedScanner lists the MyGo apps installed on this computer. The
// scanners that read the disk for each app call known first, so that they
// read only the apps the catalog may know. Tests replace it.
var installedScanner = platformInstalledApps

// launchInstalledApp starts an app of installedApp. It is a variable so
// tests can record the call.
var launchInstalledApp = func(app installedApp) {
	go func() {
		_ = mygo.Shell.LaunchApp(mygo.InstalledApp{
			Name: app.Name,
			ID:   app.Identifier,
			Path: app.Path,
		})
	}()
}

// refreshInstalled looks for MyGo apps installed on this computer in the
// background, and merges them into the acquired apps on the UI thread.
func (s *store) refreshInstalled() {
	if s.installedBusy {
		return
	}
	s.installedBusy = true
	s.invalidate()

	known := catalogNameMatcher(s.catalog)
	scan := func() {
		apps, err := installedScanner(known)
		if err != nil {
			log.Printf("mygo-store: detecting installed apps: %v", err)
		}
		s.onMain(func() {
			s.installedBusy = false
			// A failed scan that found nothing keeps what the last one knew.
			if err == nil || len(apps) > 0 {
				s.setInstalled(apps)
			}
			s.invalidate()
		})
	}
	if s.async != nil {
		s.async(scan)
		return
	}
	go scan()
}

// catalogNameMatcher reports whether a name may be one of the catalog's
// apps, by the name, slug or identifier an installed app may show.
func catalogNameMatcher(catalog *Catalog) func(string) bool {
	names := make(map[string]bool, len(catalog.Apps)*3)
	for i := range catalog.Apps {
		app := &catalog.Apps[i]
		for _, name := range []string{app.Name, app.Slug, app.Identifier} {
			if key := normalizeName(name); key != "" {
				names[key] = true
			}
		}
	}
	return func(name string) bool {
		return names[normalizeName(name)]
	}
}

// setInstalled keeps the detected apps that the catalog knows, on the UI
// thread where the view reads them.
func (s *store) setInstalled(detected []installedApp) {
	apps := matchInstalled(s.catalog, detected)
	s.installedBySlug = make(map[string]installedApp, len(apps))
	for _, app := range apps {
		s.installedBySlug[app.Slug] = app
	}
}

// matchInstalled keeps the detected apps that the catalog knows: by
// identifier first, then by the name the system shows, then by the file it
// runs from.
func matchInstalled(catalog *Catalog, detected []installedApp) []installedApp {
	byIdentifier := make(map[string]string, len(catalog.Apps))
	byName := make(map[string]string, len(catalog.Apps)*2)
	for i := range catalog.Apps {
		app := &catalog.Apps[i]
		if identifier := normalizeIdentifier(app.Identifier); identifier != "" {
			if _, ok := byIdentifier[identifier]; !ok {
				byIdentifier[identifier] = app.Slug
			}
		}
		for _, name := range []string{app.Name, app.Slug} {
			key := normalizeName(name)
			if key == "" {
				continue
			}
			if _, ok := byName[key]; !ok {
				byName[key] = app.Slug
			}
		}
	}

	seen := make(map[string]bool, len(detected))
	apps := make([]installedApp, 0, len(detected))
	for _, app := range detected {
		slug := ""
		if identifier := normalizeIdentifier(app.Identifier); identifier != "" {
			slug = byIdentifier[identifier]
		}
		if slug == "" {
			slug = byName[normalizeName(app.Name)]
		}
		if slug == "" {
			slug = byName[normalizeName(fileStem(app.Path))]
		}
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		app.Slug = slug
		apps = append(apps, app)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return apps
}

// installedApp returns the installed app of a catalog slug.
func (s *store) installedApp(slug string) (installedApp, bool) {
	app, ok := s.installedBySlug[slug]
	return app, ok
}

// isAcquired says whether an app was downloaded here or is installed: the
// acquired page lists both.
func (s *store) isAcquired(slug string) bool {
	if s.prefs.Acquired[slug] {
		return true
	}
	_, ok := s.installedBySlug[slug]
	return ok
}

// acquiredCount counts the catalog's acquired apps, as the sidebar says.
func (s *store) acquiredCount() int {
	n := 0
	for i := range s.catalog.Apps {
		if s.isAcquired(s.catalog.Apps[i].Slug) {
			n++
		}
	}
	return n
}

// openInstalled starts an app installed on this computer, and reports
// whether it did.
func (s *store) openInstalled(slug string) bool {
	app, ok := s.installedApp(slug)
	if !ok {
		return false
	}
	launchInstalledApp(app)
	return true
}

// normalizeName keeps the letters and digits of a name, lowercased, so that
// "MyGo Store", "mygo-store" and "MyGoStore" compare equal.
func normalizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeIdentifier lowercases a bundle identifier, which systems compare
// without case.
func normalizeIdentifier(identifier string) string {
	return strings.ToLower(strings.TrimSpace(identifier))
}

// fileStem is the name of the file or bundle an app runs from, without its
// extension: "Denova" for Denova.exe or Denova.app.
func fileStem(path string) string {
	path = strings.TrimSpace(path)
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		path = path[i+1:]
	}
	base := path
	lower := strings.ToLower(base)
	for _, ext := range []string{".app", ".desktop", ".exe", ".appimage"} {
		if strings.HasSuffix(lower, ext) {
			return base[:len(base)-len(ext)]
		}
	}
	return base
}

// myGoMarker is in every executable `mygo build` builds: the Go linker
// keeps the package paths of the app in the binary, for stack traces.
const myGoMarker = "github.com/egoist/mygo"

// isMyGoExecutable reports whether an executable was built by `mygo build`,
// which the macOS and Linux scanners use to tell MyGo apps apart.
func isMyGoExecutable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	marker := []byte(myGoMarker)
	overlap := len(marker) - 1
	tail := make([]byte, 0, overlap)
	buf := make([]byte, 256*1024)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			data := make([]byte, 0, len(tail)+n)
			data = append(data, tail...)
			data = append(data, buf[:n]...)
			if bytes.Contains(data, marker) {
				return true
			}
			if len(data) > overlap {
				tail = append(tail[:0], data[len(data)-overlap:]...)
			} else {
				tail = append(tail[:0], data...)
			}
		}
		if err == io.EOF {
			return false
		}
		if err != nil {
			return false
		}
	}
}
