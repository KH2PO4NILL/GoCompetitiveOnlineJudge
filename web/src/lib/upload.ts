export const maxArchiveBytes = 128 * 1024 * 1024

export function validateArchive(file: File) {
  if (!/\.zip$/i.test(file.name) || file.size <= 0) throw new Error('请选择非空 ZIP 数据包')
  if (file.size > maxArchiveBytes) throw new Error('浏览器上传暂限 128 MiB；服务端可能配置更低的限制')
}

/** 校验预签名上传地址；预签名 URL 不得泄露到日志或界面 */
export function storageUrl(value: string): URL {
  let url: URL
  try { url = new URL(value) } catch { throw new Error('对象存储上传地址无效') }
  const safe = ['https:', 'http:'].includes(url.protocol)
    && !url.username && !url.password && !url.hash
    && (location.protocol !== 'https:' || url.protocol === 'https:')
  if (!safe) throw new Error('对象存储上传地址不安全')
  return url
}

export async function archiveDigest(file: File, signal: AbortSignal): Promise<string> {
  validateArchive(file)
  if (!crypto.subtle) throw new Error('文件校验需要 HTTPS 或 localhost 安全环境')
  const bytes = await file.arrayBuffer()
  signal.throwIfAborted()
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  signal.throwIfAborted()
  return Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, '0')).join('')
}

export async function uploadArchive(target: string, file: File, signal: AbortSignal, progress: (percent: number) => void): Promise<void> {
  const url = storageUrl(target)
  signal.throwIfAborted()
  if (url.origin === location.origin) {
    const response = await fetch(url, {
      method: 'PUT', body: file, credentials: 'omit', redirect: 'error', referrerPolicy: 'no-referrer',
      signal: AbortSignal.any([signal, AbortSignal.timeout(120_000)]), cache: 'no-store',
    })
    if (!response.ok) throw new Error(`对象存储上传失败（HTTP ${response.status}）`)
    progress(100)
    return
  }
  await new Promise<void>((resolve, reject) => {
    const request = new XMLHttpRequest()
    const abort = () => request.abort()
    const finish = (error?: Error) => {
      signal.removeEventListener('abort', abort)
      if (error) reject(error)
      else resolve()
    }
    request.open('PUT', url.href)
    request.withCredentials = false
    request.timeout = 120_000
    request.upload.onprogress = (event) => { if (event.lengthComputable) progress(Math.round(event.loaded * 100 / event.total)) }
    request.onload = () => finish(request.status >= 200 && request.status < 300 ? undefined : new Error(`对象存储上传失败（HTTP ${request.status}），请检查签名有效期和存储权限`))
    request.onerror = () => finish(new Error('无法连接对象存储，请检查网络及存储端 PUT / CORS 配置'))
    request.ontimeout = () => finish(new Error('上传超时，可用同一数据包重试'))
    request.onabort = () => finish(new DOMException('上传已取消', 'AbortError'))
    signal.addEventListener('abort', abort, { once: true })
    request.send(file)
  })
}
