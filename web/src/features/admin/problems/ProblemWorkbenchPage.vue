<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ApiError } from '../../../api/client'
import type { GetProblemForEditResp, Problem, UpdateProblemResp } from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { isValidId, parseIdParam } from '../../../lib/ids'
import { problemValues, type StringRecord } from '../../../lib/forms'
import { formatTime } from '../../../lib/time'
import { labelOf, problemStatuses, visibilities } from '../../../lib/labels'
import { problemDiff, samplesEqual } from '../lib/masks'
import Badge from '../../../components/ui/Badge.vue'
import ConfirmDialog from '../../../components/ui/ConfirmDialog.vue'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'
import ProblemForm from './ProblemForm.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()

const problemIdText = computed(() => String(route.params.problemId ?? ''))
watch(problemIdText, () => { loaded.value = false; editBase.value = undefined; conflict.value = false })
const idValid = computed(() => isValidId(problemIdText.value))
const problemId = computed(() => parseIdParam(route.params.problemId))

const problem = useQuery({
  queryKey: computed(() => ['admin', 'problem', problemIdText.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetProblemForEditResp>(`/problems/${problemIdText.value}`, { signal }),
})

const server = computed<Problem | undefined>(() => problem.data.value?.problem)
const testCases = computed(() => problem.data.value?.test_cases ?? [])
const revision = computed(() => server.value?.revision)

const form = ref<InstanceType<typeof ProblemForm> | null>(null)
const loaded = ref(false)
const editBase = shallowRef<Problem>()
const conflict = ref(false)

function toFormValues(source: Problem): StringRecord {
  const settings = source.judge_config.settings
  return {
    title: source.statement.title,
    description: source.statement.description,
    input_description: source.statement.input_description,
    output_description: source.statement.output_description,
    hint: source.statement.hint ?? '',
    difficulty: source.metadata.difficulty,
    visibility: source.visibility,
    checker_type: settings.checker_type,
    tags: (source.metadata.tags ?? []).join(','),
    source: source.metadata.source ?? '',
    time_limit_ms: String(settings.time_limit_ms),
    memory_limit_mb: String(settings.memory_limit_mb),
    allowed_languages: (settings.allowed_languages ?? []).join(','),
  }
}

watch([() => problem.data.value, form], ([data, formRef]) => {
  if (data && formRef && !loaded.value) {
    formRef.load(toFormValues(data.problem), data.problem.statement.samples ?? [])
    editBase.value = data.problem
    loaded.value = true
  }
}, { immediate: true })

function rebaseEdits() {
  const current = form.value?.snapshot()
  if (!current || !server.value || !editBase.value) return
  const previous = toFormValues(editBase.value)
  const latest = toFormValues(server.value)
  for (const [key,value] of Object.entries(current.values)) if (value !== previous[key]) latest[key] = value
  const samples = samplesEqual(current.samples, editBase.value.statement.samples ?? []) ? server.value.statement.samples ?? [] : current.samples
  form.value!.load(latest, samples)
  editBase.value = server.value
  conflict.value = false
}

function statusTone(value: string): 'neutral' | 'success' | 'warning' | 'danger' | 'muted' {
  if (value === 'published') return 'success'
  if (value === 'archived') return 'warning'
  if (value === 'deleted') return 'danger'
  return 'neutral'
}

// ---- 保存（PATCH） ----
const save = useAction()

async function saveChanges() {
  if (!editBase.value) return
  await save.run(async () => {
    const snapshot = form.value?.snapshot()
    if (!snapshot) throw new Error('表单尚未就绪')
    const next = problemValues(snapshot.values, snapshot.samples)
    const { patch, updateMask } = problemDiff(next, editBase.value!)
    if (!updateMask.length) return '没有需要保存的改动。'
    try {
      const response = await api.request<UpdateProblemResp>(`/problems/${problemIdText.value}`, {
        method: 'PATCH',
        body: { problem_id: problemId.value, problem: patch, update_mask: updateMask, expected_revision: editBase.value!.revision },
      })
      editBase.value = response.problem
      conflict.value = false
      await problem.refetch()
      return '已保存。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        conflict.value = true
        await problem.refetch()
      }
      throw failure
    }
  })
}

// ---- 生命周期 ----
const lifecycle = useAction()
const confirm = ref<null | 'publish' | 'archive' | 'restore' | 'delete'>(null)
const deleteText = ref('')

const lifecycleMeta = {
  publish: { title: '发布题目', message: '发布后题目对用户可见，确认继续？', confirmText: '发布' },
  archive: { title: '归档题目', message: '归档后题目不再对外展示，确认继续？', confirmText: '归档' },
  restore: { title: '恢复题目', message: '恢复后题目回到草稿状态，确认继续？', confirmText: '恢复' },
  delete: { title: '删除题目', message: '删除为危险操作，可能不可恢复。', confirmText: '删除' },
} as const

const deleteConfirmed = computed(() => deleteText.value.trim() === problemIdText.value)

async function runLifecycle(kind: 'publish' | 'archive' | 'restore' | 'delete') {
  if (revision.value === undefined) return
  if (kind === 'delete' && !deleteConfirmed.value) return
  await lifecycle.run(async () => {
    try {
      if (kind === 'delete') {
        await api.request(`/problems/${problemIdText.value}`, {
          method: 'DELETE',
          body: { problem_id: problemId.value, expected_revision: revision.value },
        })
        confirm.value = null
        await router.push({ name: 'admin-problems' })
        return '题目已删除。'
      }
      await api.request(`/problems/${problemIdText.value}/${kind}`, {
        method: 'POST',
        body: { problem_id: problemId.value, expected_revision: revision.value },
      })
      confirm.value = null
      conflict.value = false
      await problem.refetch()
      return '操作成功。'
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        conflict.value = true
        await problem.refetch()
      }
      throw failure
    }
  })
}

