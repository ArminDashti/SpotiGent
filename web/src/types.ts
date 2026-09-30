export type Provider = 'openrouter' | 'opencode' | 'openai' | 'mistral' | 'claude' | 'google'

export interface PublicSettings {
  spotify_configured: boolean
  spotify_client_id: string
  spotify_client_id_set: boolean
  spotify_client_secret_set: boolean
  spotify_redirect_uri: string
  spotify_user_id: string
  provider: Provider
  openrouter_set: boolean
  opencode_set: boolean
  openai_set: boolean
  mistral_set: boolean
  claude_set: boolean
  google_set: boolean
  openrouter_model: string
  opencode_model: string
  openai_model: string
  mistral_model: string
  claude_model: string
  google_model: string
  theme: string
}

export interface Track {
  id: string
  name: string
  artists: string[]
  album: string
  album_image: string
  duration_ms: number
  playlists: string[]
  popularity: number
}

export interface Playlist {
  id: string
  name: string
  description: string
  owner: string
  track_count: number
  public: boolean
  image: string
  tracks: Track[]
}

export interface Episode {
  id: string
  name: string
  description: string
  duration_ms: number
  release_date: string
}

export interface Show {
  id: string
  name: string
  publisher: string
  image: string
  episodes: Episode[]
}

export interface DashboardData {
  playlists: number
  unique_tracks: number
  track_entries: number
  total_duration: number
  top_artists: { name: string; plays: number }[]
  recent: { name: string; artists: string[]; played_at: string }[]
}

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export const THEMES = ['dark', 'darkplus', 'light', 'spotify', 'ocean', 'sunset', 'rose', 'mono'] as const
export type Theme = (typeof THEMES)[number]
