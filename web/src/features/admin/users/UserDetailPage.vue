<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { AdminUpdateUserResp, GetUserResp } from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { useSession } from '../../../composables/useSession'
import { isValidId, parseIdParam } from '../../../lib/ids'
import { labelOf, userRoles, userStatuses } from '../../../lib/labels'
import { canManageUser } from '../lib/permissions'
import Badge from '../../../components/ui/Badge.vue'
import ConfirmDialog from '../../../components/ui/ConfirmDialog.vue'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()
const session = useSession()

const userIdText = computed(() => String(route.params.userId ?? ''))
const idValid = computed(() => isValidId(userIdText.value))
const userId = computed(() => parseIdParam(route.params.userId))

const user = useQuery({
  queryKey: computed(() => ['admin', 'user', userIdText.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetUserResp>(`/users/${userIdText.value}`, { signal }),
})

const target = computed(() => user.data.value?.user)
const actor = computed(() => session.value?.user)
const canManage = computed(() => canManageUser(actor.value, target.value))

const canEditAccount = computed(() => canManage.value && actor.value?.role === 'super_admin' && target.value?.role === 'user')
const username = ref('')
const email = ref('')
const school = ref('0')
const newPassword = ref('')
const account = useAction()
function resetAccount() {
  if (!target.value) return
  username.value = target.value.username
  email.value = target.value.email
  school.value = String(target.value.school_id)
  newPassword.value = ''
}
watch(target, (value, previous) => {
  if (value && (!previous || String(value.user_id) !== String(previous.user_id))) {
    resetAccount()
    account.reset()
  }
}, { immediate: true })
const accountChanged = computed(() => !!target.value && (
  username.value !== target.value.username || email.value !== target.value.email ||
  school.value !== String(target.value.school_id) || newPassword.value !== ''
))
async function runAccountUpdate() {
  if (!canEditAccount.value || !accountChanged.value || !target.value) return
  await account.run(async () => {
    const body: Record<string, unknown> = { user_id: userId.value }
    if (username.value !== target.value!.username) {
      const value = username.value.trim()
      if (!value || Array.from(value).length > 32) throw new Error('用户名必须包含 1 到 32 个字符')
      body.username = value
    }
    if (email.value !== target.value!.email) {
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim())) throw new Error('请输入有效的邮箱地址')
      body.email = email.value.trim()
    }
    if (school.value !== String(target.value!.school_id)) {
      if (school.value !== '0' && !isValidId(school.value)) throw new Error('学校编号必须为有效的非负整数')
      body.school_id = school.value === '0' ? 0 : parseIdParam(school.value)
    }
    if (newPassword.value) {
      if (Array.from(newPassword.value).length < 6 || new TextEncoder().encode(newPassword.value).length > 72) throw new Error('新密码至少 6 个字符且不能超过 72 字节')
      body.new_password = newPassword.value
    }
    try {
      await api.request<AdminUpdateUserResp>(`/users/${userIdText.value}`, { method: 'PATCH', body })
    } finally { newPassword.value = '' }
    confirm.value = null
    await user.refetch()
    resetAccount()
    return '账户已更新，目标用户的原有登录会话已撤销。'
  })
}

const editableRoles = ['user', 'admin', 'super_admin']
const editableStatuses = ['normal', 'disabled', 'banned']

const role = ref('')
const status = ref('')
watch(target, (value) => {
  if (value) {
    role.value = value.role
    status.value = value.status
  }
}, { immediate: true })

const changed = computed(() => !!target.value && (role.value !== target.value.role || status.value !== target.value.status))

const update = useAction()
const remove = useAction()
const confirm = ref<null | 'update' | 'delete' | 'account'>(null)
const deleteText = ref('')

const deleteConfirmed = computed(() => !!target.value && deleteText.value.trim() === target.value.username)

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

async function runUpdate() {
  if (!target.value || !changed.value) return
  await update.run(async () => {
    const body: Record<string, unknown> = { user_id: userId.value }
    if (role.value !== target.value!.role) body.role = role.value
    if (status.value !== target.value!.status) body.status = status.value
    await api.request<AdminUpdateUserResp>(`/users/${userIdText.value}`, { method: 'PATCH', body })
    confirm.value = null
    await user.refetch()
    return '用户已更新。'
  })
}

