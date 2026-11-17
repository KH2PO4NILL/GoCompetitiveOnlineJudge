<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import type { CancelRegistrationReq, CancelRegistrationResp, RegisterContestReq, RegisterContestResp } from '../../api/contracts/app'
import { ApiError } from '../../api/client'
import { useApi } from '../../composables/useApi'
import { useAction } from '../../composables/useAction'
import { parseIdParam } from '../../lib/ids'
import {
  contestModes, contestStatuses, labelOf, participantStatuses,
  registrationPolicies, registrationStatuses, visibilities,
} from '../../lib/labels'
import { formatTime } from '../../lib/time'
import Badge from '../../components/ui/Badge.vue'
import ConfirmDialog from '../../components/ui/ConfirmDialog.vue'
import ErrorNotice from '../../components/ui/ErrorNotice.vue'
import MarkdownView from '../../components/ui/MarkdownView.vue'
import TextField from '../../components/ui/TextField.vue'
import {
  contestStatusTone, isRegistrationCancellable, participantTone,
  registrationClosedReason, registrationTone,
} from './helpers'
import { useContestContext } from './useContest'

const api = useApi()
const route = useRoute()
const { contestId, loggedIn, contest, myStatus, detail, my } = useContestContext()

const registration = computed(() => my.value?.registration)
const participant = computed(() => my.value?.participant)
const policy = computed(() => detail.value?.registration_policy ?? '')
const showInviteCode = computed(() => policy.value === 'invite_only')
const showApplicationNote = computed(() => policy.value === 'approval_required')
const cancellable = computed(() => isRegistrationCancellable(registration.value, detail.value?.registration_open ?? false))
const showForm = computed(() => (detail.value?.registration_open ?? false)
  && (!registration.value || registration.value.status === 'cancelled' || registration.value.status === 'rejected'))
const closedReason = computed(() => (detail.value ? registrationClosedReason(detail.value, Date.now()) : ''))
const canSubmitForm = computed(() => !showInviteCode.value || inviteCode.value.trim().length > 0)

const inviteCode = ref('')
const applicationNote = ref('')
const confirmCancel = ref(false)
const conflictNotice = ref('')

const register = useAction()
const cancel = useAction()

const loginReturnTo = computed(() => `/login?returnTo=${encodeURIComponent(route.fullPath)}`)

async function submitRegistration() {
  const contestPk = parseIdParam(contestId.value)
  if (!contestPk || !canSubmitForm.value) return
  // 报名无幂等键：结果未知（uncertain）后锁死，必须先核对服务端结果，绝不自动重试
  await register.run(async () => {
    const body: RegisterContestReq = { contest_id: contestPk }
    if (showInviteCode.value) body.invite_code = inviteCode.value.trim()
    if (showApplicationNote.value) body.application_note = applicationNote.value.trim()
    await api.request<RegisterContestResp>(`/contests/${contestId.value}/registrations`, { method: 'POST', body })
    inviteCode.value = ''
    applicationNote.value = ''
    conflictNotice.value = ''
    await Promise.all([myStatus.refetch(), contest.refetch()])
    return '报名请求已提交。'
  })
}

async function submitCancel() {
  const contestPk = parseIdParam(contestId.value)
  const revision = registration.value?.revision
  if (!contestPk || revision === undefined) return
  // 取消报名走乐观锁（expected_revision），同样无幂等键，uncertain 后锁死
  await cancel.run(async () => {
    try {
      const body: CancelRegistrationReq = { contest_id: contestPk, expected_revision: revision }
      await api.request<CancelRegistrationResp>(`/contests/${contestId.value}/registrations/cancel`, { method: 'POST', body })
      confirmCancel.value = false
      conflictNotice.value = ''
      await Promise.all([myStatus.refetch(), contest.refetch()])
      return '已取消报名。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        // 修订号冲突：刷新最新状态，保留用户重试入口
        conflictNotice.value = '报名状态已被更新（修订号冲突），已刷新最新状态；请确认后再决定是否重试取消。'
        confirmCancel.value = false
        await Promise.all([myStatus.refetch(), contest.refetch()])
      }
      throw failure
    }
  })
}
</script>

