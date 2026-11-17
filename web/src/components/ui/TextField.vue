<script setup lang="ts">
defineProps<{
  label: string
  name: string
  type?: string
  modelValue: string
  hint?: string
  required?: boolean
  readOnly?: boolean
  autoComplete?: string
  placeholder?: string
  rows?: number
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}
</script>

<template>
  <div class="field">
    <span class="label-row">
      <label :for="`f-${name}`">{{ label }}<template v-if="required"> *</template></label>
      <slot name="label-extra" />
    </span>
    <textarea v-if="type === 'textarea'" :id="`f-${name}`" :name="name" :value="modelValue" :rows="rows ?? 4" :required="required" :readonly="readOnly" :autocomplete="autoComplete" :placeholder="placeholder" @input="onInput" />
    <input v-else :id="`f-${name}`" :name="name" :type="type ?? 'text'" :value="modelValue" :required="required" :readonly="readOnly" :autocomplete="autoComplete" :placeholder="placeholder" @input="onInput">
    <span v-if="hint" class="field-hint">{{ hint }}</span>
    <slot name="after" />
  </div>
</template>
