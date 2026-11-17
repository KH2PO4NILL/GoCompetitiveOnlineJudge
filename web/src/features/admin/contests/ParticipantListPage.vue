<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { ListParticipantsResp } from '../../../api/contracts/admin'
import { useApi } from '../../../composables/useApi'
import { hasNextPage, isValidId, pageNumber, readQuery } from '../../../lib/ids'
import { labelOf, participantStatuses } from '../../../lib/labels'
import { formatTime } from '../../../lib/time'
import Badge from '../../../components/ui/Badge.vue'
import Pager from '../../../components/ui/Pager.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20
const contestIdText = computed(() => String(route.params.contestId ?? ''))
const idValid = computed(() => isValidId(contestIdText.value))
const status = ref(readQuery(route.query.status))
const page = computed(() => pageNumber(readQuery(route.query.page) || null))

watch(() => route.query.status, (value) => { status.value = readQuery(value) })

const participants = useQuery({
  queryKey: computed(() => ['admin', 'contest', contestIdText.value, 'participants', status.value, page.value]),
  enabled: idValid,
  queryFn: ({ signal }) => {
    const params = new URLSearchParams({ page: String(page.value), page_size: String(PAGE_SIZE) })
    if (status.value) params.set('status', status.value)
    return api.request<ListParticipantsResp>(`/contests/${contestIdText.value}/participants?${params}`, { signal })
  },
})

const rows = computed(() => participants.data.value?.participants ?? [])
const total = computed(() => participants.data.value?.total)

function statusTone(value: string): 'success' | 'warning' | 'danger' {
  if (value === 'normal') return 'success'
  if (value === 'withdrawn') return 'warning'
  return 'danger'
}

function applyStatus(value: string) {
  void router.push({ query: { ...route.query, status: value || undefined, page: undefined } })
}

function gotoPage(next: number) {
  void router.push({ query: { ...route.query, page: next > 1 ? String(next) : undefined } })
}
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-contests' }">比赛管理</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <RouterLink :to="`/admin/contests/${contestIdText}`">#{{ contestIdText }}</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>参赛名单</span>
    </nav>
    <div class="page-head">
      <h1>参赛名单</h1>
      <span v-if="total !== undefined" class="small muted">共 {{ total }} 人</span>
    </div>

    <p v-if="!idValid" class="notice notice-error" role="alert">比赛编号无效。</p>
    <div v-else class="panel">
      <form class="filter-bar" @submit.prevent="applyStatus(status)">
        <div class="field">
          <label for="participant-status">参赛状态</label>
          <select id="participant-status" v-model="status" @change="applyStatus(status)">
            <option value="">全部</option>
            <option v-for="(label, value) in participantStatuses" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field filter-actions">
          <span class="sr-only"><label for="participant-submit">操作</label></span>
          <button id="participant-submit" class="sm" type="submit">筛选</button>
        </div>
      </form>

      <StateBlock
        :pending="participants.isPending.value" :error="participants.error.value"
        :empty="!rows.length" empty-title="暂无参赛者"
        @retry="participants.refetch()"
      >
        <p class="small muted">当前筛选条件下没有参赛者。报名通过后会自动加入名单。</p>
      </StateBlock>

      <template v-if="rows.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col" class="num">用户编号</th>
                <th scope="col">状态</th>
                <th scope="col">取得资格时间</th>
                <th scope="col">更新时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in rows" :key="String(item.user_id)">
                <td class="num mono">{{ item.user_id }}</td>
                <td><Badge :tone="statusTone(item.status)">{{ labelOf(participantStatuses, item.status) }}</Badge></td>
                <td class="small muted">{{ formatTime(item.qualified_at) }}</td>
                <td class="small muted">{{ formatTime(item.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="total !== undefined ? hasNextPage(total, page, PAGE_SIZE) : false"
          :total="total" :disabled="participants.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
</style>
