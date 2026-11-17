<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

const route = useRoute()
const admin = computed(() => route.meta.scope === 'admin')
</script>

<template>
  <div class="state-block" style="padding-block: 96px">
    <h1 style="margin: 0">404</h1>
    <p style="margin: 0">{{ admin ? '管理页面不存在。' : '页面不存在。' }}</p>
    <RouterLink class="btn quiet sm" :to="admin ? '/admin' : '/'">返回{{ admin ? '管理工作台' : '首页' }}</RouterLink>
  </div>
</template>
