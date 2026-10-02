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
  <div class="flex h-full gap-4 min-h-[60vh]">
    <!-- History sidebar -->
    <aside class="hidden md:flex w-60 shrink-0 flex-col card p-3">
      <button class="btn-primary w-full mb-3" :disabled="busy" @click="newChat">＋ New chat</button>
      <div class="text-xs uppercase tracking-wider text-muted px-1 mb-1">History</div>
      <div class="flex-1 overflow-y-auto space-y-1">
        <div
          v-for="s in sessions"
          :key="s.id"
          class="group flex items-center gap-1 rounded-lg px-2 py-2 text-sm cursor-pointer transition-colors"
          :class="s.id === currentSessionId ? 'bg-accent/15 text-accent font-semibold' : 'text-muted hover:bg-surface2 hover:text-content'"
          @click="openSession(s)"
        >
          <span class="flex-1 min-w-0">
            <span class="block truncate">{{ s.title }}</span>
            <span class="block text-xs opacity-70 truncate">{{ fmtWhen(s.updated_at) }}</span>
          </span>
          <button
            class="opacity-0 group-hover:opacity-100 text-xs hover:text-red-400 px-1"
            title="Delete chat"
            @click.stop="removeSession(s.id)"
          >
            ✕
          </button>
        </div>
        <div v-if="sessions.length === 0" class="text-xs text-muted px-1 py-2">
          No saved conversations yet — chats are stored automatically.
        </div>
      </div>
    </aside>

    <!-- Conversation -->
    <div class="flex-1 flex flex-col min-w-0">
      <div class="flex items-center justify-between mb-3">
        <span class="pill">{{ providerLabel }}</span>
        <div class="flex gap-2">
          <button class="btn-ghost md:hidden" @click="newChat">＋ New</button>
          <button class="btn-ghost" @click="newChat">Clear chat</button>
        </div>
      </div>

      <div ref="listEl" class="flex-1 card p-4 space-y-3 overflow-y-auto min-h-[40vh]">
        <div v-if="loadingHistory" class="h-full flex items-center justify-center text-sm text-muted">
          Loading conversation…
        </div>
        <div v-else-if="history.length === 0" class="h-full flex flex-col items-center justify-center text-center text-muted gap-3">
          <div class="text-4xl">✨</div>
          <div class="text-sm max-w-md">
            Ask the AI to manage your Spotify — it can search tracks, create playlists, add or remove songs, and start
            playback using the tools wired to your account. Conversations are saved to your history.
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
  </div>
</template>
