<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ApiError } from '../../../api/client'
import type {
  Contest,
  ContestProblem,
  GetContestForEditResp,
  Int64,
  SetContestProblemsResp,
  UpdateContestResp,
} from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { isValidId, parseIdParam } from '../../../lib/ids'
import { contestValues, type StringRecord } from '../../../lib/forms'
import { freezePolicy } from '../../../lib/freeze'
import { formatTime, toLocalInput } from '../../../lib/time'
import { contestModes, contestStatuses, labelOf, registrationPolicies, visibilities } from '../../../lib/labels'
import { contestDiff } from '../lib/masks'
import Badge from '../../../components/ui/Badge.vue'
import ConfirmDialog from '../../../components/ui/ConfirmDialog.vue'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'
import ContestForm from './ContestForm.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const contestIdText = computed(() => String(route.params.contestId ?? ''))
watch(contestIdText, () => { loaded.value = false; editBase.value = undefined; conflict.value = false })
const idValid = computed(() => isValidId(contestIdText.value))
const contestId = computed(() => parseIdParam(route.params.contestId))

const contest = useQuery({
  queryKey: computed(() => ['admin', 'contest', contestIdText.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetContestForEditResp>(`/contests/${contestIdText.value}`, { signal }),
})

const server = computed<Contest | undefined>(() => contest.data.value?.contest)
const revision = computed(() => server.value?.revision)

function statusTone(value: string): 'neutral' | 'success' | 'warning' | 'danger' | 'muted' | 'accent' {
  if (value === 'running') return 'accent'
  if (value === 'finished') return 'success'
  if (value === 'cancelled' || value === 'deleted') return 'danger'
  if (value === 'archived') return 'warning'
  return 'neutral'
}

// ---- 编辑表单 ----
const form = ref<InstanceType<typeof ContestForm> | null>(null)
const loaded = ref(false)
const editBase = shallowRef<Contest>()
const conflict = ref(false)

function toFormValues(source: Contest): StringRecord {
  const editable = source.editable
  const freezeAt = editable.scoreboard_freeze_at
  const freezeSeconds = editable.scoreboard_freeze_before_end_seconds
  return {
    title: editable.title,
    description: editable.description,
    mode: editable.mode,
    start_at: toLocalInput(editable.start_at),
    end_at: toLocalInput(editable.end_at),
    registration_start_at: toLocalInput(editable.registration_start_at),
    registration_end_at: toLocalInput(editable.registration_end_at),
    registration_policy: editable.registration_policy,
    visibility: editable.visibility,
    scoreboard_visibility: editable.scoreboard_visibility,
    invite_code: editable.invite_code ?? '',
    freeze_policy: freezeAt ? 'at' : freezeSeconds != null ? 'before_end' : 'none',
    freeze_at: toLocalInput(freezeAt),
    freeze_seconds: freezeSeconds != null ? String(freezeSeconds) : '',
  }
}

watch([() => contest.data.value, form], ([data, formRef]) => {
  if (data && formRef && !loaded.value) {
    formRef.load(toFormValues(data.contest))
    editBase.value = data.contest
    loaded.value = true
  }
}, { immediate: true })

function rebaseEdits() {
  const current = form.value?.snapshot()
  if (!current || !server.value || !editBase.value) return
  const previous = toFormValues(editBase.value)
  const latest = toFormValues(server.value)
  for (const [key,value] of Object.entries(current)) if (value !== previous[key]) latest[key] = value
  form.value!.load(latest)
  editBase.value = server.value
  conflict.value = false
}

const save = useAction()

async function saveChanges() {
  if (!editBase.value) return
  await save.run(async () => {
    const snapshot = form.value?.snapshot()
    if (!snapshot) throw new Error('表单尚未就绪')
    const next = contestValues(snapshot)
    const { patch, updateMask } = contestDiff(next, editBase.value!)
    const freeze = freezePolicy(snapshot, editBase.value!.editable)
    const update_mask = [...new Set([...updateMask, ...freeze.updateMask])]
    if (!update_mask.length) return '没有需要保存的改动。'
    try {
      const response = await api.request<UpdateContestResp>(`/contests/${contestIdText.value}`, {
        method: 'PATCH',
        body: {
          contest_id: contestId.value,
          contest: { ...patch, ...freeze.fields },
          update_mask,
          expected_revision: editBase.value!.revision,
        },
      })
      editBase.value = response.contest
      conflict.value = false
      await contest.refetch()
      return '已保存。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        conflict.value = true
        await contest.refetch()
      }
      throw failure
    }
  })
}

// ---- 生命周期 ----
const lifecycle = useAction()
type Lifecycle = 'publish' | 'cancel' | 'archive' | 'restore' | 'delete'
const confirm = ref<Lifecycle | null>(null)
const deleteText = ref('')
const cancelReason = ref('')

const lifecycleMeta = {
  publish: { title: '发布比赛', message: '发布后比赛将按时间对用户开放，确认继续？', confirmText: '发布' },
  cancel: { title: '取消比赛', message: '取消后比赛不可再参与，原因会记录在案。', confirmText: '取消比赛' },
  archive: { title: '归档比赛', message: '归档后比赛不再对外展示，确认继续？', confirmText: '归档' },
  restore: { title: '恢复比赛', message: '恢复后比赛回到草稿状态，确认继续？', confirmText: '恢复' },
  delete: { title: '删除比赛', message: '删除为危险操作，可能不可恢复。', confirmText: '删除' },
} as const

const deleteConfirmed = computed(() => deleteText.value.trim() === contestIdText.value)
const cancelConfirmed = computed(() => cancelReason.value.trim().length > 0)

async function runLifecycle(kind: Lifecycle) {
  if (revision.value === undefined) return
  if (kind === 'delete' && !deleteConfirmed.value) return
  if (kind === 'cancel' && !cancelConfirmed.value) return
  await lifecycle.run(async () => {
    try {
      if (kind === 'delete') {
        await api.request(`/contests/${contestIdText.value}`, {
          method: 'DELETE',
          body: { contest_id: contestId.value, expected_revision: revision.value },
        })
        confirm.value = null
        await router.push({ name: 'admin-contests' })
        return '比赛已删除。'
      }
      const body = kind === 'cancel'
        ? { contest_id: contestId.value, expected_revision: revision.value, reason: cancelReason.value.trim() }
        : { contest_id: contestId.value, expected_revision: revision.value }
      await api.request(`/contests/${contestIdText.value}/${kind}`, { method: 'POST', body })
      confirm.value = null
      conflict.value = false
      await contest.refetch()
      return '操作成功。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        conflict.value = true
        await contest.refetch()
      }
      throw failure
    }
  })
}

