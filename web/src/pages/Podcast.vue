<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api'
import type { Show } from '../types'

const shows = ref<Show[]>([])
const loading = ref(true)
const error = ref('')
const openShow = ref<string | null>(null)

onMounted(async () => {
  try {
    const res = await api.podcasts()
    shows.value = res.shows
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

function toggle(id: string) {
  openShow.value = openShow.value === id ? null : id
}

function fmtDuration(ms: number): string {
  const h = Math.floor(ms / 3600000)
  const m = Math.round((ms % 3600000) / 60000)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="error" class="card p-4 text-sm text-amber-400">{{ error }}</div>
    <div v-if="loading" class="text-sm text-muted">Loading your podcasts…</div>

    <div v-else class="grid md:grid-cols-2 gap-4">
      <section v-for="s in shows" :key="s.id" class="card overflow-hidden">
        <div class="flex gap-4 p-4">
          <img v-if="s.image" :src="s.image" alt="" class="w-20 h-20 rounded-lg object-cover" loading="lazy" />
          <span v-else class="w-20 h-20 rounded-lg bg-surface flex items-center justify-center text-3xl">🎙️</span>
          <div class="flex-1 min-w-0">
            <div class="font-semibold truncate">{{ s.name }}</div>
            <div class="text-xs text-muted">{{ s.publisher }}</div>
            <button class="btn-ghost mt-3 text-xs" @click="toggle(s.id)">
              {{ openShow === s.id ? 'Hide' : 'Latest episodes' }}
            </button>
          </div>
        </div>

        <div v-if="openShow === s.id" class="border-t border-line divide-y divide-line/50 max-h-80 overflow-y-auto">
          <div v-for="ep in s.episodes" :key="ep.id" class="px-4 py-3 text-sm">
            <div class="flex items-start justify-between gap-3">
              <span class="font-medium">{{ ep.name }}</span>
              <span class="text-xs text-muted whitespace-nowrap">{{ ep.release_date }} · {{ fmtDuration(ep.duration_ms) }}</span>
            </div>
            <p class="text-xs text-muted mt-1 line-clamp-2" v-html="ep.description" />
          </div>
          <div v-if="s.episodes.length === 0" class="px-4 py-3 text-sm text-muted">No episodes loaded.</div>
        </div>
      </section>
    </div>

    <div v-if="!loading && shows.length === 0 && !error" class="card p-6 text-sm text-muted">
      No saved podcasts found in your library.
    </div>
  </div>
</template>
