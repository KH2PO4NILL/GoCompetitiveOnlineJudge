<script setup lang="ts">
import { useJudgePolling } from '../../composables/useJudgePolling'
import { encode } from '../../api/json'
import { computed, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { CreateSubmissionReq, CreateSubmissionResp, GetProblemResp, ListSubmissionsResp, SubmissionSummary } from '../../api/contracts/app'
import { useAction } from '../../composables/useAction'
import { useApi } from '../../composables/useApi'
import { useCodeDraft } from '../../composables/useCodeDraft'
import { isValidId, parseIdParam, requireId } from '../../lib/ids'
import { isPending } from '../../lib/judge'
import { checkerTypes, labelOf } from '../../lib/labels'
import ErrorNotice from '../../components/ui/ErrorNotice.vue'
import MarkdownView from '../../components/ui/MarkdownView.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import SubmissionTable from '../../components/data/SubmissionTable.vue'
import { submitBlockReason } from './helpers'
import { useContestContext } from './useContest'

const api = useApi()
const route = useRoute()
const polling = useJudgePolling()
watch(() => route.fullPath, polling.reset)
const router = useRouter()

const contestId = computed(() => String(route.params.contestId ?? ''))
const problemId = computed(() => String(route.params.problemId ?? ''))
const idsValid = computed(() => isValidId(contestId.value) && isValidId(problemId.value))

const { detail, my } = useContestContext()

const problem = useQuery({
  queryKey: computed(() => ['problem', problemId.value, 'contest', contestId.value]),
  enabled: idsValid,
  queryFn: ({ signal }) => api.request<GetProblemResp>(
    `/problems/${problemId.value}?contest_id=${contestId.value}`, { signal }),
})

const info = computed(() => problem.data.value?.problem)

const mySubmissions = useQuery({
  queryKey: computed(() => ['submissions', 'contest', contestId.value, 'problem', problemId.value]),
  enabled: idsValid,
  queryFn: ({ signal }) => api.request<ListSubmissionsResp>(
    `/submissions?contest_id=${contestId.value}&problem_id=${problemId.value}&page=1&page_size=20`, { signal }),
  // 存在未终态提交时 4s 轮询；全部终态后自动停止
  refetchInterval: (queryClient) => {
    const data = queryClient.state.data as ListSubmissionsResp | undefined
    return polling.interval(data?.submissions?.some(isPending) ?? false, queryClient.state.error, 4000)
  },
})

const submissionRows = computed<SubmissionSummary[]>(() => mySubmissions.data.value?.submissions ?? [])

const canSubmit = computed(() => my.value?.can_submit === true)
const blockedReason = computed(() => submitBlockReason(my.value, detail.value?.status))

const languages = computed(() => info.value?.judge_settings.allowed_languages ?? [])
// 草稿 key 与比赛/题目绑定；ContestLayout 在切换题目时会重挂载本组件
const draft = useCodeDraft(() => `contest:${contestId.value}:${problemId.value}`, [])
watch(languages, (allowed) => {
  if (allowed.length && !allowed.includes(draft.language.value)) draft.language.value = allowed[0]
}, { immediate: true })

const { busy, error, uncertain, run } = useAction()

// 幂等保护：结果未知后复用同一 client_request_id，由服务端去重
let requestId = ''
let requestFingerprint = ''

const lineCount = computed(() => draft.source.value.split('\n').length)
const charCount = computed(() => [...draft.source.value].length)

async function submit() {
  if (!idsValid.value || !canSubmit.value || !draft.language.value || !draft.source.value.trim()) return
  const contestPk = parseIdParam(contestId.value)
  const problemPk = parseIdParam(problemId.value)
  if (!contestPk || !problemPk) return
  await run(async () => {
    const fingerprint = encode({ problem_id: problemPk, contest_id: contestPk, language: draft.language.value, source_code: draft.source.value })
    if (!uncertain.value || fingerprint !== requestFingerprint) {
      requestId = crypto.randomUUID()
      requestFingerprint = fingerprint
    }
    const body: CreateSubmissionReq = {
      problem_id: problemPk,
      contest_id: contestPk,
      language: draft.language.value,
      source_code: draft.source.value,
      client_request_id: requestId,
    }
    const resp = await api.request<CreateSubmissionResp>('/submissions', { method: 'POST', body })
    const submissionId = requireId(resp?.submission?.summary?.submission_id, '提交编号')
    draft.discard()
    await router.push(`/submissions/${submissionId}`)
  }, { allowUncertain: true })
}

</script>

<template>
  <div>
    <p v-if="polling.paused.value" class="notice notice-warning" role="status">自动刷新已暂停，评测时间超过五分钟。<button class="link" type="button" @click="polling.reset(); mySubmissions.refetch()">继续刷新</button></p>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="`/contests/${contestId}`">{{ detail?.title ?? '比赛概览' }}</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <RouterLink :to="`/contests/${contestId}/problems`">题目</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>#{{ problemId }}</span>
    </nav>
    <p v-if="!idsValid" class="notice notice-error" role="alert">比赛或题目编号无效。</p>
    <StateBlock v-else :pending="problem.isPending.value" :error="problem.error.value" @retry="problem.refetch()">
      <template #content>
        <div v-if="info" class="stack">
          <div class="grid-2">
            <div class="stack">
              <section class="panel">
                <div class="panel-body">
                  <div class="page-head title-head">
                    <h1>{{ info.statement.title }}</h1>
                    <span class="row title-badges">
                      <span class="small muted mono">#{{ info.problem_id }}</span>
                    </span>
                  </div>
                  <MarkdownView :content="info.statement.description" />
                </div>
                <div class="panel-body">
                  <h2>输入格式</h2>
                  <MarkdownView :content="info.statement.input_description" />
                  <h2>输出格式</h2>
                  <MarkdownView :content="info.statement.output_description" />
                </div>
                <div v-if="info.statement.samples?.length" class="panel-body">
                  <h2>样例</h2>
                  <div v-for="(sample, index) in info.statement.samples" :key="index" class="sample-group">
                    <div class="sample-grid">
                      <div class="sample-box">
                        <h4>输入 #{{ index + 1 }}</h4>
                        <pre>{{ sample.input }}</pre>
                      </div>
                      <div class="sample-box">
                        <h4>输出 #{{ index + 1 }}</h4>
                        <pre>{{ sample.output }}</pre>
                      </div>
                    </div>
                    <p v-if="sample.explanation" class="small muted sample-explanation">{{ sample.explanation }}</p>
                  </div>
                </div>
                <div v-if="info.statement.hint" class="panel-body">
                  <h2>提示</h2>
                  <MarkdownView :content="info.statement.hint" />
                </div>
              </section>
            </div>
            <aside class="stack">
              <section class="panel">
                <div class="panel-head"><h2>提交</h2></div>
                <div class="panel-body">
                  <template v-if="canSubmit">
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
                    </div>
                    <p v-if="uncertain" class="notice notice-warning" role="alert">
                      上一次提交结果未知：请先在下方提交列表中确认是否已受理；未受理时可再次点击提交，系统会复用同一请求标识，不会重复入库。
                    </p>
                    <ErrorNotice :error="error" />
                  </template>
                  <template v-else>
                    <p class="notice notice-warning" role="status">{{ blockedReason || '当前无法提交。' }}</p>
                    <RouterLink class="btn block quiet overview-link" :to="`/contests/${contestId}`">前往比赛概览</RouterLink>
                  </template>
                </div>
              </section>
              <section class="panel">
                <div class="panel-head"><h2>题目信息</h2></div>
                <div class="panel-body">
                  <dl class="kv-list">
                    <div><dt>时间限制</dt><dd class="mono">{{ info.judge_settings.time_limit_ms }} ms</dd></div>
                    <div><dt>内存限制</dt><dd class="mono">{{ info.judge_settings.memory_limit_mb }} MB</dd></div>
                    <div><dt>比较器</dt><dd>{{ labelOf(checkerTypes, info.judge_settings.checker_type) }}</dd></div>
                  </dl>
                  <h3 class="side-subtitle">支持语言</h3>
                  <span class="tag-list">
                    <span v-for="language in info.judge_settings.allowed_languages" :key="language" class="tag mono">{{ language }}</span>
                  </span>
                </div>
              </section>
            </aside>
          </div>

          <section class="panel">
            <div class="panel-head">
              <h2>我在本题的提交</h2>
              <span class="small muted">存在未终态提交时每 4 秒自动刷新</span>
            </div>
            <div aria-live="polite">
              <StateBlock
                :pending="mySubmissions.isPending.value" :error="mySubmissions.error.value"
                :empty="!submissionRows.length" empty-title="暂无提交"
                @retry="polling.reset(); mySubmissions.refetch()"
              >
                <p class="small muted">在右侧编辑器中提交第一份代码。</p>
              </StateBlock>
              <SubmissionTable v-if="submissionRows.length" :submissions="submissionRows" />
            </div>
          </section>
        </div>
      </template>
    </StateBlock>
  </div>
</template>

<style scoped>
.title-head { margin-bottom: var(--space-4); }
.title-head h1 { margin: 0; }
.title-badges { flex-shrink: 0; }
.sample-group + .sample-group { margin-top: var(--space-4); }
.sample-explanation { margin: var(--space-2) 0 0; }
.submit-row { margin-top: var(--space-3); }
.overview-link { margin-top: var(--space-3); }
.side-subtitle { margin: var(--space-4) 0 var(--space-2); font-size: var(--text-xs); color: var(--muted); font-weight: 500; }
</style>
