import { z } from 'zod'
import type { LoginResp, TokenPair, UserInfo } from './contracts/app'
import { decode, encode } from './json'

export type Scope = 'app' | 'admin'

export type Session = { user: UserInfo; tokens: TokenPair } | null

export type RequestOptions = {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: unknown
  signal?: AbortSignal
  anonymous?: boolean
}

const integer = z.union([z.number().int().safe(), z.bigint()])
const tokensSchema = z.object({ access_token: z.string().min(1), refresh_token: z.string().min(1), expires_in: integer })
const loginSchema = z.object({
  tokens: tokensSchema,
  user: z.object({ user_id: integer, username: z.string(), email: z.string(), role: z.string(), status: z.string(), school_id: integer }),
})

const REQUEST_TIMEOUT_MS = 30_000

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public retryAfter?: string,
    // 写请求结果未知时为 true：界面必须阻止自动重试，引导用户先核对结果。
    public uncertain = false,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export function isAdminUser(user: UserInfo | undefined): boolean {
  return user?.status === 'normal' && (user.role === 'admin' || user.role === 'super_admin')
}

/**
 * 单一 scope 的 HTTP 客户端：会话仅存内存；明确的 access_token_invalid 触发单飞 refresh 后重放一次；
 * 会话变更（epoch 递增）时中止所有在途请求并清空对应查询缓存。
 */
export class ApiClient {
  private session: Session = null
  private epoch = 0
  private notice = ''
  private restored = false
  private restoreFlight: Promise<void> | undefined
  private refreshFlight: Promise<void> | undefined
  private lifetime = new AbortController()
  private listeners = new Set<() => void>()

  constructor(
    readonly scope: Scope,
    private transport: typeof fetch = (...args) => fetch(...args),
  ) {}

