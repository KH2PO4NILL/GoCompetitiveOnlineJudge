<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { GetProblemResp, CreateSubmissionResp, ListProblemsResp } from '../../api/contracts/app'
import { useAction } from '../../composables/useAction'
import { useApi } from '../../composables/useApi'
import { useCodeDraft } from '../../composables/useCodeDraft'
import { encode } from '../../api/json'
import { isValidId, parseIdParam, requireId } from '../../lib/ids'
import ErrorNotice from '../../components/ui/ErrorNotice.vue'
import StateBlock from '../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const routeProblemId = computed(() => String(route.params.problemId ?? ''))
const queryProblemId = computed(() => {
  const value = route.query.problem_id
  const text = Array.isArray(value) ? value[0] ?? '' : value ?? ''
  return isValidId(text) ? text : ''
})

const problemId = ref(routeProblemId.value || queryProblemId.value)
watch([routeProblemId, queryProblemId], () => {
  problemId.value = routeProblemId.value || queryProblemId.value
})

const idValid = computed(() => isValidId(problemId.value))

// 题目选择列表（无路由参数时展示）
const choices = useQuery({
  queryKey: ['submit', 'problem-choices'],
  enabled: computed(() => !idValid.value),
  queryFn: ({ signal }) => api.request<ListProblemsResp>('/problems?page=1&page_size=100', { signal }),
})

const problem = useQuery({
  queryKey: computed(() => ['problem', problemId.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetProblemResp>(`/problems/${problemId.value}`, { signal }),
})

const languages = computed(() => problem.data.value?.problem.judge_settings.allowed_languages ?? [])
const draft = useCodeDraft(() => `practice:${problemId.value}`, [])
watch(languages, (allowed) => {
  if (allowed.length && !allowed.includes(draft.language.value)) draft.language.value = allowed[0]
}, { immediate: true })

const { busy, error, uncertain, run } = useAction()

// 非幂等保护：结果未知后复用同一 client_request_id，由服务端去重
let requestId = ''
let requestFingerprint = ''

const lineCount = computed(() => draft.source.value.split('\n').length)
const charCount = computed(() => [...draft.source.value].length)

async function submit() {
  if (!idValid.value || !draft.language.value || !draft.source.value.trim()) return
  await run(async () => {
    const payload = { problem_id: parseIdParam(problemId.value), language: draft.language.value, source_code: draft.source.value }
    const fingerprint = encode(payload)
    if (!uncertain.value || fingerprint !== requestFingerprint) {
      requestId = crypto.randomUUID()
      requestFingerprint = fingerprint
    }
    const resp = await api.request<CreateSubmissionResp>('/submissions', {
      method: 'POST',
      body: { ...payload, client_request_id: requestId },
    })
    const submissionId = requireId(resp?.submission?.summary?.submission_id, '提交编号')
    draft.discard()
    await router.push(`/submissions/${submissionId}`)
  }, { allowUncertain: true })
}

</script>

<template>
  <div>
    <div class="page-head">
      <h1>提交代码</h1>
      <RouterLink v-if="idValid" class="small" :to="`/problems/${problemId}`">← 返回题面</RouterLink>
    </div>

    <div v-if="!routeProblemId" class="panel picker-panel">
      <div class="panel-body">
        <div class="field picker-field">
          <label for="problem-picker">选择题目</label>
          <div class="row">
            <select id="problem-picker" v-model="problemId" class="picker-select">
              <option value="" disabled>{{ choices.data.value ? '选择题目…' : '加载中…' }}</option>
              <option v-for="item in choices.data.value?.problems ?? []" :key="String(item.problem_id)" :value="String(item.problem_id)">
                #{{ item.problem_id }} · {{ item.title }}
              </option>
            </select>
            <input
              v-if="!choices.data.value" v-model="problemId" type="text" inputmode="numeric"
              placeholder="或输入题目编号" aria-label="题目编号" class="picker-input"
            >
          </div>
        </div>
        <ErrorNotice :error="choices.error.value" />
      </div>
    </div>

    <p v-if="problemId && !idValid" class="notice notice-error" role="alert">题目编号无效。</p>

    <StateBlock v-else-if="idValid" :pending="problem.isPending.value" :error="problem.error.value" @retry="problem.refetch()">
      <template #content>
        <div v-if="problem.data.value" class="stack">
          <div class="panel">
            <div class="panel-head">
              <h2 class="wrap-anywhere">
                <RouterLink :to="`/problems/${problemId}`">#{{ problemId }} · {{ problem.data.value.problem.statement.title }}</RouterLink>
              </h2>
              <span class="small muted mono">
                {{ problem.data.value.problem.judge_settings.time_limit_ms }}ms / {{ problem.data.value.problem.judge_settings.memory_limit_mb }}MB
              </span>
            </div>
            <div class="panel-body">
              <div class="code-editor">
                <div class="code-toolbar">
                  <label for="language-select">语言</label>
                  <select id="language-select" v-model="draft.language.value" :disabled="busy">
                    <option v-for="language in languages" :key="language" :value="language">{{ language }}</option>
                  </select>
                  <span class="spacer" />
                  <span>草稿仅保存在内存中</span>
                </div>
                <textarea
                  v-model="draft.source.value"
                  class="code-area"
                  :disabled="busy"
                  placeholder="在此粘贴或编写代码…"
                  aria-label="源代码编辑器"
                  spellcheck="false"
                />
                <div class="code-status">
                  <span>{{ lineCount }} 行 · {{ charCount }} 字符</span>
                  <span>{{ draft.language.value || '未选择语言' }}</span>
                </div>
              </div>
              <div class="row submit-row">
                <button type="button" :disabled="busy || !draft.source.value.trim() || !draft.language.value" @click="submit">
                  {{ busy ? '提交中…' : uncertain ? '结果未知 · 核对后可重试' : '提交评测' }}
                </button>
                <RouterLink class="small" :to="`/submissions?problem_id=${problemId}`">本题评测记录</RouterLink>
              </div>
              <p v-if="uncertain" class="notice notice-warning" role="alert">
                上一次提交结果未知：请先在评测记录中确认是否已受理；未受理时可再次点击提交，系统会复用同一请求标识，不会重复入库。
              </p>
              <ErrorNotice :error="error" />
            </div>
          </div>
        </div>
      </template>
    </StateBlock>
  </div>
</template>

<style scoped>
.picker-panel { margin-bottom: var(--space-4); }
.picker-field { margin-bottom: 0; }
.picker-select { flex: 1 1 280px; }
.picker-input { flex: 0 1 160px; }
.submit-row { margin-top: var(--space-4); }
</style>
