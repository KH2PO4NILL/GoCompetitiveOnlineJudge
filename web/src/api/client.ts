import { z } from 'zod'
import type { UserInfo, TokenPair, LoginResp } from './contracts/app'
import { decode, encode } from './json'

const integer = z.union([z.number().int().safe(), z.bigint()])
const tokensSchema = z.object({ access_token: z.string().min(1), refresh_token: z.string().min(1), expires_in: integer })
const loginSchema = z.object({ tokens: tokensSchema, user: z.object({
  user_id: integer, username: z.string(), email: z.string(), role: z.string(), status: z.string(), school_id: integer,
}) })

export type Scope = 'app' | 'admin'
type Session = { user: UserInfo; tokens: TokenPair } | null
type Options = { method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'; body?: unknown; signal?: AbortSignal; anonymous?: boolean }

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public retryAfter?: string, public uncertain = false) {
    super(message)
  }
}

export function isAdmin(user: UserInfo | undefined): boolean {
  return user?.status === 'normal' && ['admin', 'super_admin'].includes(user.role)
}

export class ApiClient {
  private session: Session = null
  private epoch = 0
  private listeners = new Set<() => void>()
  private refreshFlight: Promise<void> | undefined
  private lifetime = new AbortController()

  constructor(readonly scope: Scope, private transport: typeof fetch = (...args) => fetch(...args)) {}

  snapshot = (): Session => this.session
  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }

  private replace(session: Session) {
    this.lifetime.abort()
    this.lifetime = new AbortController()
    this.epoch++
    this.refreshFlight = undefined
    this.session = session
    for (const listener of this.listeners) listener()
  }

  clear = () => this.replace(null)

  async login(path: string, body: unknown): Promise<UserInfo> {
    this.clear()
    const epoch = this.epoch
    const response = await this.request<LoginResp>(path, { method: 'POST', body, anonymous: true })
    const parsed = loginSchema.safeParse(response)
    if (!parsed.success) throw new ApiError(502, 'invalid_response', '认证服务响应不符合接口契约')
    if (epoch !== this.epoch) throw new ApiError(401, 'session_changed', '会话已变更，请重新登录')
    if (this.scope === 'admin' && !isAdmin(parsed.data.user)) {
      await this.wire('/auth/logout', { method: 'POST', body: parsed.data.tokens }, parsed.data.tokens.access_token).catch(() => undefined)
      throw new ApiError(403, 'permission_denied', '此账号没有管理权限')
    }
    this.replace(parsed.data)
    return parsed.data.user
  }

  async logout(): Promise<void> {
    const tokens = this.session?.tokens
    this.clear()
    if (tokens) await this.wire('/auth/logout', { method: 'POST', body: tokens }, tokens.access_token)
  }

  updateUser(user: UserInfo) {
    if (this.session) {
      if (String(user.user_id) !== String(this.session.user.user_id)) throw new ApiError(401, 'session_changed', '账号已变更')
      this.session = { ...this.session, user }
      for (const listener of this.listeners) listener()
    }
  }

  private async refresh(epoch: number): Promise<void> {
    if (!this.refreshFlight) {
      const tokens = this.session?.tokens
      if (!tokens) throw new ApiError(401, 'unauthenticated', '请先登录')
      const flight = (async () => {
        try {
          const response = await this.wire('/auth/token/refresh', { method: 'POST', body: { refresh_token: tokens.refresh_token }, signal: this.lifetime.signal })
          const parsed = z.object({ tokens: tokensSchema }).safeParse(response)
          if (!parsed.success) throw new ApiError(502, 'invalid_response', '刷新响应不符合接口契约，请重新登录')
          if (epoch !== this.epoch || !this.session) throw new ApiError(401, 'session_changed', '会话已变更')
          this.session = { ...this.session, tokens: parsed.data.tokens }
          for (const listener of this.listeners) listener()
        } catch (error) {
          if (epoch === this.epoch) this.clear()
          throw error
        }
      })()
      this.refreshFlight = flight
      void flight.finally(() => { if (this.refreshFlight === flight) this.refreshFlight = undefined }).catch(() => undefined)
    }
    await this.refreshFlight
  }

  async request<Response>(path: string, options: Options = {}): Promise<Response> {
    const epoch = this.epoch
    const access = options.anonymous ? undefined : this.session?.tokens.access_token
    const signals = [this.lifetime.signal, ...(options.signal ? [options.signal] : [])]
    const requestOptions = { ...options, signal: AbortSignal.any(signals) }
    try {
      const result = await this.wire(path, requestOptions, access)
      if (epoch !== this.epoch) throw new ApiError(401, 'session_changed', '会话已变更')
      return result as Response
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 401 || !access || epoch !== this.epoch || options.signal?.aborted) throw error
      if (access === this.session?.tokens.access_token) await this.refresh(epoch)
      if (epoch !== this.epoch || !this.session) throw new ApiError(401, 'session_changed', '请重新登录')
      try {
        const result = await this.wire(path, requestOptions, this.session.tokens.access_token)
        if (epoch !== this.epoch) throw new ApiError(401, 'session_changed', '会话已变更')
        return result as Response
      } catch (retryError) {
        if (retryError instanceof ApiError && retryError.status === 401 && epoch === this.epoch) this.clear()
        throw retryError
      }
    }
  }

  private async wire(path: string, options: Options, access?: string): Promise<unknown> {
    if (!path.startsWith('/') || path.startsWith('//') || path.includes('..')) throw new Error('无效的 API 路径')
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 30_000)
    const method = options.method ?? 'GET'
    const writing = method !== 'GET'
    try {
      let response: Response
      try {
        response = await this.transport(`${this.scope === 'admin' ? '/api/v1/admin' : '/api/v1'}${path}`, {
          method,
          headers: { Accept: 'application/json', ...(options.body === undefined ? {} : { 'Content-Type': 'application/json' }), ...(access ? { Authorization: `Bearer ${access}` } : {}) },
          body: options.body === undefined ? undefined : encode(options.body),
          signal: AbortSignal.any([controller.signal, ...(options.signal ? [options.signal] : [])]),
          cache: 'no-store',
          credentials: 'omit',
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
        const error = z.object({ code: z.string(), message: z.string() }).safeParse(body)
        throw new ApiError(response.status, error.success ? error.data.code : 'http_error', error.success ? error.data.message : `请求失败（HTTP ${response.status}）`, response.headers.get('Retry-After') ?? undefined, writing && response.status >= 500)
      }
      return body
    } finally { clearTimeout(timeout) }
  }
}
