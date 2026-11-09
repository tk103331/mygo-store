package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func newTestStore(t *testing.T) (*store, *ui.Tester) {
	t.Helper()
	t.Setenv("MYGO_STORE_CONFIG", t.TempDir())
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
	return s, tt
}

func TestOpenDetailAndFavorite(t *testing.T) {
	s, tt := newTestStore(t)
	app := s.appBySlug("denova")
	if app == nil {
		t.Skip("denova is not in the catalog")
	}

	if err := tt.Click("Denova"); err != nil {
		t.Fatalf("click Denova: %v", err)
	}
	if s.detail != "denova" {
		t.Fatalf("detail = %q after clicking an app", s.detail)
	}
	if !tt.HasText("关于") || !tt.HasText(app.Description) {
		t.Errorf("detail texts %q", tt.Texts())
	}

	if err := tt.Click("收藏应用"); err != nil {
		t.Fatalf("click favorite: %v", err)
	}
	if !s.prefs.Favorites["denova"] {
		t.Error("the app was not favorited")
	}
	tt.Frame()
	if err := tt.Click("取消收藏应用"); err != nil {
		t.Fatalf("click unfavorite: %v", err)
	}
	if s.prefs.Favorites["denova"] {
		t.Error("the app was not unfavorited")
	}

	if err := tt.Click("返回"); err != nil {
		t.Fatalf("click back: %v", err)
	}
	if s.detail != "" {
		t.Errorf("detail = %q after going back", s.detail)
	}
}

func TestAcquireWithoutRepoOpensTheAppPage(t *testing.T) {
	s, tt := newTestStore(t)
	app := s.appBySlug("clips")
	if app == nil {
		t.Skip("clips is not in the catalog")
	}

	var opened []string
	previous := openExternal
	openExternal = func(url string) { opened = append(opened, url) }
	t.Cleanup(func() { openExternal = previous })

	s.openDetail("clips")
	tt.Frame()
	if err := tt.Click("获取"); err != nil {
		t.Fatalf("click acquire: %v", err)
	}
	if !s.prefs.Acquired["clips"] {
		t.Error("the app was not marked acquired")
	}
	if len(opened) != 1 || opened[0] != app.URL {
		t.Errorf("opened %q, want %q", opened, app.URL)
	}
	tt.Frame()
	if !tt.HasText("已获取") {
		t.Errorf("texts %q after acquiring", tt.Texts())
	}
}

func TestDetailShowsDownloadState(t *testing.T) {
	s, tt := newTestStore(t)
	if s.appBySlug("denova") == nil {
		t.Skip("denova is not in the catalog")
	}
	s.releases().set("denova", acquireState{
		Phase:    phaseDownloading,
		Progress: 0.42,
		Asset:    "Denova-1.2.0-arm64.dmg",
		Message:  "正在下载安装包…",
	})
	s.openDetail("denova")
	tt.Frame()

	if !tt.HasText("下载中 42%") {
		t.Errorf("no progress in the button: %q", tt.Texts())
	}
	if !tt.HasText("正在下载 Denova-1.2.0-arm64.dmg · 42%") {
		t.Errorf("no progress line: %q", tt.Texts())
	}
}

func TestDetailShowsDownloadedInstaller(t *testing.T) {
	s, tt := newTestStore(t)
	if s.appBySlug("denova") == nil {
		t.Skip("denova is not in the catalog")
	}
	path := filepath.Join(t.TempDir(), "Denova-1.2.0-arm64.dmg")
	if err := os.WriteFile(path, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.releases().set("denova", acquireState{
		Phase:   phaseReady,
		Path:    path,
		Asset:   "Denova-1.2.0-arm64.dmg",
		Tag:     "v1.2.0",
		Source:  "https://example.com/release",
		Message: "安装包已下载",
	})
	s.openDetail("denova")
	tt.Frame()

	if !tt.HasText("打开安装包") || !tt.HasText("在 Finder 中显示") {
		t.Errorf("texts %q after downloading", tt.Texts())
	}
}

func TestCommandFFocusesSearch(t *testing.T) {
	s, tt := newTestStore(t)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("den")
	tt.Frame()
	if s.query != "den" {
		t.Fatalf("query = %q after Cmd+F and typing", s.query)
	}
}

func TestSearch(t *testing.T) {
	s, tt := newTestStore(t)
	if err := tt.Click("搜索应用"); err != nil {
		// The field may not carry its label as text in the access tree;
		// click it by position instead.
		tt.ClickAt(100, 79)
	}
	tt.Type("git")
	tt.Frame()
	if s.query != "git" {
		t.Fatalf("query = %q after typing", s.query)
	}
	if !tt.HasText("搜索") || !strings.Contains(strings.Join(tt.Texts(), "\n"), "个结果") {
		t.Errorf("texts %q after searching", tt.Texts())
	}
}

func TestCategoryNavigation(t *testing.T) {
	s, tt := newTestStore(t)
	if err := tt.Click("开发工具"); err != nil {
		t.Fatalf("click category: %v", err)
	}
	if s.nav != "group:dev" {
		t.Fatalf("nav = %q after clicking a category", s.nav)
	}
	apps := s.filterApps()
	if len(apps) == 0 {
		t.Fatal("no apps in the dev group")
	}
	for _, app := range apps {
		if app.Group != "dev" {
			t.Errorf("%s in group %q, want dev", app.Slug, app.Group)
		}
	}
}

func TestFilters(t *testing.T) {
	s, tt := newTestStore(t)
	s.selectNav("group:dev")
	tt.Frame()

	if err := tt.Click("Web 前端"); err != nil {
		t.Fatalf("click the web filter: %v", err)
	}
	if s.kind != "web" {
		t.Fatalf("kind = %q after clicking the web filter", s.kind)
	}
	for _, app := range s.filterApps() {
		if app.Kind != "web" {
			t.Errorf("%s is %q, want web", app.Slug, app.Kind)
		}
	}
}

func TestSortMenu(t *testing.T) {
	s, tt := newTestStore(t)
	s.selectNav("group:dev")
	tt.Frame()

	if err := tt.Click("评分最高"); err != nil {
		t.Fatalf("click the sort menu: %v", err)
	}
	if items := tt.Menu(); len(items) == 0 {
		t.Fatal("the sort menu has no items")
	}
	if err := tt.ChooseMenuItem("最新收录"); err != nil {
		t.Fatalf("choose sort mode: %v", err)
	}
	if s.sort != sortNewest {
		t.Fatalf("sort = %v after choosing the newest", s.sort)
	}
	apps := s.filterApps()
	for i := 1; i < len(apps); i++ {
		if apps[i-1].Added < apps[i].Added {
			t.Errorf("%s added %s before %s added %s", apps[i-1].Slug, apps[i-1].Added, apps[i].Slug, apps[i].Added)
		}
	}
}

func TestEscapeLeavesDetail(t *testing.T) {
	s, tt := newTestStore(t)
	if app := s.appBySlug("denova"); app == nil {
		t.Skip("denova is not in the catalog")
	}
	s.openDetail("denova")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.detail != "" {
		t.Errorf("detail = %q after Escape", s.detail)
	}
}

func TestCommandOneLeavesDetail(t *testing.T) {
	s, tt := newTestStore(t)
	if app := s.appBySlug("denova"); app == nil {
		t.Skip("denova is not in the catalog")
	}
	s.openDetail("denova")
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key1)
	tt.Frame()
	if s.detail != "" {
		t.Errorf("detail = %q after Cmd+1", s.detail)
	}
	if s.nav != "discover" {
		t.Errorf("nav = %q after Cmd+1", s.nav)
	}
}
