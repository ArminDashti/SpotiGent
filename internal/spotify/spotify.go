// Package spotify implements a minimal client for the Spotify Web API.
//
// Authentication uses Spotify's OAuth 2.0 Authorization Code flow.
package spotify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiBase   = "https://api.spotify.com/v1"
	tokenURL  = "https://accounts.spotify.com/api/token"
	Scopes    = "user-read-email user-read-private playlist-read-private playlist-read-collaborative playlist-modify-public playlist-modify-private user-library-read user-library-modify user-top-read user-follow-read user-read-playback-state user-modify-playback-state user-read-currently-playing user-read-recently-played"
	userAgent = "SpotiGent/1.0"
)

// Client talks to the Spotify Web API on behalf of one user.
type Client struct {
	HTTP *http.Client
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// TokenResponse contains credentials returned by Spotify's token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

func (c *Client) tokenRequest(ctx context.Context, clientID, clientSecret string, form url.Values) (TokenResponse, error) {
	var body TokenResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return body, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))
	req.Header.Set("User-Agent", userAgent)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return body, fmt.Errorf("token request failed: %w", err)
	}
	defer res.Body.Close()

	var response struct {
		TokenResponse
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return body, fmt.Errorf("invalid token response (HTTP %d)", res.StatusCode)
	}
	if response.Error != "" {
		return body, fmt.Errorf("Spotify auth: %s — %s", response.Error, response.ErrorDesc)
	}
	if res.StatusCode >= http.StatusBadRequest || response.AccessToken == "" {
		return body, fmt.Errorf("Spotify auth failed (HTTP %d)", res.StatusCode)
	}
	return response.TokenResponse, nil
}

// ExchangeCode trades an authorization code for access and refresh tokens.
func (c *Client) ExchangeCode(ctx context.Context, clientID, clientSecret, redirectURI, code string) (TokenResponse, error) {
	return c.tokenRequest(ctx, clientID, clientSecret, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	})
}

// RefreshAccessToken exchanges a refresh token for a new access token.
func (c *Client) RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (TokenResponse, error) {
	return c.tokenRequest(ctx, clientID, clientSecret, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

// Token returns an already-authorized access token.
func (c *Client) Token(_ context.Context, accessToken string) (string, error) {
	if strings.TrimSpace(accessToken) == "" {
		return "", fmt.Errorf("Spotify is not connected — open Settings to connect")
	}
	return accessToken, nil
}

// APIError carries the Spotify error message and HTTP status.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("Spotify API %d: %s", e.Status, e.Message) }

func (c *Client) get(ctx context.Context, secret, path string, out any, query url.Values) error {
	tok, err := c.Token(ctx, secret)
	if err != nil {
		return err
	}
	full := apiBase + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("User-Agent", userAgent)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNoContent {
		return nil
	}
	if res.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		return &APIError{Status: res.StatusCode, Message: e.Error.Message}
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// ---- Shared Spotify payload types ----

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Images  []Image  `json:"images"`
	Artists []Artist `json:"artists"`
	Tracks  *struct {
		Total int `json:"total"`
	} `json:"tracks"`
	EpisodeCount int `json:"episode_count,omitempty"`
}

type Track struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Artists    []Artist `json:"artists"`
	Album      Album    `json:"album"`
	DurationMS int      `json:"duration_ms"`
	Popularity int      `json:"popularity"`
}

type SavedTrack struct {
	AddedAt string `json:"added_at"`
	Track   Track  `json:"track"`
}

type PlaylistTrackItem struct {
	AddedAt string `json:"added_at"`
	Track   Track  `json:"track"`
	Item    Track  `json:"item"`
}

type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	Collaborate bool   `json:"collaborative"`
	Owner       struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	Tracks struct {
		Total int    `json:"total"`
		Href  string `json:"href"`
	} `json:"tracks"`
	Items struct {
		Total int `json:"total"`
	} `json:"items"`
	Images []Image `json:"images"`
}

func (p Playlist) ItemCount() int {
	if p.Items.Total != 0 || p.Tracks.Total == 0 {
		return p.Items.Total
	}
	return p.Tracks.Total
}

type User struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       string  `json:"email"`
	Country     string  `json:"country"`
	Images      []Image `json:"images"`
}

type Show struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Publisher   string  `json:"publisher"`
	Description string  `json:"description"`
	Images      []Image `json:"images"`
	Episodes    *struct {
		Total int `json:"total"`
	} `json:"episodes"`
	TotalEpisodes int `json:"total_episodes,omitempty"`
}

type SavedShow struct {
	AddedAt string `json:"added_at"`
	Show    Show   `json:"show"`
}

type Episode struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	DurationMS  int     `json:"duration_ms"`
	ReleaseDate string  `json:"release_date"`
	Images      []Image `json:"images"`
	Show        *struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Publisher string `json:"publisher"`
	} `json:"show"`
}

// ---- Endpoints ----

// Me returns the current user profile.
func (c *Client) Me(ctx context.Context, secret string) (*User, error) {
	var u User
	if err := c.get(ctx, secret, "/me", &u, nil); err != nil {
		return nil, err
	}
	return &u, nil
}

