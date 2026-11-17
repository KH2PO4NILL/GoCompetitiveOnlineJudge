<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useSession } from '../../composables/useSession'
import { labelOf, userRoles } from '../../lib/labels'

const session = useSession()
const user = computed(() => session.value?.user)

const entries = [
  { name: 'admin-problems', title: '题目管理', desc: '创建、编辑、发布题目与测试数据。' },
  { name: 'admin-submissions', title: '提交与重判', desc: '筛选提交记录，单个或批量重判。' },
  { name: 'admin-contests', title: '比赛管理', desc: '编排比赛、审核报名、封榜与生命周期。' },
  { name: 'admin-users', title: '用户管理', desc: '查看用户，调整角色与状态。' },
] as const
</script>

<template>
  <div>
    <div class="page-head">
      <h1>管理工作台</h1>
      <span v-if="user" class="small muted">
        {{ user.username }} · {{ labelOf(userRoles, user.role) }}
      </span>
    </div>

    <div class="stack">
      <section class="panel">
        <div class="panel-head"><h2>管理入口</h2></div>
        <div class="panel-body">
          <ul class="entry-list">
            <li v-for="entry in entries" :key="entry.name" class="entry">
              <RouterLink class="entry-title" :to="{ name: entry.name }">{{ entry.title }}</RouterLink>
              <span class="small muted">{{ entry.desc }}</span>
            </li>
          </ul>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head"><h2>快捷操作</h2></div>
        <div class="panel-body">
          <div class="row">
            <RouterLink class="btn" :to="{ name: 'admin-problem-new' }">创建题目</RouterLink>
            <RouterLink class="btn btn-quiet" :to="{ name: 'admin-contest-new' }">创建比赛</RouterLink>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.entry-list { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--space-3); }
.entry { display: grid; gap: 2px; }
.entry-title { font-weight: 500; }
</style>
