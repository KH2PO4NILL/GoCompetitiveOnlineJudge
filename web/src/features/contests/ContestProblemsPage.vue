<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { ListContestProblemsResp } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { hasNextPage, pageNumber, readQuery } from '../../lib/ids'
import Pager from '../../components/ui/Pager.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import { canViewProblems } from './helpers'
import { useContestContext } from './useContest'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 50

const { contestId, idValid, loggedIn, myStatus, detail, my } = useContestContext()

const page = computed(() => pageNumber(readQuery(route.query.page) || null))

// 可见性判定与 ContestLayout 的题目 tab 一致；未知（登录但状态未加载完）时不发请求
const access = computed(() => canViewProblems(loggedIn.value, my.value, detail.value?.status))
const accessPending = computed(() => loggedIn.value && my.value === undefined && myStatus.isPending.value)

const problems = useQuery({
  queryKey: computed(() => ['contest', contestId.value, 'problems', page.value]),
  enabled: computed(() => idValid.value && access.value),
  queryFn: ({ signal }) => api.request<ListContestProblemsResp>(
    `/contests/${contestId.value}/problems?page=${page.value}&page_size=${PAGE_SIZE}`, { signal }),
})

function gotoPage(next: number) {
  void router.push({ query: { ...route.query, page: next > 1 ? String(next) : undefined } })
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1>比赛题目</h1>
      <RouterLink class="small" :to="`/contests/${contestId}`">← 返回概览</RouterLink>
    </div>
    <p v-if="!idValid" class="notice notice-error" role="alert">比赛编号无效。</p>
    <div v-else-if="accessPending" class="panel">
      <div class="state-block" role="status">
        <span class="spinner" aria-hidden="true" />
        <span>正在确认题目可见性…</span>
      </div>
    </div>
    <div v-else-if="!access" class="panel">
      <div class="state-block">
        <strong>暂时无法查看题目</strong>
        <p v-if="!loggedIn" class="small muted">
          登录后才能确认题目可见性；比赛结束后题目将公开。
          <RouterLink :to="`/login?returnTo=${encodeURIComponent(route.fullPath)}`">去登录 →</RouterLink>
        </p>
        <p v-else class="small muted">
          当前账号没有查看题目的权限。
          <RouterLink :to="`/contests/${contestId}`">返回比赛概览报名 →</RouterLink>
        </p>
      </div>
    </div>
    <div v-else class="panel">
      <StateBlock
        :pending="problems.isPending.value" :error="problems.error.value"
        :empty="!problems.data.value?.problems?.length" empty-title="暂无可见题目"
        @retry="problems.refetch()"
      >
        <p class="small muted">
          题目可能尚未发布，或当前账号没有查看权限。
          <RouterLink :to="`/contests/${contestId}`">返回比赛概览 →</RouterLink>
        </p>
      </StateBlock>
      <p v-if="problems.error.value" class="back-link small">
        <RouterLink :to="`/contests/${contestId}`">← 返回比赛概览</RouterLink>
      </p>
      <template v-if="problems.data.value?.problems?.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">题号</th>
                <th scope="col">题目</th>
                <th scope="col" class="num">满分</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="problem in problems.data.value.problems" :key="String(problem.problem_id)">
                <td class="mono">{{ problem.label || '—' }}</td>
                <td>
                  <RouterLink
                    class="problem-link"
                    :to="{ name: 'contest-problem', params: { contestId, problemId: String(problem.problem_id) } }"
                  >
                    #{{ problem.problem_id }}
                  </RouterLink>
                </td>
                <td class="num">{{ problem.max_score }}</td>
              </tr>
            </tbody>
          </table>
        </div>
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
.problem-link { font-weight: 500; }
.back-link { padding: 0 var(--space-4) var(--space-3); margin: 0; }
</style>
