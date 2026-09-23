import { useState, type ReactNode } from 'react'
import { ApiError } from '../api/client'

export function ErrorNotice({ error }: { error: unknown }) {
  if (!error) return null
  const labels: Record<number, string> = { 401: '请重新登录', 403: '没有权限', 404: '内容不存在', 409: '数据冲突，请重新加载并核对输入', 412: '当前状态不允许该操作', 429: '请求过于频繁', 503: '服务暂不可用' }
  return <div role="alert" className="notice error">
    <strong>{error instanceof ApiError ? labels[error.status] ?? '请求未完成' : '操作未完成'}</strong>
    <p>{error instanceof Error ? error.message : '发生未知错误'}</p>
    {error instanceof ApiError && error.retryAfter && <p>服务端建议等待：{error.retryAfter}</p>}
    {error instanceof ApiError && error.uncertain && <p>不要直接重复写入；请先查询结果。</p>}
  </div>
}

export function ActionForm({ children, action, label = '保存', onSuccess }: { children: ReactNode; action: (values: Record<string, string>) => Promise<void>; label?: string; onSuccess?: string }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>()
  const [success, setSuccess] = useState(false)
  return <form onSubmit={async (event) => {
    event.preventDefault()
    if (busy) return
    const values = Object.fromEntries([...new FormData(event.currentTarget)].map(([key, value]) => [key, String(value)]))
    setBusy(true); setError(undefined); setSuccess(false)
    try { await action(values); setSuccess(true) } catch (failure) { setError(failure) } finally { setBusy(false) }
  }}>
    <fieldset disabled={busy}>{children}<button type="submit">{busy ? '处理中…' : label}</button></fieldset>
    <ErrorNotice error={error} />
    {success && onSuccess && <p role="status" className="notice">{onSuccess}</p>}
  </form>
}

export function Field({ label, name, type = 'text', defaultValue, required = true, autoComplete }: { label: string; name: string; type?: string; defaultValue?: string; required?: boolean; autoComplete?: string }) {
  return <label>{label}<input name={name} type={type} defaultValue={defaultValue} required={required} autoComplete={autoComplete} /></label>
}
