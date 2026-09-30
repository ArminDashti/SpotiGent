// Package store persists user-editable settings in a JSON file.
// Everything is stored in plain JSON so the file is easy to inspect
// and back up; this file contains OAuth credentials and API keys.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Settings is the single user-editable settings document. It is safe to
// extend with new fields: json.Unmarshal keeps unknown fields untouched
// on disk and missing fields get zero values in memory.
type Settings struct {
	// Spotify
	SpotifyClientID     string    `json:"spotify_client_id,omitempty"`
	SpotifyClientSecret string    `json:"spotify_client_secret,omitempty"`
	SpotifyAccessToken  string    `json:"spotify_access_token,omitempty"`
	SpotifyRefreshToken string    `json:"spotify_refresh_token,omitempty"`
	SpotifyTokenExpiry  time.Time `json:"spotify_token_expiry,omitempty"`
	SpotifyUserID       string    `json:"spotify_user_id,omitempty"`

	// AI providers
	Provider         string `json:"provider"` // "openrouter" | "opencode" | "openai" | "mistral" | "claude" | "google"
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	OpenCodeAPIKey   string `json:"opencode_api_key"`
	OpenRouterModel  string `json:"openrouter_model"`
	OpenCodeModel    string `json:"opencode_model"`
	OpenAIAPIKey     string `json:"openai_api_key,omitempty"`
	MistralAPIKey    string `json:"mistral_api_key,omitempty"`
	ClaudeAPIKey     string `json:"claude_api_key,omitempty"`
	GoogleAPIKey     string `json:"google_api_key,omitempty"`
	OpenAIModel      string `json:"openai_model,omitempty"`
	MistralModel     string `json:"mistral_model,omitempty"`
	ClaudeModel      string `json:"claude_model,omitempty"`
	GoogleModel      string `json:"google_model,omitempty"`

	// Appearance
	Theme string `json:"theme"`
}

// Defaults returns the initial settings used when no file exists yet.
func Defaults() Settings {
	return Settings{
		Provider:        "openrouter",
		OpenRouterModel: "openai/gpt-4.1-mini",
		OpenCodeModel:   "glm-5.3-flash",
		OpenAIModel:     "gpt-4.1-mini",
		MistralModel:    "mistral-large-latest",
		ClaudeModel:     "claude-sonnet-4-5",
		GoogleModel:     "gemini-2.5-flash",
		Theme:           "dark",
	}
}

// Store is a concurrency-safe JSON file store for Settings.
type Store struct {
	mu   sync.RWMutex
	path string
}

// New creates a Store backed by the file at path (directories created).
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

// Load returns the persisted settings, or defaults when no file exists.
func (s *Store) Load() (Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadLocked()
}

// Save atomically writes the settings to disk (temp file + rename).
func (s *Store) Save(st Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(st)
}

// Update loads, applies fn, and saves atomically under a single write
// lock. The previous implementation loaded under a read lock and saved
// under a separate write lock, so two concurrent updates (e.g. theme
// auto-save racing a Spotify credential save) could interleave and the
// second save would silently discard the first update.
func (s *Store) Update(fn func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.loadLocked()
	if err != nil {
		return err
	}
	fn(&st)
	return s.saveLocked(st)
}

// loadLocked reads settings from disk without locking.
func (s *Store) loadLocked() (Settings, error) {

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Defaults(), nil
		}
		return Defaults(), err
	}
	st := Defaults()
	if err := json.Unmarshal(raw, &st); err != nil {
		return Defaults(), err
	}
	if st.Provider == "" {
		st.Provider = Defaults().Provider
	}
	if st.Theme == "" {
		st.Theme = Defaults().Theme
	}
	return st, nil
}

// saveLocked writes the settings to disk (temp file + rename) without locking.
func (s *Store) saveLocked(st Settings) error {
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
