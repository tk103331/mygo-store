package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// detailView shows one app: its icon, actions, screenshot and facts.
func (s *store) detailView(c *ui.Context, t *ui.Theme, app *App) {
	ui.Scroll(c).Key("detail:" + app.Slug).Grow(1).Children(func() {
		ui.Column(c).Padding(14, 32, 46).Gap(22).MaxWidth(1000).Margin(0, ui.Auto).Children(func() {
			s.backRow(c, t, app)

			ui.Row(c).Gap(20).AlignItems(ui.Start).Children(func() {
				icon := s.appIcon(c, app, 92)
				icon.Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, app.Name).FontSize(30).Bold().LetterSpacing(-0.4)
					ui.Text(c, "by "+app.Author).FontSize(13).TextColor(t.TextMuted)
					s.tagRow(c, t, app)
					s.scoreRow(c, t, app)
				})
			})

			s.actions(c, t, app)
			s.releaseStatus(c, t, app)

			s.shot(c, t, app)

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "关于").FontSize(17).Bold()
				ui.Text(c, app.Description).FontSize(14).LineHeight(1.65).MaxWidth(780)
			})

			s.info(c, t, app)

			if others := s.sameAuthor(app); len(others) > 0 {
				ui.Column(c).Gap(6).Children(func() {
					ui.Textf(c, "%s 的其他作品", app.Author).FontSize(17).Bold()
					ui.Grid(c).Columns(s.columns(c)).GapX(18).GapY(1).Children(func() {
						for i := range others {
							s.appRow(c, &others[i], 76)
						}
					})
				})
			}

			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconInfo).Size(12, 12).TextColor(t.TextMuted)
				ui.Text(c, "资料来自 Awesome MyGo；获取会识别当前系统，优先下载匹配的 GitHub Release 安装包。").
					FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func (s *store) backRow(c *ui.Context, t *ui.Theme, app *App) {
	ui.Row(c).AlignItems(ui.Center).Children(func() {
		back := ui.ButtonBase(c).Gap(2).Padding(2, 0).Label("返回").Cursor(ui.CursorPointer).Shrink(0)
		back.Children(func() {
			ui.Icon(c, iconBack).Size(12, 12).TextColor(t.Accent)
			ui.Text(c, "返回").FontSize(12).TextColor(t.Accent)
		})
		if back.Clicked() {
			s.detail = ""
		}
		ui.Spacer(c)
		ui.Text(c, app.categoryLabel()).FontSize(11.5).TextColor(t.TextMuted)
		ui.Icon(c, iconChevron).Size(9, 9).TextColor(t.TextMuted)
		ui.Text(c, app.Name).FontSize(11.5).TextColor(t.TextMuted)
	})
}

func (s *store) tagRow(c *ui.Context, t *ui.Theme, app *App) {
	ui.Row(c).Gap(5).Wrap().Children(func() {
		s.tag(c, t, app.categoryLabel(), groupByID(app.Group).Icon)
		s.tag(c, t, app.kindLabel(), nil)
		for _, platform := range app.platformLabels() {
			s.tag(c, t, platform, nil)
		}
	})
}

func (s *store) tag(c *ui.Context, t *ui.Theme, text string, icon *ui.SVG) {
	ui.Row(c).Height(22).Padding(0, 8).Radius(6).Gap(4).AlignItems(ui.Center).
		Background(t.Surface).Border(1, t.Border).Shrink(0).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(11, 11).TextColor(t.TextMuted)
		}
		ui.Text(c, text).FontSize(10.5).TextColor(t.TextMuted)
	})
}

func (s *store) scoreRow(c *ui.Context, t *ui.Theme, app *App) {
	color := scoreTier(c, app.Score)
	ui.Row(c).Gap(5).AlignItems(ui.Center).Padding(2, 0).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 6).Radius(5).
			Background(color.Alpha(0.12)).Shrink(0).Children(func() {
			ui.Icon(c, iconStar).Size(10, 10).TextColor(color)
			ui.Textf(c, "%.1f", app.Score).FontSize(11.5).Bold().TextColor(color)
		})
		ui.Text(c, "评分 / 100").FontSize(10.5).TextColor(t.TextMuted)
		if s.prefs.Acquired[app.Slug] {
			ui.Text(c, "·").FontSize(10.5).TextColor(t.TextMuted)
			ui.Text(c, "已获取").FontSize(10.5).TextColor(t.Accent)
		}
	})
}

