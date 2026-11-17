import type { ContestDetail, ContestRegistration, IcpcProblemResult, Int64, MyContestStatus, ScoreboardProblemResult, ScoreboardRow } from '../../api/contracts/app'
import { formatDuration } from '../../lib/time'

export type BadgeTone = 'neutral' | 'success' | 'danger' | 'warning' | 'accent' | 'muted'

/** 比赛状态 → 徽章色调（文字标签始终展示，不只靠颜色） */
export function contestStatusTone(status: string): BadgeTone {
  switch (status) {
    case 'running': return 'accent'
    case 'published': return 'success'
    case 'finished': return 'muted'
    case 'cancelled': return 'danger'
    case 'draft': return 'warning'
    case 'archived': return 'muted'
    default: return 'neutral'
  }
}

/** 报名状态 → 徽章色调 */
export function registrationTone(status: string): BadgeTone {
  switch (status) {
    case 'pending': return 'warning'
    case 'approved': return 'success'
    case 'rejected': return 'danger'
    case 'cancelled': return 'muted'
    default: return 'neutral'
  }
}

/** 参赛者状态 → 徽章色调 */
export function participantTone(status: string): BadgeTone {
  switch (status) {
    case 'normal': return 'success'
    case 'disqualified': return 'danger'
    case 'withdrawn': return 'muted'
    default: return 'neutral'
  }
}

/**
 * 题目可见性：登录用户以服务端 can_view_problems 为准；
 * 未登录仅在比赛已结束时可见（与后端公开策略一致），其余情况无法确认 → 不可见。
 */
export function canViewProblems(loggedIn: boolean, myStatus: MyContestStatus | undefined, contestStatus: string | undefined): boolean {
  if (loggedIn) return myStatus?.can_view_problems ?? false
  return contestStatus === 'finished'
}

/** registration_open=false 时给出可解释的原因（纯函数，nowMs 便于测试） */
export function registrationClosedReason(detail: ContestDetail, nowMs: number): string {
  switch (detail.status) {
    case 'draft': return '比赛尚未发布，暂不开放报名。'
    case 'cancelled': return '比赛已取消，不再开放报名。'
    case 'finished': return '比赛已结束，报名通道已关闭。'
    case 'archived': return '比赛已归档，报名通道已关闭。'
    default: break
  }
  const end = detail.registration_end_at ? Date.parse(detail.registration_end_at) : Number.NaN
  if (Number.isFinite(end) && nowMs > end) return '报名已截止。'
  const start = detail.registration_start_at ? Date.parse(detail.registration_start_at) : Number.NaN
  if (Number.isFinite(start) && nowMs < start) return '报名尚未开始，请留意报名窗口。'
  if (detail.status === 'running') return '比赛进行中，报名通道已关闭。'
  return '当前不开放报名。'
}

/** 报名是否仍可取消：报名开放且状态为 pending/approved（携带 revision 乐观锁） */
export function isRegistrationCancellable(registration: ContestRegistration | undefined, registrationOpen: boolean): boolean {
  if (!registration || !registrationOpen) return false
  return registration.status === 'pending' || registration.status === 'approved'
}

/** 无法提交时的原因说明；可提交时返回空串 */
export function submitBlockReason(myStatus: MyContestStatus | undefined, contestStatus: string | undefined): string {
  if (myStatus?.can_submit) return ''
  switch (contestStatus) {
    case 'draft': return '比赛尚未发布，无法提交。'
    case 'published': return '比赛尚未开始，开始后方可提交。'
    case 'finished': return '比赛已结束，无法继续提交。'
    case 'cancelled': return '比赛已取消，无法提交。'
    case 'archived': return '比赛已归档，无法继续提交。'
    default: break
  }
  if (!myStatus) return '正在确认参赛状态…'
  const registration = myStatus.registration
  if (!registration || registration.status === 'cancelled') return '你尚未报名本场比赛，请先在概览页完成报名。'
  if (registration.status === 'pending') return '报名审核中，通过后方可提交。'
  if (registration.status === 'rejected') return '报名未通过审核，无法提交。'
  const participant = myStatus.participant
  if (participant?.status === 'disqualified') return '参赛资格已被取消，无法提交。'
  if (participant?.status === 'withdrawn') return '你已退出本场比赛，无法提交。'
  return '当前无法提交。'
}

/** ICPC 错误尝试 → "-n" 文本；0 次返回空串 */
export function icpcAttemptsText(wrongAttempts: number | undefined): string {
  if (wrongAttempts === undefined || !Number.isFinite(wrongAttempts) || wrongAttempts <= 0) return ''
  return `-${Math.floor(wrongAttempts)}`
}

/** ICPC 单元格文本：通过 ✓ + 罚时（h:mm:ss / m:ss），未通过展示 -n 或 — */
export function icpcCellText(result: IcpcProblemResult | undefined): string {
  if (!result) return '—'
  const attempts = icpcAttemptsText(result.wrong_attempts)
  if (result.solved) return [attempts, '✓', formatDuration(result.penalty_seconds)].filter(Boolean).join(' ')
  return attempts || '—'
}

/** ICPC 总罚时（h:mm:ss）；缺失返回 — */
export function icpcTotalPenaltyText(seconds: Int64 | undefined): string {
  if (seconds === undefined || seconds === null) return '—'
  return formatDuration(seconds)
}

/** IOI/OI 得分文本：score / max_score；缺失返回 — */
export function scoreText(score: Int64 | undefined, maxScore: Int64 | undefined): string {
  if (score === undefined || score === null) return '—'
  if (maxScore === undefined || maxScore === null) return `${String(score)}`
  return `${String(score)} / ${String(maxScore)}`
}

/** IOI/OI 单题单元格文本；缺失对应赛制结果返回 — */
export function scoreCellText(result: ScoreboardProblemResult | undefined): string {
  if (!result) return '—'
  if (result.ioi) return scoreText(result.ioi.score, result.ioi.max_score)
  if (result.oi) return scoreText(result.oi.score, result.oi.max_score)
  return '—'
}

/** IOI/OI 行总分文本（保持 int64 精度，不做 Number 转换） */
export function totalScoreText(row: ScoreboardRow): string {
  if (row.ioi) return String(row.ioi.total_score)
  if (row.oi) return String(row.oi.total_score)
  return '—'
}
