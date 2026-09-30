package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"spotigent/internal/spotify"
)

// Catalog is a lightweight in-memory snapshot of the user's library that
// tools can query without hammering the Spotify API on every step.
type Catalog struct {
	Playlists []spotify.Playlist
	// Tracks maps playlist ID -> tracks in that playlist.
	Tracks map[string][]spotify.PlaylistTrackItem
	// AllTracks is the union across playlists (used by the Musics page).
	AllTracks []PlaylistTrack
}

// PlaylistTrack is one track plus the playlist it appeared in.
type PlaylistTrack struct {
	PlaylistID   string `json:"playlist_id"`
	PlaylistName string `json:"playlist_name"`
	AddedAt      string `json:"added_at"`
	spotify.Track
}

// Tools returns the OpenAI-style tool definitions exposed to the model.
func Tools() []Tool {
	tools := []Tool{
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "list_playlists",
				Description: "List all playlists in the user's Spotify library with names, IDs, track counts and owners.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "search_tracks",
				Description: "Search the Spotify catalog for tracks by name, artist or album. Returns track IDs usable by other tools.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Free text search query, e.g. 'artist:Daft Punk track:One More Time'",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of results (1-50, default 10)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "search_shows",
				Description: "Search the Spotify catalog for podcasts (shows) by name or publisher.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Free text search query",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of results (1-50, default 10)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "create_playlist",
				Description: "Create a new playlist in the user's account and optionally add tracks to it in the same step.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Playlist name",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "Short playlist description (optional)",
						},
						"public": map[string]any{
							"type":        "boolean",
							"description": "Whether the playlist is public (default false)",
						},
						"track_ids": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Spotify track IDs to add after creation (optional)",
						},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "add_tracks_to_playlist",
				Description: "Add existing tracks (by Spotify track ID) to one of the user's playlists by playlist ID.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playlist_id": map[string]any{"type": "string", "description": "Playlist ID (from list_playlists)"},
						"track_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
							"description": "Spotify track IDs to add",
						},
					},
					"required": []string{"playlist_id", "track_ids"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "remove_tracks_from_playlist",
				Description: "Remove tracks (by Spotify track ID) from one of the user's playlists.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playlist_id": map[string]any{"type": "string", "description": "Playlist ID (from list_playlists)"},
						"track_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
							"description": "Spotify track IDs to remove",
						},
					},
					"required": []string{"playlist_id", "track_ids"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "play_tracks",
				Description: "Start playback of the given tracks on the user's active Spotify device.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"track_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
							"description": "Spotify track IDs to play",
						},
					},
					"required": []string{"track_ids"},
				},
			},
		},
		{
			Type: "function",
			Function: struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}{
				Name:        "recommend_from_library",
				Description: "Analyze the user's playlists and surface candidate tracks for recommendations. Use this before building recommendation playlists.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playlist_name": map[string]any{
							"type":        "string",
							"description": "Optional: restrict analysis to the playlist with this name",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Max number of candidate seeds to return (default 20)",
						},
					},
				},
			},
		},
	}
	return tools
}

// Agent executes the model <-> tool loop until the model produces a
// final answer or the step budget is exhausted.
type Agent struct {
	SP      *spotify.Client
	Secret  string
	Cfg     Config
	Catalog *Catalog
	UserID  string

	MaxSteps int
}

