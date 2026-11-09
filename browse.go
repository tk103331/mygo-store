package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
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
	query := strings.TrimSpace(s.query)
	if query != "" {
		return "搜索", fmt.Sprintf("“%s” · %d 个结果", query, len(apps))
	}
	switch s.nav {
	case "favorites":
		return "收藏", fmt.Sprintf("%d 款应用留在了这里。", len(apps))
	case "acquired":
		return "已获取", fmt.Sprintf("从应用市场获取过的 %d 款应用。", len(apps))
	}
	if strings.HasPrefix(s.nav, "group:") {
		group := groupByID(strings.TrimPrefix(s.nav, "group:"))
		return group.Label, fmt.Sprintf("%s · %d 款", group.Blurb, len(apps))
	}
	return "应用", fmt.Sprintf("%d 款应用", len(apps))
}

func (s *store) filterBar(c *ui.Context, t *ui.Theme, count int) {
	ui.Row(c).Gap(7).AlignItems(ui.Center).Padding(10, 0, 8).Children(func() {
		s.filterChip(c, t, "全部", s.kind == "", func() {
			s.kind = ""
			s.resetBrowsing()
		})
		s.filterChip(c, t, "原生 UI", s.kind == "native", func() {
			s.kind = "native"
			s.resetBrowsing()
		})
		s.filterChip(c, t, "Web 前端", s.kind == "web", func() {
			s.kind = "web"
			s.resetBrowsing()
		})

		ui.Box(c).Width(1).Height(16).Background(t.Border).MarginX(3)

		platform := platformLabel(s.platform)
		options := []string{"全部平台", "macOS", "Windows", "Linux"}
		sel := ui.Select(c, &platform, options).Label("平台").Width(108).Height(26)
		if sel.Changed() {
			s.platform = platformID(platform)
			s.resetBrowsing()
		}

		ui.Spacer(c)
		ui.Textf(c, "%d 款", count).FontSize(11.5).TextColor(t.TextMuted)
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
	icon := iconSparkle
	title := "这里还什么都没有"
	message := "换一个分类，或清空搜索看看。"
	switch {
	case strings.TrimSpace(s.query) != "":
		icon = iconDev
		title = "没有找到匹配的应用"
		message = fmt.Sprintf("“%s” 没有匹配的名称、作者或分类。", strings.TrimSpace(s.query))
	case s.nav == "favorites":
		icon = iconHeart
		title = "还没有收藏"
		message = "在应用详情里点一下心形，就会留在这里。"
	case s.nav == "acquired":
		icon = iconDownload
		title = "还没有获取过应用"
		message = "打开一个应用，点“获取”就会出现在这里。"
	}
	ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
		ui.Icon(c, icon).Size(34, 34).TextColor(t.TextMuted).Opacity(0.5)
		ui.Text(c, title).FontSize(17).Bold()
		ui.Text(c, message).FontSize(12.5).TextColor(t.TextMuted)
		if s.kind != "" || s.platform != "" {
			if ui.Button(c, "清除筛选").Clicked() {
				s.kind = ""
				s.platform = ""
				s.resetBrowsing()
			}
		}
	})
}

func platformLabel(platform string) string {
	switch platform {
	case "mac":
		return "macOS"
	case "win":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return "全部平台"
	}
}

func platformID(label string) string {
	switch label {
	case "macOS":
		return "mac"
	case "Windows":
		return "win"
	case "Linux":
		return "linux"
	default:
		return ""
	}
}