// ---- 封榜 ----
const freezeAction = useAction()

async function toggleFreeze() {
  if (!server.value || revision.value === undefined) return
  const frozen = server.value.scoreboard_frozen
  await freezeAction.run(async () => {
    try {
      await api.request(`/contests/${contestIdText.value}/scoreboard/${frozen ? 'unfreeze' : 'freeze'}`, {
        method: 'POST',
        body: { contest_id: contestId.value, expected_revision: revision.value },
      })
      await contest.refetch()
      return frozen ? '已解除封榜。' : '已封榜。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        conflict.value = true
        await contest.refetch()
      }
      throw failure
    }
  })
}

// ---- 题目编排 ----
type ArrangeRow = { problem_id: string; label: string; display_order: string; max_score: string }

const arrange = ref<ArrangeRow[]>([])
const arrangeLoaded = ref(false)
const arrangeRevision = shallowRef<Int64>()
const arrangeConflict = ref(false)
watch(contestIdText, () => { arrangeLoaded.value = false; arrangeRevision.value = undefined; arrangeConflict.value = false })

function toRows(problems: ContestProblem[]): ArrangeRow[] {
  return problems.map((item) => ({
    problem_id: String(item.problem_id),
    label: item.label,
    display_order: String(item.display_order),
    max_score: String(item.max_score),
  }))
}

watch([() => contest.data.value?.problems, server], ([problems, data]) => {
  if (data && problems && !arrangeLoaded.value) {
    arrange.value = toRows(problems)
    arrangeRevision.value = data.revision
    arrangeLoaded.value = true
  }
}, { immediate: true })

function addRow() {
  const nextOrder = arrange.value.length + 1
  arrange.value.push({
    problem_id: '',
    label: String.fromCharCode(65 + ((nextOrder - 1) % 26)),
    display_order: String(nextOrder),
    max_score: '100',
  })
}

function removeRow(index: number) {
  arrange.value.splice(index, 1)
}

