<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { ListContestsResp } from '../../../api/contracts/admin'
import { useApi } from '../../../composables/useApi'
import { hasNextPage, isValidId, pageNumber, readQuery } from '../../../lib/ids'
import { listParams } from '../../../lib/forms'
import { contestModes, contestStatuses, labelOf, registrationPolicies, visibilities } from '../../../lib/labels'
import { formatTime } from '../../../lib/time'
import Badge from '../../../components/ui/Badge.vue'
import Pager from '../../../components/ui/Pager.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20
const FIELDS = ['keyword', 'status', 'mode']

const keyword = ref(readQuery(route.query.keyword))
const status = ref(readQuery(route.query.status))
const mode = ref(readQuery(route.query.mode))

watch(() => route.query, (query) => {
  keyword.value = readQuery(query.keyword)
  status.value = readQuery(query.status)
  mode.value = readQuery(query.mode)
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

const contests = useQuery({
  queryKey: computed(() => ['admin', 'contests', listing.value.params.toString()]),
  queryFn: ({ signal }) => api.request<ListContestsResp>(`/contests?${listing.value.params}`, { signal }),
})

const rows = computed(() => contests.data.value?.contests ?? [])
const page = computed(() => listing.value.page)

function statusTone(value: string): 'neutral' | 'success' | 'warning' | 'danger' | 'muted' | 'accent' {
  if (value === 'running') return 'accent'
  if (value === 'finished') return 'success'
  if (value === 'cancelled' || value === 'deleted') return 'danger'
  if (value === 'archived') return 'warning'
  return 'neutral'
}

function applyFilters(nextPage = 1) {
  const query: Record<string, string> = {}
  if (keyword.value.trim()) query.keyword = keyword.value.trim()
  if (status.value) query.status = status.value
  if (mode.value) query.mode = mode.value
  if (nextPage > 1) query.page = String(nextPage)
  void router.push({ query })
}

function resetFilters() {
  keyword.value = ''
  status.value = ''
  mode.value = ''
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
      <h1>比赛管理</h1>
      <div class="actions">
        <RouterLink class="btn" :to="{ name: 'admin-contest-new' }">创建比赛</RouterLink>
      </div>
    </div>

    <div class="panel">
      <form class="filter-bar" @submit.prevent="applyFilters()">
        <div class="field grow">
          <label for="cf-list-keyword">关键词</label>
          <input id="cf-list-keyword" v-model="keyword" type="search" placeholder="比赛标题">
        </div>
        <div class="field">
          <label for="cf-list-status">状态</label>
          <select id="cf-list-status" v-model="status">
            <option value="">全部</option>
            <option v-for="(label, value) in contestStatuses" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="cf-list-mode">赛制</label>
          <select id="cf-list-mode" v-model="mode">
            <option value="">全部</option>
            <option v-for="(label, value) in contestModes" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field filter-actions">
          <span class="sr-only"><label for="cf-list-submit">操作</label></span>
          <span class="row">
            <button id="cf-list-submit" class="sm" type="submit">筛选</button>
            <button class="quiet sm" type="button" @click="resetFilters">重置</button>
          </span>
        </div>
      </form>

      <StateBlock
        :pending="contests.isPending.value" :error="contests.error.value"
        :empty="!rows.length" empty-title="没有匹配的比赛"
        @retry="contests.refetch()"
      >
        <p class="small muted">调整筛选条件后重试，或创建新比赛。</p>
      </StateBlock>

      <template v-if="rows.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col" class="num">#</th>
                <th scope="col">标题</th>
                <th scope="col">赛制</th>
                <th scope="col">状态</th>
                <th scope="col">可见性</th>
                <th scope="col">报名策略</th>
                <th scope="col" class="num">报名中</th>
                <th scope="col" class="num">参赛人数</th>
                <th scope="col">开始时间</th>
                <th scope="col">结束时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="contest in rows" :key="String(contest.contest_id)">
                <td class="num">{{ contest.contest_id }}</td>
                <td>
                  <RouterLink class="contest-link" :to="`/admin/contests/${contest.contest_id}`">{{ contest.title }}</RouterLink>
                </td>
                <td class="small">{{ labelOf(contestModes, contest.mode) }}</td>
                <td><Badge :tone="statusTone(contest.status)" :running="contest.status === 'running'">{{ labelOf(contestStatuses, contest.status) }}</Badge></td>
                <td class="small">{{ labelOf(visibilities, contest.visibility) }}</td>
                <td class="small">{{ labelOf(registrationPolicies, contest.registration_policy) }}</td>
                <td class="num small">{{ contest.registration_open ? '是' : '否' }}</td>
                <td class="num small">{{ contest.participant_count }}</td>
                <td class="num small muted">{{ formatTime(contest.start_at) }}</td>
                <td class="num small muted">{{ formatTime(contest.end_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="contests.data.value ? hasNextPage(contests.data.value.total, page, PAGE_SIZE) : false"
          :total="contests.data.value?.total" :disabled="contests.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
.contest-link { font-weight: 500; overflow-wrap: anywhere; }
</style>
