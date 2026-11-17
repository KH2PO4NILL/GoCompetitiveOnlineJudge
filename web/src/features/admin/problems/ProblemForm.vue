<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { ProblemSample } from '../../../api/contracts/admin'
import type { StringRecord } from '../../../lib/forms'
import { checkerTypes, difficulties, visibilities } from '../../../lib/labels'
import TextField from '../../../components/ui/TextField.vue'

/**
 * 题目编辑表单（创建 / 工作台复用）。
 * 内部持有可编辑状态，通过 load() 预填、snapshot() 读取，
 * 使工作台在服务端 409 刷新时不必重置用户当前编辑。
 */
const defaults: StringRecord = {
  title: '',
  description: '',
  input_description: '',
  output_description: '',
  hint: '',
  difficulty: 'medium',
  visibility: 'private',
  checker_type: 'standard',
  tags: '',
  source: '',
  time_limit_ms: '1000',
  memory_limit_mb: '256',
  allowed_languages: 'cpp20-v3',
}

const values = reactive<StringRecord>({ ...defaults })
const samples = ref<ProblemSample[]>([{ input: '', output: '', explanation: '' }])

function load(next: StringRecord, nextSamples: ProblemSample[]) {
  Object.assign(values, defaults, next)
  samples.value = nextSamples.length
    ? nextSamples.map((item) => ({ input: item.input, output: item.output, explanation: item.explanation ?? '' }))
    : [{ input: '', output: '', explanation: '' }]
}

function snapshot(): { values: StringRecord; samples: ProblemSample[] } {
  return { values: { ...values }, samples: samples.value.map((item) => ({ ...item })) }
}

function addSample() {
  samples.value.push({ input: '', output: '', explanation: '' })
}

function removeSample(index: number) {
  samples.value.splice(index, 1)
}

defineExpose({ load, snapshot })
</script>

<template>
  <div class="stack">
    <TextField v-model="values.title" label="题目名称" name="title" required placeholder="如 A + B Problem" />

    <div class="grid-two">
      <div class="field">
        <label for="pf-difficulty">难度</label>
        <select id="pf-difficulty" v-model="values.difficulty" name="difficulty">
          <option v-for="(label, value) in difficulties" :key="value" :value="value">{{ label }}</option>
        </select>
      </div>
      <div class="field">
        <label for="pf-visibility">可见性</label>
        <select id="pf-visibility" v-model="values.visibility" name="visibility">
          <option v-for="(label, value) in visibilities" :key="value" :value="value">{{ label }}</option>
        </select>
      </div>
      <div class="field">
        <label for="pf-checker">比较器</label>
        <select id="pf-checker" v-model="values.checker_type" name="checker_type">
          <option v-for="(label, value) in checkerTypes" :key="value" :value="value">{{ label }}</option>
        </select>
      </div>
      <TextField v-model="values.source" label="来源" name="source" placeholder="选填，如 NOI2020" />
    </div>

    <TextField
      v-model="values.description" label="题目描述" name="description" type="textarea" :rows="8"
      hint="支持 Markdown（含 LaTeX 公式）。"
    />
    <TextField v-model="values.input_description" label="输入格式" name="input_description" type="textarea" :rows="4" hint="支持 Markdown。" />
    <TextField v-model="values.output_description" label="输出格式" name="output_description" type="textarea" :rows="4" hint="支持 Markdown。" />
    <TextField v-model="values.hint" label="提示" name="hint" type="textarea" :rows="3" hint="支持 Markdown，选填。" />

    <section class="samples">
      <div class="row-between samples-head">
        <h3>样例</h3>
        <button class="quiet sm" type="button" @click="addSample">添加样例</button>
      </div>
      <p v-if="!samples.length" class="small muted">暂无样例，可点击“添加样例”。</p>
      <div v-for="(sample, index) in samples" :key="index" class="sample-editor">
        <div class="row-between sample-editor-head">
          <strong class="small">样例 {{ index + 1 }}</strong>
          <button class="danger-quiet sm" type="button" @click="removeSample(index)">删除</button>
        </div>
        <div class="grid-two">
          <div class="field">
            <label :for="`sample-in-${index}`">输入</label>
            <textarea :id="`sample-in-${index}`" v-model="sample.input" rows="4" class="mono" />
          </div>
          <div class="field">
            <label :for="`sample-out-${index}`">输出</label>
            <textarea :id="`sample-out-${index}`" v-model="sample.output" rows="4" class="mono" />
          </div>
        </div>
        <div class="field">
          <label :for="`sample-exp-${index}`">样例说明</label>
          <textarea :id="`sample-exp-${index}`" v-model="sample.explanation" rows="2" />
        </div>
      </div>
    </section>

    <div class="grid-two">
      <TextField v-model="values.time_limit_ms" label="时间限制 (ms)" name="time_limit_ms" type="number" required />
      <TextField v-model="values.memory_limit_mb" label="内存限制 (MB)" name="memory_limit_mb" type="number" required />
    </div>
    <TextField
      v-model="values.allowed_languages" label="允许语言" name="allowed_languages" required
      placeholder="逗号分隔，如 cpp20-v3,go-v3" hint="多个语言标识以逗号分隔。"
    />
    <TextField v-model="values.tags" label="标签" name="tags" placeholder="逗号分隔，如 dp,图论" hint="多个标签以逗号分隔，选填。" />
  </div>
</template>

<style scoped>
.grid-two { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 0 var(--space-4); }
.samples { display: grid; gap: var(--space-3); }
.samples-head h3 { margin: 0; }
.sample-editor { border: 1px solid var(--border); border-radius: var(--radius); padding: var(--space-3); display: grid; gap: var(--space-2); background: var(--surface-sunken); }
.sample-editor-head { align-items: center; }
</style>
