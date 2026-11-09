package main

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-store/locales"
)

const (
	sidebarWidth = 232
	titleBarMin  = 52
)

// store is the app's state and the window's view.
type store struct {
	win         *mygo.Window
	catalog     *Catalog
	catalogSync *catalogSyncer
	prefs       prefs
	images      *imageCache
	releaseMgr  *releaseManager

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

	// locale is the language of the frame being drawn, as Context.Locale
	// says it.
	locale string
}

// messages says the UI's strings in the language of the frame.
func (s *store) messages() locales.Messages {
	return locales.In(s.locale)
}

func (s *store) view(c *ui.Context) {
	t := c.Theme()
	s.locale = c.Locale()

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
	msgs := s.messages()
	bar := c.TitleBar()
	side := ui.Column(c).Width(sidebarWidth).Shrink(0)
	if !vibrant {
		side.Background(t.Surface)
	}
	side.Children(func() {
		top := ui.Box(c).Height(maxf(bar.Height, titleBarMin)).Shrink(0)
		top.DragWindow()

		ui.Box(c).Padding(12, 12, 8, 12).Shrink(0).Children(func() {
			field := ui.SearchField(c, &s.query).Placeholder(msgs.SearchPlaceholder()).
				Label(msgs.SearchAppsLabel()).FocusBind(&s.focus, "search").Width(sidebarWidth - 24)
			if field.Changed() {
				s.detail = ""
				s.discoverScroll.Y = 0
				s.grid.ScrollTo(0, ui.Start)
			}
		})

		sb := ui.Sidebar(c, &s.nav, func() {
			ui.SidebarItem(c, "discover", iconSparkle, msgs.NavDiscover())
			item := ui.SidebarItem(c, "favorites", iconHeart, msgs.NavFavorites())
			if n := len(s.prefs.Favorites); n > 0 {
				item.Children(func() { ui.Textf(c, "%d", n).FontSize(11).Opacity(0.65) })
			}
			item = ui.SidebarItem(c, "acquired", iconDownload, msgs.NavAcquired())
			if n := len(s.prefs.Acquired); n > 0 {
				item.Children(func() { ui.Textf(c, "%d", n).FontSize(11).Opacity(0.65) })
			}

			ui.SidebarSection(c, msgs.SidebarCategories(), nil, func() {
				for _, group := range groups {
					item := ui.SidebarItem(c, "group:"+group.ID, group.Icon,
						groupLabel(s.locale, group.ID))
					item.Children(func() {
						ui.Textf(c, "%d", s.groupCount(group.ID)).FontSize(11).Opacity(0.55)
					})
				}
			})
		})
		if vibrant {
			sb.Background(ui.Transparent)
		}
		sb.Grow(1).Label(msgs.SidebarLabel())
		if sb.Changed() {
			s.query = ""
			s.detail = ""
			s.discoverScroll.Y = 0
			s.grid.ScrollTo(0, ui.Start)
		}

		ui.Divider(c).MarginX(12)
		ui.Column(c).Padding(10, 14, 10, 14).Gap(3).Shrink(0).Children(func() {
			ui.Text(c, msgs.FooterStats(len(s.catalog.Apps), s.catalog.AsOf)).
				FontSize(10.5).TextColor(t.TextMuted)
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Link(c, "awesomemygo.dev", s.catalog.Source).FontSize(10.5).
					Tooltip(msgs.FooterSourceTooltip())
				ui.Textf(c, "v%s", appVersion()).FontSize(10.5).TextColor(t.TextMuted)
			})
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				synced := ui.ButtonBase(c).Height(18).Padding(0, 4).Radius(4).
					Cursor(ui.CursorPointer).Tooltip(s.catalogSync.tooltip(msgs))
				synced.Children(func() {
					ui.Text(c, s.catalogSync.statusText(msgs)).FontSize(10.5).TextColor(t.TextMuted)
				})
				if synced.Clicked() {
					s.catalogSync.checkNow()
				}
				check := ui.ButtonBase(c).Label(msgs.CheckUpdates()).Height(18).Padding(0, 4).Radius(4).
					Cursor(ui.CursorPointer).
					Tooltip(msgs.CheckUpdatesTooltip())
				check.Children(func() {
					ui.Text(c, msgs.CheckUpdates()).FontSize(10.5).TextColor(t.TextMuted)
				})
				if check.Clicked() {
					checkUpdates()
				}
			})
		})
	})

}

func (s *store) content(c *ui.Context, t *ui.Theme) {
	msgs := s.messages()
	bar := c.TitleBar()
	main := ui.Column(c).Grow(1).MinWidth(0).Background(t.Background)
	main.Children(func() {
		ui.Row(c).Height(maxf(bar.Height, titleBarMin)).Shrink(0).AlignItems(ui.Center).
			Padding(0, bar.Right+20, 0, 22).Gap(10).DragWindow().Children(func() {
			ui.Icon(c, iconSparkle).Size(14, 14).TextColor(t.Accent)
			ui.Text(c, msgs.AppTitle()).FontSize(12).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			if s.detail == "" && s.nav != "" {
				s.sortControl(c, t, msgs)
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

func (s *store) sortControl(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	ui.MenuButton(c, s.sort.Label(msgs), func(m *ui.Menu) {
		for _, mode := range []SortMode{sortRecommended, sortNewest, sortHottest} {
			if m.Item(mode.Label(msgs)).Radio(true).Checked(mode == s.sort).Chosen() {
				s.sort = mode
				s.resetBrowsing()
			}
		}
	}).Label(msgs.SortByLabel()).Height(28).Padding(0, 10).Radius(8).
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

	s := &store{
		catalog: loadBestCatalog(),
		prefs:   loadPrefs(),
		images:  newImageCache(),
		nav:     "discover",
		sort:    sortRecommended,
	}
	languageChoice = s.prefs.Locale
	menu := appMenu(func(tag string) {
		s.prefs.Locale = tag
		savePrefs(s.prefs)
	})
	mygo.App.SetMenu(menu)

	s.catalogSync = newCatalogSyncer(s)
	s.seedDownloads()
	mygo.App.OnLocaleChanged(func(locale string) {
		relabelAppMenu(menu, locale)
		if s.win != nil {
			s.win.SetTitle(locales.In(locale).AppTitle())
		}
	})
	mygo.App.WhenReady(func() {
		if s.prefs.Locale != "" {
			mygo.App.SetLocale(s.prefs.Locale)
		}
		locale := mygo.App.Locale()
		relabelAppMenu(menu, locale)
		s.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         locales.In(locale).AppTitle(),
			Width:         1180,
			Height:        780,
			MinWidth:      900,
			MinHeight:     600,
			TitleBarStyle: mygo.TitleBarHidden,
			Vibrancy:      mygo.VibrancySidebar,
			StateKey:      "main",
			Content:       ui.View(s.view),
		})
		s.catalogSync.start()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
