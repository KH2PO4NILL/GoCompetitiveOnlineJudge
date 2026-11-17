<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useRoute, useRouter } from 'vue-router'
import type { GetScoreboardResp, ScoreboardProblemResult } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { hasNextPage, pageNumber, readQuery } from '../../lib/ids'
import { contestModes, labelOf } from '../../lib/labels'
import { formatTime } from '../../lib/time'
import Pager from '../../components/ui/Pager.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import { icpcCellText, icpcTotalPenaltyText, scoreCellText, totalScoreText } from './helpers'
import { useContestContext } from './useContest'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 50

const { contestId, idValid, detail } = useContestContext()

const page = computed(() => pageNumber(readQuery(route.query.page) || null))

const scoreboard = useQuery({
  queryKey: computed(() => ['contest', contestId.value, 'scoreboard', page.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetScoreboardResp>(
    `/contests/${contestId.value}/scoreboard?page=${page.value}&page_size=${PAGE_SIZE}`, { signal }),
  // 比赛进行中 15s 轮询；进入终态后自动停止
  refetchInterval: () => (detail.value?.status === 'running' ? 15000 : false),
})

const board = computed(() => scoreboard.data.value?.scoreboard)
const isIcpc = computed(() => board.value?.mode === 'icpc')
const isRunning = computed(() => detail.value?.status === 'running')

// 列头取首行题目集合（服务端保证各行题目结构一致），64 位 ID 一律 String
const columns = computed(() => (board.value?.rows?.[0]?.problem_results ?? []).map((result) => ({
  key: String(result.problem_id),
  label: result.label || `#${String(result.problem_id)}`,
})))

const rows = computed(() => (board.value?.rows ?? []).map((row) => ({
  row,
  results: new Map<string, ScoreboardProblemResult>(
    (row.problem_results ?? []).map((result) => [String(result.problem_id), result]),
  ),
})))

function icpcCell(results: Map<string, ScoreboardProblemResult>, key: string): string {
  return icpcCellText(results.get(key)?.icpc)
}

function gotoPage(next: number) {
  void router.push({ query: { ...route.query, page: next > 1 ? String(next) : undefined } })
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1>排行榜</h1>
      <span v-if="board" class="small muted">共 {{ board.total }} 行</span>
    </div>
    <p v-if="!idValid" class="notice notice-error" role="alert">比赛编号无效。</p>
    <div v-else class="panel">
      <StateBlock
        :pending="scoreboard.isPending.value" :error="scoreboard.error.value"
        :empty="!rows.length" empty-title="暂无榜单数据"
        @retry="scoreboard.refetch()"
      >
        <p class="small muted">榜单将在比赛开始后生成，或当前账号没有查看权限。</p>
      </StateBlock>
      <template v-if="board && rows.length">
        <p v-if="board.frozen" class="notice notice-warning board-frozen" role="status">
          榜单已封榜：当前展示封榜时点的排名（视图：{{ board.view || '—' }}）。
        </p>
        <p class="small muted board-meta">
          赛制 {{ labelOf(contestModes, board.mode) }}
          · 视图 {{ board.view || '—' }}
          <template v-if="board.generated_at">· 生成于 {{ formatTime(board.generated_at) }}</template>
          <template v-if="isRunning">· 比赛进行中，每 15 秒自动刷新</template>
        </p>
        <div class="table-wrap" aria-live="polite">
          <table v-if="isIcpc">
            <thead>
              <tr>
                <th scope="col" class="num">排名</th>
                <th scope="col">用户</th>
                <th v-for="column in columns" :key="column.key" scope="col" class="num">{{ column.label }}</th>
                <th scope="col" class="num">通过</th>
                <th scope="col" class="num">总罚时</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="view in rows" :key="String(view.row.user_id)">
                <td class="num">{{ view.row.rank }}</td>
                <td class="user-cell">{{ view.row.display_name }}</td>
                <td v-for="column in columns" :key="column.key" class="num score-cell">
                  {{ icpcCell(view.results, column.key) }}
                </td>
                <td class="num">{{ view.row.icpc?.solved_count ?? '—' }}</td>
                <td class="num">{{ icpcTotalPenaltyText(view.row.icpc?.total_penalty_seconds) }}</td>
              </tr>
            </tbody>
          </table>
          <table v-else>
            <thead>
              <tr>
                <th scope="col" class="num">排名</th>
                <th scope="col">用户</th>
                <th v-for="column in columns" :key="column.key" scope="col" class="num">{{ column.label }}</th>
                <th scope="col" class="num">总分</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="view in rows" :key="String(view.row.user_id)">
                <td class="num">{{ view.row.rank }}</td>
                <td class="user-cell">{{ view.row.display_name }}</td>
                <td v-for="column in columns" :key="column.key" class="num score-cell">
                  {{ scoreCellText(view.results.get(column.key)) }}
                </td>
                <td class="num">{{ totalScoreText(view.row) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="hasNextPage(board.total, page, PAGE_SIZE)"
          :total="board.total" :disabled="scoreboard.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.board-frozen { margin: var(--space-3) var(--space-4) 0; }
.board-meta { padding: var(--space-3) var(--space-4) 0; margin: 0; }
.user-cell { overflow-wrap: anywhere; }
.score-cell { white-space: nowrap; }
</style>
