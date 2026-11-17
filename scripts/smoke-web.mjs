import { pathToFileURL } from 'node:url'

export async function smokeWeb(baseURL, report = console.log) {
  const checks = [
    { name: 'frontend', path: '/', status: 200, validate: (body, type) => type.includes('text/html') && /id=["']app["']/.test(body) },
    { name: 'vite-client', path: '/@vite/client', status: 200, validate: (body, type) => /javascript/.test(type) && body.length > 0 },
    { name: 'app-problems-proxy', path: '/api/v1/problems', status: 200, list: 'problems' },
    { name: 'app-contests-proxy', path: '/api/v1/contests', status: 200, list: 'contests' },
    { name: 'admin-auth-proxy', path: '/api/v1/admin/users/me', status: 401 },
  ]
  let failed = false
  for (const check of checks) {
    try {
      const response = await fetch(new URL(check.path, baseURL), { signal: AbortSignal.timeout(8000), redirect: 'error' })
      if (response.status !== check.status) throw new Error(`expected HTTP ${check.status}, received ${response.status}`)
      const type = response.headers.get('content-type') ?? ''
      const body = await response.text()
      if (check.validate) {
        if (!check.validate(body, type)) throw new Error('unexpected frontend content')
      } else {
        if (!type.includes('application/json')) throw new Error('expected JSON, not an HTML fallback')
        const data = JSON.parse(body)
        if (check.list) {
          if (!data || !Array.isArray(data[check.list]) || !Number.isInteger(data.total) || data.total < 0) throw new Error('invalid list response')
        } else if (data?.code !== 'unauthenticated' || typeof data.message !== 'string') {
          throw new Error('expected unauthenticated error envelope')
        }
      }
      report(`OK   ${check.name} ${check.path}`)
    } catch (error) {
      report(`FAIL ${check.name} ${check.path}: ${error.message}`)
      failed = true
    }
  }
  return !failed
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const port = process.env.WEB_PORT ?? '5173'
  if (!/^[1-9][0-9]{0,4}$/.test(port) || Number(port) > 65535) {
    console.error('WEB_PORT must be between 1 and 65535')
    process.exitCode = 1
  } else if (!await smokeWeb(`http://127.0.0.1:${port}`)) {
    process.exitCode = 1
  }
}
