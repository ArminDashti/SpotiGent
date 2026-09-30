<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import { ui } from '../stores/ui'
import { THEMES, type Provider } from '../types'

const spotifyClientID = ref('')
const spotifyClientSecret = ref('')
const connectingSpotify = ref(false)
const testResult = ref<{ ok: boolean; message: string } | null>(null)

const provider = computed({
  get: () => ui.settings?.provider ?? 'openrouter',
  set: (v: Provider) => {
    if (ui.settings) ui.settings.provider = v
  },
})

const keys = ref<Record<Provider, string>>({
  openrouter: '',
  opencode: '',
  openai: '',
  mistral: '',
  claude: '',
  google: '',
})

const models = ref<Record<Provider, string>>({
  openrouter: '',
  opencode: '',
  openai: '',
  mistral: '',
  claude: '',
  google: '',
})

const modelOptions = ref<Record<Provider, string[]>>({
  openrouter: [],
  opencode: [],
  openai: [],
  mistral: [],
  claude: [],
  google: [],
})
const savingAI = ref(false)
const savedFlash = ref(false)

const PROVIDERS: { id: Provider; label: string; hint: string; logo?: string; keyPlaceholder: string }[] = [
  { id: 'openrouter', label: 'OpenRouter', hint: 'key from openrouter.ai', logo: '/providers/openrouter.svg', keyPlaceholder: 'sk-or-v1-…' },
  { id: 'opencode', label: 'OpenCode Go', hint: '$10/mo plan · opencode.ai/auth', keyPlaceholder: 'Go key from opencode.ai/auth' },
  { id: 'openai', label: 'OpenAI', hint: 'key from platform.openai.com', logo: '/providers/openai.svg', keyPlaceholder: 'sk-…' },
  { id: 'mistral', label: 'Mistral', hint: 'key from console.mistral.ai', logo: '/providers/mistral.svg', keyPlaceholder: 'Mistral API key' },
  { id: 'claude', label: 'Claude', hint: 'key from console.anthropic.com', logo: '/providers/claude.svg', keyPlaceholder: 'sk-ant-…' },
  { id: 'google', label: 'Google', hint: 'Gemini key from AI Studio', logo: '/providers/google.svg', keyPlaceholder: 'Gemini API key' },
]

const keySetFor = (id: Provider): boolean => {
  const s = ui.settings
  if (!s) return false
  return id === 'openrouter' ? s.openrouter_set
    : id === 'opencode' ? s.opencode_set
    : id === 'openai' ? s.openai_set
    : id === 'mistral' ? s.mistral_set
    : id === 'claude' ? s.claude_set
    : s.google_set
}

const themeOptions: { id: string; label: string; colors: string[] }[] = THEMES.map((id) => {
  const labels: Record<string, string> = {
    dark: 'Dark',
    darkplus: 'Dark+',
    light: 'Light',
    spotify: 'Spotify',
    ocean: 'Ocean',
    sunset: 'Sunset',
    rose: 'Rose',
    mono: 'Mono',
  }
  return { id, label: labels[id] ?? id, colors: [] }
})

const themePreview: Record<string, [string, string]> = {
  dark: ['#0f1115', '#1db954'],
  darkplus: ['#1e1e1e', '#007acc'],
  light: ['#f6f7f9', '#1db954'],
  spotify: ['#121212', '#1db954'],
  ocean: ['#0b1220', '#38bdf8'],
  sunset: ['#1a1210', '#f97316'],
  rose: ['#170f13', '#ec4899'],
  mono: ['#0a0a0a', '#e5e5e5'],
}

onMounted(async () => {
  await ui.loadSettings().catch(() => {})
  const spotifyStatus = new URLSearchParams(location.search)
  if (spotifyStatus.get('spotify') === 'connected') {
    testResult.value = { ok: true, message: 'Connected to Spotify.' }
    history.replaceState(null, '', location.pathname)
  } else if (spotifyStatus.has('spotify_error')) {
    testResult.value = { ok: false, message: `Spotify authorization failed: ${spotifyStatus.get('spotify_error')}` }
    history.replaceState(null, '', location.pathname)
  }
  if (ui.settings) {
    models.value.openrouter = ui.settings.openrouter_model
    models.value.opencode = ui.settings.opencode_model
    models.value.openai = ui.settings.openai_model
    models.value.mistral = ui.settings.mistral_model
    models.value.claude = ui.settings.claude_model
    models.value.google = ui.settings.google_model
    // Show the stored client ID so the field never looks empty after saving.
    // (The secret is never returned by the API, so it stays write-only.)
    spotifyClientID.value = ui.settings.spotify_client_id ?? ''
  }
  try {
    const res = await fetch('/api/models')
    if (res.ok) {
      const data = await res.json()
      for (const id of Object.keys(modelOptions.value) as Provider[]) {
        if (Array.isArray(data[id])) modelOptions.value[id] = data[id]
      }
    }
  } catch {
    /* backend defaults still work */
  }
})

