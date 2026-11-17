<script setup lang="ts">
defineProps<{ page: number; hasNext: boolean; total?: number | bigint; disabled?: boolean }>()
const emit = defineEmits<{ prev: []; next: [] }>()
</script>

<template>
  <div class="pager">
    <span v-if="total !== undefined" class="page-indicator">共 {{ total }} 条</span>
    <span v-else></span>
    <span class="pager-controls">
      <button class="quiet sm" type="button" :disabled="disabled || page <= 1" @click="emit('prev')">上一页</button>
      <span class="page-indicator" aria-live="polite">第 {{ page }} 页</span>
      <button class="quiet sm" type="button" :disabled="disabled || !hasNext" @click="emit('next')">下一页</button>
    </span>
  </div>
</template>
