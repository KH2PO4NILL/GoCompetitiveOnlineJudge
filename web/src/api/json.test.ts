import { describe, expect, it } from 'vitest'
import { decode, encode } from './json'

describe('decode', () => {
  it('保留超出安全范围的 int64 精度', () => {
    const parsed = decode('{"submission_id":9223372036854775807,"total":9007199254740993}') as Record<string, unknown>
    expect(parsed.submission_id).toBe(9223372036854775807n)
    expect(parsed.total).toBe(9007199254740993n)
  })

  it('安全范围内的整数保持 number', () => {
    const parsed = decode('{"page":1,"total":42}') as Record<string, unknown>
    expect(parsed.page).toBe(1)
    expect(parsed.total).toBe(42)
  })

  it('小数不受影响', () => {
    const parsed = decode('{"ratio":0.5}') as Record<string, unknown>
    expect(parsed.ratio).toBe(0.5)
  })
})

describe('encode', () => {
  it('序列化 bigint 为精确整数字面量', () => {
    expect(encode({ problem_id: 9223372036854775807n })).toBe('{"problem_id":9223372036854775807}')
  })

  it('拒绝无法序列化的值', () => {
    expect(() => encode(undefined)).toThrow('无法序列化请求体')
  })
})