function int64FromText(value: string, label: string, minimum: bigint): Int64 {
  if (!/^\d{1,19}$/.test(value.trim())) throw new Error(`${label}必须是有效的非负整数`)
  const parsed = BigInt(value.trim())
  if (parsed < minimum || parsed > 9223372036854775807n) throw new Error(`${label}超出允许范围`)
  return parsed <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(parsed) : parsed
}

function arrangePayload(): ContestProblem[] {
  if (arrange.value.length > 200) throw new Error('一场比赛最多编排 200 道题目')
  const ids = new Set<string>()
  const labels = new Set<string>()
  return arrange.value.map((row, index) => {
    const problemId = int64FromText(row.problem_id, `第 ${index + 1} 行的题目编号`, 1n)
    const problemKey = String(problemId)
    if (ids.has(problemKey)) throw new Error(`题目 ${problemKey} 重复编排`)
    ids.add(problemKey)
    const label = row.label.trim()
    if (!label || [...label].length > 16) throw new Error(`第 ${index + 1} 行的题目标签须为 1–16 个字符`)
    if (labels.has(label)) throw new Error(`题目标签 ${label} 重复`)
    labels.add(label)
    const order = Number(row.display_order)
    if (!/^[1-9]\d*$/.test(row.display_order.trim()) || !Number.isSafeInteger(order) || order > 2147483647) {
      throw new Error(`第 ${index + 1} 行的展示顺序必须是正整数`)
    }
    const maxScore = int64FromText(row.max_score, `第 ${index + 1} 行的满分`, 0n)
    return { problem_id: problemId, label, display_order: order, max_score: maxScore }
  })
}

const arrangeAction = useAction()

async function saveArrangement() {
  if (arrangeRevision.value === undefined) return
  await arrangeAction.run(async () => {
    const problems = arrangePayload()
    try {
      const response = await api.request<SetContestProblemsResp>(`/contests/${contestIdText.value}/problems`, {
        method: 'PUT',
        body: { contest_id: contestId.value, problems, expected_revision: arrangeRevision.value },
      })
      arrange.value = toRows(response.problems ?? [])
      arrangeRevision.value = response.contest.revision
      arrangeConflict.value = false
      await contest.refetch()
      return '题目编排已保存。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        arrangeConflict.value = true
        await contest.refetch()
      }
      throw failure
    }
  })
}

function openConfirm(kind: Lifecycle) {
  deleteText.value = ''
  cancelReason.value = ''
  confirm.value = kind
}