async function connectSpotify() {
  connectingSpotify.value = true
  testResult.value = null
  try {
    // Fall back to the stored values so reconnecting works even when the
    // inputs are left untouched, and validate up front for a clear message.
    const clientID = spotifyClientID.value.trim() || ui.settings?.spotify_client_id || ''
    const clientSecret = spotifyClientSecret.value.trim()
    if (!clientID) throw new Error('Enter your Spotify client ID first.')
    if (!clientSecret && !ui.settings?.spotify_client_secret_set) {
      throw new Error('Enter your Spotify client secret first.')
    }
    const patch: Record<string, unknown> = { spotify_client_id: clientID }
    if (clientSecret) patch.spotify_client_secret = clientSecret
    const settings = await api.updateSettings(patch)
    await ui.loadSettings()
    if (!settings.spotify_client_id_set) {
      throw new Error('The client ID was not saved — the server may be out of disk space or data/settings.json may be unwritable. Check the server log and retry.')
    }
    if (!settings.spotify_client_secret_set) throw new Error('Enter your Spotify client secret first.')
    spotifyClientID.value = settings.spotify_client_id || clientID
    spotifyClientSecret.value = ''
    window.location.assign('/api/settings/spotify/login')
  } catch (e) {
    testResult.value = { ok: false, message: e instanceof Error ? e.message : String(e) }
  } finally {
    connectingSpotify.value = false
  }
}

const keyFieldFor: Record<Provider, string> = {
  openrouter: 'openrouter_api_key',
  opencode: 'opencode_api_key',
  openai: 'openai_api_key',
  mistral: 'mistral_api_key',
  claude: 'claude_api_key',
  google: 'google_api_key',
}

const modelFieldFor: Record<Provider, string> = {
  openrouter: 'openrouter_model',
  opencode: 'opencode_model',
  openai: 'openai_model',
  mistral: 'mistral_model',
  claude: 'claude_model',
  google: 'google_model',
}

async function saveAI() {
  savingAI.value = true
  try {
    const patch: Record<string, unknown> = { provider: provider.value }
    for (const id of Object.keys(models.value) as Provider[]) {
      if (models.value[id]?.trim()) patch[modelFieldFor[id]] = models.value[id].trim()
      if (keys.value[id]?.trim()) patch[keyFieldFor[id]] = keys.value[id].trim()
    }
    await api.updateSettings(patch)
    keys.value = { openrouter: '', opencode: '', openai: '', mistral: '', claude: '', google: '' }
    await ui.loadSettings()
    savedFlash.value = true
    setTimeout(() => (savedFlash.value = false), 2000)
  } finally {
    savingAI.value = false
  }
}

function setTheme(id: string) {
  ui.applyTheme(id)
  api
    .updateSettings({ theme: id })
    .then(() => ui.loadSettings())
    .catch(() => {})
}
</script>

