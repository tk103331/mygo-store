package main

import (
	"embed"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed assets
var assetFS embed.FS

// imageCache keeps decoded bitmaps for the lifetime of the window. MyGo keeps
// each one on the GPU too; decoding is the expensive part.
type imageCache struct {
	mu     sync.Mutex
	icons  map[string]*ui.Bitmap
	shots  map[string]*ui.Bitmap
	order  []string // shot slugs, oldest first
	broken map[string]bool
}

// shotCacheSize is how many screenshots stay decoded: a detail page shows
// one at a time, and each is several megabytes.
const shotCacheSize = 4

func newImageCache() *imageCache {
	return &imageCache{
		icons:  map[string]*ui.Bitmap{},
		shots:  map[string]*ui.Bitmap{},
		broken: map[string]bool{},
	}
}

func (s *store) iconBitmap(app *App) *ui.Bitmap {
	if app.Icon == "" {
		return nil
	}
	s.images.mu.Lock()
	defer s.images.mu.Unlock()
	if b, ok := s.images.icons[app.Slug]; ok {
		return b
	}
	if s.images.broken[app.Icon] {
		return nil
	}
	data, err := assetFS.ReadFile(app.Icon)
	if err != nil {
		s.images.broken[app.Icon] = true
		return nil
	}
	b, err := ui.DecodeBitmap(data)
	if err != nil {
		s.images.broken[app.Icon] = true
		return nil
	}
	s.images.icons[app.Slug] = b
	return b
}

// rememberShot caches a screenshot and evicts the least recently shown.
// The caller holds the cache lock.
func (s *store) rememberShot(slug string, b *ui.Bitmap) {
	for i, name := range s.images.order {
		if name == slug {
			s.images.order = append(s.images.order[:i], s.images.order[i+1:]...)
			break
		}
	}
	s.images.shots[slug] = b
	s.images.order = append(s.images.order, slug)
	for len(s.images.order) > shotCacheSize {
		oldest := s.images.order[0]
		s.images.order = s.images.order[1:]
		delete(s.images.shots, oldest)
	}
}

func (s *store) shotBitmap(app *App) *ui.Bitmap {
	if app.Shot == "" {
		return nil
	}
	s.images.mu.Lock()
	defer s.images.mu.Unlock()
	if b, ok := s.images.shots[app.Slug]; ok {
		s.rememberShot(app.Slug, b)
		return b
	}
	if s.images.broken[app.Shot] {
		return nil
	}
	data, err := assetFS.ReadFile(app.Shot)
	if err != nil {
		s.images.broken[app.Shot] = true
		return nil
	}
	b, err := ui.DecodeBitmap(data)
	if err != nil {
		s.images.broken[app.Shot] = true
		return nil
	}
	s.rememberShot(app.Slug, b)
	return b
}

// appIcon draws an app's icon, rounded, or a letter tile when it has none.
func (s *store) appIcon(c *ui.Context, app *App, size float32) ui.Element {
	t := c.Theme()
	radius := size * 0.23
	box := ui.Box(c).Size(size, size).Radius(radius).Clip().Shrink(0)
	if bmp := s.iconBitmap(app); bmp != nil {
		box.Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Image(c, bmp).Fill().Fit(ui.Cover)
		})
		return box
	}
	h := hue(app.Slug)
	from := ui.Oklch(0.72, 0.15, h)
	to := ui.Oklch(0.55, 0.18, h+40)
	box.LinearGradient(ui.LinearGradient{From: from, To: to, Angle: 140, Oklab: true}).Center().Children(func() {
		ui.Text(c, initial(app.Name)).FontSize(size * 0.42).Bold().
			TextColor(ui.RGBA(255, 255, 255, 0.96))
	})
	return box
}

func initial(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	runes := []rune(name)
	first := runes[0]
	if first >= 'a' && first <= 'z' {
		return strings.ToUpper(string(first))
	}
	return string(first)
}

func hue(slug string) float32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(slug))
	return float32(h.Sum32() % 360)
}

// openExternal is a variable so tests can record the URLs the app opens
// instead of handing them to the system.
var openExternal = func(url string) {
	if url == "" {
		return
	}
	go func() { _ = mygo.Shell.OpenExternal(url) }()
}
