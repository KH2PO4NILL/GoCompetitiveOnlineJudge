<script setup lang="ts">
import { computed } from 'vue'
import { describeError } from '../../lib/errors'

const props = defineProps<{ error: unknown }>()
const described = computed(() => describeError(props.error))
</script>

<template>
  <p v-if="described" class="notice notice-error" role="alert">
    <span>
      {{ described.message }}
      <template v-if="described.hint"> {{ described.hint }}</template>
      <template v-if="described.uncertain"> 本次操作结果未知：请先核对服务端结果，确认未生效后再重试，避免重复执行。</template>
    </span>
  </p>
</template>
