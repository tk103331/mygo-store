package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-store/locales"
)

func testCatalog(syncedAt string, slugs ...string) *Catalog {
	if len(slugs) == 0 {
		slugs = []string{"alpha"}
	}
	catalog := &Catalog{Source: "https://example.com/showcase", AsOf: "2030-01-01", SyncedAt: syncedAt}
	for _, slug := range slugs {
		catalog.Apps = append(catalog.Apps, App{
			Slug:      slug,
			Name:      slug,
			URL:       "https://example.com/" + slug,
			Platforms: []string{"mac"},
		})
	}
	return catalog
}

func testCatalogJSON(t *testing.T, catalog *Catalog) []byte {
	t.Helper()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("marshal the test catalog: %v", err)
	}
	return data
}

func newSyncTestStore(t *testing.T) *store {
	t.Helper()
	t.Setenv("MYGO_STORE_CONFIG", t.TempDir())
	return &store{
		catalog:   loadCatalog(),
		prefs:     newPrefs(),
		images:    newImageCache(),
		nav:       "discover",
		sort:      sortRecommended,
		runOnMain: func(fn func()) { fn() },
	}
}

func newSyncTestSyncer(t *testing.T, s *store, handler http.HandlerFunc) *catalogSyncer {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &catalogSyncer{
		store:    s,
		url:      server.URL + "/apps.json",
		client:   server.Client(),
		interval: time.Hour,
		cacheDir: t.TempDir(),
		current:  s.catalog,
	}
}

func TestParseCatalogRejectsDuplicateApps(t *testing.T) {
	data := testCatalogJSON(t, testCatalog("2030-01-01T00:00:00Z", "alpha", "alpha"))
	if _, err := parseCatalog(data); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("parseCatalog error = %v, want a duplicate slug error", err)
	}

	missing := testCatalogJSON(t, testCatalog("2030-01-01T00:00:00Z", "alpha"))
	var catalog map[string]any
	if err := json.Unmarshal(missing, &catalog); err != nil {
		t.Fatal(err)
	}
	apps := catalog["apps"].([]any)
	apps[0].(map[string]any)["name"] = ""
	broken, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseCatalog(broken); err == nil || !strings.Contains(err.Error(), "no name") {
		t.Fatalf("parseCatalog error = %v, want a missing name error", err)
	}
}