<template>
  <div class="space-y-6 max-w-3xl">
    <!-- Appearance -->
    <section class="card p-5">
      <h2 class="font-semibold mb-1">Appearance</h2>
      <p class="text-xs text-muted mb-4">Pick a theme — it is saved to your settings.</p>
      <div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-3">
        <button
          v-for="t in themeOptions"
          :key="t.id"
          class="card p-3 text-center hover:ring-2 hover:ring-accent/50"
          :class="{ 'ring-2 ring-accent': ui.settings?.theme === t.id }"
          @click="setTheme(t.id)"
        >
          <div class="h-10 rounded mb-2 flex overflow-hidden border border-line">
            <span class="flex-1" :style="{ background: themePreview[t.id]?.[0] ?? '#333' }"></span>
            <span class="flex-1" :style="{ background: themePreview[t.id]?.[1] ?? '#666' }"></span>
          </div>
          <span class="text-xs">{{ t.label }}</span>
        </button>
      </div>
    </section>

    <!-- Spotify OAuth -->
    <section class="card p-5">
      <div class="flex items-center justify-between mb-1">
        <h2 class="font-semibold">Spotify account</h2>
        <span class="pill" :class="ui.settings?.spotify_configured ? 'text-accent' : 'text-amber-400'">
          {{ ui.settings?.spotify_configured ? 'connected' : 'not connected' }}
        </span>
      </div>
      <p class="text-xs text-muted mb-4">
        Enter your app's client ID and client secret, then authorize SpotiGent on Spotify.
        Register this exact Redirect URI in your
        <a href="https://developer.spotify.com/dashboard" target="_blank" rel="noopener" class="underline">Spotify Developer Dashboard</a>:
      </p>
      <code class="block text-xs break-all mb-4">{{ ui.settings?.spotify_redirect_uri }}</code>
      <div class="space-y-2">
        <input
          v-model="spotifyClientID"
          type="text"
          class="input w-full"
          placeholder="Spotify client ID"
          autocomplete="off"
        />
        <input
          v-model="spotifyClientSecret"
          type="password"
          class="input w-full"
          :placeholder="ui.settings?.spotify_client_secret_set ? 'Client secret stored — enter only to replace' : 'Spotify client secret'"
          autocomplete="new-password"
        />
        <button class="btn-primary" :disabled="connectingSpotify || (!spotifyClientID.trim() && !ui.settings?.spotify_client_id_set)" @click="connectSpotify">
          {{ connectingSpotify ? 'Opening Spotify…' : (ui.settings?.spotify_configured ? 'Reconnect Spotify' : 'Save & connect Spotify') }}
        </button>
      </div>
      <div v-if="testResult" class="mt-3 text-sm" :class="testResult.ok ? 'text-accent' : 'text-red-400'">
        {{ testResult.message }}
      </div>
    </section>

    <!-- AI providers -->
    <section class="card p-5">
      <h2 class="font-semibold mb-1">AI provider</h2>
      <p class="text-xs text-muted mb-4">
        Keys are stored server-side in <code>data/settings.json</code>. Env vars OPENROUTER_API_KEY /
        OPENCODE_API_KEY / OPENAI_API_KEY / MISTRAL_API_KEY / ANTHROPIC_API_KEY / GOOGLE_API_KEY
        act as fallbacks.
      </p>

      <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-3 mb-4">
        <button
          v-for="p in PROVIDERS"
          :key="p.id"
          class="card p-4 text-left hover:ring-2 hover:ring-accent/50"
          :class="{ 'ring-2 ring-accent': provider === p.id }"
          @click="provider = p.id"
        >
          <div class="flex items-center gap-2">
            <img v-if="p.logo" :src="p.logo" :alt="`${p.label} logo`" class="w-6 h-6 provider-logo" />
            <div class="font-medium">{{ p.label }}</div>
          </div>
          <div class="text-xs text-muted mt-1">{{ p.hint }}</div>
          <div class="text-xs mt-1" :class="keySetFor(p.id) ? 'text-accent' : 'text-muted'">
            {{ keySetFor(p.id) ? 'API key set ✓' : 'No key yet' }}
          </div>
        </button>
      </div>

      <div class="space-y-3">
        <div v-for="p in PROVIDERS" :key="p.id" v-show="provider === p.id">
          <label class="text-xs text-muted">{{ p.label }} API key</label>
          <input
            v-model="keys[p.id]"
            type="password"
            class="input w-full"
            :placeholder="keySetFor(p.id) ? 'Key stored — enter only to replace' : p.keyPlaceholder"
            autocomplete="off"
          />
        </div>

        <div>
          <label class="text-xs text-muted">Model</label>
          <select v-model="models[provider]" class="input w-full">
            <option v-for="m in modelOptions[provider]" :key="m" :value="m">{{ m }}</option>
          </select>
        </div>

        <div class="flex items-center gap-3">
          <button class="btn-primary" :disabled="savingAI" @click="saveAI">Save AI settings</button>
          <span v-if="savedFlash" class="text-sm text-accent">Saved ✓</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.provider-logo {
  color: currentColor;
  background: transparent;
}
</style>
