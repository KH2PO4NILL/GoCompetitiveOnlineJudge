<script setup lang="ts">
import { useJudgePolling } from '../../composables/useJudgePolling'
import { computed, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute } from 'vue-router'
import type { GetSubmissionResp, SubmissionDetail } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { isValidId } from '../../lib/ids'
import { formatTime } from '../../lib/time'
import { verdicts } from '../../lib/labels'
import { isPending, resultLabel, verdictTone } from '../../lib/judge'
import Badge from '../../components/ui/Badge.vue'
import StateBlock from '../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const polling = useJudgePolling()
watch(() => route.fullPath, polling.reset)

const submissionId = computed(() => String(route.params.submissionId ?? ''))
const idValid = computed(() => isValidId(submissionId.value))

const submission = useQuery({
  queryKey: computed(() => ['submission', submissionId.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetSubmissionResp>(`/submissions/${submissionId.value}`, { signal }),
  // 判题轮询：未终态每 2.5s 拉取，终态或异常后停止
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
</script>

<template>
  <div>
    <p v-if="polling.paused.value" class="notice notice-warning" role="status">自动刷新已暂停，评测时间超过五分钟。<button class="link" type="button" @click="polling.reset(); submission.refetch()">继续刷新</button></p>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink to="/submissions">评测记录</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>#{{ submissionId }}</span>
    </nav>
    <p v-if="!idValid" class="notice notice-error" role="alert">提交编号无效。</p>
    <StateBlock v-else :pending="submission.isPending.value" :error="submission.error.value" @retry="polling.reset(); submission.refetch()">
      <template #content>
        <div v-if="summary && detail" class="stack">
          <section class="panel">
            <div class="panel-body result-head">
              <div class="result-main">
                <Badge :tone="verdictTone(summary)" :running="summary.status === 'judging'">{{ resultLabel(summary) }}</Badge>
                <p v-if="isPending(summary)" class="small muted result-note" role="status">评测进行中，页面会自动刷新，无需手动操作。</p>
              </div>
              <dl class="result-metrics">
                <div><dt>题目</dt><dd><RouterLink class="mono" :to="`/problems/${summary.problem_id}`">#{{ summary.problem_id }}</RouterLink></dd></div>
                <div><dt>语言</dt><dd class="mono">{{ summary.language }}</dd></div>
                <div><dt>得分</dt><dd class="mono">{{ summary.result_visibility === 'full' || summary.result_visibility === 'score_only' ? `${summary.score} / ${summary.max_score}` : '—' }}</dd></div>
                <div><dt>最大耗时</dt><dd class="mono">{{ maxTime !== undefined ? `${maxTime} ms` : '—' }}</dd></div>
                <div><dt>最大内存</dt><dd class="mono">{{ maxMemory !== undefined ? `${(maxMemory / 1024).toFixed(1)} MB` : '—' }}</dd></div>
                <div><dt>提交时间</dt><dd class="mono">{{ formatTime(summary.submitted_at) }}</dd></div>
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
          <p v-else-if="!isPending(summary)" class="small muted">本次提交没有可见的测试点结果。</p>

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
.output-pre {
  margin: 0; padding: var(--space-3); background: var(--surface-code);
  border: 1px solid var(--border); border-radius: var(--radius);
  max-height: 480px; white-space: pre-wrap; overflow-wrap: anywhere;
}
.source-pre { background: var(--surface); }
</style>