function openConfirm(kind: 'publish' | 'archive' | 'restore' | 'delete') {
  deleteText.value = ''
  confirm.value = kind
}

function resetLifecycle() {
  lifecycle.reset()
  void problem.refetch()
}
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-problems' }">题目管理</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>#{{ problemIdText }}</span>
    </nav>
    <div class="page-head">
      <h1>题目工作台</h1>
      <div v-if="server" class="actions">
        <RouterLink class="btn btn-quiet sm" :to="`/admin/problems/${problemIdText}/test-data`">测试数据</RouterLink>
      </div>
    </div>

    <p v-if="!idValid" class="notice notice-error" role="alert">题目编号无效。</p>
    <StateBlock v-else :pending="problem.isPending.value" :error="problem.error.value" @retry="problem.refetch()">
      <template #content>
        <div v-if="server" class="stack">
          <section class="panel">
            <div class="panel-body meta-bar">
              <div class="row">
                <Badge :tone="statusTone(server.status)">{{ labelOf(problemStatuses, server.status) }}</Badge>
                <span class="small muted">可见性：{{ labelOf(visibilities, server.visibility) }}</span>
                <span class="small muted mono">revision {{ server.revision }}</span>
                <span class="small muted mono">test_data {{ server.judge_config.test_data_version || '—' }}</span>
                <span class="small muted">更新 {{ formatTime(server.updated_at) }}</span>
              </div>
              <div class="row lifecycle-row">
                <button v-if="server.status === 'draft'" class="sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('publish')">发布</button>
                <button v-if="server.status === 'published'" class="quiet sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('archive')">归档</button>
                <button v-if="server.status === 'archived' || server.status === 'deleted'" class="quiet sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('restore')">恢复</button>
                <button class="danger-quiet sm" type="button" :disabled="lifecycle.busy.value" @click="openConfirm('delete')">删除</button>
              </div>
            </div>
          </section>

          <p v-if="conflict" class="notice notice-warning" role="alert">
            服务端已更新（最新 revision：<span class="mono">{{ server.revision }}</span>）。你的编辑内容已保留，请核对后采用新版本再保存。
            <button type="button" class="link" @click="rebaseEdits">保留编辑并采用最新版本</button>
          </p>

          <section class="panel">
            <div class="panel-head">
              <h2>编辑题目</h2>
              <span class="small muted">仅发送改动过的字段</span>
            </div>
            <div class="panel-body">
              <form @submit.prevent="saveChanges">
                <fieldset :disabled="save.busy.value">
                  <ProblemForm ref="form" />
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
                保存结果未知：请刷新核对题目是否已更新，不要盲目重试。
                <button class="link sm" type="button" @click="save.reset(); problem.refetch()">我已核对，重置</button>
              </p>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h2>测试点</h2>
              <span class="small muted">{{ testCases.length }} 个 · 版本 {{ server.judge_config.test_data_version || '—' }}</span>
            </div>
            <div v-if="testCases.length" class="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th scope="col" class="num">序号</th>
                    <th scope="col">名称</th>
                    <th scope="col">分组</th>
                    <th scope="col" class="num">分值</th>
                    <th scope="col">启用</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in testCases" :key="String(item.test_case_id)">
                    <td class="num">{{ item.ordinal }}</td>
                    <td class="mono small">{{ item.name }}</td>
                    <td>{{ item.group_name || '—' }}</td>
                    <td class="num">{{ item.score }}</td>
                    <td>{{ item.enabled ? '是' : '否' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="panel-body">
              <p class="small muted">尚无测试数据。<RouterLink :to="`/admin/problems/${problemIdText}/test-data`">前往上传 →</RouterLink></p>
            </div>
          </section>

          <p v-if="lifecycle.notice.value" class="notice notice-success" role="status">{{ lifecycle.notice.value }}</p>
          <ErrorNotice :error="lifecycle.error.value" />
          <p v-if="lifecycle.uncertain.value" class="notice notice-warning" role="alert">
            操作结果未知：请刷新核对题目状态，不要盲目重试。
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
      :danger="confirm === 'delete'"
      :busy="lifecycle.busy.value"
      @cancel="confirm = null"
      @confirm="runLifecycle(confirm)"
    >
      <div v-if="confirm === 'delete'" class="field">
        <label for="delete-confirm">请输入题目编号 <span class="mono">{{ problemIdText }}</span> 以确认</label>
        <input id="delete-confirm" v-model="deleteText" type="text" inputmode="numeric" autocomplete="off">
        <span v-if="deleteText && !deleteConfirmed" class="field-error">编号不匹配，无法删除。</span>
      </div>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.meta-bar { display: grid; gap: var(--space-3); }
.lifecycle-row { gap: var(--space-2); }
.submit-row { margin-top: var(--space-4); }
</style>
