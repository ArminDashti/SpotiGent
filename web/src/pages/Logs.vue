<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api'
import { useSortable } from '../composables/useSortable'
import type { LogEntry } from '../types'

const entries = ref<LogEntry[]>([])
const loading = ref(true)
const error = ref('')
const level = ref<'' | 'info' | 'warning' | 'error'>('')
const autoRefresh = ref(true)
let timer: number | undefined

async function load() {
  try {
    const res = await api.logs()
    entries.value = res.entries
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

const counts = computed(() => ({
  info: entries.value.filter((e) => e.level === 'info').length,
  warning: entries.value.filter((e) => e.level === 'warning').length,
  error: entries.value.filter((e) => e.level === 'error').length,
}))

const filtered = computed(() =>
  level.value ? entries.value.filter((e) => e.level === level.value) : entries.value,
)

function setLevel(l: '' | 'info' | 'warning' | 'error') {
  level.value = l
}

function fmtTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
}

const levelRank: Record<LogEntry['level'], number> = { info: 0, warning: 1, error: 2 }

// All columns sortable: Time, Level, Message.
const { setSort, indicator, sorted } = useSortable(() => filtered.value, {
  defaultKey: 'time',
  defaultAsc: false,
  accessors: {
    time: (row) => row.time,
    level: (row) => levelRank[row.level],
    message: (row) => row.message,
  },
})

onMounted(() => {
  void load()
  timer = window.setInterval(() => {
    if (autoRefresh.value) void load()
  }, 5000)
})

onUnmounted(() => {
  if (timer !== undefined) window.clearInterval(timer)
})

const levelBadge: Record<LogEntry['level'], string> = {
  info: 'text-sky-400 border-sky-400/40',
  warning: 'text-amber-400 border-amber-400/40',
  error: 'text-red-400 border-red-400/40',
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="error" class="card p-4 text-sm text-amber-400">{{ error }}</div>

    <div class="flex items-center justify-between gap-3 flex-wrap">
      <div class="flex gap-2">
        <button class="btn-ghost text-xs" :class="{ 'ring-2 ring-accent/50': level === '' }" @click="setLevel('')">
          All ({{ entries.length }})
        </button>
        <button class="btn-ghost text-xs" :class="{ 'ring-2 ring-accent/50': level === 'info' }" @click="setLevel('info')">
          ℹ️ Info ({{ counts.info }})
        </button>
        <button class="btn-ghost text-xs" :class="{ 'ring-2 ring-accent/50': level === 'warning' }" @click="setLevel('warning')">
          ⚠️ Warning ({{ counts.warning }})
        </button>
        <button class="btn-ghost text-xs" :class="{ 'ring-2 ring-accent/50': level === 'error' }" @click="setLevel('error')">
          🔴 Error ({{ counts.error }})
        </button>
      </div>
      <div class="flex items-center gap-3">
        <label class="flex items-center gap-1 text-xs text-muted cursor-pointer">
          <input v-model="autoRefresh" type="checkbox" class="accent-accent" />
          Auto-refresh
        </label>
        <button class="btn-ghost text-xs" @click="load">⟳ Refresh</button>
      </div>
    </div>

    <div v-if="loading" class="text-sm text-muted">Loading logs…</div>

    <div v-else class="card overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="table-head border-b border-line">
          <tr>
            <th class="th w-48 cursor-pointer select-none" @click="setSort('time')">Time {{ indicator('time') }}</th>
            <th class="th w-24 cursor-pointer select-none" @click="setSort('level')">Level {{ indicator('level') }}</th>
            <th class="th cursor-pointer select-none" @click="setSort('message')">Message {{ indicator('message') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(e, i) in sorted" :key="i" class="border-b border-line/50 hover:bg-surface2/60">
            <td class="td text-muted whitespace-nowrap tabular-nums">{{ fmtTime(e.time) }}</td>
            <td class="td">
              <span
                class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs border uppercase"
                :class="levelBadge[e.level]"
              >{{ e.level }}</span>
            </td>
            <td class="td" :class="e.level === 'error' ? 'text-red-400' : e.level === 'warning' ? 'text-amber-400' : 'text-content'">
              {{ e.message }}
            </td>
          </tr>
          <tr v-if="sorted.length === 0">
            <td class="td text-muted text-center" colspan="3">No log entries.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
