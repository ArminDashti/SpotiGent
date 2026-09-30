# SpotiGent 🎧

Manage your Spotify account with AI — **Gin** (Go) backend, **Vue 3 + Tailwind** frontend.

SpotiGent wraps the Spotify Web API behind a local web UI and an AI agent (OpenRouter or OpenCode Go) that can actually act on your account: search tracks, create playlists, add/remove songs, and start playback via tool calls.

## Features

- **Dashboard** — playlist/track stats, listening time, top artists, recently played
- **AI** — chat with an agent that manages your library through tools
- **Musics** — every unique track across all playlists (Song, Artists, Album, Playlists)
- **Playlists** — playlists with expandable track tables (Song, Artists, Album, Playlists)
- **Podcast** — saved shows with their latest episodes
- **Settings** — Spotify OAuth, AI provider + key + model, 8 UI themes (incl. Dark+)
- **PWA** — installable app with offline shell (manifest + service worker)

## Quick start

Requirements: Go 1.26+, Node 20+.

```bash
# 1. Backend
go run ./cmd/spotigent            # serves API + built UI on http://127.0.0.1:8080

# 2. Frontend (first time, then after UI changes)
cd web && npm install && npm run build && cd ..

# 3. Open http://127.0.0.1:8080
```

For frontend development with hot reload:

```bash
go run ./cmd/spotigent &          # API on :8080
cd web && npm run dev             # Vite on :5173, proxies /api to :8080
```

## Setup

1. **Spotify**: create an app in the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard). In **Settings**, enter its **Client ID** and **Client Secret**. Add the exact Redirect URI shown there to your Spotify app, save, then choose **Save & connect Spotify** and approve access. SpotiGent uses Spotify's Authorization Code flow and refreshes tokens automatically. Spotify refresh tokens expire after six months; reconnect when asked. Spotify Development Mode apps require the app owner to have Premium.
2. **AI**: in Settings, pick **OpenRouter** (key from openrouter.ai), **OpenCode Go** ($10/mo plan, key from opencode.ai/auth), **OpenAI** (key from platform.openai.com), **Mistral** (key from console.mistral.ai), **Claude** (key from console.anthropic.com) or **Google** Gemini (key from AI Studio), paste the key, choose a model, save. Env fallbacks: `OPENROUTER_API_KEY`, `OPENCODE_API_KEY`, `OPENAI_API_KEY`, `MISTRAL_API_KEY`, `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`.

Credentials and tokens are stored in `data/settings.json` (created at runtime,
git-ignored via `data/`).

## Release build (Windows)

`scripts/build-release.ps1` produces a signed `release/spotigent.exe` and a SHA256 checksum manifest:

- embeds CompanyName **Dashti Technologies LLC**, product metadata, and the app icon
- stamps the version (`-Version 1.1.0`)
- Authenticode-signs every executable with a Dashti Technologies LLC code-signing certificate; the signature and file hash use SHA256. `-CertThumbprint` selects a certificate when multiple are installed; the build fails if no suitable certificate is available.
- writes `release/spotigent.exe.sha256` and `release/CHECKSUMS.sha256` after signing (SHA256 for every artifact)
- `scripts/installer-win-x64.ps1` signs before installing and writes a matching `SpotiGent.exe.sha256` sidecar

Pass `-OutputDirectory <path>` to stage a release without overwriting the default `release/` folder.

```powershell
.\scripts\build-release.ps1 -Version 1.1.0
```

Antivirus/SmartScreen notes: a hash lets users verify a download but does not
by itself stop warnings — only a real certificate issued to Dashti Technologies
LLC plus built-up reputation does. For false positives, submit the file to
[Microsoft Security Intelligence](https://www.microsoft.com/en-us/wdsi/filesubmission).
Always rebuild `spotigent.exe` after pulling backend changes; a stale exe
serving a newer `web/dist` (or vice versa) causes confusing Settings errors.

## PWA

The UI ships a `manifest.webmanifest`, offline-capable `sw.js`, and icons, so
SpotiGent is installable. The service worker caches the app shell only —
`/api/*` traffic always goes to the network. Served with correct
`Cache-Control: no-cache` (worker) and MIME (manifest) headers by the Go server.

## Configuration (env)

| Variable | Default | Purpose |
|---|---|---|
| `SPOTIGENT_HOST` | `127.0.0.1` | listen host |
| `SPOTIGENT_PORT` | `8080` | listen port (or `-port`) |
| `OPENROUTER_API_KEY` | — | fallback if no UI key |
| `OPENCODE_API_KEY` | — | fallback if no UI key |
| `OPENAI_API_KEY` | — | fallback if no UI key |
| `MISTRAL_API_KEY` | — | fallback if no UI key |
| `ANTHROPIC_API_KEY` | — | fallback if no UI key (also `CLAUDE_API_KEY`) |
| `GOOGLE_API_KEY` | — | fallback if no UI key (also `GEMINI_API_KEY`) |
| `OPENROUTER_BASE_URL` | `https://openrouter.ai/api/v1` | override for proxies |
| `OPENCODE_BASE_URL` | `https://opencode.ai/zen/go/v1` | override for proxies |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | override for proxies |
| `MISTRAL_BASE_URL` | `https://api.mistral.ai/v1` | override for proxies |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | override for proxies |
| `GOOGLE_BASE_URL` | `https://generativelanguage.googleapis.com/v1beta/openai` | override for proxies |

## AI tools exposed to the model

`list_playlists`, `search_tracks`, `search_shows`, `create_playlist`, `add_tracks_to_playlist`, `remove_tracks_from_playlist`, `play_tracks`, `recommend_from_library`

Try: *"Create a focus playlist with 20 deep house tracks"* — the agent searches, creates the playlist, fills it, and reports back.

## API overview

| Route | Purpose |
|---|---|
| `GET/PUT /api/settings` | read / update settings |
| `GET /api/settings/spotify/login` | start Spotify authorization |
| `GET /api/settings/spotify/callback` | handle Spotify OAuth callback |
| `GET /api/dashboard` | stats, top artists, recent |
| `GET /api/musics` | unique tracks across playlists |
| `GET /api/playlists` | playlists with nested tracks |
| `GET /api/podcasts` | saved shows + latest episodes |
| `POST /api/ai/chat` | run the AI agent |
| `POST /api/refresh` | invalidate + rebuild library cache |

## Project layout

```
cmd/spotigent/       entrypoint
internal/config/     env config
internal/store/      settings persistence (JSON)
internal/spotify/    Spotify Web API client (OAuth, paging, actions)
internal/ai/         OpenRouter/OpenCode/OpenAI/Mistral/Claude/Google providers + agent + tools
internal/server/     Gin routes + SPA static serving
web/                 Vue 3 + Vite + Tailwind frontend
```
