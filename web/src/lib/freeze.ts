import type { ContestEditableFields, ContestPatch } from '../api/contracts/admin'

export type FreezePolicyValues = {
  freeze_policy?: string
  freeze_at?: string
  freeze_seconds?: string
  mode?: string
  start_at?: string
  end_at?: string
}

/**
 * 自动封榜策略 → 比赛补丁字段。
 * policy=none 清除既有封榜配置；at/before_end 仅适用于 ICPC。
 */
export function freezePolicy(values: FreezePolicyValues, current?: ContestEditableFields): { fields: ContestPatch; updateMask: string[] } {
  const policy = values.freeze_policy ?? 'none'
  if (policy === 'none') {
    const updateMask = current?.scoreboard_freeze_at
      ? ['scoreboard_freeze_at']
      : current?.scoreboard_freeze_before_end_seconds != null ? ['scoreboard_freeze_before_end_seconds'] : []
    return { fields: {}, updateMask }
  }
  if ((values.mode ?? current?.mode) !== 'icpc') throw new Error('自动封榜仅适用于 ICPC 比赛。')
  const start = Date.parse(values.start_at || current?.start_at || '')
  const end = Date.parse(values.end_at || current?.end_at || '')
  if (!Number.isFinite(start) || !Number.isFinite(end) || start >= end) throw new Error('请先填写有效的比赛起止时间。')
  if (policy === 'at') {
    const freeze = Date.parse(values.freeze_at ?? '')
    if (!Number.isFinite(freeze) || freeze <= start || freeze >= end) throw new Error('封榜时间必须晚于比赛开始且早于比赛结束。')
    return { fields: { scoreboard_freeze_at: new Date(freeze).toISOString() }, updateMask: ['scoreboard_freeze_at'] }
  }
  if (policy === 'before_end') {
    const seconds = values.freeze_seconds ?? ''
    if (!/^[1-9]\d{0,9}$/.test(seconds) || BigInt(seconds) > 9223372036n || BigInt(seconds) * 1000n >= BigInt(end - start)) {
      throw new Error('提前封榜秒数必须为正整数，且必须小于比赛时长。')
    }
    return { fields: { scoreboard_freeze_before_end_seconds: BigInt(seconds) }, updateMask: ['scoreboard_freeze_before_end_seconds'] }
  }
  throw new Error('无效的自动封榜方式。')
}
