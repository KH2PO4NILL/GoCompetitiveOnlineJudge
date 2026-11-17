<script setup lang="ts">
import { computed } from 'vue'
import { describeError } from '../../lib/errors'

const props = defineProps<{
  pending?: boolean
  error?: unknown
  empty?: boolean
  emptyTitle?: string
  pendingText?: string
}>()

const emit = defineEmits<{ retry: [] }>()
const described = computed(() => describeError(props.error))
</script>

<template>
  <div v-if="pending" class="state-block" role="status">
    <span class="spinner" aria-hidden="true" />
    <span>{{ pendingText ?? '加载中…' }}</span>
  </div>
  <div v-else-if="described" class="state-block error" role="alert">
    <span>
      {{ described.message }}
      <template v-if="described.hint"> {{ described.hint }}</template>
    </span>
    <button class="quiet sm" type="button" @click="emit('retry')">重试</button>
  </div>
  <div v-else-if="empty" class="state-block">
    <strong>{{ emptyTitle ?? '暂无数据' }}</strong>
    <slot />
  </div>
  <slot v-else name="content" />
</template>
