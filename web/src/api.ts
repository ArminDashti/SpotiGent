import type { ChatMessage, DashboardData, Playlist, Provider, PublicSettings, Show, Track } from './types'

class ApiError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch {
      /* keep default message */
    }
    throw new ApiError(msg)
  }
  return res.json() as Promise<T>
}

export const api = {
  getSettings: () => request<PublicSettings>('/api/settings'),

  updateSettings: (patch: Record<string, unknown>) =>
    request<PublicSettings>('/api/settings', { method: 'PUT', body: JSON.stringify(patch) }),

  dashboard: () => request<DashboardData>('/api/dashboard'),

  musics: () => request<{ tracks: Track[]; total: number }>('/api/musics'),

  playlists: () => request<{ playlists: Playlist[]; total: number }>('/api/playlists'),

  podcasts: () => request<{ shows: Show[]; total: number }>('/api/podcasts'),

  chat: (message: string, history: ChatMessage[]) =>
    request<{ reply: string }>('/api/ai/chat', {
      method: 'POST',
      body: JSON.stringify({ message, history }),
    }),

  refresh: () => request<{ ok: boolean }>('/api/refresh', { method: 'POST' }),
}

export type { ChatMessage, DashboardData, Playlist, Provider, PublicSettings, Show, Track }
