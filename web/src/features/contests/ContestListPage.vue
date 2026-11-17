<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { ListContestsResp } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { hasNextPage, pageNumber, readQuery } from '../../lib/ids'
import { contestModes, contestStatuses, labelOf } from '../../lib/labels'
import { formatTime } from '../../lib/time'
import Badge from '../../components/ui/Badge.vue'
import Pager from '../../components/ui/Pager.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import { contestStatusTone } from './helpers'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20

const STATUS_TABS = [
  { value: '', label: '全部' },
  { value: 'running', label: labelOf(contestStatuses, 'running') },
  { value: 'published', label: labelOf(contestStatuses, 'published') },
  { value: 'finished', label: labelOf(contestStatuses, 'finished') },
] as const

const keyword = ref(readQuery(route.query.keyword))
const status = ref(readQuery(route.query.status))

watch(() => route.query, (query) => {
  keyword.value = readQuery(query.keyword)
  status.value = readQuery(query.status)
})

const page = computed(() => pageNumber(readQuery(route.query.page) || null))

const search = computed(() => {
  const params = new URLSearchParams()
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
  if (status.value) params.set('status', status.value)
  return params.toString()
})

const contests = useQuery({
  queryKey: computed(() => ['contests', search.value, page.value]),
  queryFn: ({ signal }) => {
    const params = new URLSearchParams(search.value)
    params.set('page', String(page.value))
    params.set('page_size', String(PAGE_SIZE))
    return api.request<ListContestsResp>(`/contests?${params}`, { signal })
  },
})

function buildQuery(next: { keyword?: string; status?: string; page?: number }): Record<string, string> {
  const query: Record<string, string> = {}
  if (next.keyword?.trim()) query.keyword = next.keyword.trim()
  if (next.status) query.status = next.status
  if (next.page && next.page > 1) query.page = String(next.page)
  return query
}

function applyFilters() {
  void router.push({ query: buildQuery({ keyword: keyword.value, status: status.value }) })
}

function resetFilters() {
  keyword.value = ''
  status.value = ''
  void router.push({ query: {} })
}

function setStatus(value: string) {
  status.value = value
  void router.push({ query: buildQuery({ keyword: keyword.value, status: value }) })
}

function gotoPage(next: number) {
  void router.push({ query: buildQuery({ keyword: keyword.value, status: status.value, page: next }) })
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1>比赛</h1>
      <span v-if="contests.data.value" class="small muted">共 {{ contests.data.value.total }} 场</span>
    </div>
    <div class="panel">
      <nav class="status-tabs" aria-label="按状态筛选">
        <button
          v-for="tab in STATUS_TABS" :key="tab.value" type="button"
          class="quiet sm" :class="{ 'tab-active': status === tab.value }"
          :aria-current="status === tab.value ? 'true' : undefined"
          @click="setStatus(tab.value)"
        >
          {{ tab.label }}
        </button>
      </nav>
      <form class="filter-bar" @submit.prevent="applyFilters()">
        <div class="field grow">
          <label for="filter-keyword">关键词</label>
          <input id="filter-keyword" v-model="keyword" type="search" name="keyword" placeholder="比赛标题">
        </div>
        <div class="field filter-actions">
          <span class="label-row sr-only"><label>操作</label></span>
          <span class="row">
            <button class="sm" type="submit">筛选</button>
            <button class="quiet sm" type="button" @click="resetFilters">重置</button>
          </span>
        </div>
      </form>
      <StateBlock
        :pending="contests.isPending.value" :error="contests.error.value"
        :empty="!contests.data.value?.contests?.length" empty-title="没有匹配的比赛"
        @retry="contests.refetch()"
      >
        <p class="small muted">调整筛选条件后重试。</p>
      </StateBlock>
      <template v-if="contests.data.value?.contests?.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col" class="num">#</th>
                <th scope="col">比赛</th>
                <th scope="col">赛制</th>
                <th scope="col">状态</th>
                <th scope="col">开始时间</th>
                <th scope="col">结束时间</th>
                <th scope="col" class="num">报名人数</th>
                <th scope="col">报名</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="contest in contests.data.value.contests" :key="String(contest.contest_id)">
                <td class="num">{{ contest.contest_id }}</td>
                <td>
                  <RouterLink class="contest-link" :to="`/contests/${contest.contest_id}`">{{ contest.title }}</RouterLink>
                </td>
                <td><Badge tone="neutral">{{ labelOf(contestModes, contest.mode) }}</Badge></td>
                <td>
                  <Badge :tone="contestStatusTone(contest.status)" :running="contest.status === 'running'">
                    {{ labelOf(contestStatuses, contest.status) }}
                  </Badge>
                </td>
                <td class="num small muted">{{ formatTime(contest.start_at) }}</td>
                <td class="num small muted">{{ formatTime(contest.end_at) }}</td>
                <td class="num">{{ contest.participant_count }}</td>
                <td>
                  <Badge :tone="contest.registration_open ? 'success' : 'muted'">
                    {{ contest.registration_open ? '报名开放' : '未开放' }}
                  </Badge>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="hasNextPage(contests.data.value.total, page, PAGE_SIZE)"
          :total="contests.data.value.total" :disabled="contests.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.status-tabs {
  display: flex; gap: var(--space-2); flex-wrap: wrap;
  padding: var(--space-3) var(--space-4) 0;
}
.status-tabs .tab-active { border-color: var(--accent); color: var(--accent); background: var(--accent-subtle); }
.contest-link { font-weight: 500; overflow-wrap: anywhere; }
td:first-child { color: var(--muted); }
.filter-actions { flex: 0 0 auto; }
</style>
