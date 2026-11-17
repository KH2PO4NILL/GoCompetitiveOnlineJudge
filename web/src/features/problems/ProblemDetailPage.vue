<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute } from 'vue-router'
import type { GetProblemResp } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { useSession } from '../../composables/useSession'
import { isValidId } from '../../lib/ids'
import { checkerTypes, labelOf, difficulties } from '../../lib/labels'
import DifficultyBadge from '../../components/ui/DifficultyBadge.vue'
import MarkdownView from '../../components/ui/MarkdownView.vue'
import StateBlock from '../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const session = useSession()

const problemId = computed(() => String(route.params.problemId ?? ''))
const idValid = computed(() => isValidId(problemId.value))

const problem = useQuery({
  queryKey: computed(() => ['problem', problemId.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetProblemResp>(`/problems/${problemId.value}`, { signal }),
})

const info = computed(() => problem.data.value?.problem)
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink to="/problems">题库</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>{{ info?.statement.title ?? problemId }}</span>
    </nav>
    <p v-if="!idValid" class="notice notice-error" role="alert">题目编号无效。</p>
    <StateBlock v-else :pending="problem.isPending.value" :error="problem.error.value" @retry="problem.refetch()">
      <template #content>
        <div v-if="info" class="grid-2">
          <div class="stack">
            <section class="panel">
              <div class="panel-body">
                <div class="page-head title-head">
                  <h1>{{ info.statement.title }}</h1>
                  <span class="row title-badges">
                    <DifficultyBadge :difficulty="info.metadata.difficulty" />
                    <span class="small muted mono">#{{ info.problem_id }}</span>
                  </span>
                </div>
                <MarkdownView :content="info.statement.description" />
              </div>
              <div class="panel-body">
                <h2>输入格式</h2>
                <MarkdownView :content="info.statement.input_description" />
                <h2>输出格式</h2>
                <MarkdownView :content="info.statement.output_description" />
              </div>
              <div v-if="info.statement.samples?.length" class="panel-body">
                <h2>样例</h2>
                <div v-for="(sample, index) in info.statement.samples" :key="index" class="sample-group">
                  <div class="sample-grid">
                    <div class="sample-box">
                      <h4>输入 #{{ index + 1 }}</h4>
                      <pre>{{ sample.input }}</pre>
                    </div>
                    <div class="sample-box">
                      <h4>输出 #{{ index + 1 }}</h4>
                      <pre>{{ sample.output }}</pre>
                    </div>
                  </div>
                  <p v-if="sample.explanation" class="small muted sample-explanation">{{ sample.explanation }}</p>
                </div>
              </div>
              <div v-if="info.statement.hint" class="panel-body">
                <h2>提示</h2>
                <MarkdownView :content="info.statement.hint" />
              </div>
            </section>
          </div>
          <aside class="stack">
            <section class="panel">
              <div class="panel-body">
                <RouterLink v-if="session" class="btn block" :to="`/problems/${info.problem_id}/submit`">提交代码</RouterLink>
                <RouterLink v-else class="btn block" :to="`/login?returnTo=${encodeURIComponent(`/problems/${info.problem_id}/submit`)}`">登录后提交</RouterLink>
                <RouterLink class="btn block quiet submit-history" :to="`/submissions?problem_id=${info.problem_id}`">本题评测记录</RouterLink>
              </div>
            </section>
            <section class="panel">
              <div class="panel-head"><h2>题目信息</h2></div>
              <div class="panel-body">
                <dl class="kv-list">
                  <div><dt>难度</dt><dd>{{ labelOf(difficulties, info.metadata.difficulty) }}</dd></div>
                  <div><dt>时间限制</dt><dd class="mono">{{ info.judge_settings.time_limit_ms }} ms</dd></div>
                  <div><dt>内存限制</dt><dd class="mono">{{ info.judge_settings.memory_limit_mb }} MB</dd></div>
                  <div><dt>比较器</dt><dd>{{ labelOf(checkerTypes, info.judge_settings.checker_type) }}</dd></div>
                  <div v-if="info.metadata.source"><dt>来源</dt><dd>{{ info.metadata.source }}</dd></div>
                </dl>
                <h3 class="side-subtitle">支持语言</h3>
                <span class="tag-list">
                  <span v-for="language in info.judge_settings.allowed_languages" :key="language" class="tag mono">{{ language }}</span>
                </span>
                <template v-if="info.metadata.tags?.length">
                  <h3 class="side-subtitle">标签</h3>
                  <span class="tag-list">
                    <RouterLink v-for="tag in info.metadata.tags" :key="tag" class="tag" :to="`/problems?tags=${encodeURIComponent(tag)}`">{{ tag }}</RouterLink>
                  </span>
                </template>
              </div>
            </section>
          </aside>
        </div>
      </template>
    </StateBlock>
  </div>
</template>

<style scoped>
.title-head { margin-bottom: var(--space-4); }
.title-head h1 { margin: 0; }
.title-badges { flex-shrink: 0; }
.sample-group + .sample-group { margin-top: var(--space-4); }
.sample-explanation { margin: var(--space-2) 0 0; }
.submit-history { margin-top: var(--space-2); }
.side-subtitle { margin: var(--space-4) 0 var(--space-2); font-size: var(--text-xs); color: var(--muted); font-weight: 500; }
</style>
