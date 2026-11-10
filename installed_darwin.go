//go:build darwin

package main

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// platformInstalledApps lists the apps of the Applications folders that
// were built by mygo: a bundle is read only when its name or identifier may
// be one of the catalog's, as reading its executable is the costly part.
func platformInstalledApps(known func(string) bool) ([]installedApp, error) {
	home, _ := os.UserHomeDir()
	roots := []string{"/Applications"}
	if home != "" {
		roots = append(roots, filepath.Join(home, "Applications"))
	}
	self, _ := os.Executable()

	var apps []installedApp
	seen := map[string]bool{}
	for _, bundle := range appBundles(roots) {
		info, ok := readInfoPlist(filepath.Join(bundle, "Contents", "Info.plist"))
		if !ok {
			continue
		}
		name := strings.TrimSpace(info["CFBundleDisplayName"])
		if name == "" {
			name = strings.TrimSpace(info["CFBundleName"])
		}
		if name == "" {
			name = fileStem(bundle)
		}
		identifier := strings.TrimSpace(info["CFBundleIdentifier"])
		if !known(name) && !known(fileStem(bundle)) && !known(identifier) {
			continue
		}
		executable := strings.TrimSpace(info["CFBundleExecutable"])
		if executable == "" {
			continue
		}
		exe := filepath.Join(bundle, "Contents", "MacOS", executable)
		if samePath(exe, self) || !isMyGoExecutable(exe) {
			continue
		}
		key := strings.ToLower(identifier)
		if key == "" {
			key = bundle
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		version := strings.TrimSpace(info["CFBundleShortVersionString"])
		if version == "" {
			version = strings.TrimSpace(info["CFBundleVersion"])
		}
		apps = append(apps, installedApp{
			Name:       name,
			Version:    version,
			Identifier: identifier,
			Path:       bundle,
		})
	}
	return apps, nil
}

// appBundles lists the .app bundles of the roots and of their folders, as
// /Applications/Utilities holds some.
func appBundles(roots []string) []string {
	var bundles []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(root, entry.Name())
			if strings.HasSuffix(entry.Name(), ".app") {
				bundles = append(bundles, path)
				continue
			}
			inner, err := filepath.Glob(filepath.Join(path, "*.app"))
			if err != nil {
				continue
			}
			bundles = append(bundles, inner...)
		}
	}
	return bundles
}

// readInfoPlist reads the strings of an Info.plist, which mygo writes as
// XML; a binary one, which mygo does not write, reads as no values.
func readInfoPlist(path string) (map[string]string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	values := map[string]string{}
	decoder := xml.NewDecoder(io.LimitReader(file, 4<<20))
	pending := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return values, true
		}
		if err != nil {
			return values, false
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "key":
			var key string
			if err := decoder.DecodeElement(&key, &start); err != nil {
				return values, false
			}
			pending = key
		case "string":
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return values, false
			}
			if pending != "" {
				if _, exists := values[pending]; !exists {
					values[pending] = value
				}
			}
			pending = ""
		case "dict", "plist":
			// The keys this reads are the ones of the top-level dict.
		default:
			pending = ""
			if err := decoder.Skip(); err != nil {
				return values, false
			}
		}
	}
}

// samePath compares two macOS paths, which differ in case.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
