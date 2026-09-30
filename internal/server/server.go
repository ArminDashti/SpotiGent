// Package server wires the HTTP API (Gin) and static file serving.
package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"spotigent/internal/ai"
	"spotigent/internal/config"
	"spotigent/internal/spotify"
	"spotigent/internal/store"
)

// Server holds shared state for all handlers.
type Server struct {
	cfg   config.Config
	store *store.Store
	sp    *spotify.Client

	mu        sync.Mutex
	cache     *ai.Catalog
	cachedAt  time.Time
	cacheUser string

	aiMu sync.Mutex // serialize AI runs

	authMu       sync.Mutex
	oauthMu      sync.Mutex
	oauthState   string
	oauthStarted time.Time
}

// New creates the server.
func New(cfg config.Config, st *store.Store) *Server {
	return &Server{cfg: cfg, store: st, sp: spotify.New()}
}

// ---- shared helpers ----

type fail struct {
	Error string `json:"error"`
}

func (s *Server) settings() store.Settings {
	st, err := s.store.Load()
	if err != nil {
		return store.Defaults()
	}
	return st
}

// effectiveAIKey returns the UI-entered key, falling back to env.
func (s *Server) effectiveAIKey(st store.Settings) string {
	switch st.Provider {
	case "opencode":
		if st.OpenCodeAPIKey != "" {
			return st.OpenCodeAPIKey
		}
		return s.cfg.OpenCodeAPIKey
	case "openai":
		if st.OpenAIAPIKey != "" {
			return st.OpenAIAPIKey
		}
		return s.cfg.OpenAIAPIKey
	case "mistral":
		if st.MistralAPIKey != "" {
			return st.MistralAPIKey
		}
		return s.cfg.MistralAPIKey
	case "claude":
		if st.ClaudeAPIKey != "" {
			return st.ClaudeAPIKey
		}
		return s.cfg.ClaudeAPIKey
	case "google":
		if st.GoogleAPIKey != "" {
			return st.GoogleAPIKey
		}
		return s.cfg.GoogleAPIKey
	default:
		if st.OpenRouterAPIKey != "" {
			return st.OpenRouterAPIKey
		}
		return s.cfg.OpenRouterAPIKey
	}
}

func (s *Server) aiConfig(st store.Settings) ai.Config {
	cfg := ai.Config{Provider: ai.Provider(st.Provider)}
	if cfg.Provider == "" {
		cfg.Provider = ai.ProviderOpenRouter
	}
	cfg.APIKey = s.effectiveAIKey(st)
	switch cfg.Provider {
	case ai.ProviderOpenCode:
		cfg.Model = st.OpenCodeModel
		cfg.BaseURL = s.cfg.OpenCodeBaseURL
		// Stored Zen-era models are invalid on the Go plan endpoint.
		if !ai.IsOpenCodeModel(cfg.Model) {
			cfg.Model = ai.OpenCodeModels()[0]
		}
	case ai.ProviderOpenAI:
		cfg.Model = st.OpenAIModel
		cfg.BaseURL = s.cfg.OpenAIBaseURL
		if strings.TrimSpace(cfg.Model) == "" {
			cfg.Model = ai.OpenAIModels()[0]
		}
	case ai.ProviderMistral:
		cfg.Model = st.MistralModel
		cfg.BaseURL = s.cfg.MistralBaseURL
		if strings.TrimSpace(cfg.Model) == "" {
			cfg.Model = ai.MistralModels()[0]
		}
	case ai.ProviderClaude:
		cfg.Model = st.ClaudeModel
		cfg.BaseURL = s.cfg.ClaudeBaseURL
		if strings.TrimSpace(cfg.Model) == "" {
			cfg.Model = ai.ClaudeModels()[0]
		}
	case ai.ProviderGoogle:
		cfg.Model = st.GoogleModel
		cfg.BaseURL = s.cfg.GoogleBaseURL
		if strings.TrimSpace(cfg.Model) == "" {
			cfg.Model = ai.GoogleModels()[0]
		}
	default:
		cfg.Model = st.OpenRouterModel
		cfg.BaseURL = s.cfg.OpenRouterBaseURL
	}
	return cfg
}

// spotifyReady reports whether Spotify authorization is available.
func (s *Server) spotifyReady(st store.Settings) bool {
	return st.SpotifyClientID != "" && st.SpotifyClientSecret != "" && st.SpotifyRefreshToken != ""
}

