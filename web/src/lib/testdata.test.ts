import { describe, expect, it } from 'vitest'
import { prepareTestData } from './testdata'

const signal = () => new AbortController().signal
const file = (name: string, text = '') => new File([text], name)

function readStoredZIP(bytes: Uint8Array) {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const entries = new Map<string, Uint8Array>()
  const decoder = new TextDecoder()
  let offset = 0
  while (view.getUint32(offset, true) === 0x04034b50) {
    expect(view.getUint16(offset + 8, true)).toBe(0)
    const size = view.getUint32(offset + 18, true)
    const nameSize = view.getUint16(offset + 26, true)
    const name = decoder.decode(bytes.subarray(offset + 30, offset + 30 + nameSize))
    // Known CRC-32 vector for the fixture's 10.out payload.
    if (name === '10.out') expect(view.getUint32(offset + 14, true)).toBe(0xcbf43926)
    const start = offset + 30 + nameSize
    entries.set(name, bytes.slice(start, start + size))
    offset = start + size
  }
  const directoryOffset = offset
  for (const name of entries.keys()) {
    expect(view.getUint32(offset, true)).toBe(0x02014b50)
    const nameSize = view.getUint16(offset + 28, true)
    expect(decoder.decode(bytes.subarray(offset + 46, offset + 46 + nameSize))).toBe(name)
    expect(view.getUint32(view.getUint32(offset + 42, true), true)).toBe(0x04034b50)
    offset += 46 + nameSize
  }
  expect(view.getUint32(offset, true)).toBe(0x06054b50)
  expect(view.getUint16(offset + 10, true)).toBe(entries.size)
  expect(view.getUint32(offset + 16, true)).toBe(directoryOffset)
  return entries
}

describe('测试数据上传准备', () => {
  it('配对、自然排序并生成与实际字节匹配的清单，保留空文件和 UTF-8 文件名', async () => {
    const archive = await prepareTestData([
      file('10.out', '123456789'), file('2.in', '1 2\r\n'), file('10.in'),
      file('2.out', '3\n'), file('样例.in', '你好'), file('样例.out'),
    ], signal())
    const bytes = new Uint8Array(await archive.arrayBuffer())
    const entries = readStoredZIP(bytes)
    const manifest = JSON.parse(new TextDecoder().decode(entries.get('manifest.json')))
    expect(manifest.version).toBe(1)
    expect(manifest.test_cases.map((item: { name: string }) => item.name)).toEqual(['2', '10', '样例'])
    expect(manifest.test_cases.map((item: { score: number }) => item.score)).toEqual([1, 1, 1])
    for (const entry of manifest.files) {
      const actual = entries.get(entry.path)!
      expect(entry.size).toBe(actual.length)
      const hash = await crypto.subtle.digest('SHA-256', actual as Uint8Array<ArrayBuffer>)
      expect(entry.sha256).toBe(Array.from(new Uint8Array(hash), byte => byte.toString(16).padStart(2, '0')).join(''))
    }
  })

  it.each([
    [['1.in'], '缺少 .out'],
    [['1.out'], '缺少 .in'],
    [['1.in', '1.IN', '1.out'], '重复'],
    [['../1.in', '1.out'], '不合法'],
    [['1.zip', '1.in'], '不合法'],
    [['.in', '.out'], '不合法'],
  ])('拒绝无效选择 %j', async (names, message) => {
    await expect(prepareTestData(names.map(name => file(name)), signal())).rejects.toThrow(message)
  })

  it('原样传递单个 ZIP，拒绝空 ZIP', async () => {
    const archive = file('data.ZIP', 'archive')
    expect(await prepareTestData([archive], signal())).toBe(archive)
    await expect(prepareTestData([file('empty.zip')], signal())).rejects.toThrow('非空')
  })

  it('响应取消', async () => {
    const controller = new AbortController()
    controller.abort()
    await expect(prepareTestData([file('1.in'), file('1.out')], controller.signal)).rejects.toThrow()
  })
})
