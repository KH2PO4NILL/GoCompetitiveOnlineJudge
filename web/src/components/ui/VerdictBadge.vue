<script setup lang="ts">
import type { SubmissionSummary } from '../../api/contracts/app'
import { resultLabel, verdictTone } from '../../lib/judge'
import Badge from './Badge.vue'

defineProps<{ submission: SubmissionSummary }>()
</script>

<template>
  <Badge :tone="verdictTone(submission)" :running="submission.status === 'judging'">{{ resultLabel(submission) }}</Badge>
</template>
