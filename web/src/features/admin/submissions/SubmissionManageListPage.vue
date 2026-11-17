<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { BatchRejudgeSubmissionsResp, ListSubmissionsResp } from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { hasNextPage, isValidId, pageNumber, readQuery } from '../../../lib/ids'
import { listParams } from '../../../lib/forms'
import { judgeStatuses, verdicts } from '../../../lib/labels'
import { formatTime } from '../../../lib/time'
import { isPageAllSelected, selectedIds, toggleMany, toggleOne } from '../lib/selection'
import ConfirmDialog from '../../../components/ui/ConfirmDialog.vue'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import Pager from '../../../components/ui/Pager.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'
import VerdictBadge from '../../../components/ui/VerdictBadge.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20
const FIELDS = ['user_id', 'problem_id', 'contest_id', 'status', 'verdict', 'language']

const userId = ref(readQuery(route.query.user_id))
const problemId = ref(readQuery(route.query.problem_id))
const contestId = ref(readQuery(route.query.contest_id))
const status = ref(readQuery(route.query.status))
const verdict = ref(readQuery(route.query.verdict))
const language = ref(readQuery(route.query.language))

watch(() => route.query, (query) => {
  userId.value = readQuery(query.user_id)
  problemId.value = readQuery(query.problem_id)
  contestId.value = readQuery(query.contest_id)
  status.value = readQuery(query.status)
  verdict.value = readQuery(query.verdict)
  language.value = readQuery(query.language)
})

const search = computed(() => {
  const sp = new URLSearchParams()
  for (const field of FIELDS) {
    const value = readQuery(route.query[field])
    if (value) sp.set(field, value)
  }
  return sp
})

const listing = computed(() => listParams(search.value, FIELDS, isValidId, pageNumber))
const page = computed(() => listing.value.page)

const submissions = useQuery({
  queryKey: computed(() => ['admin', 'submissions', listing.value.params.toString()]),
  enabled: computed(() => !listing.value.invalid),
  queryFn: ({ signal }) => api.request<ListSubmissionsResp>(`/submissions?${listing.value.params}`, { signal }),
})

const rows = computed(() => submissions.data.value?.submissions ?? [])
const pageIds = computed(() => rows.value.map((item) => String(item.submission_id)))

// ---- 批量选择（跨页清空） ----
const selected = ref<Set<string>>(new Set())
const selectionNotice = ref('')

watch(page, () => {
  if (selected.value.size) selectionNotice.value = '翻页已清空批量选择，请重新勾选。'
  selected.value = new Set()
})

function onToggleOne(id: string) {
  selected.value = toggleOne(selected.value, id)
}
function onToggleAll(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selected.value = toggleMany(selected.value, pageIds.value, checked)
}

// ---- 批量重判 ----
const batch = useAction()
const confirmOpen = ref(false)
const reason = ref('')

function openConfirm() {
  reason.value = ''
  confirmOpen.value = true
}

async function runBatch() {
  const ids = selectedIds(selected.value)
  if (!ids.length) return
  await batch.run(async () => {
    const body: Record<string, unknown> = { submission_ids: ids }
    if (reason.value.trim()) body.reason = reason.value.trim()
    const resp = await api.request<BatchRejudgeSubmissionsResp>('/submissions/rejudge', { method: 'POST', body })
    confirmOpen.value = false
    selected.value = new Set()
    return `已受理 ${resp.accepted?.length ?? 0} / ${ids.length} 条重判请求。`
  })
}

function applyFilters(nextPage = 1) {
  const query: Record<string, string> = {}
  if (userId.value.trim()) query.user_id = userId.value.trim()
  if (problemId.value.trim()) query.problem_id = problemId.value.trim()
  if (contestId.value.trim()) query.contest_id = contestId.value.trim()
  if (status.value) query.status = status.value
  if (verdict.value) query.verdict = verdict.value
  if (language.value.trim()) query.language = language.value.trim()
  if (nextPage > 1) query.page = String(nextPage)
  void router.push({ query })
}

function resetFilters() {
  userId.value = ''
  problemId.value = ''
  contestId.value = ''
  status.value = ''
  verdict.value = ''
  language.value = ''
  void router.push({ query: {} })
}

