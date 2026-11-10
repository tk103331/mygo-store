package main

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo/ui"

	"mygo-store/locales"
)

// detailView shows one app: its icon, actions, screenshot and facts.
func (s *store) detailView(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
	ui.Scroll(c).Key("detail:" + app.Slug).Grow(1).Children(func() {
		ui.Column(c).Padding(14, 32, 46).Gap(22).MaxWidth(1000).Margin(0, ui.Auto).Children(func() {
			s.backRow(c, t, app)

			ui.Row(c).Gap(20).AlignItems(ui.Start).Children(func() {
				icon := s.appIcon(c, app, 92)
				icon.Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, app.Name).FontSize(30).Bold().LetterSpacing(-0.4)
					ui.Text(c, msgs.ByAuthor(app.Author)).FontSize(13).TextColor(t.TextMuted)
					s.tagRow(c, t, app)
					s.scoreRow(c, t, app)
				})
			})

			s.actions(c, t, app)
			s.releaseStatus(c, t, app)

			s.shot(c, t, app)

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, msgs.About()).FontSize(17).Bold()
				ui.Text(c, app.Description).FontSize(14).LineHeight(1.65).MaxWidth(780)
			})

			s.info(c, t, app)

			if others := s.sameAuthor(app); len(others) > 0 {
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, msgs.OtherWorks(app.Author)).FontSize(17).Bold()
					ui.Grid(c).Columns(s.columns(c)).GapX(18).GapY(1).Children(func() {
						for i := range others {
							s.appRow(c, &others[i], 76)
						}
					})
				})
			}

			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconInfo).Size(12, 12).TextColor(t.TextMuted)
				ui.Text(c, msgs.DetailFooter()).
					FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func (s *store) backRow(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
	ui.Row(c).AlignItems(ui.Center).Children(func() {
		back := ui.ButtonBase(c).Gap(2).Padding(2, 0).Label(msgs.Back()).Cursor(ui.CursorPointer).Shrink(0)
		back.Children(func() {
			ui.Icon(c, iconBack).Size(12, 12).TextColor(t.Accent)
			ui.Text(c, msgs.Back()).FontSize(12).TextColor(t.Accent)
		})
		if back.Clicked() {
			s.detail = ""
		}
		ui.Spacer(c)
		ui.Text(c, app.categoryLabel(s.locale)).FontSize(11.5).TextColor(t.TextMuted)
		ui.Icon(c, iconChevron).Size(9, 9).TextColor(t.TextMuted)
		ui.Text(c, app.Name).FontSize(11.5).TextColor(t.TextMuted)
	})
}

func (s *store) tagRow(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
	ui.Row(c).Gap(5).Wrap().Children(func() {
		s.tag(c, t, app.categoryLabel(s.locale), groupByID(app.Group).Icon)
		s.tag(c, t, app.kindLabel(msgs), nil)
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
	msgs := s.messages()
	color := scoreTier(c, app.Score)
	ui.Row(c).Gap(5).AlignItems(ui.Center).Padding(2, 0).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 6).Radius(5).
			Background(color.Alpha(0.12)).Shrink(0).Children(func() {
			ui.Icon(c, iconStar).Size(10, 10).TextColor(color)
			ui.Textf(c, "%.1f", app.Score).FontSize(11.5).Bold().TextColor(color)
		})
		ui.Text(c, msgs.ScoreOutOf()).FontSize(10.5).TextColor(t.TextMuted)
		if _, ok := s.installedApp(app.Slug); ok {
			ui.Text(c, "·").FontSize(10.5).TextColor(t.TextMuted)
			ui.Text(c, msgs.Installed()).FontSize(10.5).TextColor(t.Accent)
		} else if s.prefs.Acquired[app.Slug] {
			ui.Text(c, "·").FontSize(10.5).TextColor(t.TextMuted)
			ui.Text(c, msgs.Acquired()).FontSize(10.5).TextColor(t.Accent)
		}
	})
}

