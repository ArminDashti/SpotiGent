<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api'
import type { Playlist, Track } from '../types'

const playlists = ref<Playlist[]>([])
const loading = ref(true)
const error = ref('')
const expanded = ref<Set<string>>(new Set())

onMounted(async () => {
  try {
    const res = await api.playlists()
    playlists.value = res.playlists
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

function toggle(id: string) {
  const next = new Set(expanded.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  expanded.value = next
}

function fmtDuration(ms: number): string {
  const m = Math.floor(ms / 60000)
  const s = Math.floor((ms % 60000) / 1000)
  return `${m}:${String(s).padStart(2, '0')}`
}

// Sort state is shared by every expanded playlist table so the whole
// grid behaves consistently; all columns are sortable.
const sortKey = ref('index')
const sortAsc = ref(true)

function setSort(key: string) {
  if (sortKey.value === key) {
    sortAsc.value = !sortAsc.value
  } else {
    sortKey.value = key
    sortAsc.value = true
  }
}

function indicator(key: string): string {
  if (sortKey.value !== key) return '↕'
  return sortAsc.value ? '↑' : '↓'
}

type TrackValue = (row: Track, index: number) => string | number
const accessors: Record<string, TrackValue> = {
  index: (_row, index) => index,
  artists: (row) => row.artists.join(', '),
  playlists: (row) => row.playlists.join(', '),
  duration_ms: (row) => row.duration_ms,
}

function sortedFor(p: Playlist): Track[] {
  const accessor = accessors[sortKey.value]
  const value = (row: Track, index: number): string | number => {
    if (accessor) return accessor(row, index)
    const raw = (row as unknown as Record<string, unknown>)[sortKey.value]
    if (typeof raw === 'number') return raw
    if (raw == null) return ''
    if (Array.isArray(raw)) return raw.map(String).join(', ')
    return String(raw)
  }
  const dir = sortAsc.value ? 1 : -1
  return p.tracks
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const av = value(a.row, a.index)
      const bv = value(b.row, b.index)
      if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir
      return String(av).localeCompare(String(bv), undefined, { numeric: true, sensitivity: 'base' }) * dir
    })
    .map((r) => r.row)
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="error" class="card p-4 text-sm text-amber-400">{{ error }}</div>
    <div v-if="loading" class="text-sm text-muted">Loading playlists…</div>

    <section v-for="p in playlists" :key="p.id" class="card overflow-hidden">
      <button class="w-full flex items-center gap-4 p-4 text-left hover:bg-surface2/60" @click="toggle(p.id)">
        <img v-if="p.image" :src="p.image" alt="" class="w-14 h-14 rounded-lg object-cover" loading="lazy" />
        <span v-else class="w-14 h-14 rounded-lg bg-surface flex items-center justify-center text-2xl">📚</span>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="font-semibold truncate">{{ p.name }}</span>
            <span v-if="!p.public" class="pill">private</span>
          </div>
          <div class="text-xs text-muted truncate">
            {{ p.owner }} · {{ p.track_count }} tracks
            <template v-if="p.description"> · {{ p.description }}</template>
          </div>
        </div>
        <span class="text-muted">{{ expanded.has(p.id) ? '▾' : '▸' }}</span>
      </button>

      <div v-if="expanded.has(p.id)" class="border-t border-line overflow-x-auto">
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
            <tr v-for="(t, i) in sortedFor(p)" :key="t.id + i" class="border-b border-line/50 hover:bg-surface2/60">
              <td class="td text-muted">{{ i + 1 }}</td>
              <td class="td font-medium">{{ t.name }}</td>
              <td class="td text-muted">{{ t.artists.join(', ') }}</td>
              <td class="td text-muted">{{ t.album }}</td>
              <td class="td">
                <span class="pill">{{ t.playlists[0] }}</span>
              </td>
              <td class="td text-right text-muted tabular-nums">{{ fmtDuration(t.duration_ms) }}</td>
            </tr>
            <tr v-if="p.tracks.length === 0">
              <td class="td text-muted text-center" colspan="6">This playlist is empty or unreadable.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <div v-if="!loading && playlists.length === 0 && !error" class="card p-6 text-sm text-muted">
      No playlists found. Add your Spotify API key in Settings first.
    </div>
  </div>
</template>