func (s *Server) requireSpotify(c *gin.Context) (store.Settings, bool) {
	st := s.settings()
	if !s.spotifyReady(st) {
		c.AbortWithStatusJSON(http.StatusBadRequest, fail{Error: "Spotify is not connected — open Settings and connect your account"})
		return st, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	token, err := s.spotifyAccessToken(ctx)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, fail{Error: err.Error()})
		return st, false
	}
	// Carry the current access token only in this request's settings value.
	st.SpotifyAccessToken = token
	return st, true
}

func (s *Server) spotifyRedirectURI() string {
	_, port, err := net.SplitHostPort(s.cfg.Addr())
	if err != nil {
		port = "8080"
	}
	return "http://127.0.0.1:" + port + "/api/settings/spotify/callback"
}

func randomOAuthValue() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *Server) handleSpotifyLogin(c *gin.Context) {
	st := s.settings()
	if st.SpotifyClientID == "" || st.SpotifyClientSecret == "" {
		c.JSON(http.StatusBadRequest, fail{Error: "save your Spotify client ID and client secret first"})
		return
	}
	state, err := randomOAuthValue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, fail{Error: "could not start Spotify authorization"})
		return
	}
	s.oauthMu.Lock()
	s.oauthState = state
	s.oauthStarted = time.Now()
	s.oauthMu.Unlock()

	params := url.Values{
		"client_id":     {st.SpotifyClientID},
		"response_type": {"code"},
		"redirect_uri":  {s.spotifyRedirectURI()},
		"scope":         {spotify.Scopes},
		"state":         {state},
	}
	c.Redirect(http.StatusFound, "https://accounts.spotify.com/authorize?"+params.Encode())
}

func (s *Server) handleSpotifyCallback(c *gin.Context) {
	s.oauthMu.Lock()
	wantState := s.oauthState
	stateMatches := wantState != "" && subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(wantState)) == 1
	stateExpired := !s.oauthStarted.IsZero() && time.Since(s.oauthStarted) >= 10*time.Minute
	validState := stateMatches && !stateExpired
	if stateMatches || stateExpired {
		s.oauthState = ""
		s.oauthStarted = time.Time{}
	}
	s.oauthMu.Unlock()
	if !validState {
		c.String(http.StatusBadRequest, "Spotify authorization state expired or did not match. Return to SpotiGent Settings and try again.")
		return
	}
	if oauthErr := c.Query("error"); oauthErr != "" {
		c.Redirect(http.StatusSeeOther, "/settings?spotify_error="+url.QueryEscape(oauthErr))
		return
	}
	code := c.Query("code")
	if code == "" {
		c.String(http.StatusBadRequest, "Spotify did not return an authorization code.")
		return
	}

	st := s.settings()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	tokens, err := s.sp.ExchangeCode(ctx, st.SpotifyClientID, st.SpotifyClientSecret, s.spotifyRedirectURI(), code)
	if err != nil {
		c.String(http.StatusBadGateway, "Spotify authorization failed: "+err.Error())
		return
	}
	if tokens.RefreshToken == "" {
		c.String(http.StatusBadGateway, "Spotify did not return a refresh token. Reconnect and approve the requested access.")
		return
	}
	user, err := s.sp.Me(ctx, tokens.AccessToken)
	if err != nil {
		c.String(http.StatusBadGateway, "Spotify account check failed: "+err.Error())
		return
	}
	err = s.store.Update(func(current *store.Settings) {
		current.SpotifyAccessToken = tokens.AccessToken
		current.SpotifyRefreshToken = tokens.RefreshToken
		current.SpotifyTokenExpiry = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
		current.SpotifyUserID = user.ID
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "could not save Spotify authorization")
		return
	}
	s.invalidateCatalog()
	c.Redirect(http.StatusSeeOther, "/settings?spotify=connected")
}

