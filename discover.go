package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// discoverView is the home page: the highest scored apps, then a row of
// picks for every category, as Applite's Discover page lays out its shelves.
func (s *store) discoverView(c *ui.Context, t *ui.Theme) {
	msgs := s.messages()
	ui.Scroll(c).TrackScroll(&s.discoverScroll).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 32, 44).Gap(24).Children(func() {
			s.pageHeader(c, t, msgs.DiscoverTitle(),
				msgs.DiscoverSubtitle(len(s.catalog.Apps)))

			s.discoverSection(c, t, msgs.EditorsPick(), msgs.EditorsPickBlurb(),
				iconStar, s.topApps(6), "")

			for _, group := range groups {
				apps := s.groupApps(group.ID, 6)
				if len(apps) == 0 {
					continue
				}
				s.discoverSection(c, t, groupLabel(s.locale, group.ID),
					groupBlurb(s.locale, group.ID), group.Icon, apps, group.ID)
			}

			ui.Column(c).Padding(0, 4).Gap(4).Children(func() {
				ui.Text(c, msgs.DiscoverFooter()).
					FontSize(11.5).TextColor(t.TextMuted)
				ui.Link(c, "awesomemygo.dev/showcase", "https://awesomemygo.dev/showcase").FontSize(11.5)
			})
		})
	})
}

func (s *store) pageHeader(c *ui.Context, t *ui.Theme, title, subtitle string) {
	ui.Column(c).Gap(3).Shrink(0).Children(func() {
		ui.Text(c, title).FontSize(32).Bold().LetterSpacing(-0.4)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(13).TextColor(t.TextMuted)
		}
	})
}

func (s *store) discoverSection(c *ui.Context, t *ui.Theme, title, blurb string, icon *ui.SVG, apps []App, groupID string) {
	if len(apps) == 0 {
		return
	}
	msgs := s.messages()
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Icon(c, icon).Size(17, 17).TextColor(t.Text)
			ui.Text(c, title).FontSize(17).Bold()
			ui.Text(c, blurb).FontSize(11.5).TextColor(t.TextMuted).SingleLine().Grow(1).MinWidth(0)
			if groupID != "" {
				see := ui.ButtonBase(c).Label(msgs.SeeAllNamed(title)).Gap(1).Padding(2, 0).
					Cursor(ui.CursorPointer).Shrink(0)
				see.Children(func() {
					ui.Text(c, msgs.SeeAll()).FontSize(12).TextColor(t.Accent)
					ui.Icon(c, iconChevron).Size(10, 10).TextColor(t.Accent)
				})
				if see.Clicked() {
					s.selectNav("group:" + groupID)
				}
			}
		})

		cols := s.columns(c)
		ui.Grid(c).Columns(cols).GapX(18).GapY(1).Children(func() {
			for i := range apps {
				s.appRow(c, &apps[i], 76)
			}
		})
	})
}

// columns returns how many app rows fit side by side in the content area.
func (s *store) columns(c *ui.Context) int {
	width, _ := c.Size()
	width -= sidebarWidth + 64
	switch {
	case width >= 860:
		return 3
	case width >= 560:
		return 2
	default:
		return 1
	}
}

func (s *store) topApps(limit int) []App {
	apps := make([]App, len(s.catalog.Apps))
	copy(apps, s.catalog.Apps)
	sortApps(apps, s.sort)
	if len(apps) > limit {
		apps = apps[:limit]
	}
	return apps
}

// appRow is one app in a shelf or a grid: icon, name, summary and the
// action pill that the whole row leads to.
func (s *store) appRow(c *ui.Context, app *App, height float32) ui.Element {
	t := c.Theme()
	msgs := s.messages()
	row := ui.ButtonBase(c).FillWidth().Height(height).Padding(7, 9).Radius(9).
		Gap(11).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Label(msgs.AppRowLabel(app.Name, app.Summary))
	row.OnClick(func() { s.openDetail(app.Slug) })
	row.Children(func() {
		s.appIcon(c, app, 44)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, app.Name).FontSize(14).Bold().SingleLine().Grow(1).MinWidth(0)
				ui.Text(c, app.kindShort(msgs)).FontSize(9).TextColor(t.TextMuted).
					Padding(1, 4).Radius(3).Background(t.Text.Alpha(0.06)).Shrink(0)
			})
			ui.Text(c, app.Summary).FontSize(12).TextColor(t.TextMuted).MaxLines(2).LineHeight(1.35)
		})
		s.cardAction(c, t, app)
	})
	return row
}

// cardAction is the Install/Open-like pill in a card. The card itself opens
// the detail drawer, where the real action is.
func (s *store) cardAction(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
	label := msgs.Get()
	fg := t.Accent
	bg := t.Accent.Alpha(0.13)
	switch state := s.acquireState(app.Slug); state.Phase {
	case phaseSearching:
		label = msgs.SearchingShort()
	case phaseDownloading:
		if state.Progress >= 0 {
			label = fmt.Sprintf("%d%%", int(state.Progress*100))
		} else {
			label = msgs.DownloadingShort()
		}
	case phaseReady:
		label = msgs.Downloaded()
		fg = t.TextMuted
		bg = t.Text.Alpha(0.07)
	case phaseFailed:
		label = msgs.Retry()
	case phaseNoAsset:
		label = msgs.OpenWebsite()
		fg = t.TextMuted
		bg = t.Text.Alpha(0.07)
	default:
		if _, ok := s.installedApp(app.Slug); ok {
			label = msgs.Installed()
			fg = t.TextMuted
			bg = t.Text.Alpha(0.07)
		} else if s.prefs.Acquired[app.Slug] {
			label = msgs.Acquired()
			fg = t.TextMuted
			bg = t.Text.Alpha(0.07)
		}
	}
	ui.Box(c).Height(23).Padding(0, 8).Radius(6).Center().Shrink(0).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(11).Bold().TextColor(fg).SingleLine()
	})
}
