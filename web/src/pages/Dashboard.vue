<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api'
import type { DashboardData } from '../types'

const data = ref<DashboardData | null>(null)
const error = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    data.value = await api.dashboard()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

function fmtDuration(ms: number): string {
  const mins = Math.round(ms / 60000)
  const days = Math.floor(mins / 1440)
  const hours = Math.floor((mins % 1440) / 60)
  const m = mins % 60
  if (days > 0) return `${days}d ${hours}h ${m}m`
  if (hours > 0) return `${hours}h ${m}m`
  return `${m}m`
}

function fmtWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
}

const stats = [
  { key: 'playlists', label: 'Playlists', icon: '📚' },
  { key: 'unique_tracks', label: 'Unique tracks', icon: '🎵' },
  { key: 'track_entries', label: 'Playlist entries', icon: '🧾' },
  { key: 'total_duration', label: 'Total listening time', icon: '⏱️' },
] as const
</script>

<template>
  <div class="space-y-6">
    <div v-if="error" class="card p-4 text-sm text-amber-400">{{ error }}</div>

    <template v-if="data">
      <!-- Stat cards -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div v-for="s in stats" :key="s.key" class="card p-5">
          <div class="text-2xl">{{ s.icon }}</div>
          <div class="mt-2 text-2xl font-bold">
            {{ s.key === 'total_duration' ? fmtDuration(data.total_duration) : data[s.key] }}
          </div>
          <div class="text-sm text-muted">{{ s.label }}</div>
        </div>
      </div>

      <div class="grid lg:grid-cols-2 gap-6">
        <!-- Top artists -->
        <section class="card p-5">
          <h2 class="font-semibold mb-4">Top artists (last 6 months)</h2>
          <div v-if="data.top_artists.length === 0" class="text-sm text-muted">No listening history found.</div>
          <ol v-else class="space-y-3">
            <li v-for="(a, i) in data.top_artists" :key="a.name" class="flex items-center gap-3">
              <span class="w-6 text-center text-sm font-bold text-accent">{{ i + 1 }}</span>
              <span class="flex-1 text-sm truncate">{{ a.name }}</span>
              <span class="pill">{{ a.plays }} in top tracks</span>
            </li>
          </ol>
        </section>

        <!-- Recently played -->
        <section class="card p-5">
          <h2 class="font-semibold mb-4">Recently played</h2>
          <div v-if="data.recent.length === 0" class="text-sm text-muted">Nothing recent yet.</div>
          <ul v-else class="space-y-3">
            <li v-for="(r, i) in data.recent" :key="i" class="flex items-center gap-3">
              <span class="text-lg">🎧</span>
              <div class="flex-1 min-w-0">
                <div class="text-sm truncate">{{ r.name }}</div>
                <div class="text-xs text-muted truncate">{{ r.artists.join(', ') }}</div>
              </div>
              <span class="text-xs text-muted whitespace-nowrap">{{ fmtWhen(r.played_at) }}</span>
            </li>
          </ul>
        </section>
      </div>

      <!-- Quick actions -->
      <section class="card p-5">
        <h2 class="font-semibold mb-3">Quick actions</h2>
        <div class="flex flex-wrap gap-2">
          <RouterLink to="/ai" class="btn-primary">✨ Ask AI to manage</RouterLink>
          <RouterLink to="/musics" class="btn-ghost">🎵 Browse musics</RouterLink>
          <RouterLink to="/playlists" class="btn-ghost">📚 Browse playlists</RouterLink>
          <RouterLink to="/podcast" class="btn-ghost">🎙️ Browse podcasts</RouterLink>
        </div>
      </section>
    </template>

    <div v-else-if="loading" class="text-sm text-muted">Loading dashboard…</div>
  </div>
</template>