func (s *Server) spotifyAccessToken(ctx context.Context) (string, error) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	st := s.settings()
	if st.SpotifyAccessToken != "" && time.Now().Add(30*time.Second).Before(st.SpotifyTokenExpiry) {
		return st.SpotifyAccessToken, nil
	}
	if st.SpotifyRefreshToken == "" || st.SpotifyClientID == "" || st.SpotifyClientSecret == "" {
		return "", fmt.Errorf("Spotify authorization expired — reconnect in Settings")
	}
	tokens, err := s.sp.RefreshAccessToken(ctx, st.SpotifyClientID, st.SpotifyClientSecret, st.SpotifyRefreshToken)
	if err != nil {
		if strings.Contains(err.Error(), "invalid_grant") {
			_ = s.store.Update(func(current *store.Settings) {
				current.SpotifyAccessToken = ""
				current.SpotifyRefreshToken = ""
				current.SpotifyTokenExpiry = time.Time{}
				current.SpotifyUserID = ""
			})
			return "", fmt.Errorf("Spotify authorization expired — reconnect in Settings")
		}
		return "", err
	}
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = st.SpotifyRefreshToken
	}
	err = s.store.Update(func(current *store.Settings) {
		current.SpotifyAccessToken = tokens.AccessToken
		current.SpotifyRefreshToken = tokens.RefreshToken
		current.SpotifyTokenExpiry = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	})
	if err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

// getCatalog returns a cached catalog, refreshing if stale (>5 min).
func (s *Server) getCatalog(ctx context.Context, secret string) (*ai.Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil && time.Since(s.cachedAt) < 5*time.Minute {
		return s.cache, nil
	}
	cat, err := ai.BuildCatalog(ctx, s.sp, secret)
	if err != nil {
		return nil, err
	}
	s.cache = cat
	s.cachedAt = time.Now()
	return cat, nil
}

// invalidateCatalog drops the cached catalog after mutations.
func (s *Server) invalidateCatalog() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// ---- settings handlers ----

type publicSettings struct {
	SpotifyConfigured      bool   `json:"spotify_configured"`
	SpotifyClientID        string `json:"spotify_client_id,omitempty"`
	SpotifyClientIDSet     bool   `json:"spotify_client_id_set"`
	SpotifyClientSecretSet bool   `json:"spotify_client_secret_set"`
	SpotifyRedirectURI     string `json:"spotify_redirect_uri"`
	SpotifyUserID          string `json:"spotify_user_id"`
	Provider               string `json:"provider"`
	OpenRouterSet          bool   `json:"openrouter_set"`
	OpenCodeSet            bool   `json:"opencode_set"`
	OpenAISet              bool   `json:"openai_set"`
	MistralSet             bool   `json:"mistral_set"`
	ClaudeSet              bool   `json:"claude_set"`
	GoogleSet              bool   `json:"google_set"`
	OpenRouterModel        string `json:"openrouter_model"`
	OpenCodeModel          string `json:"opencode_model"`
	OpenAIModel            string `json:"openai_model"`
	MistralModel           string `json:"mistral_model"`
	ClaudeModel            string `json:"claude_model"`
	GoogleModel            string `json:"google_model"`
	Theme                  string `json:"theme"`
}

func (s *Server) handleGetSettings(c *gin.Context) {
	st := s.settings()
	c.JSON(http.StatusOK, publicSettings{
		SpotifyConfigured:      s.spotifyReady(st),
		SpotifyClientID:        st.SpotifyClientID,
		SpotifyClientIDSet:     st.SpotifyClientID != "",
		SpotifyClientSecretSet: st.SpotifyClientSecret != "",
		SpotifyRedirectURI:     s.spotifyRedirectURI(),
		SpotifyUserID:          st.SpotifyUserID,
		Provider:               st.Provider,
		OpenRouterSet:          st.OpenRouterAPIKey != "" || s.cfg.OpenRouterAPIKey != "",
		OpenCodeSet:            st.OpenCodeAPIKey != "" || s.cfg.OpenCodeAPIKey != "",
		OpenAISet:              st.OpenAIAPIKey != "" || s.cfg.OpenAIAPIKey != "",
		MistralSet:             st.MistralAPIKey != "" || s.cfg.MistralAPIKey != "",
		ClaudeSet:              st.ClaudeAPIKey != "" || s.cfg.ClaudeAPIKey != "",
		GoogleSet:              st.GoogleAPIKey != "" || s.cfg.GoogleAPIKey != "",
		OpenRouterModel:        st.OpenRouterModel,
		OpenCodeModel:          st.OpenCodeModel,
		OpenAIModel:            st.OpenAIModel,
		MistralModel:           st.MistralModel,
		ClaudeModel:            st.ClaudeModel,
		GoogleModel:            st.GoogleModel,
		Theme:                  st.Theme,
	})
}