// AllPlaylists fetches every playlist (owned + followed), handling paging.
func (c *Client) AllPlaylists(ctx context.Context, secret string) ([]Playlist, error) {
	var out []Playlist
	limit := 50
	offset := 0
	for {
		q := url.Values{"limit": {"50"}, "offset": {itoa(offset)}}
		var page struct {
			Items []json.RawMessage `json:"items"`
			Next  string            `json:"next"`
			Total int               `json:"total"`
		}
		if err := c.get(ctx, secret, "/me/playlists", &page, q); err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			var wrapped struct {
				Playlist *Playlist `json:"playlist"`
			}
			if err := json.Unmarshal(raw, &wrapped); err != nil {
				return nil, err
			}
			var playlist Playlist
			if wrapped.Playlist != nil {
				playlist = *wrapped.Playlist
			} else if err := json.Unmarshal(raw, &playlist); err != nil {
				return nil, err
			}
			if playlist.ID != "" {
				out = append(out, playlist)
			}
		}
		offset += limit
		if page.Next == "" || offset >= page.Total || len(page.Items) == 0 {
			break
		}
	}
	return out, nil
}

// PlaylistTracks fetches all tracks of one playlist, handling paging.
func (c *Client) PlaylistTracks(ctx context.Context, secret, id string) ([]PlaylistTrackItem, error) {
	var out []PlaylistTrackItem
	limit := 50
	offset := 0
	for {
		q := url.Values{"limit": {itoa(limit)}, "offset": {itoa(offset)}}
		var page struct {
			Items []PlaylistTrackItem `json:"items"`
			Next  string              `json:"next"`
			Total int                 `json:"total"`
		}
		if err := c.get(ctx, secret, "/playlists/"+url.PathEscape(id)+"/items", &page, q); err != nil {
			return nil, err
		}
		for i := range page.Items {
			if page.Items[i].Item.ID != "" {
				page.Items[i].Track = page.Items[i].Item
			}
		}
		out = append(out, page.Items...)
		offset += limit
		if page.Next == "" || offset >= page.Total || len(page.Items) == 0 {
			break
		}
	}
	return out, nil
}

// AllSavedTracks fetches the user's liked songs, handling paging.
func (c *Client) AllSavedTracks(ctx context.Context, secret string) ([]SavedTrack, error) {
	var out []SavedTrack
	limit := 50
	offset := 0
	for {
		q := url.Values{"limit": {itoa(limit)}, "offset": {itoa(offset)}}
		var page struct {
			Items []SavedTrack `json:"items"`
			Next  string       `json:"next"`
			Total int          `json:"total"`
		}
		if err := c.get(ctx, secret, "/me/tracks", &page, q); err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		offset += limit
		if page.Next == "" || offset >= page.Total || len(page.Items) == 0 {
			break
		}
	}
	return out, nil
}

// AllSavedShows fetches the user's saved podcast shows.
func (c *Client) AllSavedShows(ctx context.Context, secret string) ([]SavedShow, error) {
	var out []SavedShow
	limit := 50
	offset := 0
	for {
		q := url.Values{"limit": {itoa(limit)}, "offset": {itoa(offset)}}
		var page struct {
			Items []SavedShow `json:"items"`
			Next  string      `json:"next"`
			Total int         `json:"total"`
		}
		if err := c.get(ctx, secret, "/me/shows", &page, q); err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		offset += limit
		if page.Next == "" || offset >= page.Total || len(page.Items) == 0 {
			break
		}
	}
	return out, nil
}

// ShowEpisodes fetches episodes of one podcast show.
func (c *Client) ShowEpisodes(ctx context.Context, secret, id string) ([]Episode, error) {
	var out []Episode
	limit := 50
	offset := 0
	for {
		q := url.Values{"limit": {itoa(limit)}, "offset": {itoa(offset)}}
		var page struct {
			Items []Episode `json:"items"`
			Next  string    `json:"next"`
			Total int       `json:"total"`
		}
		if err := c.get(ctx, secret, "/shows/"+url.PathEscape(id)+"/episodes", &page, q); err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		offset += limit
		if page.Next == "" || offset >= page.Total || len(page.Items) == 0 {
			break
		}
	}
	return out, nil
}

// SavedEpisodes fetches the user's saved podcast episodes.
func (c *Client) SavedEpisodes(ctx context.Context, secret string) ([]Episode, error) {
	var out []Episode
	limit := 50
	offset := 0
	for {
		q := url.Values{"limit": {itoa(limit)}, "offset": {itoa(offset)}}
		var page struct {
			Items []struct {
				AddedAt string  `json:"added_at"`
				Episode Episode `json:"episode"`
			} `json:"items"`
			Next  string `json:"next"`
			Total int    `json:"total"`
		}
		if err := c.get(ctx, secret, "/me/episodes", &page, q); err != nil {
			return nil, err
		}
		for _, it := range page.Items {
			out = append(out, it.Episode)
		}
		offset += limit
		if page.Next == "" || offset >= page.Total || len(page.Items) == 0 {
			break
		}
	}
	return out, nil
}

