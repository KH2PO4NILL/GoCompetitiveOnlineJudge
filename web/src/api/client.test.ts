import { describe, expect, it, vi } from 'vitest'
import { ApiClient, ApiError, isAdminUser } from './client'
import type { UserInfo } from './contracts/app'

const user: UserInfo = { user_id: 7, username: 'alice', email: 'a@b.c', role: 'user', status: 'normal', school_id: 0 }
const adminUser: UserInfo = { ...user, role: 'admin' }
const tokens = { access_token: 'access-1', refresh_token: 'refresh-1', expires_in: 3600 }
const loginBody = { tokens, user }

function jsonResponse(body: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } })
}

function mockTransport(handler: (url: string, init: RequestInit) => Response | Promise<Response>) {
  return vi.fn(async (input: string | URL | Request, init?: RequestInit) => handler(String(input), init ?? {})) as unknown as typeof fetch
}

describe('ApiClient.login', () => {
  it('成功登录后持有会话', async () => {
    const client = new ApiClient('app', mockTransport(() => jsonResponse({ tokens, user })))
    const result = await client.login('/auth/login/username', { username: 'alice', password: 'x' })
    expect(result.username).toBe('alice')
    expect(client.snapshot()?.tokens.access_token).toBe('access-1')
  })

  it('响应不符合契约时拒绝建立会话', async () => {
    const client = new ApiClient('app', mockTransport(() => jsonResponse({ tokens: { access_token: '' } })))
    await expect(client.login('/auth/login/username', {})).rejects.toMatchObject({ code: 'invalid_response' })
    expect(client.snapshot()).toBeNull()
  })

  it('管理端拒绝非管理员并登出', async () => {
    const transport = mockTransport((url) =>
      url.endsWith('/auth/login/username') ? jsonResponse(loginBody) : jsonResponse({}))
    const client = new ApiClient('admin', transport)
    await expect(client.login('/auth/login/username', {})).rejects.toMatchObject({ status: 403, code: 'permission_denied' })
    expect(client.snapshot()).toBeNull()
    expect(transport).toHaveBeenCalledWith(expect.stringContaining('/api/v1/admin/auth/logout'), expect.anything())
  })

  it('管理端接受管理员角色', async () => {
    const client = new ApiClient('admin', mockTransport(() => jsonResponse({ tokens, user: adminUser })))
    await expect(client.login('/auth/login/username', {})).resolves.toMatchObject({ role: 'admin' })
  })

  it('作用域前缀正确', async () => {
    const transport = mockTransport(() => jsonResponse({ tokens, user: adminUser }))
    await new ApiClient('admin', transport).login('/auth/login/username', {})
    expect(transport).toHaveBeenCalledWith('/api/v1/admin/auth/login/username', expect.anything())
    const appTransport = mockTransport(() => jsonResponse(loginBody))
    await new ApiClient('app', appTransport).login('/auth/login/username', {})
    expect(appTransport).toHaveBeenCalledWith('/api/v1/auth/login/username', expect.anything())
  })
})

describe('ApiClient.request 刷新队列', () => {
  it('401 时单飞刷新并重放一次', async () => {
    let refreshCalls = 0
    let dataCalls = 0
    const transport = mockTransport((url, init) => {
      if (url.includes('/auth/login/username')) return jsonResponse(loginBody)
      const auth = String((init.headers as Record<string, string>).Authorization ?? '')
      if (url.includes('/auth/token/refresh')) {
        refreshCalls++
        return jsonResponse({ tokens: { ...tokens, access_token: 'access-2' } })
      }
      dataCalls++
      return auth === 'Bearer access-2' ? jsonResponse({ ok: true }) : jsonResponse({ code: 'access_token_invalid', message: 'expired' }, 401)
    })
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const [a, b] = await Promise.all([
      client.request('/problems'),
      client.request('/contests'),
    ])
    expect(a).toEqual({ ok: true })
    expect(b).toEqual({ ok: true })
    expect(refreshCalls).toBe(1)
    expect(dataCalls).toBe(4)
    expect(client.snapshot()?.tokens.access_token).toBe('access-2')
  })

  it('刷新失败时清空会话', async () => {
    const transport = mockTransport((url) =>
      url.includes('/auth/login/username') ? jsonResponse(loginBody)
        : url.includes('/auth/token/refresh') ? jsonResponse({ code: 'refresh_token_invalid', message: 'bad' }, 401)
          : jsonResponse({ code: 'access_token_invalid', message: 'expired' }, 401))
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    await expect(client.request('/problems')).rejects.toBeInstanceOf(ApiError)
    expect(client.snapshot()).toBeNull()
  })

  it('重放仍提示访问令牌无效时保持会话且不无限重试', async () => {
    const transport = mockTransport((url) =>
      url.includes('/auth/login/username') ? jsonResponse(loginBody)
        : url.includes('/auth/token/refresh') ? jsonResponse({ tokens: { ...tokens, access_token: 'access-2' } })
          : jsonResponse({ code: 'access_token_invalid', message: 'expired' }, 401))
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    await expect(client.request('/problems')).rejects.toMatchObject({ status: 401 })
    expect(client.snapshot()).not.toBeNull()
  })

  it('anonymous 请求不携带 Authorization 也不触发刷新', async () => {
    const transport = mockTransport((url, init) => {
      expect((init.headers as Record<string, string>).Authorization).toBeUndefined()
      return url.includes('refresh') ? jsonResponse({}) : jsonResponse({ code: 'x', message: 'y' }, 401)
    })
    const client = new ApiClient('app', transport)
    await expect(client.request('/auth/email-codes', { method: 'POST', body: {}, anonymous: true })).rejects.toMatchObject({ status: 401 })
  })
})