type updateSettingsReq struct {
	SpotifyClientID     *string `json:"spotify_client_id"`
	SpotifyClientSecret *string `json:"spotify_client_secret"`
	Provider            *string `json:"provider"`
	OpenRouterKey       *string `json:"openrouter_api_key"`
	OpenCodeKey         *string `json:"opencode_api_key"`
	OpenRouterModel     *string `json:"openrouter_model"`
	OpenCodeModel       *string `json:"opencode_model"`
	OpenAIKey           *string `json:"openai_api_key"`
	MistralKey          *string `json:"mistral_api_key"`
	ClaudeKey           *string `json:"claude_api_key"`
	GoogleKey           *string `json:"google_api_key"`
	OpenAIModel         *string `json:"openai_model"`
	MistralModel        *string `json:"mistral_model"`
	ClaudeModel         *string `json:"claude_model"`
	GoogleModel         *string `json:"google_model"`
	Theme               *string `json:"theme"`
}

func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req updateSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail{Error: err.Error()})
		return
	}

	err := s.store.Update(func(st *store.Settings) {
		credentialsChanged := false
		if req.SpotifyClientID != nil {
			clientID := strings.TrimSpace(*req.SpotifyClientID)
			if clientID != st.SpotifyClientID {
				credentialsChanged = true
				if req.SpotifyClientSecret == nil {
					st.SpotifyClientSecret = ""
				}
			}
			st.SpotifyClientID = clientID
		}
		if req.SpotifyClientSecret != nil {
			clientSecret := strings.TrimSpace(*req.SpotifyClientSecret)
			credentialsChanged = credentialsChanged || clientSecret != st.SpotifyClientSecret
			st.SpotifyClientSecret = clientSecret
		}
		if credentialsChanged {
			st.SpotifyAccessToken = ""
			st.SpotifyRefreshToken = ""
			st.SpotifyTokenExpiry = time.Time{}
			st.SpotifyUserID = ""
		}
		if req.Provider != nil {
			p := strings.ToLower(strings.TrimSpace(*req.Provider))
			switch p {
			case "openrouter", "opencode", "openai", "mistral", "claude", "google":
				st.Provider = p
			}
		}
		if req.OpenRouterKey != nil {
			st.OpenRouterAPIKey = strings.TrimSpace(*req.OpenRouterKey)
		}
		if req.OpenCodeKey != nil {
			st.OpenCodeAPIKey = strings.TrimSpace(*req.OpenCodeKey)
		}
		if req.OpenRouterModel != nil && strings.TrimSpace(*req.OpenRouterModel) != "" {
			st.OpenRouterModel = strings.TrimSpace(*req.OpenRouterModel)
		}
		if req.OpenCodeModel != nil && strings.TrimSpace(*req.OpenCodeModel) != "" {
			st.OpenCodeModel = strings.TrimSpace(*req.OpenCodeModel)
		}
		if req.OpenAIKey != nil {
			st.OpenAIAPIKey = strings.TrimSpace(*req.OpenAIKey)
		}
		if req.MistralKey != nil {
			st.MistralAPIKey = strings.TrimSpace(*req.MistralKey)
		}
		if req.ClaudeKey != nil {
			st.ClaudeAPIKey = strings.TrimSpace(*req.ClaudeKey)
		}
		if req.GoogleKey != nil {
			st.GoogleAPIKey = strings.TrimSpace(*req.GoogleKey)
		}
		if req.OpenAIModel != nil && strings.TrimSpace(*req.OpenAIModel) != "" {
			st.OpenAIModel = strings.TrimSpace(*req.OpenAIModel)
		}
		if req.MistralModel != nil && strings.TrimSpace(*req.MistralModel) != "" {
			st.MistralModel = strings.TrimSpace(*req.MistralModel)
		}
		if req.ClaudeModel != nil && strings.TrimSpace(*req.ClaudeModel) != "" {
			st.ClaudeModel = strings.TrimSpace(*req.ClaudeModel)
		}
		if req.GoogleModel != nil && strings.TrimSpace(*req.GoogleModel) != "" {
			st.GoogleModel = strings.TrimSpace(*req.GoogleModel)
		}
		if req.Theme != nil {
			st.Theme = strings.TrimSpace(*req.Theme)
		}
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, fail{Error: err.Error()})
		return
	}
	s.handleGetSettings(c)
}