function resetLifecycle() {
  lifecycle.reset()
  void contest.refetch()
}
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-contests' }">比赛管理</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>#{{ contestIdText }}</span>
    </nav>
    <div class="page-head">
      <h1>比赛工作台</h1>
      <div v-if="server" class="actions">
        <RouterLink class="btn btn-quiet sm" :to="`/admin/contests/${contestIdText}/registrations`">报名审核</RouterLink>
        <RouterLink class="btn btn-quiet sm" :to="`/admin/contests/${contestIdText}/participants`">参赛名单</RouterLink>
        <RouterLink class="btn btn-quiet sm" :to="`/contests/${contestIdText}/scoreboard`">查看榜单</RouterLink>
      </div>
    </div>

    <p v-if="!idValid" class="notice notice-error" role="alert">比赛编号无效。</p>
    <StateBlock v-else :pending="contest.isPending.value" :error="contest.error.value" @retry="contest.refetch()">
      <template #content>
        <div v-if="server" class="stack">
          <section class="panel">
            <div class="panel-body meta-bar">
              <div class="row">
                <Badge :tone="statusTone(server.status)" :running="server.status === 'running'">{{ labelOf(contestStatuses, server.status) }}</Badge>
                <span class="small muted">赛制：{{ labelOf(contestModes, server.editable.mode) }}</span>
                <span class="small muted">可见性：{{ labelOf(visibilities, server.editable.visibility) }}</span>
                <span class="small muted">报名策略：{{ labelOf(registrationPolicies, server.editable.registration_policy) }}</span>
                <span class="small muted mono">revision {{ server.revision }}</span>
                <span class="small muted">创建者 <span class="mono">{{ server.creator_id }}</span></span>
              </div>
              <div class="row small muted">
                <span>开始 {{ formatTime(server.editable.start_at) }}</span>
                <span>结束 {{ formatTime(server.editable.end_at) }}</span>
                <span v-if="server.editable.registration_start_at">报名开始 {{ formatTime(server.editable.registration_start_at) }}</span>
                <span v-if="server.editable.registration_end_at">报名截止 {{ formatTime(server.editable.registration_end_at) }}</span>
              </div>
              <div class="row lifecycle-row">
                <button v-if="server.status === 'draft'" class="sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('publish')">发布</button>
                <button v-if="server.status === 'published' || server.status === 'running'" class="quiet sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('cancel')">取消比赛</button>
                <button v-if="server.status === 'published' || server.status === 'finished'" class="quiet sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('archive')">归档</button>
                <button v-if="server.status === 'archived' || server.status === 'cancelled' || server.status === 'deleted'" class="quiet sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('restore')">恢复</button>
                <button class="danger-quiet sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('delete')">删除</button>
                <span class="divider" aria-hidden="true" />
                <button class="quiet sm" type="button" :disabled="freezeAction.busy.value" @click="toggleFreeze">
                  {{ freezeAction.busy.value ? '处理中…' : server.scoreboard_frozen ? '解除封榜' : '封榜' }}
                </button>
                <span v-if="server.scoreboard_frozen" class="small muted">榜单冻结于 {{ formatTime(server.scoreboard_frozen_at) }}</span>
                <span v-else-if="server.editable.scoreboard_freeze_at" class="small muted">计划封榜 {{ formatTime(server.editable.scoreboard_freeze_at) }}</span>
                <span v-else-if="server.editable.scoreboard_freeze_before_end_seconds != null" class="small muted">
                  赛前 {{ server.editable.scoreboard_freeze_before_end_seconds }} 秒自动封榜
                </span>
              </div>
            </div>
          </section>

          <p v-if="conflict" class="notice notice-warning" role="alert">
            服务端已更新（最新 revision：<span class="mono">{{ server.revision }}</span>）。你的编辑内容已保留，请核对后采用新版本再保存。
            <button type="button" class="link" @click="rebaseEdits">保留编辑并采用最新版本</button>
          </p>

          <section class="panel">
            <div class="panel-head">
              <h2>编辑比赛</h2>
              <span class="small muted">仅发送改动过的字段</span>
            </div>
            <div class="panel-body">
              <form @submit.prevent="saveChanges">
                <fieldset :disabled="save.busy.value">
                  <ContestForm ref="form" />
                  <div class="row submit-row">
                    <button type="submit" :disabled="save.busy.value">
                      {{ save.busy.value ? '保存中…' : save.uncertain.value ? '结果未知 · 核对后可重试' : '保存改动' }}
                    </button>
                  </div>
                </fieldset>
              </form>
              <p v-if="save.notice.value" class="notice notice-success" role="status">{{ save.notice.value }}</p>
              <ErrorNotice :error="save.error.value" />
              <p v-if="save.uncertain.value" class="notice notice-warning" role="alert">
                保存结果未知：请刷新核对比赛是否已更新，不要盲目重试。
                <button class="link sm" type="button" @click="save.reset(); contest.refetch()">我已核对，重置</button>
              </p>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h2>题目编排</h2>
              <span class="small muted">{{ arrange.length }} 道题 · 保存时原子替换完整编排</span>
            </div>
            <div class="panel-body">
              <div v-if="arrange.length" class="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th scope="col">题目编号</th>
                      <th scope="col">标签</th>
                      <th scope="col" class="num">展示顺序</th>
                      <th scope="col" class="num">满分</th>
                      <th scope="col" class="num">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, index) in arrange" :key="index">
                      <td>
                        <label class="sr-only" :for="`arrange-id-${index}`">第 {{ index + 1 }} 行题目编号</label>
                        <input :id="`arrange-id-${index}`" v-model="row.problem_id" type="text" inputmode="numeric" autocomplete="off">
                      </td>
                      <td>
                        <label class="sr-only" :for="`arrange-label-${index}`">第 {{ index + 1 }} 行标签</label>
                        <input :id="`arrange-label-${index}`" v-model="row.label" type="text" autocomplete="off">
                      </td>
                      <td class="num">
                        <label class="sr-only" :for="`arrange-order-${index}`">第 {{ index + 1 }} 行展示顺序</label>
                        <input :id="`arrange-order-${index}`" v-model="row.display_order" type="text" inputmode="numeric" autocomplete="off" class="narrow">
                      </td>
                      <td class="num">
                        <label class="sr-only" :for="`arrange-score-${index}`">第 {{ index + 1 }} 行满分</label>
                        <input :id="`arrange-score-${index}`" v-model="row.max_score" type="text" inputmode="numeric" autocomplete="off" class="narrow">
                      </td>
                      <td class="num">
                        <button class="danger-quiet sm" type="button" @click="removeRow(index)">移除</button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <p v-else class="small muted">尚未编排题目。</p>
              <p v-if="arrangeConflict" class="notice notice-warning" role="alert">
                服务端题目编排已变更，当前编辑已保留。最新编排：{{ contest.data.value?.problems?.map(item => `${item.label}: #${item.problem_id}`).join('，') || '空' }}。
                <button class="link" type="button" @click="arrangeRevision = server?.revision; arrangeConflict = false">已核对，采用最新版本后保存当前编排</button>
              </p>
              <div class="row arrangement-actions">
                <button class="quiet sm" type="button" :disabled="arrangeAction.busy.value" @click="addRow">添加题目</button>
                <button class="sm" type="button" :disabled="arrangeAction.busy.value" @click="saveArrangement">
                  {{ arrangeAction.busy.value ? '保存中…' : '保存编排' }}
                </button>
              </div>
              <p v-if="arrangeAction.notice.value" class="notice notice-success" role="status">{{ arrangeAction.notice.value }}</p>
              <ErrorNotice :error="arrangeAction.error.value" />
              <p v-if="arrangeAction.uncertain.value" class="notice notice-warning" role="alert">
                保存结果未知：请刷新核对题目编排，不要盲目重试。
                <button class="link sm" type="button" @click="arrangeAction.reset(); contest.refetch()">我已核对，重置</button>
              </p>
            </div>
          </section>

          <p v-if="freezeAction.notice.value" class="notice notice-success" role="status">{{ freezeAction.notice.value }}</p>
          <ErrorNotice :error="freezeAction.error.value" />
          <p v-if="freezeAction.uncertain.value" class="notice notice-warning" role="alert">
            封榜操作结果未知：请刷新核对榜单状态，不要盲目重试。
            <button class="link sm" type="button" @click="freezeAction.reset(); contest.refetch()">我已核对，重置</button>
          </p>

          <p v-if="lifecycle.notice.value" class="notice notice-success" role="status">{{ lifecycle.notice.value }}</p>
          <ErrorNotice :error="lifecycle.error.value" />
          <p v-if="lifecycle.uncertain.value" class="notice notice-warning" role="alert">
            操作结果未知：请刷新核对比赛状态，不要盲目重试。
            <button class="link sm" type="button" @click="resetLifecycle">我已核对，重置</button>
          </p>
        </div>
      </template>
    </StateBlock>

    <ConfirmDialog
      v-if="confirm"
      :title="lifecycleMeta[confirm].title"
      :message="lifecycleMeta[confirm].message"
      :confirm-text="lifecycleMeta[confirm].confirmText"
      :danger="confirm === 'delete' || confirm === 'cancel'"
      :busy="lifecycle.busy.value"
      @cancel="confirm = null"
      @confirm="runLifecycle(confirm)"
    >
      <div v-if="confirm === 'cancel'" class="field">
        <label for="cancel-reason">取消原因（必填）</label>
        <textarea id="cancel-reason" v-model="cancelReason" rows="3" />
        <span v-if="cancelReason && !cancelConfirmed" class="field-error">请填写取消原因。</span>
      </div>
      <div v-if="confirm === 'delete'" class="field">
        <label for="delete-confirm">请输入比赛编号 <span class="mono">{{ contestIdText }}</span> 以确认</label>
        <input id="delete-confirm" v-model="deleteText" type="text" inputmode="numeric" autocomplete="off">
        <span v-if="deleteText && !deleteConfirmed" class="field-error">编号不匹配，无法删除。</span>
      </div>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.meta-bar { display: grid; gap: var(--space-3); }
.lifecycle-row { gap: var(--space-2); flex-wrap: wrap; }
.divider { width: 1px; align-self: stretch; background: var(--border); margin: 0 var(--space-1); }
.submit-row { margin-top: var(--space-4); }
.arrangement-actions { margin-top: var(--space-3); }
.narrow { width: 6.5rem; }
</style>
