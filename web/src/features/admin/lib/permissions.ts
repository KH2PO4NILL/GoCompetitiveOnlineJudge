import type { UserInfo } from '../../../api/contracts/admin'

/** 角色权限等级：仅当 actor 等级严格高于 target 时可管理。 */
export const roleRanks: Record<string, number> = { user: 1, admin: 2, super_admin: 3 }

export function roleRank(role: string | undefined): number {
  return roleRanks[role ?? ''] ?? 0
}

/**
 * 是否允许 actor 管理 target：
 * - actor 必须处于 normal 状态；
 * - target 未被删除；
 * - 不能操作自己；
 * - actor 角色等级严格高于 target。
 */
export function canManageUser(actor: UserInfo | undefined, target: UserInfo | undefined): boolean {
  if (!actor || !target) return false
  if (actor.status !== 'normal') return false
  if (target.status === 'deleted') return false
  if (String(actor.user_id) === String(target.user_id)) return false
  return roleRank(actor.role) > roleRank(target.role)
}
