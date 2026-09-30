import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'dashboard', component: () => import('./pages/Dashboard.vue'), meta: { title: 'Dashboard' } },
  { path: '/ai', name: 'ai', component: () => import('./pages/AI.vue'), meta: { title: 'AI' } },
  { path: '/musics', name: 'musics', component: () => import('./pages/Musics.vue'), meta: { title: 'Musics' } },
  { path: '/playlists', name: 'playlists', component: () => import('./pages/Playlists.vue'), meta: { title: 'Playlists' } },
  { path: '/podcast', name: 'podcast', component: () => import('./pages/Podcast.vue'), meta: { title: 'Podcast' } },
  { path: '/settings', name: 'settings', component: () => import('./pages/Settings.vue'), meta: { title: 'Settings' } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export default createRouter({
  history: createWebHistory(),
  routes,
})