func (s *store) actions(c *ui.Context, t *ui.Theme, app *App) {
	favorite := s.prefs.Favorites[app.Slug]
	state := s.acquireState(app.Slug)
	label, icon, busy := s.primaryAction(app, state)

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		primary := ui.PrimaryButton(c, "").Height(30).Padding(0, 14).Radius(7).Gap(6).
			Label(label).Disabled(busy)
		primary.Children(func() {
			ui.Icon(c, icon).Size(13, 13)
			ui.Text(c, label).FontSize(12).Bold()
		})
		if primary.Clicked() && !busy {
			s.acquire(app)
		}

		if app.Repo != "" {
			if ui.Button(c, "").Height(30).Padding(0, 12).Radius(7).Gap(6).
				Label("查看源码").Tooltip("github.com/" + app.Repo).Children(func() {
				ui.Icon(c, iconSource).Size(13, 13)
				ui.Text(c, "源码").FontSize(12)
			}).Clicked() {
				openExternal("https://github.com/" + app.Repo)
			}
		}

		fav := ui.Button(c, "").Label(moreIf(favorite, "取消收藏应用", "收藏应用")).Tooltip("收藏").
			Width(34).Height(30).Radius(7).Center().Shrink(0)
		if favorite {
			fav.Background(t.Accent.Alpha(0.12))
		}
		fav.Children(func() {
			if favorite {
				ui.Icon(c, iconHeartFill).Size(14, 14).TextColor(t.Accent)
			} else {
				ui.Icon(c, iconHeart).Size(14, 14).TextColor(t.TextMuted)
			}
		})
		if fav.Clicked() {
			s.toggleFavorite(app.Slug)
		}

		ui.Spacer(c)
		ui.Text(c, s.actionHint(app, state)).
			FontSize(10.5).TextColor(t.TextMuted).SingleLine()
	})
}

// primaryAction names the 获取 button for an app's installer state.
func (s *store) primaryAction(app *App, state acquireState) (label string, icon *ui.SVG, busy bool) {
	switch state.Phase {
	case phaseSearching:
		return "查找安装包…", iconDownload, true
	case phaseDownloading:
		if state.Progress >= 0 {
			return fmt.Sprintf("下载中 %d%%", int(state.Progress*100)), iconDownload, true
		}
		return "下载中…", iconDownload, true
	case phaseReady:
		return "打开安装包", iconOpen, false
	case phaseFailed:
		return "重试获取", iconDownload, false
	case phaseNoAsset:
		return "打开官网", iconOpen, false
	}
	if s.prefs.Acquired[app.Slug] {
		if app.Repo != "" {
			return "获取安装包", iconDownload, false
		}
		return "打开网站", iconOpen, false
	}
	return "获取", iconDownload, false
}

func (s *store) actionHint(app *App, state acquireState) string {
	switch state.Phase {
	case phaseSearching:
		return "正在读取 GitHub Releases…"
	case phaseDownloading:
		return "下载完成后会保存到「下载/MyGo Store」。"
	case phaseReady:
		return "安装包已保存，点击后会交给系统打开。"
	case phaseFailed:
		return "可以重试，或直接打开应用的官方页面。"
	case phaseNoAsset:
		return "没有匹配当前系统的安装包，已打开官方页面。"
	}
	if app.Repo != "" {
		return "自动识别系统，从 GitHub Releases 下载匹配的安装包。"
	}
	return "这个应用没有公开仓库，获取会打开它的官方页面。"
}

// releaseStatus shows the progress, the downloaded file and the fallback
// under the action row.
func (s *store) releaseStatus(c *ui.Context, t *ui.Theme, app *App) {
	state := s.acquireState(app.Slug)
	if state.Phase == phaseIdle {
		return
	}
	ui.Row(c).Gap(9).AlignItems(ui.Center).Padding(1, 0).Children(func() {
		switch state.Phase {
		case phaseSearching:
			ui.Spinner(c).Size(14, 14).Shrink(0)
			ui.Text(c, state.Message).FontSize(11.5).TextColor(t.TextMuted).SingleLine()

		case phaseDownloading:
			ui.Box(c).Width(170).Shrink(0).Children(func() {
				ui.Progress(c, state.Progress).Label("下载进度")
			})
			ui.Text(c, downloadLabel(state)).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)

		case phaseReady:
			ui.Icon(c, iconCheck).Size(15, 15).TextColor(t.Accent).Shrink(0)
			ui.Textf(c, "已下载 %s", state.Asset).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)
			if state.Tag != "" && state.Source != "" {
				ui.Link(c, state.Tag, state.Source).FontSize(11)
			}
			if ui.Button(c, "在 Finder 中显示").Height(24).Padding(0, 8).Radius(6).
				FontSize(11).Tooltip(state.Path).Clicked() {
				revealPath(state.Path)
			}

		case phaseFailed:
			ui.Icon(c, iconInfo).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, state.Message).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)
			if app.URL != "" {
				ui.Link(c, "打开官网", app.URL).FontSize(11)
			}

		case phaseNoAsset:
			ui.Icon(c, iconInfo).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, state.Message).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)
			if app.URL != "" {
				ui.Link(c, "打开官网", app.URL).FontSize(11)
			}
		}
	})
}

