package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFooterCheckUpdates(t *testing.T) {
	old := checkUpdates
	t.Cleanup(func() { checkUpdates = old })

	calls := 0
	checkUpdates = func() { calls++ }

	s, tt := newTestStore(t)
	if !tt.HasText("检查更新") {
		t.Fatalf("the footer does not show 检查更新: %q", tt.Texts())
	}
	if err := tt.Click("检查更新"); err != nil {
		t.Fatalf("click 检查更新: %v", err)
	}
	if calls != 1 {
		t.Fatalf("checkUpdates called %d times, want 1", calls)
	}

	// The smallest window keeps the footer reachable.
	small := ui.NewTester(s.view, 900, 600)
	small.SetTitleBar(ui.TitleBar{Height: 52, Left: 78, Right: 14})
	small.SetDark(true)
	small.SetLocale("zh-Hans")
	small.Frame()
	if err := small.Click("检查更新"); err != nil {
		t.Fatalf("click 检查更新 in the smallest window: %v", err)
	}
	if calls != 2 {
		t.Fatalf("checkUpdates called %d times, want 2", calls)
	}
}

func TestAppVersionNeverEmpty(t *testing.T) {
	if appVersion() == "" {
		t.Fatal("appVersion returned an empty string")
	}
}