func TestCatalogSyncAppliesAndCachesANewerCatalog(t *testing.T) {
	s := newSyncTestStore(t)
	want := testCatalog("2030-01-01T00:00:00Z", "alpha", "beta")
	c := newSyncTestSyncer(t, s, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"catalog-1"`)
		_, _ = w.Write(testCatalogJSON(t, want))
	})

	if err := c.fetch(); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(s.catalog.Apps) != 2 || s.catalog.Apps[1].Slug != "beta" {
		t.Fatalf("the store did not apply the new catalog: %d apps", len(s.catalog.Apps))
	}
	if current := c.currentCatalog(); current == nil || current.SyncedAt != want.SyncedAt {
		t.Fatalf("the syncer kept %+v as its current catalog", current)
	}
	if c.etagValue() != `"catalog-1"` {
		t.Fatalf("etag = %q", c.etagValue())
	}

	cached, etag, err := loadCatalogCache(c.cacheDir)
	if err != nil {
		t.Fatalf("loadCatalogCache: %v", err)
	}
	if cached.SyncedAt != want.SyncedAt || len(cached.Apps) != 2 {
		t.Fatalf("cached catalog = %+v", cached)
	}
	if etag != `"catalog-1"` {
		t.Fatalf("cached etag = %q", etag)
	}
}

func TestCatalogSyncUsesConditionalRequests(t *testing.T) {
	s := newSyncTestStore(t)
	before := s.catalog

	requests := make(chan string, 1)
	c := newSyncTestSyncer(t, s, func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	})
	c.etag = `"catalog-1"`

	if err := c.fetch(); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got := <-requests; got != `"catalog-1"` {
		t.Fatalf("If-None-Match = %q, want the cached ETag", got)
	}
	if s.catalog != before {
		t.Error("a 304 response replaced the catalog")
	}
	if c.lastErr != nil {
		t.Errorf("lastErr = %v after a 304", c.lastErr)
	}
}

func TestCatalogSyncRejectsInvalidAndOlderCatalogs(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		s := newSyncTestStore(t)
		before := s.catalog
		c := newSyncTestSyncer(t, s, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{"))
		})

		err := c.fetch()
		if err == nil || !strings.Contains(err.Error(), "invalid catalog format") {
			t.Fatalf("fetch error = %v, want a format error", err)
		}
		if s.catalog != before {
			t.Error("an invalid response replaced the catalog")
		}
		if _, err := os.Stat(filepath.Join(c.cacheDir, catalogCacheName)); !os.IsNotExist(err) {
			t.Errorf("an invalid response wrote a cache: %v", err)
		}
	})

	t.Run("older catalog", func(t *testing.T) {
		s := newSyncTestStore(t)
		before := s.catalog
		older := testCatalog("2000-01-01T00:00:00Z", "old")
		c := newSyncTestSyncer(t, s, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(testCatalogJSON(t, older))
		})

		if err := c.fetch(); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if s.catalog != before {
			t.Error("an older catalog replaced the current one")
		}
		if _, err := os.Stat(filepath.Join(c.cacheDir, catalogCacheName)); !os.IsNotExist(err) {
			t.Errorf("an older catalog was cached: %v", err)
		}
	})
}

func TestLoadBestCatalogPrefersANewerCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MYGO_STORE_CONFIG", dir)

	embedded := loadCatalog()
	cached := testCatalog("2099-01-01T00:00:00Z", "cached-app")
	if err := saveCatalogCache(dir, testCatalogJSON(t, cached), `"cache"`); err != nil {
		t.Fatalf("saveCatalogCache: %v", err)
	}

	best := loadBestCatalog()
	if len(best.Apps) != 1 || best.Apps[0].Slug != "cached-app" {
		t.Fatalf("loadBestCatalog returned %d apps, want the cached catalog", len(best.Apps))
	}
	if !catalogNewer(cached, embedded) {
		t.Fatal("the test cache is not newer than the embedded catalog")
	}

	old := testCatalog("1999-01-01T00:00:00Z", "old-app")
	if err := saveCatalogCache(dir, testCatalogJSON(t, old), ""); err != nil {
		t.Fatalf("saveCatalogCache: %v", err)
	}
	if best := loadBestCatalog(); len(best.Apps) == 1 && best.Apps[0].Slug == "old-app" {
		t.Fatal("loadBestCatalog used an older cache")
	}
}

func TestCatalogTimeFormats(t *testing.T) {
	cases := map[string]bool{
		"2030-01-02T03:04:05Z": true,
		"2030-01-02":           true,
		"":                     false,
		"not a date":           false,
	}
	for value, ok := range cases {
		at := catalogTime(&Catalog{SyncedAt: value})
		if at.IsZero() == ok {
			t.Errorf("catalogTime(%q) = %v, want parsed=%v", value, at, ok)
		}
	}
}

func TestFooterChecksTheCatalog(t *testing.T) {
	s, tt := newTestStore(t)
	hits := make(chan struct{}, 2)
	c := newSyncTestSyncer(t, s, func(w http.ResponseWriter, r *http.Request) {
		hits <- struct{}{}
		w.WriteHeader(http.StatusNotModified)
	})
	s.catalogSync = c
	tt.Frame()

	wait := func() {
		t.Helper()
		select {
		case <-hits:
		case <-time.After(5 * time.Second):
			t.Fatal("clicking the catalog status did not start a check")
		}
	}

	if err := tt.Click(c.statusText(locales.In("zh-Hans"))); err != nil {
		t.Fatalf("click the catalog status: %v", err)
	}
	wait()

	// The smallest window still shows and accepts the click.
	small := ui.NewTester(s.view, 900, 600)
	small.SetTitleBar(ui.TitleBar{Height: 52, Left: 78, Right: 14})
	small.SetDark(true)
	small.SetLocale("zh-Hans")
	small.Frame()
	if err := small.Click(c.statusText(locales.In("zh-Hans"))); err != nil {
		t.Fatalf("click the catalog status at 900x600: %v", err)
	}
	wait()
}
