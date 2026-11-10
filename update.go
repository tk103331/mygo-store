package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"

	"mygo-store/locales"
)

// checkUpdates opens the updater's window. Tests replace it so that a
// headless click does not need the plugin.
var checkUpdates = updater.CheckForUpdates

// updaterPlugin checks for a new version shortly after launch and once a
// day while the app runs. Its window follows the app's language.
func updaterPlugin() mygo.Plugin {
	return native.New(updater.Options{})
}

// languageChoice is the language the user chose in the menu, "" while the
// app follows the system's languages. It only decides which item is
// checked.
var languageChoice = ""

const (
	menuApp            = "app"
	menuCheckUpdates   = "check-updates"
	menuLanguage       = "language"
	menuLanguageSystem = "language-system"
	menuLanguageEn     = "language-en"
	menuLanguageZh     = "language-zh"
)

// appMenu keeps the standard menus and adds the manual update check and
// the language choice. onLocale remembers a choice before the app applies
// it.
func appMenu(onLocale func(tag string)) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{ID: menuApp, Label: "MyGo Store", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			{ID: menuCheckUpdates, Label: "Check for Updates…", Click: func(*mygo.MenuItem, *mygo.Window) {
				checkUpdates()
			}},
			mygo.Separator(),
			{ID: menuLanguage, Label: "Language", Submenu: []*mygo.MenuItem{
				{ID: menuLanguageSystem, Label: "System", Type: mygo.MenuItemRadio, Checked: true,
					Click: chooseLocale("", onLocale)},
				{ID: menuLanguageEn, Label: i18n.Autonym("en"), Type: mygo.MenuItemRadio,
					Click: chooseLocale("en", onLocale)},
				{ID: menuLanguageZh, Label: i18n.Autonym("zh-Hans"), Type: mygo.MenuItemRadio,
					Click: chooseLocale("zh-Hans", onLocale)},
			}},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	})
}

// applyLocale makes the app show the language of tag, "" for the system's.
// It is a variable so that tests can record the choice without an app.
var applyLocale = mygo.App.SetLocale

// languageChoices are the languages the user may choose, with "" for the
// system's: the tag to set and the name the language calls itself.
func languageChoices(msgs locales.Messages) []struct{ tag, label string } {
	return []struct{ tag, label string }{
		{"", msgs.LanguageSystem()},
		{"en", i18n.Autonym("en")},
		{"zh-Hans", i18n.Autonym("zh-Hans")},
	}
}

// chooseLocale returns the click that makes the app show the language of
// tag, "" to follow the user's languages again.
func chooseLocale(tag string, onLocale func(string)) func(*mygo.MenuItem, *mygo.Window) {
	return func(*mygo.MenuItem, *mygo.Window) {
		languageChoice = tag
		onLocale(tag)
		applyLocale(tag)
	}
}

// relabelAppMenu says the app's own items in locale, as the standard items
// already say themselves in it, and checks the language in use.
func relabelAppMenu(menu *mygo.Menu, locale string) {
	msgs := locales.In(locale)
	if it := menu.ItemByID(menuApp); it != nil {
		it.SetLabel(msgs.AppTitle())
	}
	if it := menu.ItemByID(menuCheckUpdates); it != nil {
		it.SetLabel(msgs.CheckUpdatesMenu())
	}
	if it := menu.ItemByID(menuLanguage); it != nil {
		it.SetLabel(msgs.LanguageMenu())
	}
	if it := menu.ItemByID(menuLanguageSystem); it != nil {
		it.SetLabel(msgs.LanguageSystem())
	}
	checks := map[string]string{
		menuLanguageSystem: "",
		menuLanguageEn:     "en",
		menuLanguageZh:     "zh-Hans",
	}
	for id, tag := range checks {
		if it := menu.ItemByID(id); it != nil {
			it.SetChecked(languageChoice == tag)
		}
	}
}

// appVersion is the version linked into packaged builds, or "dev" when
// running from source.
func appVersion() string {
	if version := mygo.App.Version(); version != "" {
		return version
	}
	return "dev"
}
