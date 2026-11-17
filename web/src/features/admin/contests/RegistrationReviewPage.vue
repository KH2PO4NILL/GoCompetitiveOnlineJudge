<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ApiError } from '../../../api/client'
import type {
  ContestRegistration,
  ListRegistrationsResp,
  ReviewRegistrationResp,
} from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { hasNextPage, isValidId, pageNumber, readQuery } from '../../../lib/ids'
import { registrationStatuses, labelOf } from '../../../lib/labels'
import { formatTime } from '../../../lib/time'
import Badge from '../../../components/ui/Badge.vue'
import ConfirmDialog from '../../../components/ui/ConfirmDialog.vue'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import Pager from '../../../components/ui/Pager.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20
const contestIdText = computed(() => String(route.params.contestId ?? ''))
const idValid = computed(() => isValidId(contestIdText.value))
const contestId = computed(() => String(contestIdText.value))
const status = ref(readQuery(route.query.status))
const page = computed(() => pageNumber(readQuery(route.query.page) || null))

watch(() => route.query.status, (value) => { status.value = readQuery(value) })

const registrations = useQuery({
  queryKey: computed(() => ['admin', 'contest', contestId.value, 'registrations', status.value, page.value]),
  enabled: idValid,
  queryFn: ({ signal }) => {
    const params = new URLSearchParams({ page: String(page.value), page_size: String(PAGE_SIZE) })
    if (status.value) params.set('status', status.value)
    return api.request<ListRegistrationsResp>(`/contests/${contestId.value}/registrations?${params}`, { signal })
  },
})

const rows = computed(() => registrations.data.value?.registrations ?? [])
const total = computed(() => registrations.data.value?.total)

function statusTone(value: string): 'neutral' | 'success' | 'danger' | 'muted' {
  if (value === 'approved') return 'success'
  if (value === 'rejected') return 'danger'
  if (value === 'cancelled') return 'muted'
  return 'neutral'
}

// ---- 审核 ----
type Decision = 'approve' | 'reject'
const review = useAction()
const reviewing = ref<{ registration: ContestRegistration; decision: Decision } | null>(null)
const reviewNote = ref('')
const conflict = ref(false)

function openReview(registration: ContestRegistration, decision: Decision) {
  reviewNote.value = ''
  reviewing.value = { registration, decision }
}