// TopTracks fetches the user's top tracks (short/medium/long term).
func (c *Client) TopTracks(ctx context.Context, secret, term string, limit int) ([]Track, error) {
	q := url.Values{"limit": {itoa(limit)}, "time_range": {term}}
	var page struct {
		Items []Track `json:"items"`
	}
	if err := c.get(ctx, secret, "/me/top/tracks", &page, q); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// RecentlyPlayed fetches the last played tracks.
func (c *Client) RecentlyPlayed(ctx context.Context, secret string, limit int) ([]struct {
	Track      Track  `json:"track"`
	PlayedAt   string `json:"played_at"`
	ContextURI string `json:"context"`
}, error) {
	q := url.Values{"limit": {itoa(limit)}}
	var page struct {
		Items []struct {
			Track      Track  `json:"track"`
			PlayedAt   string `json:"played_at"`
			ContextURI string `json:"context"`
		} `json:"items"`
	}
	if err := c.get(ctx, secret, "/me/player/recently-played", &page, q); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// Search performs a catalog search (used by the AI tools).
func (c *Client) Search(ctx context.Context, secret, q, types string, limit int) (map[string]any, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}
	query := url.Values{
		"q":     {q},
		"type":  {types},
		"limit": {itoa(limit)},
	}
	var out map[string]any
	if err := c.get(ctx, secret, "/search", &out, query); err != nil {
		return nil, err
	}
	return out, nil
}

// CreatePlaylist creates a playlist for the authorized Spotify user.
func (c *Client) CreatePlaylist(ctx context.Context, secret, _ string, name, description string, isPublic bool) (*Playlist, error) {
	tok, err := c.Token(ctx, secret)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"name":        name,
		"description": description,
		"public":      isPublic,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/me/playlists", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		return nil, &APIError{Status: res.StatusCode, Message: e.Error.Message}
	}
	var pl Playlist
	if err := json.NewDecoder(res.Body).Decode(&pl); err != nil {
		return nil, err
	}
	return &pl, nil
}

// AddTracksToPlaylist appends up to 100 track URIs per request, chunked.
func (c *Client) AddTracksToPlaylist(ctx context.Context, secret, playlistID string, trackIDs []string) error {
	tok, err := c.Token(ctx, secret)
	if err != nil {
		return err
	}
	for start := 0; start < len(trackIDs); start += 100 {
		end := start + 100
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		uris := make([]string, 0, end-start)
		for _, id := range trackIDs[start:end] {
			uris = append(uris, "spotify:track:"+id)
		}
		body, _ := json.Marshal(map[string]any{"uris": uris})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/playlists/"+url.PathEscape(playlistID)+"/items", strings.NewReader(string(body)))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent)
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		res.Body.Close()
		if res.StatusCode >= 400 {
			var e struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.NewDecoder(res.Body).Decode(&e)
			return &APIError{Status: res.StatusCode, Message: e.Error.Message}
		}
	}
	return nil
}

// RemoveTracksFromPlaylist removes occurrences of track URIs.
func (c *Client) RemoveTracksFromPlaylist(ctx context.Context, secret, playlistID string, trackIDs []string) error {
	tok, err := c.Token(ctx, secret)
	if err != nil {
		return err
	}
	tracks := make([]map[string]string, 0, len(trackIDs))
	for _, id := range trackIDs {
		tracks = append(tracks, map[string]string{"uri": "spotify:track:" + id})
	}
	body, _ := json.Marshal(map[string]any{"items": tracks})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiBase+"/playlists/"+url.PathEscape(playlistID)+"/items", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		return &APIError{Status: res.StatusCode, Message: e.Error.Message}
	}
	return nil
}

// ReorderPlaylistTracks moves a track to a new position.
func (c *Client) ReorderPlaylistTracks(ctx context.Context, secret, playlistID string, rangeStart, insertBefore int) error {
	tok, err := c.Token(ctx, secret)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"range_start":   rangeStart,
		"insert_before": insertBefore,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiBase+"/playlists/"+url.PathEscape(playlistID)+"/items", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		return &APIError{Status: res.StatusCode, Message: e.Error.Message}
	}
	return nil
}

// DeletePlaylist removes a playlist from the current user's library.
func (c *Client) DeletePlaylist(ctx context.Context, secret, playlistID string) error {
	tok, err := c.Token(ctx, secret)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"uris": []string{"spotify:playlist:" + playlistID}})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiBase+"/me/library", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		return &APIError{Status: res.StatusCode, Message: e.Error.Message}
	}
	return nil
}

// StartPlayback starts playback of track URIs on the active device.
func (c *Client) StartPlayback(ctx context.Context, secret string, trackIDs []string) error {
	tok, err := c.Token(ctx, secret)
	if err != nil {
		return err
	}
	uris := make([]string, 0, len(trackIDs))
	for _, id := range trackIDs {
		uris = append(uris, "spotify:track:"+id)
	}
	body, _ := json.Marshal(map[string]any{"uris": uris})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiBase+"/me/player/play", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		return &APIError{Status: res.StatusCode, Message: e.Error.Message}
	}
	return nil
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
