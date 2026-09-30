<script setup lang="ts">
import { onMounted } from 'vue'
import Shell from './components/Shell.vue'
import { ui } from './stores/ui'

onMounted(() => {
  ui.loadSettings().catch(() => {
    /* backend unreachable — pages will surface errors */
  })
})
</script>

<template>
  <Shell />
</template>