function gotoPage(next: number) {
  const base = Object.fromEntries(search.value.entries())
  if (next > 1) base.page = String(next)
  else delete base.page
  void router.push({ query: base })
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1>提交与重判</h1>
      <div class="actions">
        <button class="btn sm" type="button" :disabled="!selected.size || batch.busy.value" @click="openConfirm">
          批量重判{{ selected.size ? `（${selected.size}）` : '' }}
        </button>
      </div>
    </div>

    <p v-if="listing.invalid" class="notice notice-error" role="alert">筛选参数中的编号无效。</p>
    <div v-else class="panel">
      <form class="filter-bar" @submit.prevent="applyFilters()">
        <div class="field">
          <label for="f-user">用户编号</label>
          <input id="f-user" v-model="userId" type="text" inputmode="numeric">
        </div>
        <div class="field">
          <label for="f-problem">题目编号</label>
          <input id="f-problem" v-model="problemId" type="text" inputmode="numeric">
        </div>
        <div class="field">
          <label for="f-contest">比赛编号</label>
          <input id="f-contest" v-model="contestId" type="text" inputmode="numeric">
        </div>
        <div class="field">
          <label for="f-status">状态</label>
          <select id="f-status" v-model="status">
            <option value="">全部</option>
            <option v-for="(label, value) in judgeStatuses" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="f-verdict">结果</label>
          <select id="f-verdict" v-model="verdict">
            <option value="">全部</option>
            <option v-for="(label, value) in verdicts" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="f-language">语言</label>
          <input id="f-language" v-model="language" type="text" placeholder="如 cpp20-v3">
        </div>
        <div class="field filter-actions">
          <span class="sr-only"><label for="f-submit">操作</label></span>
          <span class="row">
            <button id="f-submit" class="sm" type="submit">筛选</button>
            <button class="quiet sm" type="button" @click="resetFilters">重置</button>
          </span>
        </div>
      </form>

      <div class="panel-body batch-bar">
        <p v-if="selectionNotice" class="notice notice-warning small" role="status">{{ selectionNotice }}</p>
        <p v-if="batch.notice.value" class="notice notice-success" role="status">{{ batch.notice.value }}</p>
        <ErrorNotice :error="batch.error.value" />
        <p v-if="batch.uncertain.value" class="notice notice-warning" role="alert">
          批量重判结果未知：请核对提交状态，不要盲目重试。
          <button class="link sm" type="button" @click="batch.reset()">我已核对，重置</button>
        </p>
      </div>

      <StateBlock
        :pending="submissions.isPending.value" :error="submissions.error.value"
        :empty="!rows.length" empty-title="暂无提交记录"
        @retry="submissions.refetch()"
      >
        <p class="small muted">调整筛选条件后重试。</p>
      </StateBlock>

      <template v-if="rows.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">
                  <input
                    type="checkbox" :checked="isPageAllSelected(selected, pageIds)"
                    aria-label="选择本页全部提交" @change="onToggleAll"
                  >
                </th>
                <th scope="col" class="num">#</th>
                <th scope="col" class="num">用户</th>
                <th scope="col" class="num">题目</th>
                <th scope="col" class="num">比赛</th>
                <th scope="col">结果</th>
                <th scope="col">语言</th>
                <th scope="col">提交时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in rows" :key="String(item.submission_id)">
                <td>
                  <input
                    type="checkbox" :checked="selected.has(String(item.submission_id))"
                    :aria-label="`选择提交 ${item.submission_id}`" @change="onToggleOne(String(item.submission_id))"
                  >
                </td>
                <td class="num">
                  <RouterLink :to="`/admin/submissions/${item.submission_id}`">{{ item.submission_id }}</RouterLink>
                </td>
                <td class="num">{{ item.user_id }}</td>
                <td class="num">{{ item.problem_id }}</td>
                <td class="num">{{ item.contest_id ? String(item.contest_id) : '—' }}</td>
                <td><VerdictBadge :submission="item" /></td>
                <td class="mono small">{{ item.language }}</td>
                <td class="num small muted">{{ formatTime(item.submitted_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="submissions.data.value ? hasNextPage(submissions.data.value.total, page, PAGE_SIZE) : false"
          :total="submissions.data.value?.total" :disabled="submissions.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>

    <ConfirmDialog
      v-if="confirmOpen"
      title="批量重判"
      :message="`将对选中的 ${selected.size} 条提交发起重判，确认继续？`"
      confirm-text="重判"
      :busy="batch.busy.value"
      @cancel="confirmOpen = false"
      @confirm="runBatch"
    >
      <div class="field">
        <label for="batch-reason">重判原因（选填）</label>
        <input id="batch-reason" v-model="reason" type="text" autocomplete="off">
      </div>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
.batch-bar { display: grid; gap: var(--space-2); padding-bottom: 0; }
.batch-bar:empty { display: none; }
</style>
