//go:build !windows && !darwin && !linux

package main

// platformInstalledApps lists no apps on a system the app cannot read the
// installed apps of.
func platformInstalledApps(func(string) bool) ([]installedApp, error) {
	return nil, nil
}
