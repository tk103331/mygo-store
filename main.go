package main

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	sidebarWidth = 232
	titleBarMin  = 52
)

// store is the app's state and the window's view.
type store struct {
	win        *mygo.Window
	catalog    *Catalog
	prefs      prefs
	images     *imageCache
	releaseMgr *releaseManager

	// runOnMain moves preference changes to the UI thread. Tests replace it
	// with a direct call; nil means mygo.RunOnMain.
	runOnMain func(func())

	// nav is the sidebar's choice: discover, favorites, acquired, or
	// "group:<id>".
	nav      string
	query    string
	focus    string
	detail   string
	sort     SortMode
	kind     string // "", native or web
	platform string // "", mac, win or linux

	discoverScroll ui.ScrollState
	grid           ui.GridState
	visible        []App
}

func (s *store) view(c *ui.Context) {
	t := c.Theme()

	if c.Shortcut(ui.Cmd, ui.KeyF) {
		s.focus = "search"
	}
	if c.Shortcut(ui.Cmd, ui.Key1) {
		s.selectNav("discover")
	}
	if c.Shortcut(ui.Cmd, ui.Key2) {
		s.selectNav("favorites")
	}
	if c.Shortcut(ui.Cmd, ui.Key3) {
		s.selectNav("acquired")
	}
	if s.detail != "" && c.Shortcut(ui.Cmd, ui.KeyBracketLeft) {
		s.detail = ""
	}
	if s.detail != "" && c.Shortcut(0, ui.KeyEscape) {
		s.detail = ""
	}

	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	}

	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	row.Children(func() {
		s.sidebar(c, t, vibrant)
		ui.Box(c).Width(1).Background(t.Border)
		s.content(c, t)
	})
}

func (s *store) selectNav(nav string) {
	if s.nav == nav && s.query == "" && s.detail == "" {
		return
	}
	s.nav = nav
	s.query = ""
	s.detail = ""
	s.discoverScroll.Y = 0
	s.grid.ScrollTo(0, ui.Start)
}

func (s *store) resetBrowsing() {
	s.detail = ""
	s.discoverScroll.Y = 0
	s.grid.ScrollTo(0, ui.Start)
}

func (s *store) openDetail(slug string) {
	s.detail = slug
}

// sidebar draws the search field, the navigation and the categories.
func (s *store) sidebar(c *ui.Context, t *ui.Theme, vibrant bool) {
	bar := c.TitleBar()
	side := ui.Column(c).Width(sidebarWidth).Shrink(0)
	if !vibrant {
		side.Background(t.Surface)
	}
	side.Children(func() {
		top := ui.Box(c).Height(maxf(bar.Height, titleBarMin)).Shrink(0)
		top.DragWindow()

		ui.Box(c).Padding(12, 12, 8, 12).Shrink(0).Children(func() {
			field := ui.SearchField(c, &s.query).Placeholder("搜索应用、作者或分类").
				Label("搜索应用").FocusBind(&s.focus, "search").Width(sidebarWidth - 24)
			if field.Changed() {
				s.detail = ""
				s.discoverScroll.Y = 0
				s.grid.ScrollTo(0, ui.Start)
			}
		})

		sb := ui.Sidebar(c, &s.nav, func() {
			ui.SidebarItem(c, "discover", iconSparkle, "发现")
			item := ui.SidebarItem(c, "favorites", iconHeart, "收藏")
			if n := len(s.prefs.Favorites); n > 0 {
				item.Children(func() { ui.Textf(c, "%d", n).FontSize(11).Opacity(0.65) })
			}
			item = ui.SidebarItem(c, "acquired", iconDownload, "已获取")
			if n := len(s.prefs.Acquired); n > 0 {
				item.Children(func() { ui.Textf(c, "%d", n).FontSize(11).Opacity(0.65) })
			}

			ui.SidebarSection(c, "分类", nil, func() {
				for _, group := range groups {
					item := ui.SidebarItem(c, "group:"+group.ID, group.Icon, group.Label)
					item.Children(func() {
						ui.Textf(c, "%d", s.groupCount(group.ID)).FontSize(11).Opacity(0.55)
					})
				}
			})
		})
		if vibrant {
			sb.Background(ui.Transparent)
		}
		sb.Grow(1).Label("浏览")
		if sb.Changed() {
			s.query = ""
			s.detail = ""
			s.discoverScroll.Y = 0
			s.grid.ScrollTo(0, ui.Start)
		}

		ui.Divider(c).MarginX(12)
		ui.Column(c).Padding(10, 14, 10, 14).Gap(3).Shrink(0).Children(func() {
			ui.Textf(c, "%d 款应用 · 数据截至 %s", len(s.catalog.Apps), s.catalog.AsOf).
				FontSize(10.5).TextColor(t.TextMuted)
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Link(c, "awesomemygo.dev", s.catalog.Source).FontSize(10.5).
						Tooltip("数据来源：Awesome MyGo")
					ui.Textf(c, "v%s", appVersion()).FontSize(10.5).TextColor(t.TextMuted)
				})
				check := ui.ButtonBase(c).Label("检查更新").Height(18).Padding(0, 4).Radius(4).
					Cursor(ui.CursorPointer).
					Tooltip("启动后自动检查，也可以随时手动检查")
				check.Children(func() {
					ui.Text(c, "检查更新").FontSize(10.5).TextColor(t.TextMuted)
				})
				if check.Clicked() {
					checkUpdates()
				}
			})
		})
	})

}

