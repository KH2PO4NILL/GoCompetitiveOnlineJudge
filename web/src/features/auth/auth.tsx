import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import { useApi, useSession } from '../../app/context'
import type { GetCurrentUserResp, SendEmailCodeResp } from '../../api/contracts/app'
import { ActionForm, ErrorNotice, Field } from '../../components/shared'

function returnPath(value: string | null, admin: boolean) {
  const fallback = admin ? '/admin' : '/account'
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) return fallback
  if (admin ? !value.startsWith('/admin/') : value.startsWith('/admin')) return fallback
  return value
}

export function LoginPage() {
  const api = useApi()
  const navigate = useNavigate()
  const [search] = useSearchParams()
  const admin = api.scope === 'admin'
  const [mode, setMode] = useState('username')
  const [email, setEmail] = useState('')
  const [codeError, setCodeError] = useState<unknown>()
  const [sending, setSending] = useState(false)
  const [retryAt, setRetryAt] = useState(0)
  const [clock, setClock] = useState(Date.now())
  const [notice, setNotice] = useState('')
  useEffect(() => { const timer = setInterval(() => setClock(Date.now()), 1000); return () => clearInterval(timer) }, [])
  const remaining = Math.max(0, Math.ceil((retryAt - clock) / 1000))
  const withCode = ['email-code', 'register', 'reset'].includes(mode)
  const purpose = mode === 'register' ? 'register' : mode === 'reset' ? 'reset_password' : 'login'
  return <section className="auth-card"><p className="eyebrow">{admin ? 'ADMIN ACCESS' : 'WELCOME TO GCOJ'}</p>
    <h1>{admin ? '登录管理工作台' : '继续你的解题之旅'}</h1>
    <p className="muted">会话仅保存在内存中，刷新页面后需重新登录。用户端和管理端分别登录。</p>
    <label>认证方式<select value={mode} onChange={(event) => { setMode(event.target.value); setNotice(''); setCodeError(undefined) }}>
      <option value="username">用户名密码登录</option><option value="email-password">邮箱密码登录</option><option value="email-code">邮箱验证码登录</option>
      {!admin && <option value="register">注册账号</option>}<option value="reset">找回密码</option>
    </select></label>
    <ActionForm key={mode} label={mode === 'register' ? '创建账号' : mode === 'reset' ? '重置密码' : '登录'} action={async (values) => {
      const body: Record<string, string> = { ...values }
      if (mode !== 'username') body.email = z.email().parse(email)
      if (mode === 'register' || mode === 'reset') {
        await api.request(mode === 'register' ? '/auth/register' : '/auth/password/reset', { method: 'POST', body, anonymous: true })
        setNotice(mode === 'register' ? '注册成功，请登录。' : '密码已重置，请使用新密码登录。'); setMode('username')
      } else {
        await api.login(`/auth/login/${mode}`, body)
        navigate(returnPath(search.get('returnTo'), admin), { replace: true })
      }
    }}>
      {(mode === 'username' || mode === 'register') && <Field label="用户名" name="username" autoComplete="username" />}
      {mode !== 'username' && <label>邮箱<input name="email" type="email" required autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} /></label>}
      {mode !== 'email-code' && <Field label={mode === 'reset' ? '新密码' : '密码'} name={mode === 'reset' ? 'new_password' : 'password'} type="password" autoComplete={['register', 'reset'].includes(mode) ? 'new-password' : 'current-password'} />}
      {withCode && <><Field label="邮箱验证码" name="code" autoComplete="one-time-code" /><button type="button" className="secondary" disabled={sending || remaining > 0} onClick={async () => {
        setSending(true); setCodeError(undefined)
        try {
          const response = await api.request<SendEmailCodeResp>('/auth/email-codes', { method: 'POST', body: { email: z.email().parse(email), purpose }, anonymous: true })
          setRetryAt(Date.now() + response.retry_after * 1000); setClock(Date.now()); setNotice(`验证码已发送，有效期 ${response.expires_in} 秒。`)
        } catch (error) { setCodeError(error) } finally { setSending(false) }
      }}>{sending ? '发送中…' : remaining ? `${remaining} 秒后可重发` : '发送验证码'}</button></>}
    </ActionForm>
    <ErrorNotice error={codeError} />{notice && <p role="status" className="notice">{notice}</p>}
    <Link to={admin ? '/login' : '/admin/login'}>{admin ? '返回用户登录' : '管理员入口'}</Link>
  </section>
}

export function AccountPage() {
  const api = useApi()
  const session = useSession()
  const [email, setEmail] = useState('')
  const [codeReadyAt, setCodeReadyAt] = useState(0)
  if (!session) return null
  return <><h1>账号与安全</h1><p className="muted">{session.user.email} · {session.user.role}</p><div className="cards">
    <section className="panel"><h2>个人资料</h2><ActionForm onSuccess="资料已保存" action={async (values) => {
      const school = values.school_id.trim()
      if (school && !/^\d+$/.test(school)) throw new Error('学校 ID 必须是非负整数')
      const response = await api.request<GetCurrentUserResp>('/users/me', { method: 'PATCH', body: { username: values.username, ...(school ? { school_id: BigInt(school) } : {}) } })
      api.updateUser(response.user)
    }}><Field label="用户名" name="username" defaultValue={session.user.username} /><Field label="学校 ID（留空不修改）" name="school_id" defaultValue={String(session.user.school_id)} required={false} /></ActionForm></section>
    <section className="panel"><h2>修改密码</h2><ActionForm onSuccess="密码已修改；请退出并重新登录其他设备。" action={async (values) => { await api.request('/users/me/password', { method: 'PUT', body: values }) }}>
      <Field label="当前密码" name="current_password" type="password" autoComplete="current-password" /><Field label="新密码" name="new_password" type="password" autoComplete="new-password" />
    </ActionForm></section>
    <section className="panel"><h2>修改邮箱</h2><ActionForm label="发送验证邮件" onSuccess="已发送，请检查新邮箱。" action={async () => {
      if (Date.now() < codeReadyAt) throw new Error(`请在 ${Math.ceil((codeReadyAt - Date.now()) / 1000)} 秒后重试`)
      const result = await api.request<SendEmailCodeResp>('/auth/email-codes', { method: 'POST', body: { email: z.email().parse(email), purpose: 'change_email' } })
      setCodeReadyAt(Date.now() + result.retry_after * 1000)
    }}><label>新邮箱<input type="email" value={email} required onChange={(event) => setEmail(event.target.value)} /></label></ActionForm>
      <ActionForm label="确认修改邮箱" onSuccess="邮箱已修改" action={async (values) => {
        const response = await api.request<GetCurrentUserResp>('/users/me/email', { method: 'PUT', body: { new_email: z.email().parse(email), code: values.code } })
        api.updateUser(response.user)
      }}><Field label="新邮箱验证码" name="code" autoComplete="one-time-code" /></ActionForm>
    </section>
  </div></>
}
