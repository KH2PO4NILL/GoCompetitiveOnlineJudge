<script setup lang="ts">
import { computed } from 'vue'
import { difficultyTones, labelOf, difficulties } from '../../lib/labels'
import Badge from './Badge.vue'

type Tone = 'neutral' | 'success' | 'danger' | 'warning' | 'accent' | 'muted'

const props = defineProps<{ difficulty?: string }>()
const tone = computed<Tone>(() => (difficultyTones[props.difficulty ?? ''] ?? 'muted') as Tone)
const label = computed(() => labelOf(difficulties, props.difficulty))
</script>

<template>
  <Badge :tone="tone">{{ label }}</Badge>
</template>
