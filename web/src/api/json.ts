import { isInteger, parse, stringify } from 'lossless-json'

// int64 ID 精确性：超出 Number.MAX_SAFE_INTEGER 的整数解析为 bigint，禁止经过不安全的 number 转换。
export function decode(text: string): unknown {
  return parse(text, undefined, {
    parseNumber: (value) => {
      const number = Number(value)
      return isInteger(value) && !Number.isSafeInteger(number) ? BigInt(value) : number
    },
  })
}

export function encode(value: unknown): string {
  const result = stringify(value)
  if (result === undefined) throw new Error('无法序列化请求体')
  return result
}
