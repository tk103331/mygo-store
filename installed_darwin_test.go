//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInfoPlist(t *testing.T) {
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>Denova</string>
	<key>CFBundleIdentifier</key>
	<string>com.example.denova</string>
	<key>CFBundleShortVersionString</key>
	<string>1.2.0</string>
	<key>CFBundleExecutable</key>
	<string>Denova</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>CFBundleDocumentTypes</key>
	<array>
		<dict>
			<key>CFBundleTypeName</key>
			<string>Denova Document</string>
		</dict>
	</array>
</dict>
</plist>
`
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	info, ok := readInfoPlist(path)
	if !ok {
		t.Fatal("the plist did not parse")
	}
	want := map[string]string{
		"CFBundleName":               "Denova",
		"CFBundleIdentifier":         "com.example.denova",
		"CFBundleShortVersionString": "1.2.0",
		"CFBundleExecutable":         "Denova",
	}
	for key, value := range want {
		if info[key] != value {
			t.Errorf("%s = %q, want %q", key, info[key], value)
		}
	}
}
