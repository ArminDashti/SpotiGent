import type { ChatMessage, ChatSession, DashboardData, HistoryItem, LogEntry, Playlist, Provider, PublicSettings, Show, Track } from './types'

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

  chat: (message: string, history: ChatMessage[], sessionId?: string) =>
    request<{ reply: string; session_id: string }>('/api/ai/chat', {
      method: 'POST',
      body: JSON.stringify({ message, history, session_id: sessionId ?? '' }),
    }),

  chats: () => request<{ chats: ChatSession[] }>('/api/chats'),

  chatSession: (id: string) => request<ChatSession>(`/api/chats/${id}`),

  deleteChat: (id: string) => request<{ ok: boolean }>(`/api/chats/${id}`, { method: 'DELETE' }),

  history: () => request<{ items: HistoryItem[]; total: number }>('/api/history'),

  logs: (level?: string) =>
    request<{ entries: LogEntry[]; total: number }>(`/api/logs${level ? `?level=${level}` : ''}`),

  refresh: () => request<{ ok: boolean }>('/api/refresh', { method: 'POST' }),
}

export type { ChatMessage, ChatSession, DashboardData, HistoryItem, LogEntry, Playlist, Provider, PublicSettings, Show, Track }
