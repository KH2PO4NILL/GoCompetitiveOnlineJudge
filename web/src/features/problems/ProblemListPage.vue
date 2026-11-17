<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useRoute, useRouter } from 'vue-router'
import type { ListProblemsResp } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { hasNextPage, pageNumber, readQuery } from '../../lib/ids'
import { difficulties } from '../../lib/labels'
import ProblemTable from '../../components/data/ProblemTable.vue'
import Pager from '../../components/ui/Pager.vue'
import StateBlock from '../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20

const keyword = ref(readQuery(route.query.keyword))
const difficulty = ref(readQuery(route.query.difficulty))
const tags = ref(readQuery(route.query.tags))

watch(() => route.query, (query) => {
  keyword.value = readQuery(query.keyword)
  difficulty.value = readQuery(query.difficulty)
  tags.value = readQuery(query.tags)
})

const page = computed(() => pageNumber(readQuery(route.query.page) || null))

const search = computed(() => {
  const params = new URLSearchParams()
  if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
  if (difficulty.value) params.set('difficulty', difficulty.value)
  for (const tag of tags.value.split(/[,，\s]+/).filter(Boolean)) params.append('tags', tag)
  return params.toString()
})

const problems = useQuery({
  queryKey: computed(() => ['problems', search.value, page.value]),
  queryFn: ({ signal }) => {
    const params = new URLSearchParams(search.value)
    params.set('page', String(page.value))
    params.set('page_size', String(PAGE_SIZE))
    return api.request<ListProblemsResp>(`/problems?${params}`, { signal })
  },
})

function applyFilters(nextPage = 1) {
  const query: Record<string, string> = {}
  if (keyword.value.trim()) query.keyword = keyword.value.trim()
  if (difficulty.value) query.difficulty = difficulty.value
  if (tags.value.trim()) query.tags = tags.value.trim()
  if (nextPage > 1) query.page = String(nextPage)
  void router.push({ query })
}

function resetFilters() {
  keyword.value = ''
  difficulty.value = ''
  tags.value = ''
  void router.push({ query: {} })
}

function gotoPage(next: number) {
  const query = { ...route.query, page: next > 1 ? String(next) : undefined }
  void router.push({ query })
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1>题库</h1>
      <span v-if="problems.data.value" class="small muted">共 {{ problems.data.value.total }} 题</span>
    </div>
    <div class="panel">
      <form class="filter-bar" @submit.prevent="applyFilters()">
        <div class="field grow">
          <label for="filter-keyword">关键词</label>
          <input id="filter-keyword" v-model="keyword" type="search" name="keyword" placeholder="题目标题 / 内容">
        </div>
        <div class="field">
          <label for="filter-difficulty">难度</label>
          <select id="filter-difficulty" v-model="difficulty" name="difficulty">
            <option value="">全部</option>
            <option v-for="(label, value) in difficulties" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="filter-tags">标签</label>
          <input id="filter-tags" v-model="tags" type="text" name="tags" placeholder="逗号分隔">
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
        :pending="problems.isPending.value" :error="problems.error.value"
        :empty="!problems.data.value?.problems?.length" empty-title="没有匹配的题目"
        @retry="problems.refetch()"
      >
        <p class="small muted">调整筛选条件后重试。</p>
      </StateBlock>
      <template v-if="problems.data.value?.problems?.length">
        <ProblemTable :problems="problems.data.value.problems" />
        <Pager
          :page="page" :has-next="hasNextPage(problems.data.value.total, page, PAGE_SIZE)"
          :total="problems.data.value.total" :disabled="problems.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
</style>
