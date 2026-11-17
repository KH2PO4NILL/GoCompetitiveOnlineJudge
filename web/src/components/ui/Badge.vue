<script setup lang="ts">
defineProps<{ tone?: 'neutral' | 'success' | 'danger' | 'warning' | 'accent' | 'muted'; running?: boolean }>()
</script>

<template>
  <span class="badge" :class="[`badge-${tone ?? 'neutral'}`, { 'badge-running': running }]">
    <span v-if="running" class="dot" aria-hidden="true" />
    <slot />
  </span>
</template>
