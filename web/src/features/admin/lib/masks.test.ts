import { describe, expect, it } from 'vitest'
import type { Contest, ContestEditableFields, Problem, ProblemEditableFields } from '../../../api/contracts/admin'
import { contestDiff, problemDiff, samplesEqual, stringArrayEqual } from './masks'

function editable(overrides: Partial<ProblemEditableFields> = {}): ProblemEditableFields {
  return {
    statement: {
      title: 'A + B',
      description: 'desc',
      input_description: 'in',
      output_description: 'out',
      hint: '',
      samples: [{ input: '1 2', output: '3', explanation: '' }],
    },
    difficulty: 'easy',
    tags: ['math'],
    source: 'src',
    visibility: 'private',
    judge_settings: { time_limit_ms: 1000, memory_limit_mb: 256, allowed_languages: ['cpp20-v3'], checker_type: 'standard' },
    ...overrides,
  }
}

function serverProblem(overrides: Partial<ProblemEditableFields> = {}): Problem {
  const base = editable(overrides)
  return {
    problem_id: 1,
    statement: base.statement,
    metadata: { difficulty: base.difficulty, tags: base.tags, source: base.source, author_id: 7 },
    judge_config: { settings: base.judge_settings, test_data_version: 'v1', checker_file_key: '', interactor_file_key: '' },
    status: 'draft',
    visibility: base.visibility,
    revision: 3,
  }
}

describe('samplesEqual / stringArrayEqual', () => {
  it('比较样例与字符串数组', () => {
    expect(samplesEqual([{ input: 'a', output: 'b', explanation: '' }], [{ input: 'a', output: 'b', explanation: '' }])).toBe(true)
    expect(samplesEqual([{ input: 'a', output: 'b', explanation: '' }], [{ input: 'a', output: 'c', explanation: '' }])).toBe(false)
    expect(samplesEqual([], [{ input: 'a', output: 'b', explanation: '' }])).toBe(false)
    expect(stringArrayEqual(['x', 'y'], ['x', 'y'])).toBe(true)
    expect(stringArrayEqual(['x', 'y'], ['y', 'x'])).toBe(false)
  })
})

describe('problemDiff', () => {
  it('无改动时 update_mask 为空', () => {
    const { patch, updateMask } = problemDiff(editable(), serverProblem())
    expect(updateMask).toEqual([])
    expect(patch).toEqual({})
  })

  it('仅发送改动过的顶层字段', () => {
    const next = editable({ difficulty: 'hard', visibility: 'public' })
    const { patch, updateMask } = problemDiff(next, serverProblem())
    expect(updateMask).toEqual(['difficulty', 'visibility'])
    expect(patch.difficulty).toBe('hard')
    expect(patch.visibility).toBe('public')
    expect(patch.statement).toBeUndefined()
  })

  it('样例变化触发 statement', () => {
    const next = editable()
    next.statement = { ...next.statement, samples: [{ input: '1 2', output: '4', explanation: '' }] }
    const { updateMask } = problemDiff(next, serverProblem())
    expect(updateMask).toEqual(['statement'])
  })

  it('judge_settings 任一子字段变化即整体下发', () => {
    const next = editable()
    next.judge_settings = { ...next.judge_settings, time_limit_ms: 2000 }
    const { patch, updateMask } = problemDiff(next, serverProblem())
    expect(updateMask).toEqual(['judge_settings'])
    expect(patch.judge_settings?.time_limit_ms).toBe(2000)
  })

  it('标签顺序变化视为改动', () => {
    const next = editable({ tags: ['math', 'dp'] })
    const { updateMask } = problemDiff(next, serverProblem())
    expect(updateMask).toContain('tags')
  })
})

function contestEditable(overrides: Partial<ContestEditableFields> = {}): ContestEditableFields {
  return {
    title: 'Contest',
    description: 'desc',
    mode: 'icpc',
    start_at: '2026-01-01T00:00:00.000Z',
    end_at: '2026-01-01T05:00:00.000Z',
    registration_policy: 'open',
    visibility: 'public',
    scoreboard_visibility: 'public',
    ...overrides,
  }
}

function serverContest(overrides: Partial<ContestEditableFields> = {}): Contest {
  return {
    contest_id: 5,
    creator_id: 7,
    editable: contestEditable(overrides),
    status: 'draft',
    revision: 2,
    scoreboard_frozen: false,
  }
}

describe('contestDiff', () => {
  it('无改动时 update_mask 为空', () => {
    const { patch, updateMask } = contestDiff(contestEditable(), serverContest())
    expect(updateMask).toEqual([])
    expect(patch).toEqual({})
  })

  it('标题与赛制改动被记录', () => {
    const next = contestEditable({ title: 'New', mode: 'ioi' })
    const { patch, updateMask } = contestDiff(next, serverContest())
    expect(updateMask).toEqual(['title', 'mode'])
    expect(patch.title).toBe('New')
    expect(patch.mode).toBe('ioi')
  })

  it('新增可选邀请码被记录', () => {
    const next = contestEditable({ invite_code: 'secret' })
    const { updateMask } = contestDiff(next, serverContest())
    expect(updateMask).toContain('invite_code')
  })

  it('清空可选报名时间只下发 mask，不下发空串', () => {
    const server = serverContest({ registration_end_at: '2026-01-01T00:00:00.000Z' })
    const next = contestEditable({ registration_end_at: '' })
    const { patch, updateMask } = contestDiff(next, server)
    expect(updateMask).toContain('registration_end_at')
    expect(patch.registration_end_at).toBeUndefined()
  })
})
