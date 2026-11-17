import { describe, expect, it } from 'vitest'
import type { ContestDetail, ContestRegistration, IcpcProblemResult, MyContestStatus, ScoreboardRow } from '../../api/contracts/app'
import {
  canViewProblems, contestStatusTone, icpcAttemptsText, icpcCellText, icpcTotalPenaltyText,
  isRegistrationCancellable, participantTone, registrationClosedReason, registrationTone,
  scoreCellText, scoreText, submitBlockReason, totalScoreText,
} from './helpers'

function contestDetail(overrides: Partial<ContestDetail> = {}): ContestDetail {
  return {
    contest_id: 1,
    title: '测试比赛',
    description: '',
    mode: 'icpc',
    status: 'published',
    start_at: '2026-01-02T00:00:00Z',
    end_at: '2026-01-02T05:00:00Z',
    registration_policy: 'open',
    visibility: 'public',
    scoreboard_visibility: 'public',
    registration_open: true,
    scoreboard_frozen: false,
    participant_count: 0,
    ...overrides,
  }
}

function registration(overrides: Partial<ContestRegistration> = {}): ContestRegistration {
  return {
    registration_id: 10,
    contest_id: 1,
    user_id: 2,
    status: 'pending',
    application_note: '',
    reviewer_id: 0,
    review_note: '',
    revision: 1,
    ...overrides,
  }
}

function myStatus(overrides: Partial<MyContestStatus> = {}): MyContestStatus {
  return { contest_id: 1, can_view_problems: false, can_submit: false, ...overrides }
}

function icpcResult(overrides: Partial<IcpcProblemResult> = {}): IcpcProblemResult {
  return { solved: false, wrong_attempts: 0, penalty_seconds: 0, first_to_solve: false, ...overrides }
}

const T0 = Date.parse('2026-01-01T12:00:00Z')

describe('contestStatusTone', () => {
  it('覆盖全部比赛状态', () => {
    expect(contestStatusTone('running')).toBe('accent')
    expect(contestStatusTone('published')).toBe('success')
    expect(contestStatusTone('finished')).toBe('muted')
    expect(contestStatusTone('cancelled')).toBe('danger')
    expect(contestStatusTone('draft')).toBe('warning')
    expect(contestStatusTone('archived')).toBe('muted')
  })

  it('未知状态回退中性色', () => {
    expect(contestStatusTone('weird')).toBe('neutral')
  })
})

describe('registrationTone / participantTone', () => {
  it('报名与参赛状态色调', () => {
    expect(registrationTone('pending')).toBe('warning')
    expect(registrationTone('approved')).toBe('success')
    expect(registrationTone('rejected')).toBe('danger')
    expect(registrationTone('cancelled')).toBe('muted')
    expect(participantTone('normal')).toBe('success')
    expect(participantTone('disqualified')).toBe('danger')
    expect(participantTone('withdrawn')).toBe('muted')
    expect(participantTone('unknown')).toBe('neutral')
  })
})

describe('canViewProblems', () => {
  it('登录用户以服务端 can_view_problems 为准', () => {
    expect(canViewProblems(true, myStatus({ can_view_problems: true }), 'running')).toBe(true)
    expect(canViewProblems(true, myStatus({ can_view_problems: false }), 'running')).toBe(false)
  })

  it('登录但状态未加载完时视为不可见（未知）', () => {
    expect(canViewProblems(true, undefined, 'running')).toBe(false)
  })

  it('未登录仅比赛已结束时可见', () => {
    expect(canViewProblems(false, undefined, 'finished')).toBe(true)
    expect(canViewProblems(false, undefined, 'running')).toBe(false)
    expect(canViewProblems(false, undefined, 'published')).toBe(false)
    expect(canViewProblems(false, undefined, undefined)).toBe(false)
  })
})

