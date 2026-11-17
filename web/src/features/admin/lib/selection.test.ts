import { describe, expect, it } from 'vitest'
import { isPageAllSelected, selectedIds, toggleMany, toggleOne } from './selection'

describe('toggleOne', () => {
  it('切换单个 id 且不修改原集合', () => {
    const base = new Set(['1'])
    const added = toggleOne(base, '2')
    expect([...added].sort()).toEqual(['1', '2'])
    expect([...base]).toEqual(['1'])
    const removed = toggleOne(added, '1')
    expect([...removed]).toEqual(['2'])
  })
})

describe('toggleMany', () => {
  it('批量勾选与取消', () => {
    const checked = toggleMany(new Set(), ['1', '2', '3'], true)
    expect(checked.size).toBe(3)
    const partial = toggleMany(checked, ['2'], false)
    expect([...partial].sort()).toEqual(['1', '3'])
  })
})

describe('isPageAllSelected', () => {
  it('空页返回 false，全选返回 true', () => {
    expect(isPageAllSelected(new Set(['1']), [])).toBe(false)
    expect(isPageAllSelected(new Set(['1', '2']), ['1', '2'])).toBe(true)
    expect(isPageAllSelected(new Set(['1']), ['1', '2'])).toBe(false)
  })
})

describe('selectedIds', () => {
  it('转换为 bigint 并过滤非法值', () => {
    const ids = selectedIds(new Set(['1', '9007199254740993', '0', 'abc', '-5']))
    expect(ids).toEqual([1n, 9007199254740993n])
  })
})