  snapshot = (): Session => this.session
  sessionMessage = (): string => this.notice

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }

  private emit() {
    for (const listener of this.listeners) listener()
  }

  private replaceSession(session: Session, notice = '') {
    this.notice = notice
    this.restored = true
    this.lifetime.abort()
    this.lifetime = new AbortController()
    this.epoch++
    this.refreshFlight = undefined
    this.session = session
    this.emit()
  }

  clear = () => this.replaceSession(null)

  async login(path: string, body: unknown): Promise<UserInfo> {
    const epoch = this.epoch
    const response = await this.request<LoginResp>(path, { method: 'POST', body, anonymous: true })
    const parsed = loginSchema.safeParse(response)
    if (!parsed.success) throw new ApiError(502, 'invalid_response', '认证服务响应不符合接口契约')
    if (epoch !== this.epoch) throw new ApiError(401, 'session_changed', '会话已变更，请重新登录')
    if (this.scope === 'admin' && !isAdminUser(parsed.data.user)) {
      // 非管理员拿到 token 也立即登出，避免管理端持有无效凭据
      await this.wire('/auth/logout', { method: 'POST', body: parsed.data.tokens }, parsed.data.tokens.access_token).catch(() => undefined)
      throw new ApiError(403, 'permission_denied', '此账号没有管理权限')
    }
    this.replaceSession(parsed.data)
    return parsed.data.user
  }

  async logout(): Promise<void> {
    const tokens = this.session?.tokens
    this.clear()
    await this.wire('/auth/logout', { method: 'POST', body: tokens ?? {} }, tokens?.access_token).catch(() => undefined)
  }

  updateUser(user: UserInfo) {
    if (!this.session) return
    if (String(user.user_id) !== String(this.session.user.user_id)) throw new ApiError(401, 'session_changed', '账号已变更，请重新登录')
    this.session = { ...this.session, user }
    this.emit()
  }

  private async refresh(epoch: number): Promise<void> {
    if (!this.refreshFlight) {
      const tokens = this.session?.tokens
      if (!tokens) throw new ApiError(401, 'unauthenticated', '请先登录')
      const flight = (async () => {
        try {
          const response = await this.browserSessionLock(() => this.wire('/auth/token/refresh', { method: 'POST', body: { refresh_token: tokens.refresh_token }, signal: this.lifetime.signal }))
          const parsed = z.object({ tokens: tokensSchema }).safeParse(response)
          if (!parsed.success) throw new ApiError(502, 'invalid_response', '刷新响应不符合接口契约，请重新登录')
          if (epoch !== this.epoch || !this.session) throw new ApiError(401, 'session_changed', '会话已变更')
          this.session = { ...this.session, tokens: parsed.data.tokens }
          this.emit()
        } catch (error) {
          if (epoch === this.epoch && terminalSessionError(error)) this.replaceSession(null, '登录会话已失效，请重新登录。')
          throw error
        }
      })()
      this.refreshFlight = flight
      void flight.finally(() => { if (this.refreshFlight === flight) this.refreshFlight = undefined }).catch(() => undefined)
    }
    await this.refreshFlight
  }

  async request<Response>(path: string, options: RequestOptions = {}): Promise<Response> {
    const epoch = this.epoch
    const access = options.anonymous ? undefined : this.session?.tokens.access_token
    const requestOptions = { ...options, signal: AbortSignal.any([this.lifetime.signal, ...(options.signal ? [options.signal] : [])]) }
    try {
      const result = await this.wire(path, requestOptions, access)
      if (epoch !== this.epoch) throw new ApiError(401, 'session_changed', '会话已变更')
      return result as Response
    } catch (error) {
      const retryable = error instanceof ApiError && error.status === 401 && error.code === 'access_token_invalid' && access !== undefined && epoch === this.epoch && !options.signal?.aborted
      if (!retryable) {
        if (access && epoch === this.epoch && terminalSessionError(error)) this.replaceSession(null, '登录会话已失效，请重新登录。')
        throw error
      }
      // 仅当仍持有触发 401 的同一 access token 时才刷新，避免并发请求重复刷新
      if (access === this.session?.tokens.access_token) await this.refresh(epoch)
      if (epoch !== this.epoch || !this.session) throw new ApiError(401, 'session_changed', '请重新登录')
      try {
        const result = await this.wire(path, requestOptions, this.session.tokens.access_token)
        if (epoch !== this.epoch) throw new ApiError(401, 'session_changed', '会话已变更')
        return result as Response
      } catch (retryError) {
        if (terminalSessionError(retryError) && epoch === this.epoch) this.replaceSession(null, '登录会话已失效，请重新登录。')
        throw retryError
      }
    }
  }

  private async browserSessionLock<T>(fn: () => Promise<T>): Promise<T> {
    if (typeof navigator !== 'undefined' && navigator.locks) return await navigator.locks.request(`gcoj-session-${this.scope}`, fn)
    return fn()
  }

  /** Restore through a scoped HttpOnly cookie. No credentials enter browser storage. */
  async restore(): Promise<void> {
    if (this.restored || this.session) return
    if (!this.restoreFlight) {
      const epoch = this.epoch
      const flight = this.browserSessionLock(async () => {
        try {
          const response = await this.wire('/auth/token/refresh', { method: 'POST', body: { refresh_token: '' }, signal: this.lifetime.signal })
          const parsed = z.object({ tokens: tokensSchema }).safeParse(response)
          if (!parsed.success) throw new ApiError(502, 'invalid_response', '会话恢复响应不符合接口契约')
          const profile = await this.wire('/users/me', { signal: this.lifetime.signal }, parsed.data.tokens.access_token)
          const result = loginSchema.safeParse({ ...profile as object, tokens: parsed.data.tokens })
          if (!result.success) throw new ApiError(502, 'invalid_response', '会话用户响应不符合接口契约')
          if (epoch !== this.epoch) return
          if (this.scope === 'admin' && !isAdminUser(result.data.user)) {
            await this.wire('/auth/logout', { method: 'POST', body: result.data.tokens }, result.data.tokens.access_token).catch(() => undefined)
            this.replaceSession(null, '此账号没有管理权限。')
            return
          }
          this.replaceSession(result.data)
        } catch (error) {
          if (epoch !== this.epoch) return
          if (terminalSessionError(error)) this.replaceSession(null, '登录会话已失效，请重新登录。')
          else if (error instanceof ApiError && error.status === 401) this.restored = true
          else { this.notice = '暂时无法恢复会话，请稍后重试。'; this.emit() }
        }
      })
      this.restoreFlight = flight
      void flight.finally(() => { if (this.restoreFlight === flight) this.restoreFlight = undefined }).catch(() => undefined)
    }
    await this.restoreFlight
  }

  private async wire(path: string, options: RequestOptions, access?: string): Promise<unknown> {
    if (!path.startsWith('/') || path.startsWith('//') || path.split('?')[0].includes('..')) throw new Error('无效的 API 路径')
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
    const method = options.method ?? 'GET'
    const writing = method !== 'GET'
    try {
      let response: Response
      try {
        response = await this.transport(`${this.scope === 'admin' ? '/api/v1/admin' : '/api/v1'}${path}`, {
          method,
          headers: {
            Accept: 'application/json',
            'X-GCOJ-Browser-Session': '1',
            ...(options.body === undefined ? {} : { 'Content-Type': 'application/json' }),
            ...(access ? { Authorization: `Bearer ${access}` } : {}),
          },
          body: options.body === undefined ? undefined : encode(options.body),
          signal: AbortSignal.any([controller.signal, ...(options.signal ? [options.signal] : [])]),
          cache: 'no-store',
          credentials: 'same-origin',
          redirect: 'error',
        })
      } catch (error) {
        if (options.signal?.aborted) throw error
        throw new ApiError(0, controller.signal.aborted ? 'timeout' : 'network_error', writing ? '请求结果未知，请先核对结果，不要直接重复操作' : '网络不可用或请求超时，请重试', undefined, writing)
      }
      let body: unknown
      try {
        const text = await response.text()
        body = text ? decode(text) : undefined
      } catch {
        throw new ApiError(response.status, 'invalid_response', writing ? '响应无法读取，操作结果未知，请先核对结果' : '服务响应无法读取', undefined, writing)
      }
      if (!response.ok) {
        const parsed = z.object({ code: z.string(), message: z.string() }).safeParse(body)
        throw new ApiError(
          response.status,
          parsed.success ? parsed.data.code : 'http_error',
          parsed.success ? parsed.data.message : `请求失败（HTTP ${response.status}）`,
          response.headers.get('Retry-After') ?? undefined,
          writing && response.status >= 500,
        )
      }
      return body
    } finally {
      clearTimeout(timeout)
    }
  }
}

function terminalSessionError(error: unknown): boolean {
 return error instanceof ApiError && (
  (error.status === 401 && (error.code === 'refresh_token_invalid' || error.code === 'session_invalid')) ||
  (error.status === 403 && error.code === 'account_unavailable')
 )
}
