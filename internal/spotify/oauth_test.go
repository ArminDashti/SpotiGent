package spotify

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestExchangeCodeUsesAuthorizationCodeGrant(t *testing.T) {
	clientID, clientSecret := "client-id", "client-secret"
	sp := &Client{HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != tokenURL {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret))
		if got := req.Header.Get("Authorization"); got != wantAuth {
			t.Errorf("Authorization = %q, want %q", got, wantAuth)
		}
		if got := req.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", got)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{
			"grant_type":   "authorization_code",
			"code":         "one-time-code",
			"redirect_uri": "http://127.0.0.1:8080/callback",
		} {
			if got := req.Form.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)),
			Header:     make(http.Header),
		}, nil
	})}}

	tokens, err := sp.ExchangeCode(context.Background(), clientID, clientSecret, "http://127.0.0.1:8080/callback", "one-time-code")
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh" || tokens.ExpiresIn != 3600 {
		t.Fatalf("unexpected token response: %#v", tokens)
	}
}

func TestRefreshUsesRefreshTokenGrant(t *testing.T) {
	sp := &Client{HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := req.Form.Get("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q", got)
		}
		if got := req.Form.Get("refresh_token"); got != "saved-refresh" {
			t.Errorf("refresh_token = %q", got)
		}
		if _, exists := req.Form["password"]; exists {
			t.Error("password grant field must not be sent")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","expires_in":3600}`)),
			Header:     make(http.Header),
		}, nil
	})}}

	tokens, err := sp.RefreshAccessToken(context.Background(), "id", "secret", "saved-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "new-access" || tokens.RefreshToken != "" {
		t.Fatalf("unexpected token response: %#v", tokens)
	}
}

func TestTokenReturnsSuppliedAccessToken(t *testing.T) {
	sp := New()
	got, err := sp.Token(context.Background(), "bearer-token")
	if err != nil || got != "bearer-token" {
		t.Fatalf("Token() = %q, %v", got, err)
	}
	if _, err := sp.Token(context.Background(), " "); err == nil {
		t.Fatal("Token() accepted an empty access token")
	}
}

func TestCurrentPlaylistEndpointsAndPayloads(t *testing.T) {
	sp := &Client{HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload string
		switch req.URL.Path {
		case "/v1/me/playlists":
			payload = `{"items":[{"id":"playlist","name":"Mix","items":{"total":1}}],"total":1}`
		case "/v1/playlists/playlist/items":
			payload = `{"items":[{"added_at":"2026-01-01T00:00:00Z","item":{"id":"track","name":"Song"}}],"total":1}`
		default:
			t.Fatalf("unexpected Spotify endpoint: %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(payload)),
			Header:     make(http.Header),
		}, nil
	})}}

	playlists, err := sp.AllPlaylists(context.Background(), "access")
	if err != nil {
		t.Fatal(err)
	}
	if len(playlists) != 1 || playlists[0].ItemCount() != 1 {
		t.Fatalf("unexpected playlists: %#v", playlists)
	}
	items, err := sp.PlaylistTracks(context.Background(), "access", "playlist")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Track.ID != "track" {
		t.Fatalf("unexpected playlist items: %#v", items)
	}
}
