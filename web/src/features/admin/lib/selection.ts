/**
 * 批量选择（跨页清空）纯逻辑。选择集合以 String(id) 为键，避免 bigint/number 混用。
 */
export function toggleOne(selected: ReadonlySet<string>, id: string): Set<string> {
  const next = new Set(selected)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  return next
}

export function toggleMany(selected: ReadonlySet<string>, ids: readonly string[], checked: boolean): Set<string> {
  const next = new Set(selected)
  for (const id of ids) {
    if (checked) next.add(id)
    else next.delete(id)
  }
  return next
}

/** 当前页是否已全部选中（页内非空且每个 id 都在集合中）。 */
export function isPageAllSelected(selected: ReadonlySet<string>, ids: readonly string[]): boolean {
  return ids.length > 0 && ids.every((id) => selected.has(id))
}

/** 把选择集合转换为请求用的 int64 数组（保持插入顺序，过滤非法值）。 */
export function selectedIds(selected: ReadonlySet<string>): bigint[] {
  const result: bigint[] = []
  for (const id of selected) {
    if (/^[1-9]\d{0,18}$/.test(id)) result.push(BigInt(id))
  }
  return result
}
