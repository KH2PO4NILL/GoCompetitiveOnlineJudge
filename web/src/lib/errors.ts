import { ApiError } from '../api/client'

export type ErrorDescription = { message: string; hint?: string; uncertain?: boolean }

const hints: Record<number, string> = {
  401: '登录状态已失效，请重新登录。',
  403: '当前账号没有执行此操作的权限。',
  404: '目标资源不存在或已被删除。',
  409: '与服务端状态冲突（可能已被他人修改），请刷新后重试。',
  422: '提交的内容未通过服务端校验，请检查后重试。',
  429: '请求过于频繁，请稍后再试。',
}

/** 把任意异常转换为可展示、可恢复的用户文案；不泄露 Token、验证码或源码。 */
export function describeError(error: unknown): ErrorDescription | undefined {
  if (error === undefined || error === null) return undefined
  if (error instanceof DOMException && error.name === 'AbortError') return undefined
  if (error instanceof ApiError) {
    const hint = error.status === 401 && !['session_invalid', 'refresh_token_invalid', 'access_token_invalid'].includes(error.code) ? undefined : error.retryAfter ? `可在 ${error.retryAfter} 秒后重试。` : hints[error.status]
    return { message: error.message, hint, uncertain: error.uncertain }
  }
  if (error instanceof Error && error.message) return { message: error.message }
  return { message: '操作失败，请重试。' }
}