async function submitReview() {
  const current = reviewing.value
  if (!current) return
  await review.run(async () => {
    try {
      const response = await api.request<ReviewRegistrationResp>(
        `/contests/${contestId.value}/registrations/${String(current.registration.registration_id)}/review`,
        {
          method: 'POST',
          body: {
            contest_id: current.registration.contest_id,
            registration_id: current.registration.registration_id,
            decision: current.decision,
            ...(reviewNote.value.trim() ? { review_note: reviewNote.value.trim() } : {}),
            expected_revision: current.registration.revision,
          },
        },
      )
      reviewing.value = null
      conflict.value = false
      await registrations.refetch()
      if (current.decision === 'approve') {
        return response.participant ? '报名已通过，参赛者已创建。' : '报名已通过。'
      }
      return '报名已拒绝。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        conflict.value = true
        await registrations.refetch()
      }
      throw failure
    }
  })
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
      <span>报名审核</span>
    </nav>
    <div class="page-head">
      <h1>报名审核</h1>
      <span v-if="total !== undefined" class="small muted">共 {{ total }} 条</span>
    </div>

    <p v-if="!idValid" class="notice notice-error" role="alert">比赛编号无效。</p>
    <div v-else class="panel">
      <form class="filter-bar" @submit.prevent="applyStatus(status)">
        <div class="field">
          <label for="reg-status">报名状态</label>
          <select id="reg-status" v-model="status" @change="applyStatus(status)">
            <option value="">全部</option>
            <option v-for="(label, value) in registrationStatuses" :key="value" :value="value">{{ label }}</option>
          </select>
        </div>
        <div class="field filter-actions">
          <span class="sr-only"><label for="reg-submit">操作</label></span>
          <button id="reg-submit" class="sm" type="submit">筛选</button>
        </div>
      </form>

      <p v-if="conflict" class="notice notice-warning" role="alert">
        该报名已被其他操作更新，列表已刷新。请核对最新状态后重新审核。
      </p>

      <StateBlock
        :pending="registrations.isPending.value" :error="registrations.error.value"
        :empty="!rows.length" empty-title="暂无报名记录"
        @retry="registrations.refetch()"
      >
        <p class="small muted">当前筛选条件下没有报名记录。</p>
      </StateBlock>

      <template v-if="rows.length">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col" class="num">报名编号</th>
                <th scope="col" class="num">用户</th>
                <th scope="col">状态</th>
                <th scope="col">申请说明</th>
                <th scope="col">审核备注</th>
                <th scope="col" class="num">rev</th>
                <th scope="col">提交时间</th>
                <th scope="col">审核时间</th>
                <th scope="col" class="num">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in rows" :key="String(item.registration_id)">
                <td class="num">{{ item.registration_id }}</td>
                <td class="num mono">{{ item.user_id }}</td>
                <td><Badge :tone="statusTone(item.status)">{{ labelOf(registrationStatuses, item.status) }}</Badge></td>
                <td class="note-cell">{{ item.application_note || '—' }}</td>
                <td class="note-cell">
                  {{ item.review_note || '—' }}
                  <span v-if="item.reviewer_id" class="small muted">（审核人 {{ item.reviewer_id }}）</span>
                </td>
                <td class="num">{{ item.revision }}</td>
                <td class="small muted">{{ formatTime(item.created_at) }}</td>
                <td class="small muted">{{ item.reviewed_at ? formatTime(item.reviewed_at) : '—' }}</td>
                <td class="num">
                  <span v-if="item.status === 'pending'" class="row actions-cell">
                    <button class="sm" type="button" :disabled="review.busy.value" @click="openReview(item, 'approve')">通过</button>
                    <button class="danger-quiet sm" type="button" :disabled="review.busy.value" @click="openReview(item, 'reject')">拒绝</button>
                  </span>
                  <span v-else class="small muted">—</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager
          :page="page" :has-next="total !== undefined ? hasNextPage(total, page, PAGE_SIZE) : false"
          :total="total" :disabled="registrations.isFetching.value"
          @prev="gotoPage(page - 1)" @next="gotoPage(page + 1)"
        />
      </template>

      <div class="panel-body feedback">
        <p v-if="review.notice.value" class="notice notice-success" role="status">{{ review.notice.value }}</p>
        <ErrorNotice :error="review.error.value" />
        <p v-if="review.uncertain.value" class="notice notice-warning" role="alert">
          审核结果未知：请刷新核对报名状态，不要盲目重试。
          <button class="link sm" type="button" @click="review.reset(); registrations.refetch()">我已核对，重置</button>
        </p>
      </div>
    </div>

    <ConfirmDialog
      v-if="reviewing"
      :title="reviewing.decision === 'approve' ? '通过报名' : '拒绝报名'"
      :message="reviewing.decision === 'approve' ? '通过后将创建参赛者记录。' : '拒绝后用户可以重新提交申请（如策略允许）。'"
      :confirm-text="reviewing.decision === 'approve' ? '通过' : '拒绝'"
      :danger="reviewing.decision === 'reject'"
      :busy="review.busy.value"
      @cancel="reviewing = null"
      @confirm="submitReview"
    >
      <dl class="kv-list">
        <div><dt>报名编号</dt><dd class="mono">{{ reviewing.registration.registration_id }}</dd></div>
        <div><dt>用户编号</dt><dd class="mono">{{ reviewing.registration.user_id }}</dd></div>
        <div><dt>当前 revision</dt><dd class="mono">{{ reviewing.registration.revision }}</dd></div>
      </dl>
      <div v-if="reviewing.registration.application_note" class="field">
        <span class="small muted">申请说明</span>
        <p class="note-cell">{{ reviewing.registration.application_note }}</p>
      </div>
      <div class="field">
        <label for="review-note">审核备注（选填）</label>
        <textarea id="review-note" v-model="reviewNote" rows="3" maxlength="2000" />
      </div>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.filter-actions { flex: 0 0 auto; }
.note-cell { max-width: 28rem; white-space: pre-wrap; overflow-wrap: anywhere; }
.actions-cell { gap: var(--space-1); flex-wrap: nowrap; }
.feedback { display: grid; gap: var(--space-2); }
</style>
