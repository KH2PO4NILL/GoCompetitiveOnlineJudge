<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps<{ title: string; message?: string; confirmText?: string; danger?: boolean; busy?: boolean }>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()

const confirmButton = ref<HTMLButtonElement>()
let restoreFocus: HTMLElement | null = null

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Tab') {
    const dialog = confirmButton.value?.closest('[role="alertdialog"]')
    const controls = Array.from(dialog?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex="0"]') ?? [])
    const first = controls[0]
    const last = controls.at(-1)
    if (event.shiftKey && (document.activeElement === first || !dialog?.contains(document.activeElement))) { event.preventDefault(); last?.focus() }
    else if (!event.shiftKey && (document.activeElement === last || !dialog?.contains(document.activeElement))) { event.preventDefault(); first?.focus() }
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    if (!props.busy) emit('cancel')
  }
}

onMounted(() => {
  restoreFocus = document.activeElement as HTMLElement | null
  confirmButton.value?.focus()
  document.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  restoreFocus?.focus?.()
})
</script>

<template>
  <Teleport to="body">
    <div class="dialog-backdrop" @click.self="!props.busy && emit('cancel')">
      <div class="dialog" role="alertdialog" aria-modal="true" :aria-label="props.title">
        <h2>{{ props.title }}</h2>
        <p v-if="props.message" class="dialog-message">{{ props.message }}</p>
        <slot />
        <div class="dialog-actions">
          <button class="quiet" type="button" :disabled="props.busy" @click="emit('cancel')">取消</button>
          <button ref="confirmButton" :class="props.danger ? 'danger' : ''" type="button" :disabled="props.busy" @click="emit('confirm')">
            {{ props.busy ? '处理中…' : props.confirmText ?? '确认' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