func (s *store) actions(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
	favorite := s.prefs.Favorites[app.Slug]
	state := s.acquireState(app.Slug)
	label, icon, busy := s.primaryAction(app, state, msgs)

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		primary := ui.PrimaryButton(c, "").Height(30).Padding(0, 14).Radius(7).Gap(6).
			Label(label).Disabled(busy)
		primary.Children(func() {
			ui.Icon(c, icon).Size(13, 13)
			ui.Text(c, label).FontSize(12).Bold()
		})
		if primary.Clicked() && !busy {
			if state.Phase != phaseIdle || !s.openInstalled(app.Slug) {
				s.acquire(app)
			}
		}

		if _, ok := s.installedApp(app.Slug); ok && state.Phase == phaseIdle && app.Repo != "" {
			if ui.Button(c, "").Height(30).Padding(0, 12).Radius(7).Gap(6).
				Label(msgs.GetInstaller()).Tooltip(msgs.HintAutoDetect()).Children(func() {
				ui.Icon(c, iconDownload).Size(13, 13)
				ui.Text(c, msgs.GetInstaller()).FontSize(12)
			}).Clicked() {
				s.acquire(app)
			}
		}

		if app.Repo != "" {
			if ui.Button(c, "").Height(30).Padding(0, 12).Radius(7).Gap(6).
				Label(msgs.ViewSource()).Tooltip("github.com/" + app.Repo).Children(func() {
				ui.Icon(c, iconSource).Size(13, 13)
				ui.Text(c, msgs.Source()).FontSize(12)
			}).Clicked() {
				openExternal("https://github.com/" + app.Repo)
			}
		}

		fav := ui.Button(c, "").Label(moreIf(favorite, msgs.UnfavoriteApp(), msgs.FavoriteApp())).
			Tooltip(msgs.Favorite()).
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
		ui.Text(c, s.actionHint(app, state, msgs)).
			FontSize(10.5).TextColor(t.TextMuted).SingleLine()
	})
}

// primaryAction names the 获取 button for an app's installer state.
func (s *store) primaryAction(app *App, state acquireState, msgs locales.Messages) (label string, icon *ui.SVG, busy bool) {
	switch state.Phase {
	case phaseSearching:
		return msgs.FindInstaller(), iconDownload, true
	case phaseDownloading:
		if state.Progress >= 0 {
			return msgs.DownloadingPercent(int(state.Progress * 100)), iconDownload, true
		}
		return msgs.DownloadingInstaller(), iconDownload, true
	case phaseReady:
		return msgs.OpenInstaller(), iconOpen, false
	case phaseFailed:
		return msgs.RetryGet(), iconDownload, false
	case phaseNoAsset:
		return msgs.OpenWebsite(), iconOpen, false
	}
	if _, ok := s.installedApp(app.Slug); ok {
		return msgs.OpenApp(), iconOpen, false
	}
	if s.prefs.Acquired[app.Slug] {
		if app.Repo != "" {
			return msgs.GetInstaller(), iconDownload, false
		}
		return msgs.OpenSite(), iconOpen, false
	}
	return msgs.Get(), iconDownload, false
}

func (s *store) actionHint(app *App, state acquireState, msgs locales.Messages) string {
	switch state.Phase {
	case phaseSearching:
		return msgs.HintSearching()
	case phaseDownloading:
		return msgs.HintDownloading()
	case phaseReady:
		return msgs.HintReady()
	case phaseFailed:
		return msgs.HintFailed()
	case phaseNoAsset:
		return msgs.HintNoAsset()
	}
	if installed, ok := s.installedApp(app.Slug); ok {
		if installed.Version != "" {
			return msgs.InstalledVersion(installed.Version)
		}
		return msgs.Installed()
	}
	if app.Repo != "" {
		return msgs.HintAutoDetect()
	}
	return msgs.HintNoRepo()
}

// acquireMessage says an acquisition state in the language of msgs.
func (s *store) acquireMessage(msgs locales.Messages, state acquireState) string {
	switch state.Phase {
	case phaseSearching:
		return msgs.SearchingReleases()
	case phaseDownloading:
		return msgs.DownloadingInstallerMessage()
	case phaseReady:
		return msgs.InstallerDownloaded()
	case phaseFailed:
		switch state.Reason {
		case reasonRateLimited:
			return msgs.RateLimited()
		case reasonDownloadFailed:
			return msgs.DownloadFailed(state.Detail)
		}
		return msgs.LookupFailed()
	case phaseNoAsset:
		switch state.Reason {
		case reasonNoReleased:
			return msgs.NoPublishedReleases()
		case reasonNoAsset:
			return msgs.NoMatchingAsset(platformTitle(msgs, runtime.GOOS, runtime.GOARCH))
		}
		return msgs.NoReleases()
	}
	return ""
}

