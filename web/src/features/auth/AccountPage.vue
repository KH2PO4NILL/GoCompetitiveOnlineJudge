<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useRouter } from 'vue-router'
import { z } from 'zod'
import type { GetCurrentUserResp, SendEmailCodeResp, UpdateProfileResp, ChangeEmailResp } from '../../api/contracts/app'
import { useAction } from '../../composables/useAction'
import { useApi } from '../../composables/useApi'
import { useCountdown } from '../../composables/useCountdown'
import { useSession } from '../../composables/useSession'
import { userRoles, userStatuses, labelOf } from '../../lib/labels'
import ErrorNotice from '../../components/ui/ErrorNotice.vue'
import StateBlock from '../../components/ui/StateBlock.vue'
import TextField from '../../components/ui/TextField.vue'

const api = useApi()
const router = useRouter()
const session = useSession()

const me = useQuery({
  queryKey: ['account', 'me'],
  queryFn: ({ signal }) => api.request<GetCurrentUserResp>('/users/me', { signal }),
})
const user = computed(() => me.data.value?.user ?? session.value?.user)

// ---- 资料 ----
const profile = useAction()
const username = ref('')
const schoolId = ref('')

async function updateProfile() {
  await profile.run(async () => {
    const body: Record<string, unknown> = {}
    if (username.value.trim()) body.username = username.value.trim()
    if (schoolId.value.trim()) {
      if (!/^\d{1,19}$/.test(schoolId.value.trim())) throw new Error('学校编号须为非负整数')
      body.school_id = BigInt(schoolId.value.trim())
    }
    if (!Object.keys(body).length) throw new Error('请至少修改一项资料')
    const resp = await api.request<UpdateProfileResp>('/users/me', { method: 'PATCH', body })
    api.updateUser(resp.user)
    username.value = ''
    schoolId.value = ''
    await me.refetch()
    return '资料已更新。'
  })
}

// ---- 修改邮箱 ----
const emailAction = useAction()
const newEmail = ref('')
const emailCode = ref('')
const sendingCode = ref(false)
const codeSendError = ref<unknown>()
const { remaining, start } = useCountdown()

async function sendEmailCode() {
  sendingCode.value = true
  codeSendError.value = undefined
  try {
    const parsed = z.email().safeParse(newEmail.value.trim())
    if (!parsed.success) throw new Error('请输入有效的新邮箱地址')
    const resp = await api.request<SendEmailCodeResp>('/auth/email-codes', {
      method: 'POST',
      body: { email: parsed.data, purpose: 'change_email' },
    })
    start(resp.retry_after)
    emailAction.notice.value = `验证码已发送至新邮箱，${resp.expires_in} 秒内有效。`
  } catch (failure) {
    codeSendError.value = failure
  } finally {
    sendingCode.value = false
  }
}

async function changeEmail() {
  await emailAction.run(async () => {
    const parsed = z.email().parse(newEmail.value.trim())
    const resp = await api.request<ChangeEmailResp>('/users/me/email', {
      method: 'PUT',
      body: { new_email: parsed, code: emailCode.value },
    })
    api.updateUser(resp.user)
    newEmail.value = ''
    emailCode.value = ''
    await me.refetch()
    return '邮箱已更新。'
  })
}

// ---- 修改密码 ----
const passwordAction = useAction()
const currentPassword = ref('')
const newPassword = ref('')

async function changePassword() {
  await passwordAction.run(async () => {
    await api.request('/users/me/password', {
      method: 'PUT',
      body: { current_password: currentPassword.value, new_password: newPassword.value },
    })
    currentPassword.value = ''
    newPassword.value = ''
    return '密码已修改，当前会话继续有效；其他设备需重新登录。'
  })
}

