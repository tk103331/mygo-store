//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDesktopEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "denova.desktop")
	entry := `[Desktop Entry]
Type=Application
Name=Denova
Name[zh_CN]=德诺瓦
Exec=/usr/bin/denova %U
Icon=denova
Terminal=false
`
	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	desktop, ok := readDesktopEntry(path)
	if !ok {
		t.Fatal("the entry did not parse")
	}
	if desktop.name != "Denova" || desktop.exec != "/usr/bin/denova %U" {
		t.Errorf("entry = %+v, want Denova and its Exec line", desktop)
	}
}

func TestReadHiddenDesktopEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hidden.desktop")
	entry := "[Desktop Entry]\nName=Hidden\nExec=/usr/bin/hidden\nNoDisplay=true\n"
	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readDesktopEntry(path); ok {
		t.Error("a hidden entry reads as an app")
	}
}

func TestDesktopExecutable(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "denova")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := desktopExecutable(`"` + exe + `" %U`); got != exe {
		t.Errorf("desktopExecutable = %q, want %q", got, exe)
	}
	if got := desktopExecutable("denova %U"); got != "" {
		t.Errorf("a command that is not a path reads as %q", got)
	}
}