describe('registrationClosedReason', () => {
  it('按比赛状态优先给出原因', () => {
    expect(registrationClosedReason(contestDetail({ status: 'draft' }), T0)).toContain('尚未发布')
    expect(registrationClosedReason(contestDetail({ status: 'cancelled' }), T0)).toContain('已取消')
    expect(registrationClosedReason(contestDetail({ status: 'finished' }), T0)).toContain('已结束')
    expect(registrationClosedReason(contestDetail({ status: 'archived' }), T0)).toContain('已归档')
  })

  it('报名窗口边界：截止之后 / 开始之前', () => {
    const detail = contestDetail({
      status: 'published',
      registration_start_at: '2025-12-30T00:00:00Z',
      registration_end_at: '2025-12-31T00:00:00Z',
    })
    expect(registrationClosedReason(detail, T0)).toContain('已截止')
    expect(registrationClosedReason(detail, Date.parse('2025-12-29T00:00:00Z'))).toContain('尚未开始')
    // 恰好在截止时间点（nowMs === end）不算截止之后
    expect(registrationClosedReason(detail, Date.parse('2025-12-31T00:00:00Z'))).not.toContain('已截止')
  })

  it('进行中且无窗口信息时给出通用文案', () => {
    const detail = contestDetail({ status: 'running', registration_start_at: undefined, registration_end_at: undefined })
    expect(registrationClosedReason(detail, T0)).toContain('进行中')
    const published = contestDetail({ status: 'published', registration_start_at: undefined, registration_end_at: undefined })
    expect(registrationClosedReason(published, T0)).toContain('当前不开放报名')
  })

  it('非法时间字符串不参与判断', () => {
    const detail = contestDetail({ status: 'published', registration_start_at: 'not-a-date', registration_end_at: 'not-a-date' })
    expect(registrationClosedReason(detail, T0)).toBe('当前不开放报名。')
  })
})

describe('isRegistrationCancellable', () => {
  it('仅报名开放且 pending/approved 时可取消', () => {
    expect(isRegistrationCancellable(registration({ status: 'pending' }), true)).toBe(true)
    expect(isRegistrationCancellable(registration({ status: 'approved' }), true)).toBe(true)
    expect(isRegistrationCancellable(registration({ status: 'rejected' }), true)).toBe(false)
    expect(isRegistrationCancellable(registration({ status: 'cancelled' }), true)).toBe(false)
  })

  it('报名关闭或缺少 registration 时不可取消', () => {
    expect(isRegistrationCancellable(registration({ status: 'approved' }), false)).toBe(false)
    expect(isRegistrationCancellable(undefined, true)).toBe(false)
  })
})

describe('submitBlockReason', () => {
  it('可提交时返回空串', () => {
    expect(submitBlockReason(myStatus({ can_submit: true }), 'running')).toBe('')
  })

  it('比赛状态优先于报名状态', () => {
    expect(submitBlockReason(myStatus(), 'published')).toContain('尚未开始')
    expect(submitBlockReason(myStatus(), 'finished')).toContain('已结束')
    expect(submitBlockReason(myStatus(), 'cancelled')).toContain('已取消')
    expect(submitBlockReason(myStatus(), 'archived')).toContain('已归档')
    expect(submitBlockReason(myStatus(), 'draft')).toContain('尚未发布')
  })

  it('进行中按报名/参赛状态解释', () => {
    expect(submitBlockReason(undefined, 'running')).toContain('正在确认')
    expect(submitBlockReason(myStatus(), 'running')).toContain('尚未报名')
    expect(submitBlockReason(myStatus({ registration: registration({ status: 'cancelled' }) }), 'running')).toContain('尚未报名')
    expect(submitBlockReason(myStatus({ registration: registration({ status: 'pending' }) }), 'running')).toContain('审核中')
    expect(submitBlockReason(myStatus({ registration: registration({ status: 'rejected' }) }), 'running')).toContain('未通过审核')
    expect(submitBlockReason(myStatus({
      registration: registration({ status: 'approved' }),
      participant: { contest_id: 1, user_id: 2, status: 'disqualified' },
    }), 'running')).toContain('资格已被取消')
    expect(submitBlockReason(myStatus({
      registration: registration({ status: 'approved' }),
      participant: { contest_id: 1, user_id: 2, status: 'withdrawn' },
    }), 'running')).toContain('已退出')
  })
})

