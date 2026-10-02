<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import { useSortable } from '../composables/useSortable'
import type { Track } from '../types'

const tracks = ref<Track[]>([])
const total = ref(0)
const loading = ref(true)
const error = ref('')
const query = ref('')

onMounted(async () => {
  try {
    const res = await api.musics()
    tracks.value = res.tracks
    total.value = res.total
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

function fmtDuration(ms: number): string {
  const m = Math.floor(ms / 60000)
  const s = Math.floor((ms % 60000) / 1000)
  return `${m}:${String(s).padStart(2, '0')}`
}

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return tracks.value
  return tracks.value.filter(
    (t) =>
      t.name.toLowerCase().includes(q) ||
      t.album.toLowerCase().includes(q) ||
      t.artists.some((a) => a.toLowerCase().includes(q)) ||
      t.playlists.some((p) => p.toLowerCase().includes(q)),
  )
})

// Every column is sortable: #, Song, Artists, Album, Playlists, Time.
const { setSort, indicator, sorted } = useSortable(() => filtered.value, {
  defaultKey: 'name',
  accessors: {
    index: (_row, index) => index,
    artists: (row) => row.artists.join(', '),
    playlists: (row) => row.playlists.join(', '),
    duration_ms: (row) => row.duration_ms,
  },
})
</script>

<template>
  <div class="space-y-4">
    <div v-if="error" class="card p-4 text-sm text-amber-400">{{ error }}</div>

    <div class="flex items-center justify-between gap-3 flex-wrap">
      <input v-model="query" class="input w-72" placeholder="Search song, artist, album or playlist…" />
      <span class="text-sm text-muted">
        {{ filtered.length }} of {{ total }} unique tracks
      </span>
    </div>

    <div v-if="loading" class="text-sm text-muted">Loading your library…</div>

    <div v-else class="card overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="table-head border-b border-line">
          <tr>
            <th class="th w-10 cursor-pointer select-none" @click="setSort('index')"># {{ indicator('index') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('name')">Song {{ indicator('name') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('artists')">Artists {{ indicator('artists') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('album')">Album {{ indicator('album') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('playlists')">Playlists {{ indicator('playlists') }}</th>
            <th class="th w-20 text-right cursor-pointer select-none" @click="setSort('duration_ms')">Time {{ indicator('duration_ms') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(t, i) in sorted" :key="t.id" class="border-b border-line/50 hover:bg-surface2/60">
            <td class="td text-muted">{{ i + 1 }}</td>
            <td class="td">
              <div class="flex items-center gap-3">
                <img
                  v-if="t.album_image"
                  :src="t.album_image"
                  alt=""
                  class="w-9 h-9 rounded object-cover"
                  loading="lazy"
                />
                <span v-else class="w-9 h-9 rounded bg-surface2 flex items-center justify-center">🎵</span>
                <span class="font-medium">{{ t.name }}</span>
              </div>
            </td>
            <td class="td text-muted">{{ t.artists.join(', ') }}</td>
            <td class="td text-muted">{{ t.album }}</td>
            <td class="td">
              <div class="flex flex-wrap gap-1">
                <span v-for="p in t.playlists" :key="p" class="pill">{{ p }}</span>
              </div>
            </td>
            <td class="td text-right text-muted tabular-nums">{{ fmtDuration(t.duration_ms) }}</td>
          </tr>
          <tr v-if="sorted.length === 0">
            <td class="td text-muted text-center" colspan="6">No tracks found.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
