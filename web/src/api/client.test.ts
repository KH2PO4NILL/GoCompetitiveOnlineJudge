import { describe, expect, it, vi } from 'vitest'
import { ApiClient, ApiError } from './client'
import { decode, encode } from './json'

const user = { user_id: 1, username: 'test-user', email: 'test@example.invalid', role: 'user', status: 'normal', school_id: 0 }
const tokens = { access_token: 'test-access', refresh_token: 'test-refresh', expires_in: 900 }
const login = { user, tokens }
const reply = (body: unknown, status = 200) => new Response(encode(body), { status, headers: { 'Content-Type': 'application/json' } })
const denied = () => reply({ code: 'unauthenticated', message: 'expired' }, 401)
function deferred<Value>() {
  let resolve!: (value: Value) => void
  const promise = new Promise<Value>((complete) => { resolve = complete })
  return { promise, resolve }
}

describe('lossless HTTP numbers', () => {
  it('preserves int64 through parsing and serialization without converting strings', () => {
    const source = '{"id":9223372036854775807,"revision":9007199254740993,"page":1,"score":1.5,"title":"9007199254740993"}'
    expect(encode(decode(source))).toBe(source)
    expect(decode(source)).toEqual({ id: 9223372036854775807n, revision: 9007199254740993n, page: 1, score: 1.5, title: '9007199254740993' })
  })
  it('rejects malformed and duplicate-key JSON', () => {
    expect(() => decode('{')).toThrow()
    expect(() => decode('{"id":1,"id":2}')).toThrow()
  })
})