func (s *store) content(c *ui.Context, t *ui.Theme) {
	bar := c.TitleBar()
	main := ui.Column(c).Grow(1).MinWidth(0).Background(t.Background)
	main.Children(func() {
		ui.Row(c).Height(maxf(bar.Height, titleBarMin)).Shrink(0).AlignItems(ui.Center).
			Padding(0, bar.Right+20, 0, 22).Gap(10).DragWindow().Children(func() {
			ui.Icon(c, iconSparkle).Size(14, 14).TextColor(t.Accent)
			ui.Text(c, "MyGo 应用市场").FontSize(12).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			if s.detail == "" && s.nav != "" {
				s.sortControl(c, t)
			}
		})
		ui.Divider(c)

		if s.detail != "" {
			if app := s.appBySlug(s.detail); app != nil {
				s.detailView(c, t, app)
				return
			}
			s.detail = ""
		}
		if s.nav == "discover" && strings.TrimSpace(s.query) == "" {
			s.discoverView(c, t)
			return
		}
		s.browseView(c, t)
	})
}

func (s *store) sortControl(c *ui.Context, t *ui.Theme) {
	ui.MenuButton(c, s.sort.Label(), func(m *ui.Menu) {
		for _, mode := range []SortMode{sortRecommended, sortNewest, sortHottest} {
			if m.Item(mode.Label()).Radio(true).Checked(mode == s.sort).Chosen() {
				s.sort = mode
				s.resetBrowsing()
			}
		}
	}).Label("排序方式").Height(28).Padding(0, 10).Radius(8).
		Border(1, t.Border).FontSize(12)
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func main() {
	mygo.Use(updaterPlugin())
	mygo.App.SetMenu(appMenu())

	s := &store{
		catalog: loadCatalog(),
		prefs:   loadPrefs(),
		images:  newImageCache(),
		nav:     "discover",
		sort:    sortRecommended,
	}
	s.seedDownloads()
	mygo.App.WhenReady(func() {
		s.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "MyGo 应用市场",
			Width:         1180,
			Height:        780,
			MinWidth:      900,
			MinHeight:     600,
			TitleBarStyle: mygo.TitleBarHidden,
			Vibrancy:      mygo.VibrancySidebar,
			StateKey:      "main",
			Content:       ui.View(s.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