func downloadLabel(state acquireState) string {
	if state.Progress >= 0 {
		return fmt.Sprintf("正在下载 %s · %d%%", state.Asset, int(state.Progress*100))
	}
	return "正在下载 " + state.Asset
}

func moreIf(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func (s *store) shot(c *ui.Context, t *ui.Theme, app *App) {
	if bmp := s.shotBitmap(app); bmp != nil {
		ui.Box(c).FillWidth().AspectRatio(16.0/9).Radius(14).
			Border(1, t.Border).Clip().Shadow(0, 10, 28, 0, ui.RGBA(0, 0, 0, 0.16)).
			Background(t.Surface).Children(func() {
			ui.Image(c, bmp).Fill().Fit(ui.Cover)
		})
		return
	}
	from := ui.Oklch(0.34, 0.06, hue(app.Slug))
	to := ui.Oklch(0.22, 0.05, hue(app.Slug)+50)
	ui.Box(c).FillWidth().Height(190).Radius(14).
		Border(1, t.Border).Clip().Shadow(0, 10, 28, 0, ui.RGBA(0, 0, 0, 0.16)).
		LinearGradient(ui.LinearGradient{From: from, To: to, Angle: 135, Oklab: true}).
		Center().Children(func() {
		ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
			icon := s.appIcon(c, app, 54)
			icon.Opacity(0.9)
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, app.Name).FontSize(20).Bold().TextColor(ui.RGBA(255, 255, 255, 0.92))
				ui.Text(c, "截图暂缺").FontSize(11).TextColor(ui.RGBA(255, 255, 255, 0.55))
			})
		})
	})
}

func (s *store) info(c *ui.Context, t *ui.Theme, app *App) {
	license := app.License
	if license == "" {
		license = "未标注"
	}
	stars := app.starsText()
	if stars == "" {
		stars = "—"
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "信息").FontSize(17).Bold()
		ui.Grid(c).ColumnTracks(ui.Fr(1), ui.Fr(1)).GapX(40).GapY(8).Children(func() {
			s.infoText(c, t, "分类", app.categoryLabel())
			s.infoText(c, t, "界面", app.kindLabel())
			s.infoText(c, t, "平台", joinOr(app.platformLabels(), "未标注"))
			s.infoText(c, t, "作者", app.Author)
			s.infoText(c, t, "收录", app.addedDate())
			s.infoText(c, t, "评分", fmt.Sprintf("%.1f / 100", app.Score))
			if app.Repo != "" {
				s.infoLink(c, t, "源码", "github.com/"+app.Repo, "https://github.com/"+app.Repo)
			} else {
				s.infoText(c, t, "源码", "闭源")
			}
			s.infoText(c, t, "协议", license)
			s.infoText(c, t, "点赞", fmt.Sprint(app.Likes))
			s.infoText(c, t, "Star", stars)
		})
	})
}

func (s *store) infoText(c *ui.Context, t *ui.Theme, label, value string) {
	ui.Row(c).Gap(10).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(t.TextMuted).Width(58).Shrink(0)
		ui.Text(c, value).FontSize(12.5).SingleLine().Grow(1).MinWidth(0)
	})
}

func (s *store) infoLink(c *ui.Context, t *ui.Theme, label, text, url string) {
	ui.Row(c).Gap(10).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(t.TextMuted).Width(58).Shrink(0)
		ui.Link(c, text, url).FontSize(12.5).SingleLine()
	})
}

func joinOr(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}
	out := items[0]
	for _, item := range items[1:] {
		out += " · " + item
	}
	return out
}
