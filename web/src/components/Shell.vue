<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { ui } from '../stores/ui'

const route = useRoute()

const nav = [
  { to: '/', label: 'Dashboard', icon: '🏠' },
  { to: '/ai', label: 'AI', icon: '✨' },
  { to: '/musics', label: 'Musics', icon: '🎵' },
  { to: '/playlists', label: 'Playlists', icon: '📚' },
  { to: '/podcast', label: 'Podcast', icon: '🎙️' },
  { to: '/history', label: 'History', icon: '🕘' },
  { to: '/logs', label: 'Logs', icon: '📜' },
  { to: '/settings', label: 'Settings', icon: '⚙️' },
]

const title = computed(() => (route.meta.title as string) ?? 'SpotiGent')
const subtitle = computed(() => (route.meta.subtitle as string) ?? '')
const spotifyReady = computed(() => ui.settings?.spotify_configured ?? false)
const refreshing = ref(false)

async function refresh() {
  refreshing.value = true
  try {
    await api.refresh()
  } finally {
    refreshing.value = false
  }
}
</script>

<template>
  <div class="flex h-full gap-4 p-4">
    <!-- Sidebar -->
    <aside class="hidden md:flex w-60 shrink-0 flex-col rounded-2xl border border-line bg-surface2 p-4">
      <div class="flex items-center gap-3 px-2 pb-6 pt-2">
        <span class="flex h-10 w-10 items-center justify-center rounded-xl bg-accent text-xl text-accentfg">🎧</span>
        <div>
          <div class="font-bold leading-tight">SpotiGent</div>
          <div class="text-xs text-muted">AI Spotify Manager</div>
        </div>
      </div>

      <nav class="flex-1 space-y-1">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors"
          :class="route.path === item.to ? 'bg-accent/15 text-accent font-semibold' : 'text-muted hover:bg-surface hover:text-content'"
        >
          <span class="w-5 text-center">{{ item.icon }}</span>
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="mt-4 rounded-xl border border-line bg-surface px-3 py-2.5 text-xs text-muted">
        <div v-if="spotifyReady" class="flex items-center gap-2">
          <span class="inline-block h-2 w-2 rounded-full bg-accent"></span>
          Spotify connected
        </div>
        <div v-else class="flex items-center gap-2">
          <span class="inline-block h-2 w-2 rounded-full bg-amber-400"></span>
          <span>
            Spotify key missing —
            <RouterLink to="/settings" class="underline hover:text-content">Settings</RouterLink>
          </span>
        </div>
      </div>
    </aside>

    <!-- Main -->
    <div class="flex min-w-0 flex-1 flex-col rounded-2xl border border-line bg-surface2">
      <header class="flex items-center justify-between gap-4 border-b border-line px-6 py-4">
        <div class="flex min-w-0 items-center gap-3">
          <span class="text-xl md:hidden">🎧</span>
          <div class="min-w-0">
            <h1 class="truncate text-lg font-bold leading-tight">{{ title }}</h1>
            <p v-if="subtitle" class="truncate text-xs text-muted">{{ subtitle }}</p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <RouterLink to="/settings" class="btn-ghost rounded-full md:hidden">⚙️</RouterLink>
          <button class="btn-ghost rounded-full" :disabled="refreshing" @click="refresh">
            <span :class="{ 'animate-spin': refreshing }">⟳</span>
            Refresh
          </button>
          <span class="hidden h-9 w-9 items-center justify-center rounded-full bg-accent text-sm font-semibold text-accentfg md:flex">
            🎧
          </span>
        </div>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>
