<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import ScopeProvider from './core/ScopeProvider.vue'
import type { Scope } from './api/client'
import { getRuntime } from './core/runtime'

const route = useRoute()
const scope = computed<Scope>(() => (route.meta.scope as Scope | undefined) ?? 'app')
const runtime = computed(() => getRuntime(scope.value))
</script>

<template>
  <ScopeProvider :key="scope" :runtime="runtime">
    <RouterView />
  </ScopeProvider>
</template>
