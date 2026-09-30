<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { api } from '../api'
import type { ChatMessage } from '../types'
import { ui } from '../stores/ui'

const history = ref<ChatMessage[]>([])
const input = ref('')
const busy = ref(false)
const error = ref('')
const listEl = ref<HTMLElement | null>(null)

const providerLabel = computed(() => {
  const s = ui.settings
  if (!s) return 'AI'
  const names: Record<string, string> = {
    openrouter: 'OpenRouter',
    opencode: 'OpenCode Go',
    openai: 'OpenAI',
    mistral: 'Mistral',
    claude: 'Claude',
    google: 'Google',
  }
  const keyFlags: Record<string, boolean> = {
    openrouter: !!s.openrouter_set,
    opencode: !!s.opencode_set,
    openai: !!s.openai_set,
    mistral: !!s.mistral_set,
    claude: !!s.claude_set,
    google: !!s.google_set,
  }
  const name = names[s.provider] ?? s.provider
  return `${name} · ${keyFlags[s.provider] ? 'key ✓' : 'key missing'}`
})

const suggestions = [
  'Create a chill lo-fi playlist with 15 tracks',
  'Which artists appear most across my playlists?',
  'Make a workout playlist from my favorite genres',
  'Find podcasts about technology and list them',
]

watch(history, () => {
  nextTick(() => listEl.value?.scrollTo({ top: listEl.value.scrollHeight, behavior: 'smooth' }))
}, { deep: true })

async function send(text?: string) {
  const message = (text ?? input.value).trim()
  if (!message || busy.value) return
  input.value = ''
  error.value = ''
  history.value.push({ role: 'user', content: message })
  busy.value = true
  try {
    const res = await api.chat(message, history.value.slice(0, -1))
    history.value.push({ role: 'assistant', content: res.reply })
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    history.value.push({ role: 'assistant', content: `⚠️ ${error.value}` })
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full min-h-[60vh]">
    <div class="flex items-center justify-between mb-3">
      <span class="pill">{{ providerLabel }}</span>
      <button class="btn-ghost" @click="history = []">Clear chat</button>
    </div>

    <div ref="listEl" class="flex-1 card p-4 space-y-3 overflow-y-auto min-h-[40vh]">
      <div v-if="history.length === 0" class="h-full flex flex-col items-center justify-center text-center text-muted gap-3">
        <div class="text-4xl">✨</div>
        <div class="text-sm max-w-md">
          Ask the AI to manage your Spotify — it can search tracks, create playlists, add or remove songs, and start
          playback using the tools wired to your account.
        </div>
        <div class="flex flex-wrap gap-2 justify-center">
          <button v-for="s in suggestions" :key="s" class="btn-ghost text-xs" @click="send(s)">{{ s }}</button>
        </div>
      </div>

      <template v-for="(m, i) in history" :key="i">
        <div class="flex" :class="m.role === 'user' ? 'justify-end' : 'justify-start'">
          <div
            class="max-w-[75%] rounded-2xl px-4 py-2 text-sm whitespace-pre-wrap border"
            :class="m.role === 'user' ? 'bg-accent/15 border-accent/30' : 'bg-surface border-line'"
          >
            {{ m.content }}
            <span v-if="busy && i === history.length - 1 && m.role === 'user'" class="ml-1 animate-pulse">▋</span>
          </div>
        </div>
      </template>
    </div>

    <div class="mt-3 flex gap-2">
      <input
        v-model="input"
        class="input flex-1"
        placeholder="e.g. Create a 'Focus@Work' playlist with 20 deep house tracks"
        :disabled="busy"
        @keydown.enter="send()"
      />
      <button class="btn-primary" :disabled="busy || !input.trim()" @click="send()">
        {{ busy ? 'Thinking…' : 'Send' }}
      </button>
    </div>
  </div>
</template>
