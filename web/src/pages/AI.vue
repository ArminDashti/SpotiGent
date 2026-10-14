<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import type { ChatMessage, ChatSession } from '../types'
import { ui } from '../stores/ui'

const history = ref<ChatMessage[]>([])
const input = ref('')
const busy = ref(false)
const error = ref('')
const listEl = ref<HTMLElement | null>(null)

const sessions = ref<ChatSession[]>([])
const currentSessionId = ref('')
const loadingHistory = ref(false)

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
const suggestionIcons = ['🎵', '📊', '🏃', '🎙️']

watch(history, () => {
  nextTick(() => listEl.value?.scrollTo({ top: listEl.value.scrollHeight, behavior: 'smooth' }))
}, { deep: true })

async function loadSessions() {
  try {
    const res = await api.chats()
    sessions.value = res.chats
  } catch {
    /* history sidebar is non-critical */
  }
}

async function openSession(sess: ChatSession) {
  if (busy.value) return
  loadingHistory.value = true
  error.value = ''
  try {
    const full = await api.chatSession(sess.id)
    currentSessionId.value = full.id
    history.value = (full.messages ?? []).map((m) => ({ role: m.role, content: m.content }))
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loadingHistory.value = false
  }
}

function newChat() {
  if (busy.value) return
  currentSessionId.value = ''
  history.value = []
  error.value = ''
}

async function removeSession(id: string) {
  try {
    await api.deleteChat(id)
    if (currentSessionId.value === id) newChat()
    await loadSessions()
  } catch {
    /* keep the list as-is on failure */
  }
}

function fmtWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
}

onMounted(loadSessions)

async function send(text?: string) {
  const message = (text ?? input.value).trim()
  if (!message || busy.value) return
  input.value = ''
  error.value = ''
  history.value.push({ role: 'user', content: message })
  busy.value = true
  try {
    const res = await api.chat(message, history.value.slice(0, -1), currentSessionId.value)
    currentSessionId.value = res.session_id || currentSessionId.value
    history.value.push({ role: 'assistant', content: res.reply })
    await loadSessions()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    history.value.push({ role: 'assistant', content: `⚠️ ${error.value}` })
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex h-full min-h-[60vh] gap-4">
    <!-- History sidebar -->
    <aside class="hidden w-64 shrink-0 flex-col rounded-2xl border border-line bg-surface p-3 md:flex">
      <button class="btn-primary mb-4 w-full rounded-xl" :disabled="busy" @click="newChat">
        <span>⊕</span> New chat
      </button>
      <div class="mb-2 flex items-center gap-2 px-2 text-xs uppercase tracking-wider text-muted">
        <span>🕘</span> History
      </div>
      <div class="flex-1 space-y-1 overflow-y-auto">
        <div
          v-for="s in sessions"
          :key="s.id"
          class="group flex cursor-pointer items-center gap-1 rounded-xl px-3 py-2.5 text-sm transition-colors"
          :class="s.id === currentSessionId ? 'bg-accent/15 text-accent font-semibold' : 'text-muted hover:bg-surface2 hover:text-content'"
          @click="openSession(s)"
        >
          <span class="min-w-0 flex-1">
            <span class="block truncate">{{ s.title }}</span>
            <span class="block truncate text-xs opacity-70">{{ fmtWhen(s.updated_at) }}</span>
          </span>
          <button
            class="px-1 text-xs opacity-0 hover:text-red-400 group-hover:opacity-100"
            title="Delete chat"
            @click.stop="removeSession(s.id)"
          >
            ✕
          </button>
        </div>
        <div v-if="sessions.length === 0" class="px-2 py-3 text-xs text-muted">
          No saved conversations yet — chats are stored automatically.
        </div>
      </div>
    </aside>

    <!-- Conversation -->
    <div class="flex min-w-0 flex-1 flex-col">
      <div class="mb-4 flex items-center justify-between">
        <span class="pill gap-2 rounded-full px-3 py-1.5">
          <span class="inline-block h-2 w-2 rounded-full bg-accent"></span>
          {{ providerLabel }}
        </span>
        <div class="flex gap-2">
          <button class="btn-ghost rounded-full md:hidden" @click="newChat">＋ New</button>
          <button class="btn-ghost rounded-full" @click="newChat">Clear chat</button>
        </div>
      </div>

      <div ref="listEl" class="flex min-h-[40vh] flex-1 flex-col overflow-y-auto rounded-2xl border border-line bg-surface p-6">
        <div v-if="loadingHistory" class="flex h-full items-center justify-center text-sm text-muted">
          Loading conversation…
        </div>

        <div
          v-else-if="history.length === 0"
          class="flex h-full flex-col items-center justify-center gap-5 text-center"
        >
          <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-accent/15 text-3xl">✨</div>
          <div>
            <h2 class="text-xl font-bold">Ask the AI to manage your Spotify</h2>
            <p class="mx-auto mt-2 max-w-lg text-sm text-muted">
              It can search tracks, create playlists, add or remove songs, and start playback using the tools wired
              to your account. Conversations are saved to your history.
            </p>
          </div>
          <div class="grid w-full max-w-2xl grid-cols-1 gap-3 sm:grid-cols-2">
            <button
              v-for="(s, i) in suggestions"
              :key="s"
              class="flex items-center gap-3 rounded-xl border border-line bg-surface2 px-4 py-3 text-left text-sm transition-colors hover:border-accent/50 hover:bg-surface2/80"
              @click="send(s)"
            >
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-accent/15">{{ suggestionIcons[i] }}</span>
              <span>{{ s }}</span>
            </button>
          </div>
        </div>

        <div v-else class="space-y-3">
          <template v-for="(m, i) in history" :key="i">
            <div class="flex" :class="m.role === 'user' ? 'justify-end' : 'justify-start'">
              <div
                class="max-w-[75%] whitespace-pre-wrap rounded-2xl border px-4 py-2.5 text-sm"
                :class="m.role === 'user' ? 'border-accent/30 bg-accent/15' : 'border-line bg-surface2'"
              >
                {{ m.content }}
                <span v-if="busy && i === history.length - 1 && m.role === 'user'" class="ml-1 animate-pulse">▋</span>
              </div>
            </div>
          </template>
        </div>
      </div>

      <div class="mt-4 flex items-center gap-2 rounded-2xl border border-line bg-surface p-2 pl-4 focus-within:ring-2 focus-within:ring-accent/50">
        <input
          v-model="input"
          class="min-w-0 flex-1 bg-transparent py-2 text-sm text-content placeholder:text-muted focus:outline-none"
          placeholder="e.g. Create a 'Focus@Work' playlist with 20 deep house tracks"
          :disabled="busy"
          @keydown.enter="send()"
        />
        <button class="btn-primary rounded-xl px-5" :disabled="busy || !input.trim()" @click="send()">
          <span>▷</span>
          {{ busy ? 'Thinking…' : 'Send' }}
        </button>
      </div>
    </div>
  </div>
</template>
