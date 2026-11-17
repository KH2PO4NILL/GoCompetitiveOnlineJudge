<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { z } from 'zod'
import type { SendEmailCodeResp } from '../../api/contracts/app'
import { useAction } from '../../composables/useAction'
import { useApi } from '../../composables/useApi'
import { useCountdown } from '../../composables/useCountdown'
import ErrorNotice from '../../components/ui/ErrorNotice.vue'
import TextField from '../../components/ui/TextField.vue'

const api = useApi()
const route = useRoute()
const router = useRouter()
const admin = api.scope === 'admin'

type Mode = 'username' | 'email-password' | 'email-code' | 'register' | 'reset'
const mode = ref<Mode>('username')
const username = ref('')
const email = ref('')
const password = ref('')
const code = ref('')

const { busy, error, notice, run } = useAction()
const { remaining, start } = useCountdown()
const codeError = ref<unknown>()
const sending = ref(false)

const needsCode = computed(() => mode.value === 'email-code' || mode.value === 'register' || mode.value === 'reset')
const codePurpose = computed(() => (mode.value === 'register' ? 'register' : mode.value === 'reset' ? 'reset_password' : 'login'))
const submitLabel = computed(() => (mode.value === 'register' ? '创建账号' : mode.value === 'reset' ? '重置密码' : '登录'))
const codeHint = computed(() =>
  mode.value === 'register' ? '仅向未注册邮箱发送。' : mode.value === 'reset' ? '仅向已注册邮箱发送。' : '仅向状态正常的已注册账号发送。')

const titles: Record<Mode, string> = {
  'username': '登录',
  'email-password': '登录',
  'email-code': '验证码登录',
  'register': '注册账号',
  'reset': '找回密码',
}

function switchMode(next: Mode) {
  mode.value = next
  codeError.value = undefined
}

// 登录后回跳：仅允许同 scope 的站内路径，防开放重定向
function returnPath(): string {
  const fallback = admin ? '/admin' : '/'
  const value = route.query.returnTo
  const path = typeof value === 'string' ? value : ''
  if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\')) return fallback
  if (admin ? !path.startsWith('/admin') : path.startsWith('/admin')) return fallback
  return path
}

async function sendCode() {
  sending.value = true
  codeError.value = undefined
  try {
    const parsed = z.email().safeParse(email.value.trim())
    if (!parsed.success) throw new Error('请输入有效的邮箱地址')
    const resp = await api.request<SendEmailCodeResp>('/auth/email-codes', {
      method: 'POST',
      body: { email: parsed.data, purpose: codePurpose.value },
      anonymous: true,
    })
    email.value = parsed.data
    start(resp.retry_after)
    notice.value = `验证码已发送，${resp.expires_in} 秒内有效。`
  } catch (failure) {
    codeError.value = failure
  } finally {
    sending.value = false
  }
}

async function submit() {
  await run(async () => {
    const current = mode.value
    if (current === 'register' || current === 'reset') {
      const parsedEmail = z.email().parse(email.value.trim())
      if (current === 'register') {
        if (!username.value.trim()) throw new Error('请填写用户名')
        await api.request('/auth/register', {
          method: 'POST',
          body: { username: username.value.trim(), email: parsedEmail, password: password.value, code: code.value },
          anonymous: true,
        })
        mode.value = 'username'
        code.value = ''
        return '注册成功，请使用新账号登录。'
      }
      if (!password.value) throw new Error('请填写新密码')
      await api.request('/auth/password/reset', {
        method: 'POST',
        body: { email: parsedEmail, code: code.value, new_password: password.value },
        anonymous: true,
      })
      mode.value = 'username'
      code.value = ''
      password.value = ''
      return '密码已重置，请使用新密码登录。'
    }
    let path: string
    let body: Record<string, string>
    if (current === 'username') {
      path = '/auth/login/username'
      body = { username: username.value, password: password.value }
    } else if (current === 'email-password') {
      path = '/auth/login/email-password'
      body = { email: z.email().parse(email.value.trim()), password: password.value }
    } else {
      path = '/auth/login/email-code'
      body = { email: z.email().parse(email.value.trim()), code: code.value }
    }
    await api.login(path, body)
    await router.replace(returnPath())
  })
}
</script>

