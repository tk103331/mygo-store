package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"mygo-store/locales"
)

// acquirePhase is where an app's installer is in its acquisition.
type acquirePhase int

const (
	phaseIdle acquirePhase = iota
	phaseSearching
	phaseDownloading
	phaseReady
	phaseFailed
	phaseNoAsset
)

// acquireState is what the app knows about one app's installer, shared by
// the view, which reads it, and the download goroutine, which writes it.
type acquireState struct {
	Phase    acquirePhase
	Progress float64 // 0..1, or -1 while the length is unknown
	Path     string  // the downloaded installer
	Asset    string  // the release asset's file name
	Tag      string  // the release tag
	Source   string  // the release page
	Reason   string  // why a phase has nothing more to do, for the view
	Detail   string  // the error behind a failure, for the view
}

// The reasons the view says in the user's language.
const (
	reasonNoReleases     = "no-releases"
	reasonNoReleased     = "no-released-release"
	reasonNoAsset        = "no-asset"
	reasonRateLimited    = "rate-limited"
	reasonDownloadFailed = "download-failed"
)

// release is the part of the GitHub Releases API the app uses.
type release struct {
	TagName    string         `json:"tag_name"`
	Name       string         `json:"name"`
	HTMLURL    string         `json:"html_url"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

var (
	errNoReleases  = errors.New("this repository has no GitHub releases")
	errRateLimited = errors.New("the GitHub API rate limit is exhausted")

	// These are variables so tests can point them at an httptest server.
	githubAPIBase     = "https://api.github.com"
	releaseHTTPClient = &http.Client{Timeout: 45 * time.Second}
	downloadClient    = &http.Client{Timeout: 30 * time.Minute}
)

const releasePageSize = 15

// releaseManager keeps one acquireState per app and runs at most two
// downloads at a time.
type releaseManager struct {
	mu       sync.Mutex
	states   map[string]acquireState
	running  map[string]bool
	sem      chan struct{}
	onChange func(slug string, state acquireState)
}

func newReleaseManager(onChange func(string, acquireState)) *releaseManager {
	return &releaseManager{
		states:   map[string]acquireState{},
		running:  map[string]bool{},
		sem:      make(chan struct{}, 2),
		onChange: onChange,
	}
}

func (m *releaseManager) state(slug string) acquireState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[slug]
}

// seed marks an installer that an earlier session downloaded as ready.
func (m *releaseManager) seed(slug, path string) {
	m.mu.Lock()
	m.states[slug] = acquireState{Phase: phaseReady, Progress: 1, Path: path}
	m.mu.Unlock()
}

func (m *releaseManager) begin(slug string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running[slug] {
		return false
	}
	m.running[slug] = true
	return true
}

func (m *releaseManager) end(slug string) {
	m.mu.Lock()
	delete(m.running, slug)
	m.mu.Unlock()
}

func (m *releaseManager) reset(slug string) {
	m.mu.Lock()
	delete(m.states, slug)
	m.mu.Unlock()
}

func (m *releaseManager) set(slug string, state acquireState) {
	m.mu.Lock()
	m.states[slug] = state
	m.mu.Unlock()
	if m.onChange != nil {
		m.onChange(slug, state)
	}
}

// readyPath returns the downloaded installer if it is still on disk.
func (m *releaseManager) readyPath(slug string) (string, bool) {
	state := m.state(slug)
	if state.Phase != phaseReady || state.Path == "" {
		return "", false
	}
	info, err := os.Stat(state.Path)
	if err != nil || info.IsDir() {
		m.reset(slug)
		return "", false
	}
	return state.Path, true
}

// startAcquire looks the app up in GitHub Releases and downloads the first
// asset that matches the running system.
func (s *store) startAcquire(app *App) {
	m := s.releases()
	if !m.begin(app.Slug) {
		return
	}
	go func() {
		defer m.end(app.Slug)

		m.set(app.Slug, acquireState{Phase: phaseSearching, Progress: -1})
		releases, err := fetchReleases(app.Repo)
		if err != nil {
			if errors.Is(err, errNoReleases) {
				m.set(app.Slug, acquireState{
					Phase:    phaseNoAsset,
					Progress: -1,
					Reason:   reasonNoReleases,
				})
				return
			}
			reason := ""
			if errors.Is(err, errRateLimited) {
				reason = reasonRateLimited
			}
			m.set(app.Slug, acquireState{Phase: phaseFailed, Progress: -1, Reason: reason})
			return
		}
		if len(releases) == 0 {
			m.set(app.Slug, acquireState{
				Phase:    phaseNoAsset,
				Progress: -1,
				Reason:   reasonNoReleased,
			})
			return
		}

		rel, asset, ok := pickRelease(releases, runtime.GOOS, runtime.GOARCH)
		if !ok {
			m.set(app.Slug, acquireState{
				Phase:    phaseNoAsset,
				Progress: -1,
				Reason:   reasonNoAsset,
			})
			return
		}

		base := acquireState{
			Phase:    phaseDownloading,
			Progress: -1,
			Asset:    asset.Name,
			Tag:      rel.TagName,
			Source:   rel.HTMLURL,
		}
		m.set(app.Slug, base)
		path, err := m.download(app.Slug, base, asset)
		if err != nil {
			m.set(app.Slug, acquireState{
				Phase:    phaseFailed,
				Progress: -1,
				Asset:    asset.Name,
				Tag:      rel.TagName,
				Source:   rel.HTMLURL,
				Reason:   reasonDownloadFailed,
				Detail:   err.Error(),
			})
			return
		}
		m.set(app.Slug, acquireState{
			Phase:    phaseReady,
			Progress: 1,
			Path:     path,
			Asset:    asset.Name,
			Tag:      rel.TagName,
			Source:   rel.HTMLURL,
		})
	}()
}

// download writes the asset under the user's Downloads folder, showing its
// progress as it goes. A .part file keeps a failed transfer from looking
// like an installer.
func (m *releaseManager) download(slug string, base acquireState, asset releaseAsset) (string, error) {
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	dir := downloadsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := safeFileName(asset.Name)
	if name == "" {
		name = slug + filepath.Ext(asset.Name)
	}
	target, err := uniquePath(dir, name)
	if err != nil {
		return "", err
	}
	temp := target + ".part"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent())
	resp, err := downloadClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("server returned %s", resp.Status)
	}

	file, err := os.Create(temp)
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		file.Close()
		if !ok {
			_ = os.Remove(temp)
		}
	}()

	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}
	state := base
	state.Phase = phaseDownloading
	buf := make([]byte, 64*1024)
	var done int64
	lastReport := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return "", writeErr
			}
			done += int64(n)
			if time.Since(lastReport) >= 100*time.Millisecond {
				lastReport = time.Now()
				state.Progress = ratio(done, total)
				m.set(slug, state)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temp, target); err != nil {
		return "", err
	}
	ok = true
	return target, nil
}

func ratio(done, total int64) float64 {
	if total <= 0 {
		return -1
	}
	p := float64(done) / float64(total)
	if p > 1 {
		return 1
	}
	return p
}

func userAgent() string {
	version := mygo.App.Version()
	if version == "" {
		version = "0.1.0"
	}
	return "MyGo-Store/" + version
}

// downloadsDir is where installers are saved: the user's Downloads folder,
// under a folder of the app's own.
func downloadsDir() string {
	if dir := os.Getenv("MYGO_STORE_DOWNLOADS"); dir != "" {
		return dir
	}
	if dir, err := mygo.App.Path(mygo.PathDownloads); err == nil && dir != "" {
		return filepath.Join(dir, "MyGo Store")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Downloads", "MyGo Store")
	}
	return filepath.Join(os.TempDir(), "MyGo Store")
}

// safeFileName keeps a release asset's name from escaping the download
// folder or asking for a path the file system rejects.
func safeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '-'
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == ".." {
		return ""
	}
	if len(name) > 180 {
		stem, ext := splitExt(name)
		if len(ext) > 20 {
			ext = ""
		}
		stem = stem[:min(len(stem), 180-len(ext))]
		name = stem + ext
	}
	return name
}

// uniquePath appends " (1)", " (2)"… when a file of the same name is
// already in the folder.
func uniquePath(dir, name string) (string, error) {
	target := filepath.Join(dir, name)
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return target, nil
	}
	stem, ext := splitExt(name)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no available file name: %s", name)
}

func splitExt(name string) (string, string) {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tar.zst"} {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)], name[len(name)-len(ext):]
		}
	}
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext), ext
}

// repoPath turns catalog values such as "owner/name",
// "https://github.com/owner/name" or "github.com/owner/name.git" into the
// "owner/name" the API wants.
func repoPath(repo string) (string, bool) {
	repo = strings.TrimSpace(repo)
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "github.com/"} {
		repo = strings.TrimPrefix(repo, prefix)
	}
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.Trim(repo, "/")
	parts := strings.Split(repo, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

func fetchReleases(repo string) ([]release, error) {
	path, ok := repoPath(repo)
	if !ok {
		return nil, errNoReleases
	}
	endpoint := fmt.Sprintf("%s/repos/%s/releases?per_page=%d",
		strings.TrimRight(githubAPIBase, "/"), path, releasePageSize)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent())
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, errNoReleases
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, errRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	var releases []release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

// pickRelease returns the newest stable release with an installer that
// suits the running system.
func pickRelease(releases []release, goos, goarch string) (release, releaseAsset, bool) {
	for _, rel := range releases {
		if rel.Draft || rel.Prerelease {
			continue
		}
		if asset, ok := pickAsset(rel.Assets, goos, goarch); ok {
			return rel, asset, true
		}
	}
	return release{}, releaseAsset{}, false
}

func pickAsset(assets []releaseAsset, goos, goarch string) (releaseAsset, bool) {
	best := -1
	var chosen releaseAsset
	for _, asset := range assets {
		score := assetScore(asset.Name, goos, goarch)
		if score > best {
			best = score
			chosen = asset
		}
	}
	return chosen, best > 0
}

// assetScore ranks how well an asset's name fits the system. Zero rejects
// it: another platform, another architecture, a checksum or a source file.
func assetScore(name, goos, goarch string) int {
	n := strings.ToLower(name)
	if n == "" {
		return 0
	}
	if isAuxiliaryAsset(n) {
		return 0
	}
	if os := detectAssetOS(n); os != "" && os != goos {
		return 0
	}
	score := extensionScore(n, goos)
	if score == 0 {
		return 0
	}
	switch arch := detectAssetArch(n); arch {
	case "universal":
		score += 8
	case "":
		// Unmarked installers are often universal.
	case goarch:
		score += 16
	default:
		return 0
	}
	if detectAssetOS(n) == goos {
		score += 6
	}
	return score
}

func isAuxiliaryAsset(name string) bool {
	for _, ext := range []string{
		".sha1", ".sha256", ".sha512", ".md5", ".asc", ".sig", ".pem", ".crt",
		".sbom", ".spdx", ".json", ".yml", ".yaml", ".txt", ".blockmap",
		".pdb", ".dsym", ".nupkg",
	} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	for _, word := range []string{
		"checksum", "sha256sum", "sha512sum", "source code", "source-code",
		"symbols", "debug", "sbom", "unsigned-hash",
	} {
		if strings.Contains(name, word) {
			return true
		}
	}
	return false
}

func detectAssetOS(name string) string {
	switch {
	case containsAny(name,
		"darwin", "macos", "mac-os", "mac_os", "osx", "os-x", "apple", "macintel",
		"-mac", "_mac", ".mac", "mac.", ".dmg", ".pkg"):
		return "darwin"
	case containsAny(name,
		"windows", "win32", "win64", "win-", "win_", "win.", ".exe", ".msi", ".msix", ".appx"):
		return "windows"
	case containsAny(name,
		"linux", "appimage", ".deb", ".rpm", ".flatpak", ".snap"):
		return "linux"
	}
	return ""
}

func detectAssetArch(name string) string {
	switch {
	case containsAny(name, "universal", "universal2", "all-arch", "all_arch", "anycpu", "noarch"):
		return "universal"
	case containsAny(name, "aarch64", "arm64", "armv8", "arm-64", "arm_64", "apple-silicon", "apple_silicon", "silicon"):
		return "arm64"
	case containsAny(name, "x86_64", "x86-64", "amd64", "x64", "win64", "intel64", "intel"):
		return "amd64"
	case containsAny(name, "i386", "i686", "x86-32", "x86_32", "ia32", "win32"):
		return "386"
	case containsAny(name, "armv7", "armhf", "armv6"):
		return "arm"
	}
	return ""
}

func extensionScore(name, goos string) int {
	switch goos {
	// The format outweighs the architecture and OS hints below, so a
	// universal .dmg still beats an architecture-specific .pkg.
	case "darwin":
		switch {
		case strings.HasSuffix(name, ".dmg"):
			return 100
		case strings.HasSuffix(name, ".pkg"):
			return 70
		case strings.HasSuffix(name, ".zip"):
			return 35
		case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"),
			strings.HasSuffix(name, ".tar.xz"), strings.HasSuffix(name, ".tar.bz2"):
			return 28
		}
	case "windows":
		switch {
		case strings.HasSuffix(name, ".exe"):
			return 100
		case strings.HasSuffix(name, ".msi"):
			return 70
		case strings.HasSuffix(name, ".msix"), strings.HasSuffix(name, ".appx"):
			return 60
		case strings.HasSuffix(name, ".zip"):
			return 35
		case strings.HasSuffix(name, ".7z"):
			return 30
		}
	case "linux":
		switch {
		case strings.HasSuffix(name, ".appimage"):
			return 100
		case strings.HasSuffix(name, ".deb"):
			return 65
		case strings.HasSuffix(name, ".rpm"):
			return 55
		case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"),
			strings.HasSuffix(name, ".tar.xz"), strings.HasSuffix(name, ".tar.bz2"):
			return 38
		case strings.HasSuffix(name, ".zip"):
			return 30
		}
	}
	return 0
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func platformTitle(msgs locales.Messages, goos, goarch string) string {
	os := map[string]string{"darwin": "macOS", "windows": "Windows", "linux": "Linux"}[goos]
	if os == "" {
		os = goos
	}
	arch := map[string]string{
		"arm64": msgs.ArchAppleSilicon(),
		"amd64": msgs.ArchIntel(),
		"386":   msgs.ArchX8632(),
		"arm":   msgs.ArchArm(),
	}[goarch]
	if arch == "" {
		arch = goarch
	}
	return msgs.PlatformWithArch(os, arch)
}
