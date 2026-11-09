package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// catalogURL is the fixed release that the daily action refreshes. The
	// tag never moves, so this URL is stable across app releases.
	catalogURL = "https://github.com/tk103331/mygo-store/releases/download/catalog-latest/apps.json"

	catalogCheckDelay    = 10 * time.Second
	catalogCheckInterval = 6 * time.Hour
	maxCatalogSize       = 8 << 20

	catalogCacheName = "catalog.json"
	catalogETagName  = "catalog.etag"
)

// catalogSyncer keeps the catalog fresh while the app runs. The embedded
// snapshot appears at once; a newer one from the release replaces it after
// the first check, and every catalogCheckInterval after that.
type catalogSyncer struct {
	store    *store
	url      string
	client   *http.Client
	interval time.Duration
	disabled bool
	cacheDir string

	mu        sync.Mutex
	current   *Catalog
	etag      string
	checking  bool
	lastCheck time.Time
	lastErr   string
}

func newCatalogSyncer(s *store) *catalogSyncer {
	interval := catalogCheckInterval
	if text := strings.TrimSpace(os.Getenv("MYGO_STORE_CATALOG_INTERVAL")); text != "" {
		if parsed, err := time.ParseDuration(text); err == nil {
			interval = parsed
		}
	}
	disabled := interval <= 0
	if text := strings.TrimSpace(os.Getenv("MYGO_STORE_CATALOG_DISABLE")); text != "" &&
		text != "0" && !strings.EqualFold(text, "false") {
		disabled = true
	}
	url := catalogURL
	if text := strings.TrimSpace(os.Getenv("MYGO_STORE_CATALOG_URL")); text != "" {
		url = text
	}

	syncer := &catalogSyncer{
		store:    s,
		url:      url,
		client:   &http.Client{Timeout: 45 * time.Second},
		interval: interval,
		disabled: disabled,
		cacheDir: prefsDir(),
		current:  s.catalog,
	}
	if _, etag, err := loadCatalogCache(syncer.cacheDir); err == nil {
		syncer.etag = etag
	}
	return syncer
}

// start checks once after launch, then on the configured interval.
func (c *catalogSyncer) start() {
	if c == nil || c.disabled {
		return
	}
	go func() {
		timer := time.NewTimer(catalogCheckDelay)
		defer timer.Stop()
		<-timer.C
		c.check()

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for range ticker.C {
			c.check()
		}
	}()
}

// checkNow starts a check without waiting for it, for the footer's button.
func (c *catalogSyncer) checkNow() {
	if c == nil || c.disabled {
		return
	}
	go c.check()
}

func (c *catalogSyncer) check() {
	if c == nil || c.disabled || !c.begin() {
		return
	}
	c.finish(c.fetch())
}

func (c *catalogSyncer) begin() bool {
	c.mu.Lock()
	if c.checking {
		c.mu.Unlock()
		return false
	}
	c.checking = true
	c.mu.Unlock()
	c.store.onMain(c.store.invalidate)
	return true
}

func (c *catalogSyncer) finish(err error) {
	c.mu.Lock()
	c.checking = false
	c.lastCheck = time.Now()
	if err != nil {
		c.lastErr = err.Error()
	} else {
		c.lastErr = ""
	}
	c.mu.Unlock()
	c.store.onMain(c.store.invalidate)
}

func (c *catalogSyncer) fetch() error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/json")
	if etag := c.etagValue(); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil
	case http.StatusOK:
	default:
		return fmt.Errorf("服务器返回 %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxCatalogSize {
		return fmt.Errorf("目录文件超过 %d MB", maxCatalogSize>>20)
	}
	catalog, err := parseCatalog(data)
	if err != nil {
		return fmt.Errorf("目录格式无效：%w", err)
	}

	etag := resp.Header.Get("ETag")
	if !catalogNewer(catalog, c.currentCatalog()) {
		c.setETag(etag)
		return nil
	}
	if err := saveCatalogCache(c.cacheDir, data, etag); err != nil {
		return fmt.Errorf("无法保存目录缓存：%w", err)
	}
	c.setETag(etag)
	c.store.onMain(func() {
		c.store.applyCatalog(catalog)
		c.setCurrent(catalog)
	})
	return nil
}

func (c *catalogSyncer) currentCatalog() *Catalog {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *catalogSyncer) setCurrent(catalog *Catalog) {
	c.mu.Lock()
	c.current = catalog
	c.mu.Unlock()
}

func (c *catalogSyncer) etagValue() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.etag
}

func (c *catalogSyncer) setETag(etag string) {
	c.mu.Lock()
	c.etag = etag
	c.mu.Unlock()
}

// statusText is the short line in the sidebar. The tooltip has the detail.
func (c *catalogSyncer) statusText() string {
	if c == nil {
		return "目录未检查"
	}
	c.mu.Lock()
	checking, disabled, lastCheck, lastErr := c.checking, c.disabled, c.lastCheck, c.lastErr
	current := c.current
	c.mu.Unlock()

	switch {
	case disabled:
		return "目录自动同步已关闭"
	case checking:
		return "正在检查目录…"
	case lastErr != "":
		return "目录检查失败"
	}
	if at := catalogTime(current); !at.IsZero() {
		return "目录已同步 " + catalogShortTime(at)
	}
	if lastCheck.IsZero() {
		return "目录未检查"
	}
	return "目录已检查"
}

func (c *catalogSyncer) tooltip() string {
	if c == nil {
		return "点击立即检查目录更新"
	}
	c.mu.Lock()
	checking, disabled, lastCheck, lastErr := c.checking, c.disabled, c.lastCheck, c.lastErr
	c.mu.Unlock()

	switch {
	case disabled:
		return "已通过环境变量关闭目录自动同步"
	case checking:
		return "正在从 GitHub Release 检查目录更新"
	case lastErr != "":
		return "上次检查失败：" + lastErr + "\n点击重试"
	case lastCheck.IsZero():
		return "启动后自动检查，点击立即检查"
	default:
		return "上次检查：" + lastCheck.Local().Format("2006-01-02 15:04") +
			"\n点击立即检查"
	}
}

func catalogShortTime(at time.Time) string {
	local := at.Local()
	now := time.Now()
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return local.Format("15:04")
	}
	return local.Format("01-02 15:04")
}

// loadCatalogCache reads the cached catalog and its ETag. A missing or
// invalid cache is not an error the app has to show: the embedded snapshot
// still works.
func loadCatalogCache(dir string) (*Catalog, string, error) {
	if dir == "" {
		return nil, "", os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(dir, catalogCacheName))
	if err != nil {
		return nil, "", err
	}
	catalog, err := parseCatalog(data)
	if err != nil {
		return nil, "", err
	}
	etag, _ := os.ReadFile(filepath.Join(dir, catalogETagName))
	return catalog, strings.TrimSpace(string(etag)), nil
}

func saveCatalogCache(dir string, data []byte, etag string) error {
	if dir == "" {
		return fmt.Errorf("no config directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, catalogCacheName), data); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, catalogETagName), []byte(etag+"\n"))
}

func writeFileAtomic(path string, data []byte) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// applyCatalog replaces the view's catalog on the UI thread.
func (s *store) applyCatalog(catalog *Catalog) {
	s.catalog = catalog
	s.resetBrowsing()
	s.invalidate()
}