async function runDelete() {
  if (!deleteConfirmed.value) return
  await remove.run(async () => {
    await api.request(`/users/${userIdText.value}`, { method: 'DELETE', body: { user_id: userId.value } })
    confirm.value = null
    await router.push({ name: 'admin-users' })
    return '用户已删除。'
  })
}

function openConfirm(kind: 'update' | 'delete') {
  deleteText.value = ''
  confirm.value = kind
}
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-users' }">用户管理</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>#{{ userIdText }}</span>
    </nav>
    <div class="page-head">
      <h1>用户详情</h1>
    </div>

    <p v-if="!idValid" class="notice notice-error" role="alert">用户编号无效。</p>
    <StateBlock v-else :pending="user.isPending.value" :error="user.error.value" @retry="user.refetch()">
      <template #content>
        <div v-if="target" class="stack user-grid">
          <section class="panel">
            <div class="panel-head"><h2>账号信息</h2></div>
            <div class="panel-body">
              <dl class="kv-list">
                <div><dt>用户编号</dt><dd class="mono">{{ target.user_id }}</dd></div>
                <div><dt>用户名</dt><dd>{{ target.username }}</dd></div>
                <div><dt>邮箱</dt><dd class="wrap-anywhere">{{ target.email }}</dd></div>
                <div><dt>角色</dt><dd><Badge :tone="roleTone(target.role)">{{ labelOf(userRoles, target.role) }}</Badge></dd></div>
                <div><dt>状态</dt><dd><Badge :tone="statusTone(target.status)">{{ labelOf(userStatuses, target.status) }}</Badge></dd></div>
                <div><dt>学校编号</dt><dd class="mono">{{ target.school_id ? String(target.school_id) : '未设置' }}</dd></div>
              </dl>
            </div>
          </section>

          <div class="stack">
            <section v-if="canEditAccount" class="panel">
              <div class="panel-head"><h2>账户资料与密码</h2></div>
              <div class="panel-body">
                <p class="small muted">仅超级管理员可修改普通用户。保存后该用户需要重新登录。密码留空表示不修改，学校编号 0 表示清空。</p>
                <form @submit.prevent="confirm = 'account'">
                  <fieldset :disabled="account.busy.value || account.uncertain.value">
                    <div class="field"><label for="account-username">用户名</label><input id="account-username" v-model="username" required maxlength="32" autocomplete="off"></div>
                    <div class="field"><label for="account-email">邮箱</label><input id="account-email" v-model="email" type="email" required maxlength="254" autocomplete="off"></div>
                    <div class="field"><label for="account-school">学校编号</label><input id="account-school" v-model="school" inputmode="numeric" required pattern="0|[1-9][0-9]*"></div>
                    <div class="field"><label for="account-password">新密码</label><input id="account-password" v-model="newPassword" type="password" autocomplete="new-password" aria-describedby="password-hint"><span id="password-hint" class="small muted">至少 6 个字符，最多 72 字节；留空保留原密码。</span></div>
                    <button type="submit" :disabled="!accountChanged">{{ account.busy.value ? '保存中…' : '保存账户资料' }}</button>
                  </fieldset>
                </form>
                <p v-if="account.notice.value" class="notice notice-success" role="status">{{ account.notice.value }}</p>
                <ErrorNotice :error="account.error.value" />
                <p v-if="account.uncertain.value" class="notice notice-warning" role="alert">账户修改结果未知，请先核对资料；密码请通过目标账号登录核对，不要盲目重复重置。<button type="button" class="link sm" @click="user.refetch(); account.reset()">已核对结果</button></p>
              </div>
            </section>
            <section class="panel">
              <div class="panel-head"><h2>调整角色 / 状态</h2></div>
              <div class="panel-body">
                <p v-if="!canManage" class="notice" role="note">
                  你没有管理该用户的权限（需角色等级更高、目标未删除且非本人）。
                </p>
                <form v-else @submit.prevent="openConfirm('update')">
                  <fieldset :disabled="update.busy.value">
                    <div class="field">
                      <label for="u-role">角色</label>
                      <select id="u-role" v-model="role">
                        <option v-for="value in editableRoles" :key="value" :value="value">{{ labelOf(userRoles, value) }}</option>
                      </select>
                    </div>
                    <div class="field">
                      <label for="u-status">状态</label>
                      <select id="u-status" v-model="status">
                        <option v-for="value in editableStatuses" :key="value" :value="value">{{ labelOf(userStatuses, value) }}</option>
                      </select>
                    </div>
                    <button type="submit" :disabled="update.busy.value || !changed">
                      {{ update.busy.value ? '保存中…' : update.uncertain.value ? '结果未知 · 核对后可重试' : '保存修改' }}
                    </button>
                  </fieldset>
                </form>
                <p v-if="update.notice.value" class="notice notice-success" role="status">{{ update.notice.value }}</p>
                <ErrorNotice :error="update.error.value" />
                <p v-if="update.uncertain.value" class="notice notice-warning" role="alert">
                  修改结果未知：请刷新核对用户角色/状态，不要盲目重试。
                  <button class="link sm" type="button" @click="update.reset(); user.refetch()">我已核对，重置</button>
                </p>
              </div>
            </section>

            <section v-if="canManage" class="panel">
              <div class="panel-head"><h2 class="danger-text">危险操作</h2></div>
              <div class="panel-body">
                <p class="small muted">删除用户为不可逆操作，请谨慎。</p>
                <button class="danger sm" type="button" :disabled="remove.busy.value" @click="openConfirm('delete')">删除用户</button>
                <ErrorNotice :error="remove.error.value" />
                <p v-if="remove.uncertain.value" class="notice notice-warning" role="alert">
                  删除结果未知：请刷新用户列表核对，不要盲目重试。
                  <button class="link sm" type="button" @click="remove.reset()">我已核对，重置</button>
                </p>
              </div>
            </section>
          </div>
        </div>
      </template>
    </StateBlock>

    <ConfirmDialog
      v-if="confirm === 'account' && canEditAccount"
      title="修改账户资料"
      :message="newPassword ? '确认修改该用户账户并重置密码？该用户的所有登录会话将失效。' : '确认修改该用户账户资料？该用户的所有登录会话将失效。'"
      confirm-text="确认保存"
      :busy="account.busy.value"
      @cancel="confirm = null"
      @confirm="runAccountUpdate"
    />
    <ConfirmDialog
      v-if="confirm === 'update'"
      title="修改用户权限"
      message="调整角色或状态会立即影响该用户的权限，确认继续？"
      confirm-text="保存修改"
      :busy="update.busy.value"
      @cancel="confirm = null"
      @confirm="runUpdate"
    >
      <dl class="kv-list">
        <div><dt>角色</dt><dd>{{ labelOf(userRoles, target?.role) }} → {{ labelOf(userRoles, role) }}</dd></div>
        <div><dt>状态</dt><dd>{{ labelOf(userStatuses, target?.status) }} → {{ labelOf(userStatuses, status) }}</dd></div>
      </dl>
    </ConfirmDialog>

    <ConfirmDialog
      v-if="confirm === 'delete'"
      title="删除用户"
      message="删除为危险操作，可能不可恢复。"
      confirm-text="删除"
      danger
      :busy="remove.busy.value"
      @cancel="confirm = null"
      @confirm="runDelete"
    >
      <div class="field">
        <label for="delete-confirm">请输入用户名 <span class="mono">{{ target?.username }}</span> 以确认</label>
        <input id="delete-confirm" v-model="deleteText" type="text" autocomplete="off">
        <span v-if="deleteText && !deleteConfirmed" class="field-error">用户名不匹配，无法删除。</span>
      </div>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.user-grid { grid-template-columns: minmax(0, 1fr); }
@media (min-width: 900px) { .user-grid { grid-template-columns: 320px minmax(0, 1fr); align-items: start; } }
</style>
