<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink } from 'vue-router'
import type { ListContestsResp, ListProblemsResp } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { useSession } from '../../composables/useSession'
import ProblemTable from '../../components/data/ProblemTable.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import ContestCarousel from './ContestCarousel.vue'
import AlgorithmDemo from './AlgorithmDemo.vue'

const api = useApi()
const session = useSession()

const latest = useQuery({
  queryKey: ['home', 'latest-problems'],
  queryFn: ({ signal }) => api.request<ListProblemsResp>('/problems?page=1&page_size=5', { signal }),
})

// 公开比赛按开始时间倒序：进行中与未开始即最近发布、仍可进入的比赛
const recentContests = useQuery({
  queryKey: ['home', 'recent-contests'],
  queryFn: ({ signal }) => api.request<ListContestsResp>('/contests?page=1&page_size=10', { signal }),
})

const carouselContests = computed(() =>
  (recentContests.data.value?.contests ?? [])
    .filter((contest) => contest.status === 'running' || contest.status === 'published')
    .slice(0, 5),
)
</script>

<template>
  <div class="stack">
    <ContestCarousel :contests="carouselContests" />

    <section class="panel">
      <div class="panel-body home-intro">
        <div>
          <h1>GCOJ 在线评测</h1>
          <p class="muted intro-text">
            浏览题库、编写解法、提交代码并跟踪真实评测结果。
            <template v-if="session">欢迎回来，{{ session.user.username }}。</template>
          </p>
          <div class="row intro-actions">
            <RouterLink class="btn" to="/problems">进入题库</RouterLink>
            <RouterLink class="btn btn-quiet" to="/contests">查看比赛</RouterLink>
            <RouterLink v-if="!session" class="btn btn-quiet" to="/login">登录 / 注册</RouterLink>
            <RouterLink v-else class="btn btn-quiet" to="/submissions">我的评测记录</RouterLink>
          </div>
        </div>
      </div>
    </section>

    <AlgorithmDemo />

    <section class="panel">
      <div class="panel-head">
        <h2>最新题目</h2>
        <RouterLink class="small" to="/problems">全部题目 →</RouterLink>
      </div>
      <StateBlock
        :pending="latest.isPending.value" :error="latest.error.value"
        :empty="!latest.data.value?.problems?.length" empty-title="暂无公开题目"
        @retry="latest.refetch()"
      >
        <template #content>
          <ProblemTable :problems="latest.data.value!.problems" />
        </template>
      </StateBlock>
    </section>
  </div>
</template>

<style scoped>
.home-intro h1 { margin-bottom: var(--space-2); }
.intro-text { margin: 0 0 var(--space-4); max-width: 560px; }
.intro-actions { gap: var(--space-2); }
</style>