func (s *store) releaseStatus(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
	state := s.acquireState(app.Slug)
	if state.Phase == phaseIdle {
		return
	}
	ui.Row(c).Gap(9).AlignItems(ui.Center).Padding(1, 0).Children(func() {
		switch state.Phase {
		case phaseSearching:
			ui.Spinner(c).Size(14, 14).Shrink(0)
			ui.Text(c, s.acquireMessage(msgs, state)).FontSize(11.5).TextColor(t.TextMuted).SingleLine()

		case phaseDownloading:
			ui.Box(c).Width(170).Shrink(0).Children(func() {
				ui.Progress(c, state.Progress).Label(msgs.DownloadProgress())
			})
			ui.Text(c, downloadLabel(msgs, state)).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)

		case phaseReady:
			ui.Icon(c, iconCheck).Size(15, 15).TextColor(t.Accent).Shrink(0)
			ui.Text(c, msgs.DownloadedFile(state.Asset)).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)
			if state.Tag != "" && state.Source != "" {
				ui.Link(c, state.Tag, state.Source).FontSize(11)
			}
			if ui.Button(c, msgs.ShowInFolder()).Height(24).Padding(0, 8).Radius(6).
				FontSize(11).Tooltip(state.Path).Clicked() {
				revealPath(state.Path)
			}

		case phaseFailed:
			ui.Icon(c, iconInfo).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, s.acquireMessage(msgs, state)).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)
			if app.URL != "" {
				ui.Link(c, msgs.OpenWebsite(), app.URL).FontSize(11)
			}

		case phaseNoAsset:
			ui.Icon(c, iconInfo).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, s.acquireMessage(msgs, state)).FontSize(11.5).TextColor(t.TextMuted).
				SingleLine().Grow(1).MinWidth(0)
			if app.URL != "" {
				ui.Link(c, msgs.OpenWebsite(), app.URL).FontSize(11)
			}
		}
	})
}

func downloadLabel(msgs locales.Messages, state acquireState) string {
	if state.Progress >= 0 {
		return msgs.DownloadingFile(state.Asset, int(state.Progress*100))
	}
	return msgs.DownloadingFileIndeterminate(state.Asset)
}

func moreIf(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func (s *store) shot(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
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
				ui.Text(c, msgs.ScreenshotMissing()).FontSize(11).TextColor(ui.RGBA(255, 255, 255, 0.55))
			})
		})
	})
}

func (s *store) info(c *ui.Context, t *ui.Theme, app *App) {
	msgs := s.messages()
	license := app.License
	if license == "" {
		license = msgs.NotSpecified()
	}
	stars := app.starsText()
	if stars == "" {
		stars = "—"
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, msgs.Information()).FontSize(17).Bold()
		ui.Grid(c).ColumnTracks(ui.Fr(1), ui.Fr(1)).GapX(40).GapY(8).Children(func() {
			s.infoText(c, t, msgs.FactCategory(), app.categoryLabel(s.locale))
			s.infoText(c, t, msgs.FactKind(), app.kindLabel(msgs))
			s.infoText(c, t, msgs.FactPlatform(), joinOr(app.platformLabels(), msgs.NotSpecified()))
			s.infoText(c, t, msgs.FactAuthor(), app.Author)
			s.infoText(c, t, msgs.FactAdded(), app.addedDate())
			s.infoText(c, t, msgs.FactScore(), fmt.Sprintf("%.1f / 100", app.Score))
			if app.Repo != "" {
				s.infoLink(c, t, msgs.FactSource(), "github.com/"+app.Repo, "https://github.com/"+app.Repo)
			} else {
				s.infoText(c, t, msgs.FactSource(), msgs.ClosedSource())
			}
			s.infoText(c, t, msgs.FactLicense(), license)
			s.infoText(c, t, msgs.FactLikes(), fmt.Sprint(app.Likes))
			s.infoText(c, t, msgs.FactStars(), stars)
		})
	})
}

func (s *store) infoText(c *ui.Context, t *ui.Theme, label, value string) {
	ui.Row(c).Gap(10).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(t.TextMuted).Width(72).Shrink(0)
		ui.Text(c, value).FontSize(12.5).SingleLine().Grow(1).MinWidth(0)
	})
}

func (s *store) infoLink(c *ui.Context, t *ui.Theme, label, text, url string) {
	ui.Row(c).Gap(10).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(t.TextMuted).Width(72).Shrink(0)
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