describe('icpcAttemptsText', () => {
  it('0 与缺失返回空串，正数返回 -n', () => {
    expect(icpcAttemptsText(0)).toBe('')
    expect(icpcAttemptsText(undefined)).toBe('')
    expect(icpcAttemptsText(1)).toBe('-1')
    expect(icpcAttemptsText(42)).toBe('-42')
  })

  it('非法值不产生文本', () => {
    expect(icpcAttemptsText(-3)).toBe('')
    expect(icpcAttemptsText(Number.NaN)).toBe('')
  })
})

describe('icpcCellText', () => {
  it('通过且零罚时：仅 ✓ 与 0:00', () => {
    expect(icpcCellText(icpcResult({ solved: true, penalty_seconds: 0 }))).toBe('✓ 0:00')
  })

  it('通过且有错误尝试：-n ✓ 罚时', () => {
    expect(icpcCellText(icpcResult({ solved: true, wrong_attempts: 2, penalty_seconds: 3661 }))).toBe('-2 ✓ 1:01:01')
  })

  it('未通过仅展示错误尝试', () => {
    expect(icpcCellText(icpcResult({ wrong_attempts: 3 }))).toBe('-3')
    expect(icpcCellText(icpcResult())).toBe('—')
    expect(icpcCellText(undefined)).toBe('—')
  })

  it('bigint 罚时保持精度', () => {
    expect(icpcCellText(icpcResult({ solved: true, penalty_seconds: 7325n }))).toBe('✓ 2:02:05')
  })
})

describe('icpcTotalPenaltyText', () => {
  it('格式化为 h:mm:ss / m:ss', () => {
    expect(icpcTotalPenaltyText(0)).toBe('0:00')
    expect(icpcTotalPenaltyText(59)).toBe('0:59')
    expect(icpcTotalPenaltyText(600)).toBe('10:00')
    expect(icpcTotalPenaltyText(36000)).toBe('10:00:00')
    expect(icpcTotalPenaltyText(90061n)).toBe('25:01:01')
  })

  it('缺失返回 —', () => {
    expect(icpcTotalPenaltyText(undefined)).toBe('—')
  })
})

describe('scoreText / scoreCellText / totalScoreText', () => {
  it('IOI/OI 单题得分', () => {
    expect(scoreText(30, 100)).toBe('30 / 100')
    expect(scoreText(9007199254740993n, 9007199254740993n)).toBe('9007199254740993 / 9007199254740993')
    expect(scoreText(undefined, 100)).toBe('—')
    expect(scoreText(30, undefined)).toBe('30')
  })

  it('scoreCellText 按赛制取结果', () => {
    expect(scoreCellText(undefined)).toBe('—')
    expect(scoreCellText({ problem_id: 1, label: 'A' })).toBe('—')
    expect(scoreCellText({ problem_id: 1, label: 'A', ioi: { score: 40, max_score: 100, submission_count: 2 } })).toBe('40 / 100')
    expect(scoreCellText({ problem_id: 1, label: 'A', oi: { score: 60n, max_score: 100n, submission_count: 1 } })).toBe('60 / 100')
  })

  it('totalScoreText 保持 int64 精度', () => {
    const base = { ordinal: 1, user_id: 2, display_name: 'u', problem_results: [], rank: 1 } satisfies Omit<ScoreboardRow, 'rank' | 'ordinal'> & { rank: number; ordinal: number }
    expect(totalScoreText({ ...base, ioi: { total_score: 240 } })).toBe('240')
    expect(totalScoreText({ ...base, oi: { total_score: 9007199254740993n } })).toBe('9007199254740993')
    expect(totalScoreText({ ...base })).toBe('—')
  })
})