// Run processes one user message and returns the assistant's final text.
func (a *Agent) Run(ctx context.Context, history []Message, userMsg string) (string, error) {
	if a.MaxSteps == 0 {
		a.MaxSteps = 8
	}

	messages := make([]Message, 0, len(history)+2)
	messages = append(messages, Message{Role: "system", Content: systemPrompt()})
	messages = append(messages, history...)
	messages = append(messages, Message{Role: "user", Content: userMsg})

	tools := Tools()

	for step := 0; step < a.MaxSteps; step++ {
		res, err := Complete(ctx, a.Cfg, chatRequest{
			Model:       a.Cfg.Model,
			Messages:    messages,
			Tools:       tools,
			MaxTokens:   2048,
			Temperature: 0.4,
		})
		if err != nil {
			return "", err
		}

		msg := res.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			return strings.TrimSpace(msg.Content), nil
		}

		for _, tc := range msg.ToolCalls {
			result := a.callTool(ctx, tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}
	return "", fmt.Errorf("agent exceeded %d tool steps without a final answer", a.MaxSteps)
}

// callTool dispatches one tool call and always returns a JSON string.
func (a *Agent) callTool(ctx context.Context, name, rawArgs string) string {
	var args map[string]any
	if strings.TrimSpace(rawArgs) != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return toolErr("invalid arguments JSON: " + err.Error())
		}
	}

	secret := a.Secret
	sp := a.SP

	switch name {
	case "list_playlists":
		out := make([]map[string]any, 0, len(a.Catalog.Playlists))
		for _, p := range a.Catalog.Playlists {
			out = append(out, map[string]any{
				"id":          p.ID,
				"name":        p.Name,
				"tracks":      p.Tracks.Total,
				"owner":       p.Owner.DisplayName,
				"public":      p.Public,
				"description": p.Description,
			})
		}
		return toolOK(map[string]any{"playlists": out, "count": len(out)})

	case "search_tracks":
		q, _ := args["query"].(string)
		if q == "" {
			return toolErr("query is required")
		}
		limit := intArg(args, "limit", 10)
		res, err := sp.Search(ctx, secret, q, "track", limit)
		if err != nil {
			return toolErr(err.Error())
		}
		return toolOK(simplifyTracks(res))

	case "search_shows":
		q, _ := args["query"].(string)
		if q == "" {
			return toolErr("query is required")
		}
		limit := intArg(args, "limit", 10)
		res, err := sp.Search(ctx, secret, q, "show", limit)
		if err != nil {
			return toolErr(err.Error())
		}
		return toolOK(simplifyShows(res))

	case "create_playlist":
		plName, _ := args["name"].(string)
		if plName == "" {
			return toolErr("name is required")
		}
		desc, _ := args["description"].(string)
		isPublic, _ := args["public"].(bool)
		pl, err := sp.CreatePlaylist(ctx, secret, a.UserID, plName, desc, isPublic)
		if err != nil {
			return toolErr(err.Error())
		}
		if rawIDs, ok := args["track_ids"].([]any); ok && len(rawIDs) > 0 {
			ids := toStrings(rawIDs)
			if err := sp.AddTracksToPlaylist(ctx, secret, pl.ID, ids); err != nil {
				return toolOK(map[string]any{
					"playlist": pl,
					"warning":  "created but failed to add tracks: " + err.Error(),
				})
			}
		}
		return toolOK(map[string]any{"playlist": pl, "status": "created"})

	case "add_tracks_to_playlist":
		plID, _ := args["playlist_id"].(string)
		rawIDs, _ := args["track_ids"].([]any)
		if plID == "" || len(rawIDs) == 0 {
			return toolErr("playlist_id and track_ids are required")
		}
		if err := sp.AddTracksToPlaylist(ctx, secret, plID, toStrings(rawIDs)); err != nil {
			return toolErr(err.Error())
		}
		return toolOK(map[string]any{"status": "added", "count": len(rawIDs)})

	case "remove_tracks_from_playlist":
		plID, _ := args["playlist_id"].(string)
		rawIDs, _ := args["track_ids"].([]any)
		if plID == "" || len(rawIDs) == 0 {
			return toolErr("playlist_id and track_ids are required")
		}
		if err := sp.RemoveTracksFromPlaylist(ctx, secret, plID, toStrings(rawIDs)); err != nil {
			return toolErr(err.Error())
		}
		return toolOK(map[string]any{"status": "removed", "count": len(rawIDs)})

	case "play_tracks":
		rawIDs, _ := args["track_ids"].([]any)
		if len(rawIDs) == 0 {
			return toolErr("track_ids is required")
		}
		if err := sp.StartPlayback(ctx, secret, toStrings(rawIDs)); err != nil {
			return toolErr(err.Error())
		}
		return toolOK(map[string]any{"status": "playing", "count": len(rawIDs)})

	case "recommend_from_library":
		plName, _ := args["playlist_name"].(string)
		limit := intArg(args, "limit", 20)

		var source []PlaylistTrack
		for _, t := range a.Catalog.AllTracks {
			if plName == "" || strings.EqualFold(t.PlaylistName, plName) {
				source = append(source, t)
			}
		}
		if len(source) == 0 {
			return toolOK(map[string]any{"candidates": []any{}, "note": "no tracks matched; is the library loaded?"})
		}

		// Rank by popularity, sprinkle in some deep cuts for variety.
		sort.SliceStable(source, func(i, j int) bool {
			return source[i].Popularity > source[j].Popularity
		})
		step := len(source) / limit
		if step < 1 {
			step = 1
		}
		candidates := make([]map[string]any, 0, limit)
		for i := 0; i < len(source) && len(candidates) < limit; i += step {
			t := source[i]
			artistNames := make([]string, 0, len(t.Artists))
			for _, ar := range t.Artists {
				artistNames = append(artistNames, ar.Name)
			}
			candidates = append(candidates, map[string]any{
				"id":       t.ID,
				"name":     t.Name,
				"artists":  artistNames,
				"album":    t.Album.Name,
				"playlist": t.PlaylistName,
			})
		}
		return toolOK(map[string]any{"candidates": candidates})

	default:
		return toolErr("unknown tool: " + name)
	}
}