describe('ApiClient 写保护与错误模型', () => {
  it('写请求网络错误标记 uncertain', async () => {
    const transport = mockTransport((url) => {
      if (url.includes('login')) return jsonResponse(loginBody)
      return Promise.reject(new TypeError('fetch failed'))
    })
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const error = await client.request('/submissions', { method: 'POST', body: {} }).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).uncertain).toBe(true)
    expect((error as ApiError).message).toContain('结果未知')
  })

  it('写请求 5xx 标记 uncertain，读请求不标记', async () => {
    const transport = mockTransport((url) => {
      if (url.includes('login')) return jsonResponse(loginBody)
      return jsonResponse({ code: 'internal', message: 'boom' }, 500)
    })
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const writeError = await client.request('/submissions', { method: 'POST', body: {} }).catch((e: unknown) => e) as ApiError
    expect(writeError.uncertain).toBe(true)
    const readError = await client.request('/problems').catch((e: unknown) => e) as ApiError
    expect(readError.uncertain).toBe(false)
  })

  it('透传服务端错误 code/message 与 Retry-After', async () => {
    const transport = mockTransport((url) =>
      url.includes('login') ? jsonResponse(loginBody)
        : jsonResponse({ code: 'rate_limited', message: '太频繁' }, 429, { 'Retry-After': '30' }))
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const error = await client.request('/problems').catch((e: unknown) => e) as ApiError
    expect(error.status).toBe(429)
    expect(error.code).toBe('rate_limited')
    expect(error.message).toBe('太频繁')
    expect(error.retryAfter).toBe('30')
  })

  it('拒绝非法路径', async () => {
    const client = new ApiClient('app', mockTransport(() => jsonResponse({})))
    await expect(client.request('//evil')).rejects.toThrow('无效的 API 路径')
    await expect(client.request('/a/../b')).rejects.toThrow('无效的 API 路径')
    await expect(client.request('relative')).rejects.toThrow('无效的 API 路径')
  })

  it('外部 signal 中止时按原样抛出且不触发刷新', async () => {
    const controller = new AbortController()
    const transport = mockTransport((url) => {
      if (url.includes('login')) return jsonResponse(loginBody)
      controller.abort()
      return jsonResponse({ code: 'access_token_invalid', message: 'expired' }, 401)
    })
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    await expect(client.request('/problems', { signal: controller.signal })).rejects.toMatchObject({ status: 401 })
    expect(client.snapshot()).not.toBeNull()
  })
})

describe('会话隔离与订阅', () => {
  it('logout 清空会话并通知服务端', async () => {
    const transport = mockTransport((url) => (url.includes('login') ? jsonResponse(loginBody) : jsonResponse({})))
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    await client.logout()
    expect(client.snapshot()).toBeNull()
    expect(transport).toHaveBeenCalledWith('/api/v1/auth/logout', expect.objectContaining({ method: 'POST' }))
  })

  it('updateUser 拒绝跨账号更新', async () => {
    const client = new ApiClient('app', mockTransport(() => jsonResponse(loginBody)))
    await client.login('/auth/login/username', {})
    expect(() => client.updateUser({ ...user, user_id: 8 })).toThrow('账号已变更')
    client.updateUser({ ...user, username: 'alice2' })
    expect(client.snapshot()?.user.username).toBe('alice2')
  })

  it('会话替换时订阅者收到通知', async () => {
    const client = new ApiClient('app', mockTransport(() => jsonResponse(loginBody)))
    const listener = vi.fn()
    client.subscribe(listener)
    await client.login('/auth/login/username', {})
    expect(listener).toHaveBeenCalled()
  })
})