describe('session and request boundaries', () => {
  it('uses one refresh for concurrent 401 responses and retries with rotated tokens', async () => {
    const pending = deferred<Response>()
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockResolvedValueOnce(denied()).mockResolvedValueOnce(denied()).mockImplementationOnce(() => pending.promise).mockImplementation(async () => reply({ ok: true }))
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const requests = [client.request('/users/me'), client.request('/users/me')]
    await vi.waitFor(() => expect(transport).toHaveBeenCalledTimes(4))
    pending.resolve(reply({ tokens: { ...tokens, access_token: 'rotated' } }))
    expect(await Promise.all(requests)).toEqual([{ ok: true }, { ok: true }])
    expect(transport.mock.calls.filter(([path]) => String(path).endsWith('/auth/token/refresh'))).toHaveLength(1)
    expect(transport.mock.calls[4][1]?.headers).toMatchObject({ Authorization: 'Bearer rotated' })
  })

  it('does not rotate again when a late 401 used an old access token', async () => {
    const late = deferred<Response>()
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockImplementationOnce(() => late.promise).mockResolvedValueOnce(denied()).mockResolvedValueOnce(reply({ tokens: { ...tokens, access_token: 'rotated' } })).mockImplementation(async () => reply({ ok: true }))
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const first = client.request('/users/me')
    await client.request('/users/me')
    late.resolve(denied())
    await first
    expect(transport.mock.calls.filter(([path]) => String(path).endsWith('/auth/token/refresh'))).toHaveLength(1)
  })

  it('cannot resurrect a logged-out session from a delayed refresh', async () => {
    const pending = deferred<Response>()
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockResolvedValueOnce(denied()).mockImplementationOnce(() => pending.promise)
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const request = client.request('/users/me')
    const rejected = expect(request).rejects.toThrow('会话已变更')
    await vi.waitFor(() => expect(transport).toHaveBeenCalledTimes(3))
    client.clear()
    pending.resolve(reply({ tokens }))
    await rejected
    expect(client.snapshot()).toBeNull()
  })

  it('rejects stale replay data after an account change even if transport ignores abort', async () => {
    const pending = deferred<Response>()
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockResolvedValueOnce(denied()).mockResolvedValueOnce(reply({ tokens })).mockImplementationOnce(() => pending.promise)
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    const request = client.request('/users/me')
    const rejected = expect(request).rejects.toThrow('会话已变更')
    await vi.waitFor(() => expect(transport).toHaveBeenCalledTimes(4))
    client.clear(); pending.resolve(reply({ user }))
    await rejected
  })

  it('clears identity on failed refresh without retry loops', async () => {
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockImplementation(async () => denied())
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    await expect(client.request('/users/me')).rejects.toMatchObject({ status: 401 })
    expect(client.snapshot()).toBeNull()
    expect(transport).toHaveBeenCalledTimes(3)
  })

  it('rejects a second 401 after refresh without another refresh', async () => {
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockResolvedValueOnce(denied()).mockResolvedValueOnce(reply({ tokens })).mockResolvedValueOnce(denied())
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    await expect(client.request('/users/me')).rejects.toMatchObject({ status: 401 })
    expect(client.snapshot()).toBeNull()
    expect(transport).toHaveBeenCalledTimes(4)
  })

  it('keeps public login failures out of refresh logic', async () => {
    const transport = vi.fn<typeof fetch>().mockResolvedValue(denied())
    const client = new ApiClient('app', transport)
    await expect(client.login('/auth/login/username', {})).rejects.toMatchObject({ status: 401 })
    expect(transport).toHaveBeenCalledTimes(1)
  })

  it('rejects non-admin login and attempts to revoke the issued session', async () => {
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockResolvedValueOnce(reply({}))
    const client = new ApiClient('admin', transport)
    await expect(client.login('/auth/login/username', {})).rejects.toMatchObject({ status: 403 })
    expect(client.snapshot()).toBeNull()
    expect(transport.mock.calls[1][0]).toBe('/api/v1/admin/auth/logout')
  })

  it('separates user and admin tokens and never sends browser cookies', async () => {
    const appTransport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockResolvedValueOnce(reply({}))
    const adminTransport = vi.fn<typeof fetch>().mockResolvedValue(reply({}))
    const app = new ApiClient('app', appTransport)
    const admin = new ApiClient('admin', adminTransport)
    await app.login('/auth/login/username', {})
    await admin.request('/users')
    expect(adminTransport.mock.calls[0][1]).toMatchObject({ credentials: 'omit', cache: 'no-store', redirect: 'error' })
    expect(adminTransport.mock.calls[0][1]?.headers).not.toHaveProperty('Authorization')
    await app.logout()
    expect(app.snapshot()).toBeNull()
  })

  it('does not replay an uncertain write after network failure', async () => {
    const transport = vi.fn<typeof fetch>().mockRejectedValue(new TypeError('offline'))
    const client = new ApiClient('app', transport)
    await expect(client.request('/submissions', { method: 'POST', body: {} })).rejects.toMatchObject({ uncertain: true, code: 'network_error' })
    expect(transport).toHaveBeenCalledTimes(1)
  })

  it('preserves conflict and retry-after details without retrying writes', async () => {
    for (const status of [403, 404, 409, 412, 429, 503]) {
      const transport = vi.fn<typeof fetch>().mockResolvedValue(new Response(encode({ code: 'server_code', message: 'server message' }), { status, headers: { 'Retry-After': '12' } }))
      await expect(new ApiClient('admin', transport).request('/problems/1/publish', { method: 'POST', body: { expected_revision: 9007199254740993n } })).rejects.toMatchObject({ status, code: 'server_code', retryAfter: '12' })
      expect(transport).toHaveBeenCalledTimes(1)
      expect(transport.mock.calls[0][1]?.body).toBe('{"expected_revision":9007199254740993}')
    }
  })

  it('clears local identity even when remote logout fails', async () => {
    const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(reply(login)).mockRejectedValueOnce(new Error('offline'))
    const client = new ApiClient('app', transport)
    await client.login('/auth/login/username', {})
    await expect(client.logout()).rejects.toBeInstanceOf(ApiError)
    expect(client.snapshot()).toBeNull()
  })

  it('rejects an invalid successful authentication response without storing tokens', async () => {
    const transport = vi.fn<typeof fetch>().mockResolvedValue(reply({ user, tokens: { access_token: 'incomplete' } }))
    const client = new ApiClient('app', transport)
    await expect(client.login('/auth/login/username', {})).rejects.toMatchObject({ code: 'invalid_response' })
    expect(client.snapshot()).toBeNull()
  })

  it('passes cancellation to transport and does not classify it as retryable network loss', async () => {
    const controller = new AbortController()
    const transport = vi.fn<typeof fetch>().mockImplementation(async (_path, options) => {
      controller.abort()
      options?.signal?.throwIfAborted()
      return reply({})
    })
    await expect(new ApiClient('app', transport).request('/users/me', { signal: controller.signal })).rejects.toMatchObject({ name: 'AbortError' })
    expect(transport).toHaveBeenCalledTimes(1)
  })
})
