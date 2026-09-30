import { reactive } from 'vue'
import { api } from '../api'
import type { PublicSettings } from '../types'

export const ui = reactive({
  settings: null as PublicSettings | null,
  loadingSettings: true,

  applyTheme(theme?: string) {
    const t = theme || this.settings?.theme || 'dark'
    document.documentElement.setAttribute('data-theme', t)
  },

  async loadSettings() {
    this.loadingSettings = true
    try {
      this.settings = await api.getSettings()
      this.applyTheme()
    } finally {
      this.loadingSettings = false
    }
  },
})