<template>
  <div v-if="detail" class="grid-2">
    <div class="stack">
      <section class="panel">
        <div class="panel-head"><h2>比赛公告</h2></div>
        <div class="panel-body">
          <MarkdownView v-if="detail.description" :content="detail.description" />
          <p v-else class="small muted">主办方没有提供比赛描述。</p>
        </div>
      </section>
    </div>
    <aside class="stack">
      <section class="panel">
        <div class="panel-head"><h2>报名</h2></div>
        <div class="panel-body">
          <template v-if="!loggedIn">
            <p class="small muted">登录后可报名参赛并查看个人参赛状态。</p>
            <RouterLink class="btn block" :to="loginReturnTo">登录后报名</RouterLink>
          </template>
          <div v-else-if="myStatus.isPending.value" class="state-block" role="status">
            <span class="spinner" aria-hidden="true" />
            <span>正在加载参赛状态…</span>
          </div>
          <template v-else>
            <ErrorNotice :error="myStatus.error.value" />
            <div v-if="registration" class="row reg-status-row">
              <span class="small muted">报名状态</span>
              <Badge :tone="registrationTone(registration.status)">{{ labelOf(registrationStatuses, registration.status) }}</Badge>
            </div>
            <div v-if="participant" class="row reg-status-row">
              <span class="small muted">参赛资格</span>
              <Badge :tone="participantTone(participant.status)">{{ labelOf(participantStatuses, participant.status) }}</Badge>
            </div>
            <p v-if="registration?.review_note" class="small muted reg-review-note">审核备注：{{ registration.review_note }}</p>
            <p v-if="conflictNotice" class="notice notice-warning reg-notice" role="alert">{{ conflictNotice }}</p>
            <p v-if="register.notice.value" class="notice notice-success reg-notice" role="status">{{ register.notice.value }}</p>
            <p v-if="cancel.notice.value" class="notice notice-success reg-notice" role="status">{{ cancel.notice.value }}</p>

            <template v-if="showForm">
              <form class="reg-form" @submit.prevent="submitRegistration()">
                <TextField
                  v-if="showInviteCode" v-model="inviteCode" label="邀请码" name="invite-code"
                  required placeholder="输入主办方提供的邀请码" auto-complete="off"
                />
                <TextField
                  v-if="showApplicationNote" v-model="applicationNote" label="申请备注" name="application-note"
                  type="textarea" :rows="3" placeholder="向审核人说明你的参赛情况（可选）"
                />
                <button type="submit" :disabled="register.busy.value || register.uncertain.value || !canSubmitForm">
                  {{ register.busy.value ? '提交中…' : register.uncertain.value ? '结果未知 · 请先核对' : (policy === 'approval_required' ? '提交报名申请' : '报名参赛') }}
                </button>
              </form>
              <ErrorNotice :error="register.error.value" />
            </template>
            <p v-else-if="!detail.registration_open && (!registration || registration.status === 'cancelled' || registration.status === 'rejected')" class="notice reg-notice">
              {{ closedReason }}
            </p>

            <template v-if="cancellable">
              <button class="danger-quiet block cancel-btn" type="button" :disabled="cancel.busy.value || cancel.uncertain.value" @click="confirmCancel = true">
                {{ cancel.uncertain.value ? '结果未知 · 请先核对' : '取消报名' }}
              </button>
              <ErrorNotice :error="cancel.error.value" />
            </template>
          </template>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head"><h2>比赛信息</h2></div>
        <div class="panel-body">
          <dl class="kv-list">
            <div><dt>赛制</dt><dd>{{ labelOf(contestModes, detail.mode) }}</dd></div>
            <div>
              <dt>状态</dt>
              <dd><Badge :tone="contestStatusTone(detail.status)" :running="detail.status === 'running'">{{ labelOf(contestStatuses, detail.status) }}</Badge></dd>
            </div>
            <div><dt>开始时间</dt><dd class="mono">{{ formatTime(detail.start_at) }}</dd></div>
            <div><dt>结束时间</dt><dd class="mono">{{ formatTime(detail.end_at) }}</dd></div>
            <div><dt>报名窗口</dt><dd class="mono">{{ formatTime(detail.registration_start_at) }} — {{ formatTime(detail.registration_end_at) }}</dd></div>
            <div><dt>报名策略</dt><dd>{{ labelOf(registrationPolicies, detail.registration_policy) }}</dd></div>
            <div><dt>榜单可见性</dt><dd>{{ labelOf(visibilities, detail.scoreboard_visibility) }}</dd></div>
            <div><dt>封榜状态</dt><dd>{{ detail.scoreboard_frozen ? '已封榜' : '未封榜' }}</dd></div>
            <div><dt>报名人数</dt><dd class="mono">{{ detail.participant_count }}</dd></div>
          </dl>
        </div>
      </section>
    </aside>

    <ConfirmDialog
      v-if="confirmCancel" title="取消报名" danger
      message="取消后如需再次参赛，须在报名开放期间重新报名。确定要取消吗？"
      confirm-text="确认取消报名" :busy="cancel.busy.value"
      @confirm="submitCancel()" @cancel="confirmCancel = false"
    />
  </div>
</template>

<style scoped>
.reg-status-row { justify-content: space-between; gap: var(--space-2); }
.reg-status-row + .reg-status-row { margin-top: var(--space-2); }
.reg-review-note { margin: var(--space-2) 0 0; overflow-wrap: anywhere; }
.reg-notice { margin: var(--space-3) 0 0; }
.reg-form { display: grid; gap: var(--space-3); margin-top: var(--space-3); }
.reg-form:first-child { margin-top: 0; }
.reg-form .field { margin-bottom: 0; }
.cancel-btn { margin-top: var(--space-3); }
</style>