// ---- Spotify data handlers ----

type playlistDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	TrackCount  int    `json:"track_count"`
	Public      bool   `json:"public"`
	Image       string `json:"image"`
}

func firstImage(imgs []spotify.Image) string {
	if len(imgs) == 0 {
		return ""
	}
	// Prefer the smallest reasonably-sized image for list views.
	best := imgs[0]
	for _, im := range imgs {
		if im.Width > 0 && im.Width < best.Width {
			best = im
		}
	}
	return best.URL
}

type trackDTO struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	AlbumImage string   `json:"album_image"`
	DurationMS int      `json:"duration_ms"`
	Playlists  []string `json:"playlists"`
	Popularity int      `json:"popularity"`
}

// handleMusics returns the deduplicated track list across all playlists.
func (s *Server) handleMusics(c *gin.Context) {
	st, ok := s.requireSpotify(c)
	if !ok {
		return
	}
	cat, err := s.getCatalog(c.Request.Context(), st.SpotifyAccessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, fail{Error: err.Error()})
		return
	}

	byID := map[string]*trackDTO{}
	var order []string
	for _, t := range cat.AllTracks {
		dto, exists := byID[t.ID]
		if !exists {
			artists := make([]string, 0, len(t.Artists))
			for _, a := range t.Artists {
				artists = append(artists, a.Name)
			}
			dto = &trackDTO{
				ID:         t.ID,
				Name:       t.Name,
				Artists:    artists,
				Album:      t.Album.Name,
				AlbumImage: firstImage(t.Album.Images),
				DurationMS: t.DurationMS,
				Popularity: t.Popularity,
			}
			byID[t.ID] = dto
			order = append(order, t.ID)
		}
		dto.Playlists = append(dto.Playlists, t.PlaylistName)
	}

	out := make([]trackDTO, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	c.JSON(http.StatusOK, gin.H{"tracks": out, "total": len(out)})
}

// handlePlaylists returns playlists with their tracks nested.
func (s *Server) handlePlaylists(c *gin.Context) {
	st, ok := s.requireSpotify(c)
	if !ok {
		return
	}
	cat, err := s.getCatalog(c.Request.Context(), st.SpotifyAccessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, fail{Error: err.Error()})
		return
	}

	type playlistWithTracks struct {
		playlistDTO
		Tracks []trackDTO `json:"tracks"`
	}
	out := make([]playlistWithTracks, 0, len(cat.Playlists))
	for _, p := range cat.Playlists {
		pwt := playlistWithTracks{
			playlistDTO: playlistDTO{
				ID:          p.ID,
				Name:        p.Name,
				Description: p.Description,
				Owner:       p.Owner.DisplayName,
				TrackCount:  p.ItemCount(),
				Public:      p.Public,
				Image:       firstImage(p.Images),
			},
			Tracks: []trackDTO{},
		}
		for _, item := range cat.Tracks[p.ID] {
			if item.Track.ID == "" {
				continue
			}
			artists := make([]string, 0, len(item.Track.Artists))
			for _, a := range item.Track.Artists {
				artists = append(artists, a.Name)
			}
			pwt.Tracks = append(pwt.Tracks, trackDTO{
				ID:         item.Track.ID,
				Name:       item.Track.Name,
				Artists:    artists,
				Album:      item.Track.Album.Name,
				AlbumImage: firstImage(item.Track.Album.Images),
				DurationMS: item.Track.DurationMS,
				Playlists:  []string{p.Name},
			})
		}
		out = append(out, pwt)
	}
	c.JSON(http.StatusOK, gin.H{"playlists": out, "total": len(out)})
}

