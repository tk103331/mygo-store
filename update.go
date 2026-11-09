package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
)

// checkUpdates opens the updater's window. Tests replace it so that a
// headless click does not need the plugin.
var checkUpdates = updater.CheckForUpdates

// updaterPlugin checks for a new version shortly after launch and once a
// day while the app runs. Its window follows the app's Chinese interface.
func updaterPlugin() mygo.Plugin {
	return native.New(updater.Options{Language: "zh-Hans"})
}

// appMenu keeps the standard menus and adds the manual update check.
func appMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "MyGo Store", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) {
				checkUpdates()
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

// appVersion is the version linked into packaged builds, or "dev" when
// running from source.
func appVersion() string {
	if version := mygo.App.Version(); version != "" {
		return version
	}
	return "dev"
}