async function logout() {
  await api.logout()
  await router.push('/')
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1>个人信息</h1>
      <button class="quiet sm" type="button" @click="logout">退出登录</button>
    </div>

    <div v-if="user" class="stack account-grid">
      <section class="panel">
        <div class="panel-head"><h2>账号</h2></div>
        <div class="panel-body">
          <dl class="kv-list">
            <div><dt>用户编号</dt><dd class="mono">{{ user.user_id }}</dd></div>
            <div><dt>用户名</dt><dd>{{ user.username }}</dd></div>
            <div><dt>邮箱</dt><dd class="wrap-anywhere">{{ user.email }}</dd></div>
            <div><dt>角色</dt><dd>{{ labelOf(userRoles, user.role) }}</dd></div>
            <div><dt>状态</dt><dd>{{ labelOf(userStatuses, user.status) }}</dd></div>
            <div><dt>学校编号</dt><dd class="mono">{{ user.school_id ? String(user.school_id) : '未设置' }}</dd></div>
          </dl>
        </div>
      </section>

      <div class="stack">
        <section class="panel">
          <div class="panel-head"><h2>修改资料</h2></div>
          <div class="panel-body">
            <form @submit.prevent="updateProfile">
              <fieldset :disabled="profile.busy.value">
                <TextField v-model="username" label="新用户名" name="username" :placeholder="user.username" auto-complete="username" />
                <TextField v-model="schoolId" label="学校编号" name="school_id" placeholder="选填，非负整数" />
                <button type="submit" :disabled="profile.busy.value">{{ profile.busy.value ? '保存中…' : '保存资料' }}</button>
              </fieldset>
            </form>
            <p v-if="profile.notice.value" class="notice notice-success" role="status">{{ profile.notice.value }}</p>
            <ErrorNotice :error="profile.error.value" />
            <p v-if="profile.uncertain.value" class="notice notice-warning" role="alert">保存结果未知，请刷新页面核对资料是否已更新，不要盲目重试。</p>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head"><h2>修改邮箱</h2></div>
          <div class="panel-body">
            <form @submit.prevent="changeEmail">
              <fieldset :disabled="emailAction.busy.value || sendingCode">
                <TextField v-model="newEmail" label="新邮箱" name="new_email" type="email" auto-complete="email" :read-only="sendingCode" />
                <div class="code-row">
                  <TextField v-model="emailCode" label="验证码" name="email_code" auto-complete="one-time-code" hint="验证码将发送到新邮箱。" />
                  <button class="quiet code-send" type="button" :disabled="sendingCode || remaining > 0" @click="sendEmailCode">
                    {{ sendingCode ? '发送中…' : remaining > 0 ? `${remaining}s 后重发` : '发送验证码' }}
                  </button>
                </div>
                <button type="submit" :disabled="emailAction.busy.value">{{ emailAction.busy.value ? '提交中…' : '确认修改邮箱' }}</button>
              </fieldset>
            </form>
            <p v-if="emailAction.notice.value" class="notice notice-success" role="status">{{ emailAction.notice.value }}</p>
            <ErrorNotice :error="emailAction.error.value" />
            <ErrorNotice :error="codeSendError" />
            <p v-if="emailAction.uncertain.value" class="notice notice-warning" role="alert">修改结果未知，请刷新页面核对当前邮箱，不要盲目重试。</p>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head"><h2>修改密码</h2></div>
          <div class="panel-body">
            <form @submit.prevent="changePassword">
              <fieldset :disabled="passwordAction.busy.value">
                <TextField v-model="currentPassword" label="当前密码" name="current_password" type="password" auto-complete="current-password" />
                <TextField v-model="newPassword" label="新密码" name="new_password" type="password" auto-complete="new-password" />
                <button type="submit" :disabled="passwordAction.busy.value">{{ passwordAction.busy.value ? '提交中…' : '修改密码' }}</button>
              </fieldset>
            </form>
            <p v-if="passwordAction.notice.value" class="notice notice-success" role="status">{{ passwordAction.notice.value }}</p>
            <ErrorNotice :error="passwordAction.error.value" />
            <p v-if="passwordAction.uncertain.value" class="notice notice-warning" role="alert">修改结果未知：旧密码若仍可登录说明未生效，确认后再重试。</p>
          </div>
        </section>
      </div>
    </div>
    <StateBlock v-else :pending="me.isPending.value" :error="me.error.value" @retry="me.refetch()" />
  </div>
</template>

<style scoped>
.account-grid { grid-template-columns: minmax(0, 1fr); }
@media (min-width: 900px) { .account-grid { grid-template-columns: 320px minmax(0, 1fr); align-items: start; } }
.code-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: var(--space-2); align-items: start; }
.code-send { margin-top: 24px; height: 34px; }
</style>