// handlePodcasts returns saved shows with recent episodes.
func (s *Server) handlePodcasts(c *gin.Context) {
	st, ok := s.requireSpotify(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	shows, err := s.sp.AllSavedShows(ctx, st.SpotifyAccessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, fail{Error: err.Error()})
		return
	}

	type episodeDTO struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		DurationMS  int    `json:"duration_ms"`
		ReleaseDate string `json:"release_date"`
	}
	type showDTO struct {
		ID        string       `json:"id"`
		Name      string       `json:"name"`
		Publisher string       `json:"publisher"`
		Image     string       `json:"image"`
		Episodes  []episodeDTO `json:"episodes"`
	}
	out := make([]showDTO, 0, len(shows))
	for _, sh := range shows {
		dto := showDTO{
			ID:        sh.Show.ID,
			Name:      sh.Show.Name,
			Publisher: sh.Show.Publisher,
			Image:     firstImage(sh.Show.Images),
			Episodes:  []episodeDTO{},
		}
		episodes, err := s.sp.ShowEpisodes(ctx, st.SpotifyAccessToken, sh.Show.ID)
		if err == nil {
			for i, ep := range episodes {
				if i >= 10 {
					break // keep payload light: 10 latest episodes per show
				}
				dto.Episodes = append(dto.Episodes, episodeDTO{
					ID:          ep.ID,
					Name:        ep.Name,
					Description: ep.Description,
					DurationMS:  ep.DurationMS,
					ReleaseDate: ep.ReleaseDate,
				})
			}
		}
		out = append(out, dto)
	}
	c.JSON(http.StatusOK, gin.H{"shows": out, "total": len(out)})
}

// ---- Dashboard handler ----

func (s *Server) handleDashboard(c *gin.Context) {
	st, ok := s.requireSpotify(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	cat, err := s.getCatalog(ctx, st.SpotifyAccessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, fail{Error: err.Error()})
		return
	}

	// Top artists from top tracks (medium term).
	topTracks, _ := s.sp.TopTracks(ctx, st.SpotifyAccessToken, "medium_term", 20)
	artistCount := map[string]int{}
	for _, t := range topTracks {
		for _, a := range t.Artists {
			artistCount[a.Name]++
		}
	}
	type artistCountDTO struct {
		Name  string `json:"name"`
		Plays int    `json:"plays"`
	}
	topArtists := make([]artistCountDTO, 0, 5)
	for name, n := range artistCount {
		topArtists = append(topArtists, artistCountDTO{Name: name, Plays: n})
	}
	// simple insertion sort on plays (small slice)
	for i := 1; i < len(topArtists); i++ {
		for j := i; j > 0 && topArtists[j].Plays > topArtists[j-1].Plays; j-- {
			topArtists[j], topArtists[j-1] = topArtists[j-1], topArtists[j]
		}
	}
	if len(topArtists) > 5 {
		topArtists = topArtists[:5]
	}

	recentRaw, _ := s.sp.RecentlyPlayed(ctx, st.SpotifyAccessToken, 8)
	type recentDTO struct {
		Name     string   `json:"name"`
		Artists  []string `json:"artists"`
		PlayedAt string   `json:"played_at"`
	}
	recent := make([]recentDTO, 0, len(recentRaw))
	for _, r := range recentRaw {
		artists := make([]string, 0, len(r.Track.Artists))
		for _, a := range r.Track.Artists {
			artists = append(artists, a.Name)
		}
		recent = append(recent, recentDTO{Name: r.Track.Name, Artists: artists, PlayedAt: r.PlayedAt})
	}

	uniqueTracks := map[string]bool{}
	totalDuration := int64(0)
	for _, t := range cat.AllTracks {
		uniqueTracks[t.ID] = true
		totalDuration += int64(t.DurationMS)
	}

	c.JSON(http.StatusOK, gin.H{
		"playlists":      len(cat.Playlists),
		"unique_tracks":  len(uniqueTracks),
		"track_entries":  len(cat.AllTracks),
		"total_duration": totalDuration,
		"top_artists":    topArtists,
		"recent":         recent,
	})
}

// ---- AI handler ----

type chatReq struct {
	Message string       `json:"message"`
	History []ai.Message `json:"history"`
}

type chatRes struct {
	Reply string `json:"reply"`
}

