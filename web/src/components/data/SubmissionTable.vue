<script setup lang="ts">
import { RouterLink } from 'vue-router'
import type { SubmissionSummary } from '../../api/contracts/app'
import { formatTime } from '../../lib/time'
import VerdictBadge from '../ui/VerdictBadge.vue'

defineProps<{ submissions: SubmissionSummary[]; showUser?: boolean }>()
</script>

<template>
  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th scope="col" class="num">#</th>
          <th v-if="showUser" scope="col" class="num">用户</th>
          <th scope="col" class="num">题目</th>
          <th scope="col">结果</th>
          <th scope="col">语言</th>
          <th scope="col">提交时间</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="submission in submissions" :key="String(submission.submission_id)">
          <td class="num">
            <RouterLink :to="`/submissions/${submission.submission_id}`">{{ submission.submission_id }}</RouterLink>
          </td>
          <td v-if="showUser" class="num">{{ submission.user_id }}</td>
          <td class="num">
            <RouterLink :to="`/problems/${submission.problem_id}`">{{ submission.problem_id }}</RouterLink>
          </td>
          <td><VerdictBadge :submission="submission" /></td>
          <td class="mono small">{{ submission.language }}</td>
          <td class="num small muted">{{ formatTime(submission.submitted_at) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
