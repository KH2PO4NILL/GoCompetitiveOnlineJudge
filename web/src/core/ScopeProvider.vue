<script setup lang="ts">
import { onBeforeUnmount, onMounted, provide } from 'vue'
import { VUE_QUERY_CLIENT } from '@tanstack/vue-query'
import { runtimeKey } from './keys'
import type { Runtime } from './runtime'

const props = defineProps<{ runtime: Runtime }>()

provide(runtimeKey, props.runtime)
provide(VUE_QUERY_CLIENT, props.runtime.queryClient)

// 显式 mount/unmount，使窗口聚焦/网络恢复时的自动刷新订阅生效
onMounted(() => props.runtime.queryClient.mount())
onBeforeUnmount(() => props.runtime.queryClient.unmount())
</script>

<template>
  <slot />
</template>