func (s *Server) handleAIChat(c *gin.Context) {
	st, ok := s.requireSpotify(c)
	if !ok {
		return
	}
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail{Error: err.Error()})
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		c.JSON(http.StatusBadRequest, fail{Error: "message is required"})
		return
	}
	if s.effectiveAIKey(st) == "" {
		c.JSON(http.StatusBadRequest, fail{Error: "no AI provider API key configured — open Settings"})
		return
	}

	// One AI conversation at a time keeps Spotify rate limits sane.
	s.aiMu.Lock()
	defer s.aiMu.Unlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()

	cat, err := s.getCatalog(ctx, st.SpotifyAccessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, fail{Error: err.Error()})
		return
	}

	userID := st.SpotifyUserID
	if userID == "" {
		if u, err := s.sp.Me(ctx, st.SpotifyAccessToken); err == nil {
			userID = u.ID
			_ = s.store.Update(func(x *store.Settings) { x.SpotifyUserID = u.ID })
		}
	}

	agent := &ai.Agent{
		SP:       s.sp,
		Secret:   st.SpotifyAccessToken,
		Cfg:      s.aiConfig(st),
		Catalog:  cat,
		UserID:   userID,
		MaxSteps: 8,
	}

	reply, err := agent.Run(ctx, req.History, req.Message)
	if err != nil {
		c.JSON(http.StatusBadGateway, fail{Error: err.Error()})
		return
	}

	// Mutations invalidate the cached catalog.
	if strings.Contains(reply, "created") || strings.Contains(reply, "added") || strings.Contains(reply, "removed") {
		s.invalidateCatalog()
	}

	c.JSON(http.StatusOK, chatRes{Reply: reply})
}

// handleRefresh forces a catalog refresh.
func (s *Server) handleRefresh(c *gin.Context) {
	st, ok := s.requireSpotify(c)
	if !ok {
		return
	}
	s.invalidateCatalog()
	if _, err := s.getCatalog(c.Request.Context(), st.SpotifyAccessToken); err != nil {
		c.JSON(http.StatusBadGateway, fail{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- Router + static serving ----

// Router builds the Gin engine with all API routes and the SPA static handler.
func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.MaxMultipartMemory = 1 << 20

	// API
	api := r.Group("/api")
	{
		api.GET("/settings", s.handleGetSettings)
		api.PUT("/settings", s.handleUpdateSettings)
		api.GET("/settings/spotify/login", s.handleSpotifyLogin)
		api.GET("/settings/spotify/callback", s.handleSpotifyCallback)
		api.GET("/dashboard", s.handleDashboard)
		api.GET("/musics", s.handleMusics)
		api.GET("/playlists", s.handlePlaylists)
		api.GET("/podcasts", s.handlePodcasts)
		api.POST("/ai/chat", s.handleAIChat)
		api.POST("/refresh", s.handleRefresh)
	}
	api.GET("/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"openrouter": ai.OpenRouterModels(),
			"opencode":   ai.OpenCodeModels(),
			"openai":     ai.OpenAIModels(),
			"mistral":    ai.MistralModels(),
			"claude":     ai.ClaudeModels(),
			"google":     ai.GoogleModels(),
		})
	})
	api.GET("/health", func(c *gin.Context) {
		st := s.settings()
		c.JSON(http.StatusOK, gin.H{
			"status":             "ok",
			"spotify_configured": s.spotifyReady(st),
		})
	})

	// Static SPA
	staticDir := resolveStaticDir()
	if staticDir != "" {
		r.StaticFile("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))
		// PWA: serve the worker with no-cache so updates activate, and the
		// manifest with its explicit MIME type (Go's mime registry does not
		// know .webmanifest on every platform).
		r.GET("/sw.js", func(c *gin.Context) {
			c.Header("Cache-Control", "no-cache")
			c.File(filepath.Join(staticDir, "sw.js"))
		})
		r.GET("/manifest.webmanifest", func(c *gin.Context) {
			c.Header("Content-Type", "application/manifest+json")
			c.File(filepath.Join(staticDir, "manifest.webmanifest"))
		})
		r.Use(serveSPA(staticDir))
	} else {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, fail{Error: "not found"})
				return
			}
			c.String(http.StatusOK, "SpotiGent API is running. Build the frontend with `npm run build` in `web/` to serve the UI here.")
		})
	}
	return r
}

// resolveStaticDir finds web/dist relative to the executable or cwd.
func resolveStaticDir() string {
	candidates := []string{
		"web/dist",
		"../web/dist",
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "web", "dist"),
			filepath.Join(exeDir, "..", "web", "dist"),
		)
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "index.html")); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// serveSPA serves static assets and falls back to index.html for client routes.
func serveSPA(dir string) gin.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		// Serve real files; everything else falls back to index.html.
		path := filepath.Join(dir, filepath.Clean("/"+c.Request.URL.Path))
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			fs.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}
		c.Request.URL.Path = "/"
		fs.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

var _ = fmt.Sprintf
