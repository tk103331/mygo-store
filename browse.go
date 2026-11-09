package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"mygo-store/locales"
)

// browseView lists the apps of a category, the favorites, the acquired
// ones, or a search, as a grid that builds only the rows in view.
func (s *store) browseView(c *ui.Context, t *ui.Theme) {
	apps := s.filterApps()
	title, subtitle := s.browseHeading(apps)

	ui.Column(c).Grow(1).Children(func() {
		ui.Box(c).Padding(22, 32, 6).Shrink(0).Children(func() {
			s.pageHeader(c, t, title, subtitle)
			s.filterBar(c, t, len(apps))
		})

		if len(apps) == 0 {
			s.emptyState(c, t)
			return
		}

		ui.Box(c).Grow(1).Padding(8, 24, 18, 24).Children(func() {
			ui.GridView(c, &s.grid, len(apps), 290, 84, func(i int) {
				s.appRow(c, &apps[i], 78)
			}).Grow(1)
		})
	})
}

func (s *store) browseHeading(apps []App) (string, string) {
	msgs := s.messages()
	query := strings.TrimSpace(s.query)
	if query != "" {
		return msgs.SearchTitle(), msgs.SearchResults(query, len(apps))
	}
	switch s.nav {
	case "favorites":
		return msgs.FavoritesTitle(), msgs.FavoritesSubtitle(len(apps))
	case "acquired":
		return msgs.AcquiredTitle(), msgs.AcquiredSubtitle(len(apps))
	}
	if strings.HasPrefix(s.nav, "group:") {
		group := strings.TrimPrefix(s.nav, "group:")
		return groupLabel(s.locale, group),
			msgs.GroupSubtitle(groupBlurb(s.locale, group), len(apps))
	}
	return msgs.AppsTitle(), msgs.AppsSubtitle(len(apps))
}

func (s *store) filterBar(c *ui.Context, t *ui.Theme, count int) {
	msgs := s.messages()
	ui.Row(c).Gap(7).AlignItems(ui.Center).Padding(10, 0, 8).Children(func() {
		s.filterChip(c, t, msgs.FilterAll(), s.kind == "", func() {
			s.kind = ""
			s.resetBrowsing()
		})
		s.filterChip(c, t, msgs.FilterNative(), s.kind == "native", func() {
			s.kind = "native"
			s.resetBrowsing()
		})
		s.filterChip(c, t, msgs.FilterWeb(), s.kind == "web", func() {
			s.kind = "web"
			s.resetBrowsing()
		})

		ui.Box(c).Width(1).Height(16).Background(t.Border).MarginX(3)

		platform := s.platform
		sel := ui.SelectBase(c, &s.platform)
		sel.Trigger.Gap(6).Padding(2, 9).Radius(7).Height(26).Border(1, t.Border).
			FontSize(11.5).Label(msgs.PlatformLabel())
		sel.Trigger.Children(func() {
			ui.Text(c, platformLabel(s.platform, msgs)).FontSize(11.5)
			ui.Icon(c, iconChevron).Size(9, 9).TextColor(t.TextMuted)
		})
		sel.Popup(func(panel ui.Element) {
			panel.Margin(4, 0, 0, 0).Padding(4).Radius(8).Background(t.Background).
				Border(1, t.Border)
			for _, platform := range []string{"", "mac", "win", "linux"} {
				item := sel.Item(platform).Padding(5, 8).Radius(5)
				if item.Highlighted() {
					item.Background(t.Accent).TextColor(t.AccentText)
				}
				item.Children(func() {
					ui.Text(c, platformLabel(platform, msgs)).FontSize(11.5)
				})
			}
		})
		if s.platform != platform {
			s.resetBrowsing()
		}

		ui.Spacer(c)
		ui.Text(c, msgs.AppCount(count)).FontSize(11.5).TextColor(t.TextMuted)
	})
}

func (s *store) filterChip(c *ui.Context, t *ui.Theme, label string, chosen bool, action func()) {
	chip := ui.ButtonBase(c).Height(26).Padding(0, 10).Radius(7).Label(label).Cursor(ui.CursorPointer)
	if chosen {
		chip.Background(t.Accent)
	} else {
		chip.Background(t.Surface)
		chip.Border(1, t.Border)
	}
	chip.Children(func() {
		ui.Text(c, label).FontSize(11.5).Bold()
	})
	if chip.Clicked() {
		action()
	}
}

func (s *store) emptyState(c *ui.Context, t *ui.Theme) {
	msgs := s.messages()
	icon := iconSparkle
	title := msgs.EmptyTitle()
	message := msgs.EmptyMessage()
	switch {
	case strings.TrimSpace(s.query) != "":
		icon = iconDev
		title = msgs.NoMatchesTitle()
		message = msgs.NoMatchesMessage(strings.TrimSpace(s.query))
	case s.nav == "favorites":
		icon = iconHeart
		title = msgs.NoFavoritesTitle()
		message = msgs.NoFavoritesMessage()
	case s.nav == "acquired":
		icon = iconDownload
		title = msgs.NoAcquiredTitle()
		message = msgs.NoAcquiredMessage()
	}
	ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
		ui.Icon(c, icon).Size(34, 34).TextColor(t.TextMuted).Opacity(0.5)
		ui.Text(c, title).FontSize(17).Bold()
		ui.Text(c, message).FontSize(12.5).TextColor(t.TextMuted)
		if s.kind != "" || s.platform != "" {
			if ui.Button(c, msgs.ClearFilters()).Clicked() {
				s.kind = ""
				s.platform = ""
				s.resetBrowsing()
			}
		}
	})
}

// platformLabel names a platform filter's choice: "" is all platforms and
// the others keep the vendors' names.
func platformLabel(platform string, msgs locales.Messages) string {
	switch platform {
	case "mac":
		return "macOS"
	case "win":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return msgs.AllPlatforms()
	}
}
