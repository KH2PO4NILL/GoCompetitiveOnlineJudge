import { describe, expect, it } from 'vitest'
import { ApiError } from '../api/client'
import { describeError } from './errors'

describe('describeError', () => {
  it('映射常见 HTTP 状态到可恢复提示', () => {
    expect(describeError(new ApiError(401, 'session_invalid', '未登录'))?.hint).toContain('重新登录')
    expect(describeError(new ApiError(403, 'x', 'no'))?.hint).toContain('权限')
    expect(describeError(new ApiError(404, 'x', 'no'))?.hint).toContain('不存在')
    expect(describeError(new ApiError(409, 'x', 'no'))?.hint).toContain('刷新')
    expect(describeError(new ApiError(422, 'x', 'no'))?.hint).toContain('校验')
    expect(describeError(new ApiError(429, 'x', 'no'))?.hint).toContain('频繁')
  })

  it('优先展示 Retry-After', () => {
    const described = describeError(new ApiError(429, 'rate', '太频繁', '30'))
    expect(described?.hint).toContain('30 秒')
  })

  it('uncertain 透传', () => {
    expect(describeError(new ApiError(500, 'x', 'boom', undefined, true))?.uncertain).toBe(true)
    expect(describeError(new ApiError(500, 'x', 'boom'))?.uncertain).toBe(false)
  })

  it('AbortError 不展示', () => {
    expect(describeError(new DOMException('aborted', 'AbortError'))).toBeUndefined()
  })

  it('普通 Error 展示 message，未知类型展示兜底文案', () => {
    expect(describeError(new Error('表单校验失败'))?.message).toBe('表单校验失败')
    expect(describeError(42)?.message).toContain('操作失败')
    expect(describeError(undefined)).toBeUndefined()
  })

  it('不泄露非 Error 对象内部结构', () => {
    const described = describeError({ token: 'secret-value' })
    expect(described?.message).not.toContain('secret-value')
  })
})
