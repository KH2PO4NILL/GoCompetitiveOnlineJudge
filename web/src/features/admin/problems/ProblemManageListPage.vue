<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { ListProblemsResp } from '../../../api/contracts/admin'
import { useApi } from '../../../composables/useApi'
import { hasNextPage, isValidId, pageNumber, readQuery } from '../../../lib/ids'
import { listParams } from '../../../lib/forms'
import { difficulties, labelOf, problemStatuses, visibilities } from '../../../lib/labels'
import { formatTime } from '../../../lib/time'
import Badge from '../../../components/ui/Badge.vue'
import DifficultyBadge from '../../../components/ui/DifficultyBadge.vue'
import Pager from '../../../components/ui/Pager.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20
const FIELDS = ['keyword', 'status', 'visibility', 'difficulty', 'tags', 'author_id']

const keyword = ref(readQuery(route.query.keyword))
const status = ref(readQuery(route.query.status))
const visibility = ref(readQuery(route.query.visibility))
const difficulty = ref(readQuery(route.query.difficulty))
const tags = ref(readQuery(route.query.tags))
const authorId = ref(readQuery(route.query.author_id))

watch(() => route.query, (query) => {
  keyword.value = readQuery(query.keyword)
  status.value = readQuery(query.status)
  visibility.value = readQuery(query.visibility)
  difficulty.value = readQuery(query.difficulty)
  tags.value = readQuery(query.tags)
  authorId.value = readQuery(query.author_id)
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

const problems = useQuery({
  queryKey: computed(() => ['admin', 'problems', listing.value.params.toString()]),
  enabled: computed(() => !listing.value.invalid),
  queryFn: ({ signal }) => api.request<ListProblemsResp>(`/problems?${listing.value.params}`, { signal }),
})

const rows = computed(() => problems.data.value?.problems ?? [])
const page = computed(() => listing.value.page)

function statusTone(value: string): 'neutral' | 'success' | 'warning' | 'danger' | 'muted' {
  if (value === 'published') return 'success'
  if (value === 'archived') return 'warning'
  if (value === 'deleted') return 'danger'
  return 'neutral'
}

function applyFilters(nextPage = 1) {
  const query: Record<string, string> = {}
  if (keyword.value.trim()) query.keyword = keyword.value.trim()
  if (status.value) query.status = status.value
  if (visibility.value) query.visibility = visibility.value
  if (difficulty.value) query.difficulty = difficulty.value
  if (tags.value.trim()) query.tags = tags.value.trim()
  if (authorId.value.trim()) query.author_id = authorId.value.trim()
  if (nextPage > 1) query.page = String(nextPage)
  void router.push({ query })
}

function resetFilters() {
  keyword.value = ''
  status.value = ''
  visibility.value = ''
  difficulty.value = ''
  tags.value = ''
  authorId.value = ''
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
      <h1>题目管理</h1>
      <div class="actions">
        <RouterLink class="btn" :to="{ name: 'admin-problem-new' }">创建题目</RouterLink>
      </div>
    </div>

    <p v-if="listing.invalid" class="notice notice-error" role="alert">筛选参数中的编号无效。</p>
    <div v-else class="panel">
      <form class="filter-bar" @submit.prevent="applyFilters()">
        <div class="field grow">
          <label for="f-keyword">关键词</label>
          <input id="f-keyword" v-model="keyword" type="search" placeholder="标题 / 内容">
        </div>
        <div class="field">
          <label for="f-status">状态</label>
          <select id="f-status" v-model="status">
            <option value="">全部</option>
            <option v-for="(label, value) in problemStatuses" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="f-visibility">可见性</label>
          <select id="f-visibility" v-model="visibility">
            <option value="">全部</option>
            <option v-for="(label, value) in visibilities" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="f-difficulty">难度</label>
          <select id="f-difficulty" v-model="difficulty">
            <option value="">全部</option>
            <option v-for="(label, value) in difficulties" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="f-tags">标签</label>
          <input id="f-tags" v-model="tags" type="text" placeholder="逗号分隔">
        </div>
        <div class="field">
          <label for="f-author">作者编号</label>
          <input id="f-author" v-model="authorId" type="text" inputmode="numeric">
        </div>
        <div class="field filter-actions">
          <span class="sr-only"><label for="f-submit">操作</label></span>
          <span class="row">
            <button id="f-submit" class="sm" type="submit">筛选</button>
            <button class="quiet sm" type="button" @click="resetFilters">重置</button>
          </span>
        </div>
      </form>

      <StateBlock
        :pending="problems.isPending.value" :error="problems.error.value"
        :empty="!rows.length" empty-title="没有匹配的题目"
        @retry="problems.refetch()"
      >
        <p class="small muted">调整筛选条件后重试，或创建新题目。</p>
      </StateBlock>

      <template v-if="rows.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col" class="num">#</th>
                <th scope="col">标题</th>
                <th scope="col">状态</th>
                <th scope="col">可见性</th>
                <th scope="col">难度</th>
                <th scope="col">标签</th>
                <th scope="col">更新时间</th>
                <th scope="col" class="num">rev</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="problem in rows" :key="String(problem.problem_id)">
                <td class="num">{{ problem.problem_id }}</td>
                <td>
                  <RouterLink class="problem-link" :to="`/admin/problems/${problem.problem_id}`">{{ problem.title }}</RouterLink>
                </td>
                <td><Badge :tone="statusTone(problem.status)">{{ labelOf(problemStatuses, problem.status) }}</Badge></td>
                <td class="small">{{ labelOf(visibilities, problem.visibility) }}</td>
                <td><DifficultyBadge :difficulty="problem.difficulty" /></td>
                <td>
                  <span class="tag-list">
                    <span v-for="tag in problem.tags?.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
                    <span v-if="(problem.tags?.length ?? 0) > 3" class="small muted">+{{ problem.tags.length - 3 }}</span>
                  </span>
                </td>
                <td class="num small muted">{{ formatTime(problem.updated_at) }}</td>
                <td class="num small muted">{{ problem.revision }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="problems.data.value ? hasNextPage(problems.data.value.total, page, PAGE_SIZE) : false"
          :total="problems.data.value?.total" :disabled="problems.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
.problem-link { font-weight: 500; overflow-wrap: anywhere; }
</style>
