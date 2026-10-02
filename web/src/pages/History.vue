<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { useSortable } from '../composables/useSortable'
import type { HistoryItem } from '../types'

const items = ref<HistoryItem[]>([])
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    const res = await api.history()
    items.value = res.items
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

function fmtWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
}

// All columns sortable: #, Song, Artists, Album, Played, Time.
const { setSort, indicator, sorted } = useSortable(() => items.value, {
  defaultKey: 'played_at',
  defaultAsc: false,
  accessors: {
    index: (_row, index) => index,
    artists: (row) => row.artists.join(', '),
    duration_ms: (row) => row.duration_ms,
    played_at: (row) => row.played_at,
  },
})
</script>

<template>
  <div class="space-y-4">
    <div v-if="error" class="card p-4 text-sm text-amber-400">{{ error }}</div>

    <div class="flex items-center justify-between gap-3 flex-wrap">
      <span class="text-sm text-muted">Your last {{ items.length }} plays from Spotify</span>
    </div>

    <div v-if="loading" class="text-sm text-muted">Loading listening history…</div>

    <div v-else class="card overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="table-head border-b border-line">
          <tr>
            <th class="th w-10 cursor-pointer select-none" @click="setSort('index')"># {{ indicator('index') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('name')">Song {{ indicator('name') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('artists')">Artists {{ indicator('artists') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('album')">Album {{ indicator('album') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('played_at')">Played {{ indicator('played_at') }}</th>
            <th class="th w-20 text-right cursor-pointer select-none" @click="setSort('duration_ms')">Time {{ indicator('duration_ms') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(h, i) in sorted" :key="i" class="border-b border-line/50 hover:bg-surface2/60">
            <td class="td text-muted">{{ i + 1 }}</td>
            <td class="td">
              <div class="flex items-center gap-3">
                <img
                  v-if="h.album_image"
                  :src="h.album_image"
                  alt=""
                  class="w-9 h-9 rounded object-cover"
                  loading="lazy"
                />
                <span v-else class="w-9 h-9 rounded bg-surface2 flex items-center justify-center">🎧</span>
                <span class="font-medium">{{ h.name }}</span>
              </div>
            </td>
            <td class="td text-muted">{{ h.artists.join(', ') }}</td>
            <td class="td text-muted">{{ h.album }}</td>
            <td class="td text-muted whitespace-nowrap">{{ fmtWhen(h.played_at) }}</td>
            <td class="td text-right text-muted tabular-nums">{{ fmtDuration(h.duration_ms) }}</td>
          </tr>
          <tr v-if="sorted.length === 0">
            <td class="td text-muted text-center" colspan="6">No listening history yet — play something on Spotify.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
