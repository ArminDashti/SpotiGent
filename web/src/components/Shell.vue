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
  <div class="flex h-full">
    <!-- Sidebar -->
    <aside class="hidden md:flex w-56 shrink-0 flex-col border-r border-line bg-surface2">
      <div class="flex items-center gap-2 px-5 py-5">
        <span class="text-2xl">🎧</span>
        <div>
          <div class="font-bold leading-tight">SpotiGent</div>
          <div class="text-xs text-muted">AI Spotify Manager</div>
        </div>
      </div>
      <nav class="flex-1 px-3 space-y-1">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors"
          :class="route.path === item.to ? 'bg-accent/15 text-accent font-semibold' : 'text-muted hover:bg-surface hover:text-content'"
        >
          <span>{{ item.icon }}</span>
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <div class="p-4 text-xs text-muted border-t border-line">
        <div v-if="spotifyReady" class="flex items-center gap-2">
          <span class="inline-block w-2 h-2 rounded-full bg-accent"></span>
          Spotify connected
        </div>
        <div v-else class="flex items-center gap-2">
          <span class="inline-block w-2 h-2 rounded-full bg-amber-400"></span>
          <span>
            Spotify key missing —
            <RouterLink to="/settings" class="underline hover:text-content">Settings</RouterLink>
          </span>
        </div>
      </div>
    </aside>

    <!-- Main -->
    <div class="flex-1 flex flex-col min-w-0">
      <header class="flex items-center justify-between border-b border-line px-6 py-4">
        <div class="flex items-center gap-3">
          <span class="md:hidden text-xl">🎧</span>
          <h1 class="text-lg font-bold">{{ title }}</h1>
        </div>
        <div class="flex items-center gap-2">
          <RouterLink to="/settings" class="btn-ghost md:hidden">⚙️</RouterLink>
          <button class="btn-ghost" :disabled="refreshing" @click="refresh">
            <span :class="{ 'animate-spin': refreshing }">⟳</span>
            Refresh
          </button>
        </div>
      </header>
      <main class="flex-1 overflow-y-auto p-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>
