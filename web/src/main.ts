import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'

createApp(App).use(router).mount('#app')

// PWA: register the service worker in production builds only, so `npm run
// dev` hot reload is never served stale from a worker cache.
if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {
      /* offline/PWA is best-effort */
    })
  })
}
