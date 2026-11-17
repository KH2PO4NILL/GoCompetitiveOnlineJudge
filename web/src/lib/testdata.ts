import { maxArchiveBytes, validateArchive } from './upload'

const encoder = new TextEncoder()

/** Store-only ZIP: avoids compression-ratio rejection for repetitive judge data. */
function zip(entries: { name: string, bytes: Uint8Array<ArrayBuffer> }[]): File {
  const parts: BlobPart[] = []
  const directory: Uint8Array<ArrayBuffer>[] = []
  let offset = 0
  for (const entry of entries) {
    const name = encoder.encode(entry.name)
    let crc = 0xffffffff
    for (const byte of entry.bytes) {
      crc ^= byte
      for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ ((crc & 1) ? 0xedb88320 : 0)
    }
    crc = (crc ^ 0xffffffff) >>> 0
    const local = new Uint8Array(30 + name.length)
    const view = new DataView(local.buffer)
    view.setUint32(0, 0x04034b50, true)
    view.setUint16(4, 20, true)
    view.setUint16(6, 0x800, true) // UTF-8 filenames
    view.setUint16(12, 33, true) // 1980-01-01
    view.setUint32(14, crc, true)
    view.setUint32(18, entry.bytes.length, true)
    view.setUint32(22, entry.bytes.length, true)
    view.setUint16(26, name.length, true)
    local.set(name, 30)
    const central = new Uint8Array(46 + name.length)
    const record = new DataView(central.buffer)
    record.setUint32(0, 0x02014b50, true)
    record.setUint16(4, 20, true)
    central.set(local.subarray(4, 30), 6)
    record.setUint32(42, offset, true)
    central.set(name, 46)
    parts.push(local, entry.bytes)
    directory.push(central)
    offset += local.length + entry.bytes.length
  }
  const end = new Uint8Array(22)
  const footer = new DataView(end.buffer)
  footer.setUint32(0, 0x06054b50, true)
  footer.setUint16(8, entries.length, true)
  footer.setUint16(10, entries.length, true)
  footer.setUint32(12, directory.reduce((sum, item) => sum + item.length, 0), true)
  footer.setUint32(16, offset, true)
  return new File([...parts, ...directory, end], 'test-data.zip', { type: 'application/zip' })
}

export async function prepareTestData(files: File[], signal: AbortSignal): Promise<File> {
  signal.throwIfAborted()
  if (files.length === 1 && /\.zip$/i.test(files[0]!.name)) {
    validateArchive(files[0]!)
    return files[0]!
  }
  if (!files.length) throw new Error('请选择 ZIP 或成对的 .in / .out 文件')
  if (files.length > 19998) throw new Error('一次最多上传 9999 对测试文件')
  if (files.reduce((sum, file) => sum + file.size, 0) > maxArchiveBytes) {
    throw new Error('测试文件总大小不能超过 128 MiB')
  }
  const pairs = new Map<string, { input?: File, output?: File }>()
  for (const file of files) {
    const match = /^(.*)\.(in|out)$/i.exec(file.name)
    if (!match || !match[1]?.trim() || /[/\\:\x00]/.test(file.name)
      || [...match[1]].length > 128 || encoder.encode(file.name).length > 1024) {
      throw new Error(`文件名不合法：${file.name}；请选择同名的 .in / .out 文件，或单个 ZIP`)
    }
    const key = match[1]
    const pair = pairs.get(key) ?? {}
    const side = match[2]!.toLowerCase() === 'in' ? 'input' : 'output'
    if (pair[side]) throw new Error(`测试点 ${key} 有重复的 ${side === 'input' ? '.in' : '.out'} 文件`)
    pair[side] = file
    pairs.set(key, pair)
  }
  for (const [name, pair] of pairs) {
    if (!pair.input || !pair.output) throw new Error(`测试点 ${name} 缺少 ${pair.input ? '.out' : '.in'} 文件`)
  }
  if (!crypto.subtle) throw new Error('文件校验需要 HTTPS 或 localhost 安全环境')
  const entries: { name: string, bytes: Uint8Array<ArrayBuffer> }[] = []
  const manifest = {
    version: 1,
    files: [] as { path: string, sha256: string, size: number }[],
    test_cases: [] as { name: string, input_file: string, output_file: string, score: number, ordinal: number, enabled: boolean }[],
  }
  const sorted = [...pairs].sort(([a], [b]) => a.localeCompare(b, 'en', { numeric: true }) || (a < b ? -1 : a > b ? 1 : 0))
  for (const [name, pair] of sorted) {
    for (const file of [pair.input!, pair.output!]) {
      signal.throwIfAborted()
      const bytes = new Uint8Array(await file.arrayBuffer())
      const digest = await crypto.subtle.digest('SHA-256', bytes)
      signal.throwIfAborted()
      const sha256 = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('')
      entries.push({ name: file.name, bytes })
      manifest.files.push({ path: file.name, sha256, size: file.size })
    }
    manifest.test_cases.push({ name, input_file: pair.input!.name, output_file: pair.output!.name,
      score: 1, ordinal: manifest.test_cases.length + 1, enabled: true })
  }
  const bytes = encoder.encode(JSON.stringify(manifest))
  if (bytes.length > 2 * 1024 * 1024) throw new Error('测试数据清单超过 2 MiB，请减少测试点数量')
  entries.push({ name: 'manifest.json', bytes })
  const archive = zip(entries)
  signal.throwIfAborted()
  validateArchive(archive)
  return archive
}
