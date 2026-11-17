import { describe, expect, it } from 'vitest'
import type { UserInfo } from '../../../api/contracts/admin'
import { canManageUser, roleRank } from './permissions'

function user(overrides: Partial<UserInfo>): UserInfo {
  return { user_id: 1, username: 'u', email: 'e', role: 'user', status: 'normal', school_id: 0, ...overrides }
}

describe('roleRank', () => {
  it('按等级排序且未知角色为 0', () => {
    expect(roleRank('user')).toBe(1)
    expect(roleRank('admin')).toBe(2)
    expect(roleRank('super_admin')).toBe(3)
    expect(roleRank('ghost')).toBe(0)
    expect(roleRank(undefined)).toBe(0)
  })
})

describe('canManageUser', () => {
  it('super_admin 可管理 admin 与 user', () => {
    const actor = user({ user_id: 9, role: 'super_admin' })
    expect(canManageUser(actor, user({ user_id: 1, role: 'admin' }))).toBe(true)
    expect(canManageUser(actor, user({ user_id: 1, role: 'user' }))).toBe(true)
  })

  it('admin 不能管理 admin 或 super_admin', () => {
    const actor = user({ user_id: 9, role: 'admin' })
    expect(canManageUser(actor, user({ user_id: 1, role: 'admin' }))).toBe(false)
    expect(canManageUser(actor, user({ user_id: 1, role: 'super_admin' }))).toBe(false)
    expect(canManageUser(actor, user({ user_id: 1, role: 'user' }))).toBe(true)
  })

  it('不能操作自己', () => {
    const actor = user({ user_id: 9, role: 'super_admin' })
    expect(canManageUser(actor, user({ user_id: 9, role: 'user' }))).toBe(false)
  })

  it('actor 非 normal 时禁止', () => {
    const actor = user({ user_id: 9, role: 'super_admin', status: 'disabled' })
    expect(canManageUser(actor, user({ user_id: 1, role: 'user' }))).toBe(false)
  })

  it('target 已删除时禁止', () => {
    const actor = user({ user_id: 9, role: 'super_admin' })
    expect(canManageUser(actor, user({ user_id: 1, role: 'user', status: 'deleted' }))).toBe(false)
  })

  it('缺失 actor 或 target 时禁止', () => {
    expect(canManageUser(undefined, user({}))).toBe(false)
    expect(canManageUser(user({ role: 'super_admin' }), undefined)).toBe(false)
  })

  it('bigint 与 number 形式的 user_id 也能正确判等', () => {
    const actor = user({ user_id: 9007199254740993n, role: 'super_admin' })
    expect(canManageUser(actor, user({ user_id: 1, role: 'user' }))).toBe(true)
    const self = user({ user_id: 9007199254740993n, role: 'super_admin' })
    expect(canManageUser(actor, self)).toBe(false)
  })
})
