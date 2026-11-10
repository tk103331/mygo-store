//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

var (
	versionDLL = syscall.NewLazyDLL("version.dll")

	procGetFileVersionInfoSizeW = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = versionDLL.NewProc("VerQueryValueW")
)

// platformInstalledApps reads the uninstall entries that Windows keeps, in
// Settings > Apps, and keeps the ones whose executable `mygo build` marked
// with MyGoIdentifier: the NSIS installers that `mygo build` makes and
// install.ps1 write them under the app's identifier.
func platformInstalledApps(known func(string) bool) ([]installedApp, error) {
	self, _ := os.Executable()
	var apps []installedApp
	seen := map[string]bool{}
	for _, root := range []struct {
		key  registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	} {
		parent, err := registry.OpenKey(root.key, root.path, registry.READ)
		if err != nil {
			continue
		}
		names, err := parent.ReadSubKeyNames(-1)
		parent.Close()
		if err != nil {
			continue
		}
		for _, name := range names {
			app, ok := uninstallApp(root.key, root.path+`\`+name, known)
			if !ok || samePath(app.Path, self) {
				continue
			}
			key := strings.ToLower(app.Identifier)
			if key == "" {
				key = strings.ToLower(app.Path)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			apps = append(apps, app)
		}
	}
	return apps, nil
}

// uninstallApp reads one uninstall entry, and reports whether its app was
// built by mygo.
func uninstallApp(root registry.Key, path string, known func(string) bool) (installedApp, bool) {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return installedApp{}, false
	}
	defer key.Close()

	// Updates and components are not apps of their own.
	if value, _, err := key.GetIntegerValue("SystemComponent"); err == nil && value == 1 {
		return installedApp{}, false
	}
	if value, _, err := key.GetStringValue("ParentKeyName"); err == nil && strings.TrimSpace(value) != "" {
		return installedApp{}, false
	}
	name, _, _ := key.GetStringValue("DisplayName")
	name = strings.TrimSpace(name)
	if name == "" {
		return installedApp{}, false
	}
	version, _, _ := key.GetStringValue("DisplayVersion")
	icon, _, _ := key.GetStringValue("DisplayIcon")
	location, _, _ := key.GetStringValue("InstallLocation")

	for _, exe := range uninstallExecutables(icon, location) {
		info := fileVersionInfo(exe)
		identifier := strings.TrimSpace(info["MyGoIdentifier"])
		if identifier == "" {
			// An older mygo build wrote no marker into its executable, but
			// its installer still keys the entry by the app's identifier,
			// which the catalog knows.
			if candidate := filepath.Base(path); known(candidate) {
				identifier = candidate
			}
		}
		if identifier == "" {
			continue
		}
		if strings.TrimSpace(version) == "" {
			version = info["ProductVersion"]
		}
		return installedApp{
			Name:       name,
			Version:    strings.TrimSpace(version),
			Identifier: identifier,
			Path:       exe,
		}, true
	}
	return installedApp{}, false
}

// uninstallExecutables names the executables an uninstall entry may run:
// the one DisplayIcon holds, then those of its install folder.
func uninstallExecutables(icon, location string) []string {
	var exes []string
	seen := map[string]bool{}
	add := func(path string) {
		path = strings.Trim(strings.TrimSpace(path), `"`)
		if !strings.EqualFold(filepath.Ext(path), ".exe") {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		key := strings.ToLower(path)
		if seen[key] {
			return
		}
		seen[key] = true
		exes = append(exes, path)
	}

	// DisplayIcon is the executable, sometimes with an icon index: app.exe,0
	if comma := strings.LastIndex(icon, ","); comma >= 0 {
		if _, err := strconv.Atoi(strings.TrimSpace(icon[comma+1:])); err == nil {
			icon = icon[:comma]
		}
	}
	add(icon)

	location = strings.Trim(strings.TrimSpace(location), `"`)
	if location == "" {
		return exes
	}
	matches, _ := filepath.Glob(filepath.Join(location, "*.exe"))
	sort.Strings(matches)
	for _, exe := range matches {
		base := strings.ToLower(filepath.Base(exe))
		if strings.HasPrefix(base, "unins") || strings.HasSuffix(base, "uninstall.exe") {
			continue
		}
		add(exe)
	}
	return exes
}

// fileVersionInfo reads the version resource of a Windows executable.
func fileVersionInfo(path string) map[string]string {
	info := map[string]string{}
	file, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return info
	}
	var handle uint32
	size, _, _ := procGetFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(&handle)))
	if size == 0 {
		return info
	}
	data := make([]byte, size)
	if ok, _, _ := procGetFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(file)), 0, uintptr(size), uintptr(unsafe.Pointer(&data[0])),
	); ok == 0 {
		return info
	}
	value := func(name string) uintptr {
		query, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			return 0
		}
		var address uintptr
		var length uint32
		if ok, _, _ := procVerQueryValueW.Call(
			uintptr(unsafe.Pointer(&data[0])),
			uintptr(unsafe.Pointer(query)),
			uintptr(unsafe.Pointer(&address)),
			uintptr(unsafe.Pointer(&length)),
		); ok == 0 || address == 0 {
			return 0
		}
		return address
	}

	// The strings of the app's own language, the first of its translations.
	table := "040904b0"
	if address := value(`\VarFileInfo\Translation`); address != 0 {
		translation := unmanagedPointer(address)
		lang := *(*uint16)(translation)
		page := *(*uint16)(unsafe.Add(translation, 2))
		table = fmt.Sprintf("%04x%04x", lang, page)
	}
	for _, name := range []string{"MyGoIdentifier", "ProductName", "ProductVersion", "FileDescription"} {
		if address := value(`\StringFileInfo\` + table + `\` + name); address != 0 {
			info[name] = utf16String(address)
		}
	}
	return info
}

// utf16String reads a NUL-terminated UTF-16 string at address.
func utf16String(address uintptr) string {
	text := unmanagedPointer(address)
	units := make([]uint16, 0, 32)
	for i := 0; i < 4096; i++ {
		unit := *(*uint16)(unsafe.Add(text, uintptr(2*i)))
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return string(utf16.Decode(units))
}

// unmanagedPointer turns an address of memory not managed by Go into a pointer.
func unmanagedPointer(address uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&address))
}

// samePath compares two Windows paths, which differ in case.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
