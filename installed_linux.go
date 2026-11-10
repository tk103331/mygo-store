//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// platformInstalledApps lists the apps of the desktop entries menus show
// that were built by mygo. An entry is read only when its name may be one
// of the catalog's, as reading its executable is the costly part.
func platformInstalledApps(known func(string) bool) ([]installedApp, error) {
	self, _ := os.Executable()
	var apps []installedApp
	seen := map[string]bool{}
	for _, dir := range desktopDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			desktop, ok := readDesktopEntry(path)
			if !ok {
				continue
			}
			exe := desktopExecutable(desktop.exec)
			if exe == "" || samePath(exe, self) {
				continue
			}
			if !known(desktop.name) && !known(fileStem(exe)) {
				continue
			}
			if !isMyGoExecutable(exe) {
				continue
			}
			key := strings.ToLower(exe)
			if seen[key] {
				continue
			}
			seen[key] = true
			apps = append(apps, installedApp{
				Name:    desktop.name,
				Version: desktop.version,
				Path:    path,
			})
		}
	}
	return apps, nil
}

// desktopDirs lists the applications folders of the desktop, where the
// desktop entries menus show live.
func desktopDirs() []string {
	var dirs []string
	if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" {
		dirs = append(dirs, filepath.Join(dir, "applications"))
	} else if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "applications"))
	}
	data := os.Getenv("XDG_DATA_DIRS")
	if strings.TrimSpace(data) == "" {
		data = "/usr/local/share:/usr/share"
	}
	for _, dir := range filepath.SplitList(data) {
		if dir = strings.TrimSpace(dir); dir != "" {
			dirs = append(dirs, filepath.Join(dir, "applications"))
		}
	}
	return dirs
}

// desktopEntry is the part of a desktop entry the app uses.
type desktopEntry struct {
	name, exec, version string
	hidden              bool
}

// readDesktopEntry reads a .desktop file, the ones whose Desktop Entry
// section names an app that menus show.
func readDesktopEntry(path string) (desktopEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return desktopEntry{}, false
	}
	var entry desktopEntry
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		if section != "Desktop Entry" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "Name":
			entry.name = value
		case "Exec":
			entry.exec = value
		case "X-AppImage-Version":
			entry.version = value
		case "Hidden", "NoDisplay":
			if strings.EqualFold(value, "true") {
				entry.hidden = true
			}
		}
	}
	if entry.hidden || entry.name == "" || entry.exec == "" {
		return desktopEntry{}, false
	}
	return entry, true
}

// desktopExecutable names the executable an Exec line runs, before its
// arguments and field codes, or "" when it runs a command that is not a
// program of the app's own.
func desktopExecutable(exec string) string {
	var b strings.Builder
	quote := rune(0)
loop:
	for _, r := range strings.TrimSpace(exec) {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t':
			break loop
		default:
			b.WriteRune(r)
		}
	}
	path := b.String()
	if i := strings.IndexByte(path, '%'); i >= 0 {
		path = path[:i]
	}
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return ""
	}
	return path
}

// samePath compares two Linux paths, which differ in case.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
