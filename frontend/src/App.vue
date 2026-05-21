<template>
  <RouterView />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'

import { applyTheme, getStoredTheme } from './utils/theme'

onMounted(() => {
  applyTheme(getStoredTheme())
})
</script>

