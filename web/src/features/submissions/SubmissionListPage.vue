<script setup lang="ts">
import { useJudgePolling } from '../../composables/useJudgePolling'
import { computed, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { ListSubmissionsResp, SubmissionSummary } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { hasNextPage, isValidId, pageNumber, readQuery } from '../../lib/ids'
import { isPending } from '../../lib/judge'
import Pager from '../../components/ui/Pager.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import SubmissionTable from '../../components/data/SubmissionTable.vue'

const api = useApi()
const route = useRoute()
const polling = useJudgePolling()
watch(() => route.fullPath, polling.reset)
const router = useRouter()

const PAGE_SIZE = 20

const problemId = computed(() => readQuery(route.query.problem_id))
const contestId = computed(() => readQuery(route.query.contest_id))
const language = computed(() => readQuery(route.query.language))
const page = computed(() => pageNumber(readQuery(route.query.page) || null))

const filterValid = computed(() =>
  (!problemId.value || isValidId(problemId.value))
  && (!contestId.value || isValidId(contestId.value)))

const query = computed(() => {
  const params = new URLSearchParams({ page: String(page.value), page_size: String(PAGE_SIZE) })
  if (problemId.value) params.set('problem_id', problemId.value)
  if (contestId.value) params.set('contest_id', contestId.value)
  if (language.value) params.set('language', language.value)
  return params.toString()
})

const submissions = useQuery({
  queryKey: computed(() => ['submissions', query.value]),
  enabled: filterValid,
  queryFn: ({ signal }) => api.request<ListSubmissionsResp>(`/submissions?${query.value}`, { signal }),
  // 存在未终态提交时 4s 轮询；全部终态后自动停止
  refetchInterval: (queryClient) => {
    const data = queryClient.state.data as ListSubmissionsResp | undefined
    return polling.interval(data?.submissions?.some(isPending) ?? false, queryClient.state.error, 4000)
  },
})

const rows = computed<SubmissionSummary[]>(() => submissions.data.value?.submissions ?? [])

function setFilter(key: string, value: string) {
  const next: Record<string, string> = { ...Object.fromEntries(Object.entries(route.query).map(([k, v]) => [k, String(v ?? '')])) }
  if (value) next[key] = value
  else delete next[key]
  delete next.page
  void router.push({ query: next })
}
</script>

<template>
  <div>
    <p v-if="polling.paused.value" class="notice notice-warning" role="status">自动刷新已暂停，评测时间超过五分钟。<button class="link" type="button" @click="polling.reset(); submissions.refetch()">继续刷新</button></p>
    <div class="page-head">
      <h1>评测记录</h1>
      <RouterLink v-if="problemId" class="small" :to="`/problems/${problemId}`">查看题目 #{{ problemId }}</RouterLink>
    </div>
    <p v-if="!filterValid" class="notice notice-error" role="alert">筛选参数中的编号无效。</p>
    <div v-else class="panel">
      <div class="filter-bar">
        <div class="field">
          <label for="filter-problem">题目编号</label>
          <input id="filter-problem" :value="problemId" type="text" inputmode="numeric" @change="setFilter('problem_id', ($event.target as HTMLInputElement).value.trim())">
        </div>
        <div class="field">
          <label for="filter-language">语言</label>
          <input id="filter-language" :value="language" type="text" placeholder="如 cpp20-v3" @change="setFilter('language', ($event.target as HTMLInputElement).value.trim())">
        </div>
        <div class="field filter-actions">
          <span class="sr-only"><label for="filter-reset">重置</label></span>
          <button id="filter-reset" class="quiet sm" type="button" @click="setFilter('problem_id', ''); setFilter('language', '')">清除筛选</button>
        </div>
      </div>
      <StateBlock
        :pending="submissions.isPending.value" :error="submissions.error.value"
        :empty="!rows.length" empty-title="暂无提交记录"
        @retry="polling.reset(); submissions.refetch()"
      >
        <p class="small muted">
          <RouterLink to="/submit">提交第一份代码 →</RouterLink>
        </p>
      </StateBlock>
      <template v-if="rows.length">
        <SubmissionTable :submissions="rows" />
        <Pager
          :page="page" :has-next="submissions.data.value ? hasNextPage(submissions.data.value.total, page, PAGE_SIZE) : false"
          :total="submissions.data.value?.total" :disabled="submissions.isFetching.value"
          @prev="router.push({ query: { ...route.query, page: String(page - 1) || undefined } })"
          @next="router.push({ query: { ...route.query, page: String(page + 1) } })"
        />
        <p class="list-meta small muted">存在未终态提交时每 4 秒自动刷新。</p>
      </template>
    </div>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
.list-meta { padding: 0 var(--space-4) var(--space-3); margin: 0; }
</style>
