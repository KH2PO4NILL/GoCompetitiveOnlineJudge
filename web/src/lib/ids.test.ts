import { describe, expect, it } from 'vitest'
import { hasNextPage, isValidId, pageNumber, parseIdParam, requireId } from './ids'
import { isPending, resultLabel, verdictTone } from './judge'
import type { SubmissionSummary } from '../api/contracts/app'

describe('isValidId', () => {
  it('接受正 int64', () => {
    expect(isValidId('1')).toBe(true)
    expect(isValidId('9223372036854775807')).toBe(true)
  })
  it('拒绝非法值', () => {
    expect(isValidId('0')).toBe(false)
    expect(isValidId('-1')).toBe(false)
    expect(isValidId('9223372036854775808')).toBe(false)
    expect(isValidId('1.5')).toBe(false)
    expect(isValidId('1e3')).toBe(false)
    expect(isValidId('abc')).toBe(false)
    expect(isValidId('')).toBe(false)
  })
})

describe('parseIdParam', () => {
  it('安全整数返回 number，超出返回 bigint', () => {
    expect(parseIdParam('42')).toBe(42)
    expect(parseIdParam('9223372036854775807')).toBe(9223372036854775807n)
    expect(parseIdParam(['7', '8'])).toBe(7)
    expect(parseIdParam('bad')).toBeUndefined()
  })
})

describe('requireId', () => {
  it('非法 ID 抛 uncertain 契约错误', () => {
    expect(() => requireId(1.5)).toThrow(/响应无效/)
    expect(() => requireId('7')).toThrow()
    expect(() => requireId(0)).toThrow()
    expect(requireId(7)).toBe(7)
    expect(requireId(9223372036854775807n)).toBe(9223372036854775807n)
  })
})

describe('pageNumber', () => {
  it('非法输入回落为 1', () => {
    expect(pageNumber(null)).toBe(1)
    expect(pageNumber('abc')).toBe(1)
    expect(pageNumber('0')).toBe(1)
    expect(pageNumber('3')).toBe(3)
    expect(pageNumber('99999999999999')).toBe(1)
  })
})

describe('hasNextPage', () => {
  it('使用 bigint 比较避免精度问题', () => {
    expect(hasNextPage(41n, 2, 20)).toBe(true)
    expect(hasNextPage(40n, 2, 20)).toBe(false)
    expect(hasNextPage(9007199254740993n, 1, 20)).toBe(true)
  })
})

function submission(overrides: Partial<SubmissionSummary> = {}): SubmissionSummary {
  return {
    submission_id: 1, user_id: 1, problem_id: 1, language: 'cpp20-v3',
    status: 'finished', verdict: 'accepted', score: 100n, max_score: 100n,
    problem_revision: 1n, attempt_count: 1, result_visibility: 'full', ...overrides,
  }
}

describe('judge helpers', () => {
  it('isPending 识别 queued/judging', () => {
    expect(isPending(submission({ status: 'queued' }))).toBe(true)
    expect(isPending(submission({ status: 'judging' }))).toBe(true)
    expect(isPending(submission())).toBe(false)
  })

  it('resultLabel 仅在终态且 full 可见时显示 verdict', () => {
    expect(resultLabel(submission())).toBe('Accepted')
    expect(resultLabel(submission({ status: 'judging' }))).toBe('正在评测')
    expect(resultLabel(submission({ result_visibility: 'hidden' }))).toBe('评测完成')
    expect(resultLabel(submission({ verdict: 'unspecified' }))).toBe('结果未提供')
    expect(resultLabel(submission({ verdict: 'weird' }))).toBe('weird')
  })

  it('verdictTone：绿=通过 红=错误 灰=等待 蓝=运行', () => {
    expect(verdictTone(submission())).toBe('success')
    expect(verdictTone(submission({ verdict: 'wrong_answer' }))).toBe('danger')
    expect(verdictTone(submission({ verdict: 'partial_accepted' }))).toBe('warning')
    expect(verdictTone(submission({ status: 'queued' }))).toBe('neutral')
    expect(verdictTone(submission({ status: 'judging' }))).toBe('accent')
    expect(verdictTone(submission({ result_visibility: 'hidden', verdict: 'accepted' }))).toBe('muted')
    expect(verdictTone(submission({ verdict: 'system_error' }))).toBe('muted')
  })
})
