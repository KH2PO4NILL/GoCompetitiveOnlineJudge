<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { ListUsersResp } from '../../../api/contracts/admin'
import { useApi } from '../../../composables/useApi'
import { hasNextPage, isValidId, pageNumber, readQuery } from '../../../lib/ids'
import { listParams } from '../../../lib/forms'
import { labelOf, userRoles, userStatuses } from '../../../lib/labels'
import Badge from '../../../components/ui/Badge.vue'
import Pager from '../../../components/ui/Pager.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20
const FIELDS = ['keyword', 'status', 'role', 'school_id']

const keyword = ref(readQuery(route.query.keyword))
const status = ref(readQuery(route.query.status))
const role = ref(readQuery(route.query.role))
const schoolId = ref(readQuery(route.query.school_id))

watch(() => route.query, (query) => {
  keyword.value = readQuery(query.keyword)
  status.value = readQuery(query.status)
  role.value = readQuery(query.role)
  schoolId.value = readQuery(query.school_id)
})

const search = computed(() => {
  const sp = new URLSearchParams()
  for (const field of FIELDS) {
    const value = readQuery(route.query[field])
    if (value) sp.set(field, value)
  }
  return sp
})

const listing = computed(() => listParams(search.value, FIELDS, isValidId, pageNumber))
const page = computed(() => listing.value.page)

const users = useQuery({
  queryKey: computed(() => ['admin', 'users', listing.value.params.toString()]),
  enabled: computed(() => !listing.value.invalid),
  queryFn: ({ signal }) => api.request<ListUsersResp>(`/users?${listing.value.params}`, { signal }),
})

const rows = computed(() => users.data.value?.users ?? [])

function roleTone(value: string): 'neutral' | 'accent' | 'warning' {
  if (value === 'super_admin') return 'warning'
  if (value === 'admin') return 'accent'
  return 'neutral'
}
function statusTone(value: string): 'success' | 'warning' | 'danger' | 'muted' {
  if (value === 'normal') return 'success'
  if (value === 'disabled') return 'warning'
  if (value === 'banned') return 'danger'
  return 'muted'
}

function applyFilters(nextPage = 1) {
  const query: Record<string, string> = {}
  if (keyword.value.trim()) query.keyword = keyword.value.trim()
  if (status.value) query.status = status.value
  if (role.value) query.role = role.value
  if (schoolId.value.trim()) query.school_id = schoolId.value.trim()
  if (nextPage > 1) query.page = String(nextPage)
  void router.push({ query })
}

function resetFilters() {
  keyword.value = ''
  status.value = ''
  role.value = ''
  schoolId.value = ''
  void router.push({ query: {} })
}

function gotoPage(next: number) {
  const base = Object.fromEntries(search.value.entries())
  if (next > 1) base.page = String(next)
  else delete base.page
  void router.push({ query: base })
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1>用户管理</h1>
    </div>

    <p v-if="listing.invalid" class="notice notice-error" role="alert">筛选参数中的编号无效。</p>
    <div v-else class="panel">
      <form class="filter-bar" @submit.prevent="applyFilters()">
        <div class="field grow">
          <label for="f-keyword">关键词</label>
          <input id="f-keyword" v-model="keyword" type="search" placeholder="用户名 / 邮箱">
        </div>
        <div class="field">
          <label for="f-status">状态</label>
          <select id="f-status" v-model="status">
            <option value="">全部</option>
            <option v-for="(label, value) in userStatuses" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="f-role">角色</label>
          <select id="f-role" v-model="role">
            <option value="">全部</option>
            <option v-for="(label, value) in userRoles" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field">
          <label for="f-school">学校编号</label>
          <input id="f-school" v-model="schoolId" type="text" inputmode="numeric">
        </div>
        <div class="field filter-actions">
          <span class="sr-only"><label for="f-submit">操作</label></span>
          <span class="row">
            <button id="f-submit" class="sm" type="submit">筛选</button>
            <button class="quiet sm" type="button" @click="resetFilters">重置</button>
          </span>
        </div>
      </form>

      <StateBlock
        :pending="users.isPending.value" :error="users.error.value"
        :empty="!rows.length" empty-title="没有匹配的用户"
        @retry="users.refetch()"
      >
        <p class="small muted">调整筛选条件后重试。</p>
      </StateBlock>

      <template v-if="rows.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col" class="num">用户编号</th>
                <th scope="col">用户名</th>
                <th scope="col">邮箱</th>
                <th scope="col">角色</th>
                <th scope="col">状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="user in rows" :key="String(user.user_id)">
                <td class="num">
                  <RouterLink :to="`/admin/users/${user.user_id}`">{{ user.user_id }}</RouterLink>
                </td>
                <td>
                  <RouterLink class="user-link" :to="`/admin/users/${user.user_id}`">{{ user.username }}</RouterLink>
                </td>
                <td class="small wrap-anywhere">{{ user.email }}</td>
                <td><Badge :tone="roleTone(user.role)">{{ labelOf(userRoles, user.role) }}</Badge></td>
                <td><Badge :tone="statusTone(user.status)">{{ labelOf(userStatuses, user.status) }}</Badge></td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="users.data.value ? hasNextPage(users.data.value.total, page, PAGE_SIZE) : false"
          :total="users.data.value?.total" :disabled="users.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>
    </div>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
.user-link { font-weight: 500; }
</style>
