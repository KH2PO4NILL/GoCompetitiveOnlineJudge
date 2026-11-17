<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { contestModes, contestStatuses, labelOf } from '../../lib/labels'
import { formatTime } from '../../lib/time'
import Badge from '../../components/ui/Badge.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import { canViewProblems, contestStatusTone } from './helpers'
import { useContestContext } from './useContest'

const route = useRoute()
const { contestId, idValid, loggedIn, contest, myStatus, detail, my } = useContestContext()

// 工作区在不同题目间切换时强制重挂载，保证代码草稿 key 与题目一致
const viewKey = computed(() => `${String(route.name ?? '')}:${String(route.params.problemId ?? '')}`)

const problemsVisible = computed(() =>
  canViewProblems(loggedIn.value, my.value, detail.value?.status))

const problemsHint = computed(() => {
  if (!loggedIn.value) return '登录后才能确认题目可见性；比赛结束后题目公开。'
  if (myStatus.isPending.value) return '正在确认题目可见性…'
  return '当前账号没有查看题目的权限，请先报名参赛。'
})
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink to="/contests">比赛</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>{{ detail?.title ?? contestId }}</span>
    </nav>
    <p v-if="!idValid" class="notice notice-error" role="alert">比赛编号无效。</p>
    <StateBlock v-else :pending="contest.isPending.value" :error="contest.error.value" @retry="contest.refetch()">
      <template #content>
        <div v-if="detail" class="stack">
          <header class="contest-head">
            <h1>{{ detail.title }}</h1>
            <div class="row contest-meta">
              <Badge :tone="contestStatusTone(detail.status)" :running="detail.status === 'running'">
                {{ labelOf(contestStatuses, detail.status) }}
              </Badge>
              <Badge tone="neutral">{{ labelOf(contestModes, detail.mode) }}</Badge>
              <span class="small muted mono">{{ formatTime(detail.start_at) }} — {{ formatTime(detail.end_at) }}</span>
            </div>
          </header>
          <nav class="contest-tabs" aria-label="比赛页面导航">
            <RouterLink class="tab" :to="`/contests/${contestId}`" exact-active-class="tab-active">概览</RouterLink>
            <RouterLink v-if="problemsVisible" class="tab" :to="`/contests/${contestId}/problems`" active-class="tab-active">题目</RouterLink>
            <span v-else class="tab tab-disabled" aria-disabled="true" :title="problemsHint">题目</span>
            <RouterLink class="tab" :to="`/contests/${contestId}/scoreboard`" active-class="tab-active">排行榜</RouterLink>
          </nav>
          <RouterView v-slot="{ Component }">
            <component :is="Component" :key="viewKey" />
          </RouterView>
        </div>
      </template>
    </StateBlock>
    <p v-if="idValid && contest.error.value" class="small contest-error-back">
      <RouterLink to="/contests">← 返回比赛列表</RouterLink>
    </p>
  </div>
</template>

<style scoped>
.contest-head { display: grid; gap: var(--space-2); }
.contest-head h1 { margin: 0; overflow-wrap: anywhere; }
.contest-meta { row-gap: var(--space-2); }
.contest-tabs {
  display: flex; gap: var(--space-1); border-bottom: 1px solid var(--border);
  overflow-x: auto; scrollbar-width: none;
}
.contest-tabs::-webkit-scrollbar { display: none; }
.contest-tabs .tab {
  padding: 7px 14px; font-size: var(--text-sm); color: var(--ink-secondary);
  border-bottom: 2px solid transparent; margin-bottom: -1px; white-space: nowrap;
}
.contest-tabs .tab:hover { color: var(--ink); background: var(--surface-sunken); text-decoration: none; }
.contest-tabs .tab-active { color: var(--accent); border-bottom-color: var(--accent); font-weight: 500; }
.contest-tabs .tab-disabled { color: var(--faint); cursor: not-allowed; }
.contest-error-back { margin-top: var(--space-3); }
</style>
