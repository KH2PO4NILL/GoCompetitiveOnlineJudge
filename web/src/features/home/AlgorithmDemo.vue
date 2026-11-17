<script setup lang="ts">
import { ref, watch } from 'vue'
const values = [2, 5, 8, 12, 16, 23, 38, 56, 72]
const target = ref(23)
const low = ref(0)
const high = ref(values.length - 1)
const current = ref<number>()
const found = ref<number>()
const steps = ref(0)
const finished = ref(false)
const message = ref('从有序数组中查找目标，每次比较中间元素，排除一半候选项。')
function reset() {
  low.value = 0; high.value = values.length - 1
  current.value = undefined; found.value = undefined
  steps.value = 0; finished.value = false
  message.value = '从有序数组中查找目标，每次比较中间元素，排除一半候选项。'
}
watch(target, reset)
function step() {
  if (finished.value) return
  const middle = low.value + Math.floor((high.value - low.value) / 2)
  current.value = middle
  steps.value++
  const value = values[middle]
  if (value === target.value) {
    found.value = middle; finished.value = true
    message.value = `找到 ${value}，下标为 ${middle}，共比较 ${steps.value} 次。`
    return
  }
  if (value < target.value) low.value = middle + 1
  else high.value = middle - 1
  message.value = `${value} ${value < target.value ? '小于' : '大于'} ${target.value}，继续查找${value < target.value ? '右侧' : '左侧'}。`
  if (low.value > high.value) {
    finished.value = true
    message.value = `数组中没有 ${target.value}，共比较 ${steps.value} 次。`
  }
}
</script>
<template>
  <section class="panel" aria-labelledby="binary-demo-title">
    <div class="panel-head"><h2 id="binary-demo-title">二分查找演示</h2><span class="small muted">示例数据 · O(log n)</span></div>
    <div class="panel-body">
      <ol class="array" aria-label="有序数组，下标从 0 开始">
        <li v-for="(value, index) in values" :key="value" :class="{ excluded: index < low || index > high, current: index === current, found: index === found }">
          <span class="small muted">[{{ index }}]</span><strong>{{ value }}</strong>
        </li>
      </ol>
      <div class="controls">
        <label for="binary-target">查找目标</label>
        <select id="binary-target" v-model="target"><option v-for="value in [8, 23, 24, 72]" :key="value" :value="value">{{ value }}</option></select>
        <button type="button" :disabled="finished" @click="step">下一步</button>
        <button class="quiet" type="button" @click="reset">重新开始</button>
      </div>
      <p class="explanation" role="status" aria-live="polite">{{ message }}</p>
    </div>
  </section>
</template>
<style scoped>
.array { display: grid; grid-template-columns: repeat(9, minmax(0, 1fr)); gap: var(--space-2); padding: 0; margin: 0 0 var(--space-4); list-style: none; }
.array li { display: grid; gap: var(--space-1); justify-items: center; padding: var(--space-2); border: 1px solid var(--border-strong); border-radius: var(--radius-sm); font-family: var(--font-mono); }
.array strong { font-size: var(--text-md); }
.array .excluded { color: var(--muted); background: var(--surface-sunken); }
.array .current { border: 2px solid var(--accent); background: var(--accent-subtle); }
.array .found { border-color: var(--success); background: var(--success-subtle); }
.controls { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-2); }
.controls select { width: 80px; }
.explanation { margin: var(--space-3) 0 0; color: var(--ink-secondary); }
@media (max-width: 640px) { .array { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