describe('isAdminUser', () => {
  it('仅正常状态的 admin/super_admin 通过', () => {
    expect(isAdminUser({ ...user, role: 'admin' })).toBe(true)
    expect(isAdminUser({ ...user, role: 'super_admin' })).toBe(true)
    expect(isAdminUser(user)).toBe(false)
    expect(isAdminUser({ ...user, role: 'admin', status: 'disabled' })).toBe(false)
    expect(isAdminUser(undefined)).toBe(false)
  })
})

describe('会话稳定性回归', () => {
  for (const scope of ['app', 'admin'] as const) {
    const scopeUser = scope === 'admin' ? adminUser : user
    it(`${scope} 普通业务 401 不刷新或退出`, async () => {
      let refreshes = 0
      const client = new ApiClient(scope, mockTransport((url) => {
        if (url.includes('login')) return jsonResponse({ tokens, user: scopeUser })
        if (url.includes('refresh')) refreshes++
        return jsonResponse({ code: 'unauthenticated', message: '验证码错误' }, 401)
      }))
      await client.login('/auth/login/username', {})
      await expect(client.request('/users/me/email', { method: 'PUT', body: {} })).rejects.toMatchObject({ status: 401 })
      expect(refreshes).toBe(0)
      expect(client.snapshot()?.user).toEqual(scopeUser)
      expect(client.sessionMessage()).toBe('')
    })
    for (const failure of [500, 503, 'network', 'invalid-response'] as const) {
      it(`${scope} 刷新临时失败 ${failure} 保留会话，随后可以恢复`, async () => {
        let unavailable = true
        const client = new ApiClient(scope, mockTransport((url, init) => {
          if (url.includes('login')) return jsonResponse({ tokens, user: scopeUser })
          if (url.includes('refresh')) {
            if (!unavailable) return jsonResponse({ tokens: { ...tokens, access_token: 'access-2' } })
            if (failure === 'network') throw new TypeError('offline')
            if (failure === 'invalid-response') return jsonResponse({ unexpected: true })
            return jsonResponse({ code: 'unavailable', message: 'temporarily unavailable' }, failure)
          }
          return (init.headers as Record<string, string>).Authorization === 'Bearer access-2'
            ? jsonResponse({ ok: true }) : jsonResponse({ code: 'access_token_invalid', message: 'expired' }, 401)
        }))
        await client.login('/auth/login/username', {})
        await expect(client.request('/users/me')).rejects.toBeInstanceOf(ApiError)
        expect(client.snapshot()).not.toBeNull()
        unavailable = false
        await expect(client.request('/users/me')).resolves.toEqual({ ok: true })
      })
    }
    it(`${scope} 重新登录失败保持原会话`, async () => {
      let failed = false
      const client = new ApiClient(scope, mockTransport(() => failed
        ? jsonResponse({ code: 'unauthenticated', message: 'wrong password' }, 401)
        : jsonResponse({ tokens, user: scopeUser })))
      await client.login('/auth/login/username', {})
      failed = true
      await expect(client.login('/auth/login/username', {})).rejects.toMatchObject({ status: 401 })
      expect(client.snapshot()?.user).toEqual(scopeUser)
    })
    it(`${scope} 使用 Cookie 恢复且保持单飞`, async () => {
      let refreshes = 0
      const client = new ApiClient(scope, mockTransport((url, init) => {
        expect(init.credentials).toBe('same-origin')
        if (url.includes('refresh')) { refreshes++; return jsonResponse({ tokens }) }
        return jsonResponse({ user: scopeUser })
      }))
      await Promise.all([client.restore(), client.restore()])
      expect(refreshes).toBe(1)
      expect(client.snapshot()?.user).toEqual(scopeUser)
    })
    it(`${scope} 会话撤销时提供明确提示`, async () => {
      const client = new ApiClient(scope, mockTransport((url) => url.includes('login')
        ? jsonResponse({ tokens, user: scopeUser }) : jsonResponse({ code: 'session_invalid', message: 'revoked' }, 401)))
      await client.login('/auth/login/username', {})
      await expect(client.request('/users/me')).rejects.toMatchObject({ code: 'session_invalid' })
      expect(client.snapshot()).toBeNull()
      expect(client.sessionMessage()).toContain('请重新登录')
    })
  }
  it('logout 后到达的恢复响应不能复活会话', async () => {
    let finish: ((response: Response) => void) | undefined
    const client = new ApiClient('app', mockTransport((url) => url.includes('refresh')
      ? new Promise<Response>((resolve) => { finish = resolve }) : jsonResponse({ user })))
    const restoring = client.restore()
    await client.logout()
    finish!(jsonResponse({ tokens }))
    await restoring
    expect(client.snapshot()).toBeNull()
  })
})
