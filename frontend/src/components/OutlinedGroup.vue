<template>
  <fieldset class="outlined-group">
    <legend><slot name="label">{{ label }}</slot></legend>
    <slot />
  </fieldset>
</template>

<script setup lang="ts">
defineProps<{ label?: string }>()
</script>
