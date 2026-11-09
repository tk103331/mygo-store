package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// prefs is what the app remembers between launches.
type prefs struct {
	Favorites map[string]bool   `json:"favorites"`
	Acquired  map[string]bool   `json:"acquired"`
	Downloads map[string]string `json:"downloads"` // slug -> installer path
}

func newPrefs() prefs {
	return prefs{
		Favorites: map[string]bool{},
		Acquired:  map[string]bool{},
		Downloads: map[string]string{},
	}
}

func prefsDir() string {
	if dir := os.Getenv("MYGO_STORE_CONFIG"); dir != "" {
		return dir
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "MyGo Store")
}

func loadPrefs() prefs {
	p := newPrefs()
	dir := prefsDir()
	if dir == "" {
		return p
	}
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return p
	}
	_ = json.Unmarshal(data, &p)
	if p.Favorites == nil {
		p.Favorites = map[string]bool{}
	}
	if p.Acquired == nil {
		p.Acquired = map[string]bool{}
	}
	if p.Downloads == nil {
		p.Downloads = map[string]string{}
	}
	return p
}

func savePrefs(p prefs) {
	dir := prefsDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	temp := filepath.Join(dir, "state.json.tmp")
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(temp, filepath.Join(dir, "state.json"))
}

func (s *store) toggleFavorite(slug string) {
	if s.prefs.Favorites[slug] {
		delete(s.prefs.Favorites, slug)
	} else {
		s.prefs.Favorites[slug] = true
	}
	savePrefs(s.prefs)
}
