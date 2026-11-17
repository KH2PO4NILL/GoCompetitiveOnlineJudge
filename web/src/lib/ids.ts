import { ApiError } from '../api/client'
import type { Int64 } from '../api/contracts/app'

const MAX_INT64 = 9223372036854775807n

/** 校验字符串是否为合法的正 int64 ID */
export function isValidId(value: string): boolean {
  return /^[1-9]\d{0,18}$/.test(value) && BigInt(value) <= MAX_INT64
}

/** 从路由参数解析 int64 ID，非法时返回 undefined */
export function parseIdParam(value: string | string[]): Int64 | undefined {
  const text = Array.isArray(value) ? value[0] ?? '' : value ?? ''
  if (!isValidId(text)) return undefined
  return Number.isSafeInteger(Number(text)) ? Number(text) : BigInt(text)
}

/** 校验服务端响应中的 ID：非法即视为契约破坏，标记 uncertain 防止盲目重试 */
export function requireId(value: unknown, label = '编号'): Int64 {
  const ok = (typeof value === 'number' && Number.isSafeInteger(value) && value > 0)
    || (typeof value === 'bigint' && value > 0n && value <= MAX_INT64)
  if (!ok) throw new ApiError(502, 'invalid_response', `${label}响应无效，请先重新查询服务端结果`, undefined, true)
  return value as Int64
}

export function pageNumber(value: string | null): number {
  if (!value || !/^\d+$/.test(value)) return 1
  const page = Number(value)
  return Number.isSafeInteger(page) && page > 0 && page <= 2147483647 ? page : 1
}

export function hasNextPage(total: Int64, page: number, pageSize: number): boolean {
  return BigInt(total) > BigInt(page) * BigInt(pageSize)
}

export function readQuery(value: unknown): string {
  if (Array.isArray(value)) return value[0] ?? ''
  return value == null ? '' : String(value)
}