<template>
  <div class="auth-page">
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="admin ? '/admin/login' : '/'">{{ admin ? '管理端' : '首页' }}</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>{{ admin ? '管理员登录' : '登录' }}</span>
    </nav>
    <div class="panel auth-card">
      <div class="panel-body">
        <h1>{{ admin ? '管理员登录' : titles[mode] }}</h1>
        <div class="mode-tabs" role="tablist" aria-label="认证方式">
          <button role="tab" type="button" :class="{ link: true, active: mode === 'username' }" :aria-selected="mode === 'username'" @click="switchMode('username')">密码登录</button>
          <button role="tab" type="button" :class="{ link: true, active: mode === 'email-code' }" :aria-selected="mode === 'email-code'" @click="switchMode('email-code')">验证码登录</button>
          <button v-if="!admin" role="tab" type="button" :class="{ link: true, active: mode === 'register' }" :aria-selected="mode === 'register'" @click="switchMode('register')">注册</button>
          <button role="tab" type="button" :class="{ link: true, active: mode === 'reset' }" :aria-selected="mode === 'reset'" @click="switchMode('reset')">找回密码</button>
        </div>
        <form @submit.prevent="submit">
          <fieldset :disabled="busy || sending">
            <TextField
              v-if="mode === 'username' || mode === 'register'"
              v-model="username" label="用户名" name="username" auto-complete="username" :required="true"
            />
            <template v-if="mode !== 'username'">
              <TextField
                v-model="email" label="邮箱" name="email" type="email" auto-complete="email"
                :read-only="sending" :required="true"
              />
            </template>
            <TextField
              v-if="mode !== 'email-code'"
              v-model="password" :label="mode === 'reset' ? '新密码' : '密码'"
              :name="mode === 'reset' ? 'new_password' : 'password'" type="password"
              :auto-complete="mode === 'register' || mode === 'reset' ? 'new-password' : 'current-password'"
              :required="true"
            />
            <div v-if="needsCode" class="code-row">
              <TextField v-model="code" label="邮箱验证码" name="code" auto-complete="one-time-code" :required="true" :hint="codeHint" />
              <button class="quiet code-send" type="button" :disabled="sending || remaining > 0" @click="sendCode">
                {{ sending ? '发送中…' : remaining > 0 ? `${remaining}s 后重发` : '发送验证码' }}
              </button>
            </div>
            <button class="block" type="submit">{{ busy ? '处理中…' : submitLabel }}</button>
          </fieldset>
        </form>
        <p v-if="notice" class="notice notice-success" role="status">{{ notice }}</p>
        <ErrorNotice :error="error" />
        <ErrorNotice :error="codeError" />
      </div>
      <div class="panel-body auth-foot">
        <span v-if="mode === 'username' || mode === 'email-password'">
          <button class="link sm" type="button" @click="switchMode(mode === 'email-password' ? 'username' : 'email-password')">
            {{ mode === 'email-password' ? '用户名密码登录' : '邮箱密码登录' }}
          </button>
        </span>
        <RouterLink v-if="admin" to="/login">用户端登录</RouterLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-page { max-width: 420px; margin: var(--space-6) auto; }
.auth-card h1 { font-size: var(--text-lg); margin-bottom: var(--space-4); }
.mode-tabs { display: flex; gap: var(--space-1); border-bottom: 1px solid var(--border); margin-bottom: var(--space-4); }
.mode-tabs button { padding: 6px 10px; font-size: var(--text-sm); border-bottom: 2px solid transparent; border-radius: 0; color: var(--muted); }
.mode-tabs button.active { color: var(--accent); border-bottom-color: var(--accent); font-weight: 500; }
.mode-tabs button:hover { background: none; color: var(--ink); }
.mode-tabs button.active:hover { color: var(--accent); }
.code-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: var(--space-2); align-items: start; }
.code-send { margin-top: 24px; height: 34px; }
.auth-foot { display: flex; justify-content: space-between; gap: var(--space-3); font-size: var(--text-xs); }
</style>
