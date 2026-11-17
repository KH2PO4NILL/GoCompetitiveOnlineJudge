<script setup lang="ts">
import { useJudgePolling } from '../../../composables/useJudgePolling'
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute } from 'vue-router'
import type { GetSubmissionResp, RejudgeSubmissionResp, SubmissionDetail } from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { isValidId, parseIdParam } from '../../../lib/ids'
import { formatTime } from '../../../lib/time'
import { verdicts } from '../../../lib/labels'
import { isPending, resultLabel, verdictTone } from '../../../lib/judge'
import Badge from '../../../components/ui/Badge.vue'
import ConfirmDialog from '../../../components/ui/ConfirmDialog.vue'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const polling = useJudgePolling()
watch(() => route.fullPath, polling.reset)

const submissionIdText = computed(() => String(route.params.submissionId ?? ''))
const idValid = computed(() => isValidId(submissionIdText.value))
const submissionId = computed(() => parseIdParam(route.params.submissionId))

const submission = useQuery({
  queryKey: computed(() => ['admin', 'submission', submissionIdText.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetSubmissionResp>(`/submissions/${submissionIdText.value}`, { signal }),
  refetchInterval: (queryClient) => {
    const data = queryClient.state.data as GetSubmissionResp | undefined
    return polling.interval(!data || isPending(data.submission.summary), queryClient.state.error, 2500)
  },
})

const detail = computed<SubmissionDetail | undefined>(() => submission.data.value?.submission)
const summary = computed(() => detail.value?.summary)

const maxTime = computed(() => {
  const cases = detail.value?.case_results ?? []
  if (!cases.length) return undefined
  return cases.reduce((max, item) => (Number(item.time_used_ms) > max ? Number(item.time_used_ms) : max), 0)
})
const maxMemory = computed(() => {
  const cases = detail.value?.case_results ?? []
  if (!cases.length) return undefined
  return cases.reduce((max, item) => (Number(item.memory_used_kb) > max ? Number(item.memory_used_kb) : max), 0)
})
const passedCount = computed(() => (detail.value?.case_results ?? []).filter((item) => item.verdict === 'accepted').length)

const rejudge = useAction()
const confirmOpen = ref(false)
const reason = ref('')

function openConfirm() {
  reason.value = ''
  confirmOpen.value = true
}

async function runRejudge() {
  await rejudge.run(async () => {
    const body: Record<string, unknown> = { submission_id: submissionId.value }
    if (reason.value.trim()) body.reason = reason.value.trim()
    await api.request<RejudgeSubmissionResp>(`/submissions/${submissionIdText.value}/rejudge`, { method: 'POST', body })
    confirmOpen.value = false
    await submission.refetch()
    return '重判请求已受理。'
  })
}
</script>

<template>
  <div>
    <p v-if="polling.paused.value" class="notice notice-warning" role="status">自动刷新已暂停，评测时间超过五分钟。<button class="link" type="button" @click="polling.reset(); submission.refetch()">继续刷新</button></p>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-submissions' }">提交与重判</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>#{{ submissionIdText }}</span>
    </nav>
    <div class="page-head">
      <h1>提交详情</h1>
      <div class="actions">
        <RouterLink class="btn btn-quiet sm" :to="`/submissions/${submissionIdText}`">用户端视图</RouterLink>
        <button class="btn sm" type="button" :disabled="rejudge.busy.value || !summary" @click="openConfirm">重判</button>
      </div>
    </div>

    <p v-if="!idValid" class="notice notice-error" role="alert">提交编号无效。</p>
    <StateBlock v-else :pending="submission.isPending.value" :error="submission.error.value" @retry="polling.reset(); submission.refetch()">
      <template #content>
        <div v-if="summary && detail" class="stack">
          <p v-if="rejudge.notice.value" class="notice notice-success" role="status">{{ rejudge.notice.value }}</p>
          <ErrorNotice :error="rejudge.error.value" />
          <p v-if="rejudge.uncertain.value" class="notice notice-warning" role="alert">
            重判结果未知：请核对提交状态，不要盲目重试。
            <button class="link sm" type="button" @click="rejudge.reset(); submission.refetch()">我已核对，重置</button>
          </p>

          <section class="panel">
            <div class="panel-body result-head">
              <div class="result-main">
                <Badge :tone="verdictTone(summary)" :running="summary.status === 'judging'">{{ resultLabel(summary) }}</Badge>
                <p v-if="isPending(summary)" class="small muted result-note" role="status">评测进行中，页面会自动刷新。</p>
              </div>
              <dl class="result-metrics">
                <div><dt>用户</dt><dd class="mono">{{ summary.user_id }}</dd></div>
                <div><dt>题目</dt><dd class="mono">{{ summary.problem_id }}</dd></div>
                <div><dt>比赛</dt><dd class="mono">{{ summary.contest_id ? String(summary.contest_id) : '—' }}</dd></div>
                <div><dt>语言</dt><dd class="mono">{{ summary.language }}</dd></div>
                <div><dt>得分</dt><dd class="mono">{{ summary.score }} / {{ summary.max_score }}</dd></div>
                <div><dt>最大耗时</dt><dd class="mono">{{ maxTime !== undefined ? `${maxTime} ms` : '—' }}</dd></div>
                <div><dt>最大内存</dt><dd class="mono">{{ maxMemory !== undefined ? `${(maxMemory / 1024).toFixed(1)} MB` : '—' }}</dd></div>
                <div><dt>提交时间</dt><dd class="mono">{{ formatTime(summary.submitted_at) }}</dd></div>
              </dl>
              <dl class="result-metrics admin-metrics">
                <div><dt>结果可见性</dt><dd class="mono">{{ summary.result_visibility }}</dd></div>
                <div><dt>题目 revision</dt><dd class="mono">{{ summary.problem_revision }}</dd></div>
                <div><dt>尝试次数</dt><dd class="mono">{{ summary.attempt_count }}</dd></div>
                <div><dt>开始评测</dt><dd class="mono">{{ formatTime(summary.judging_started_at) }}</dd></div>
                <div><dt>评测结束</dt><dd class="mono">{{ formatTime(summary.finished_at) }}</dd></div>
              </dl>
            </div>
          </section>

          <section v-if="detail.case_results?.length" class="panel">
            <div class="panel-head">
              <h2>测试点结果</h2>
              <span class="small muted">{{ passedCount }} / {{ detail.case_results.length }} 通过</span>
            </div>
            <div class="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th scope="col" class="num">#</th>
                    <th scope="col">分组</th>
                    <th scope="col">结果</th>
                    <th scope="col" class="num">得分</th>
                    <th scope="col" class="num">耗时</th>
                    <th scope="col" class="num">内存</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in detail.case_results" :key="String(item.test_case_id)">
                    <td class="num">{{ item.ordinal }}</td>
                    <td>{{ item.group_name || '—' }}</td>
                    <td>
                      <Badge :tone="item.verdict === 'accepted' ? 'success' : item.verdict === 'system_error' ? 'muted' : 'danger'">
                        {{ verdicts[item.verdict] ?? item.verdict }}
                      </Badge>
                    </td>
                    <td class="num">{{ item.score }}</td>
                    <td class="num">{{ item.time_used_ms }} ms</td>
                    <td class="num">{{ (Number(item.memory_used_kb) / 1024).toFixed(1) }} MB</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section v-if="detail.compile_output" class="panel">
            <div class="panel-head"><h2>编译输出</h2></div>
            <div class="panel-body">
              <pre class="output-pre">{{ detail.compile_output }}</pre>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h2>源代码</h2>
              <span class="small muted mono">{{ summary.language }}</span>
            </div>
            <div class="panel-body">
              <pre class="output-pre source-pre"><code>{{ detail.source_code }}</code></pre>
            </div>
          </section>
        </div>
      </template>
    </StateBlock>

    <ConfirmDialog
      v-if="confirmOpen"
      title="重判提交"
      :message="`将对提交 #${submissionIdText} 发起重判，确认继续？`"
      confirm-text="重判"
      :busy="rejudge.busy.value"
      @cancel="confirmOpen = false"
      @confirm="runRejudge"
    >
      <div class="field">
        <label for="rejudge-reason">重判原因（选填）</label>
        <input id="rejudge-reason" v-model="reason" type="text" autocomplete="off">
      </div>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.result-head { display: grid; gap: var(--space-4); }
.result-main { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
.result-main .badge { font-size: var(--text-md); padding: 4px 14px; }
.result-note { margin: 0; }
.result-metrics {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: var(--space-2) var(--space-4); margin: 0;
}
.result-metrics > div { display: grid; gap: 2px; }
.result-metrics dt { font-size: var(--text-xs); color: var(--muted); }
.result-metrics dd { margin: 0; font-size: var(--text-sm); overflow-wrap: anywhere; }
.admin-metrics { border-top: 1px solid var(--border); padding-top: var(--space-3); }
.output-pre {
  margin: 0; padding: var(--space-3); background: var(--surface-code);
  border: 1px solid var(--border); border-radius: var(--radius);
  max-height: 480px; white-space: pre-wrap; overflow-wrap: anywhere;
}
.source-pre { background: var(--surface); }
</style>
