<script setup lang="ts">
import { onScopeDispose, ref } from 'vue'
import { useApi } from '../../composables/useApi'
const api = useApi()
const message = ref(api.sessionMessage())
onScopeDispose(api.subscribe(() => { message.value = api.sessionMessage() }))
</script>
<template><p v-if="message" class="notice notice-warning" role="alert">{{ message }}</p></template>