// ---- helpers ----

func systemPrompt() string {
	return `You are SpotiGent, an assistant embedded in a Spotify management web app.

You manage the signed-in user's Spotify account through tools: listing playlists, searching tracks and podcasts, creating playlists, adding/removing tracks, and starting playback.

Rules:
- Prefer tools over guessing. Track IDs come from search_tracks or recommend_from_library.
- When the user asks for a playlist on a theme (e.g. "focus", "gym"), search for fitting tracks, then create_playlist with those track_ids.
- Keep answers concise. Summarize what you did with names and counts.
- If a tool call fails, explain the error in plain words.`
}

func toolOK(v any) string {
	b, _ := json.Marshal(map[string]any{"ok": true, "data": v})
	return string(b)
}

func toolErr(msg string) string {
	b, _ := json.Marshal(map[string]any{"ok": false, "error": msg})
	return string(b)
}

func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key].(float64); ok && v > 0 {
		return int(v)
	}
	return def
}

func toStrings(raw []any) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// simplifyTracks trims a raw /search response to what the model needs.
func simplifyTracks(res map[string]any) any {
	tracksObj, ok := res["tracks"].(map[string]any)
	if !ok {
		return map[string]any{"tracks": []any{}}
	}
	items, _ := tracksObj["items"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		t, ok := it.(map[string]any)
		if !ok {
			continue
		}
		artists := []string{}
		if rawArtists, ok := t["artists"].([]any); ok {
			for _, ra := range rawArtists {
				if ar, ok := ra.(map[string]any); ok {
					artists = append(artists, fmt.Sprint(ar["name"]))
				}
			}
		}
		album := ""
		if alb, ok := t["album"].(map[string]any); ok {
			album = fmt.Sprint(alb["name"])
		}
		out = append(out, map[string]any{
			"id":      t["id"],
			"name":    t["name"],
			"artists": artists,
			"album":   album,
		})
	}
	return map[string]any{"tracks": out}
}

// simplifyShows trims a raw /search (type=show) response.
func simplifyShows(res map[string]any) any {
	showsObj, ok := res["shows"].(map[string]any)
	if !ok {
		return map[string]any{"shows": []any{}}
	}
	items, _ := showsObj["items"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		s, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"id":        s["id"],
			"name":      s["name"],
			"publisher": s["publisher"],
		})
	}
	return map[string]any{"shows": out}
}

// BuildCatalog fetches playlists and all their tracks into a Catalog.
func BuildCatalog(ctx context.Context, sp *spotify.Client, secret string) (*Catalog, error) {
	playlists, err := sp.AllPlaylists(ctx, secret)
	if err != nil {
		return nil, err
	}
	cat := &Catalog{Playlists: playlists, Tracks: map[string][]spotify.PlaylistTrackItem{}}
	for _, p := range playlists {
		items, err := sp.PlaylistTracks(ctx, secret, p.ID)
		if err != nil {
			continue // skip unreadable playlists rather than failing the whole catalog
		}
		cat.Tracks[p.ID] = items
		for _, item := range items {
			if item.Track.ID == "" {
				continue // local files / removed tracks have no ID
			}
			cat.AllTracks = append(cat.AllTracks, PlaylistTrack{
				PlaylistID:   p.ID,
				PlaylistName: p.Name,
				AddedAt:      item.AddedAt,
				Track:        item.Track,
			})
		}
	}
	return cat, nil
}

// ensure url import is used even if future edits drop query usage
var _ = url.QueryEscape

// ensure time import is used
var _ = time.Second
