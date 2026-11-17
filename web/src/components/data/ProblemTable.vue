<script setup lang="ts">
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import type { ProblemSummary } from '../../api/contracts/app'
import DifficultyBadge from '../ui/DifficultyBadge.vue'

defineProps<{ problems: ProblemSummary[]; to?: (problem: ProblemSummary) => RouteLocationRaw }>()
</script>

<template>
  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th scope="col" class="num">#</th>
          <th scope="col">题目</th>
          <th scope="col">难度</th>
          <th scope="col">标签</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="problem in problems" :key="String(problem.problem_id)">
          <td class="num">{{ problem.problem_id }}</td>
          <td>
            <RouterLink class="problem-link" :to="to ? to(problem) : `/problems/${problem.problem_id}`">{{ problem.title }}</RouterLink>
          </td>
          <td><DifficultyBadge :difficulty="problem.difficulty" /></td>
          <td>
            <span class="tag-list">
              <RouterLink v-for="tag in problem.tags?.slice(0, 4)" :key="tag" class="tag" :to="`/problems?tags=${encodeURIComponent(tag)}`">{{ tag }}</RouterLink>
              <span v-if="(problem.tags?.length ?? 0) > 4" class="small muted">+{{ problem.tags.length - 4 }}</span>
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.problem-link { font-weight: 500; overflow-wrap: anywhere; }
td:first-child { color: var(--muted); }
</style>
