import { isInteger, parse, stringify } from 'lossless-json'

export function decode(text: string): unknown {
  return parse(text, undefined, { parseNumber: (value) => {
    const number = Number(value)
    return isInteger(value) && !Number.isSafeInteger(number) ? BigInt(value) : number
  } })
}

export function encode(value: unknown): string {
  const result = stringify(value)
  if (result === undefined) throw new Error('无法序列化请求')
  return result
}
