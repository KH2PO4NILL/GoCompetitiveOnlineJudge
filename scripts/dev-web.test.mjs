import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { createServer } from 'node:http'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { promisify } from 'node:util'
import { fileURLToPath } from 'node:url'
import test from 'node:test'
import { smokeWeb } from './smoke-web.mjs'

const execute = promisify(execFile)
const root = fileURLToPath(new URL('../', import.meta.url))
const script = path.join(root, 'scripts/dev-web.sh')

async function listen(handler) {
  const server = createServer(handler)
  await new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', resolve)
  })
  return { server, port: server.address().port, url: `http://127.0.0.1:${server.address().port}` }
}

async function close(server) {
  server.closeAllConnections()
  await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))
}

function reply(request, response) {
  if (request.url === '/') {
    response.writeHead(200, { 'Content-Type': 'text/html' }).end('<div id="app"></div>')
  } else if (request.url === '/@vite/client') {
    response.writeHead(200, { 'Content-Type': 'text/javascript' }).end('export {}')
  } else if (request.url === '/api/v1/admin/users/me') {
    response.writeHead(401, { 'Content-Type': 'application/json' }).end(JSON.stringify({ code: 'unauthenticated', message: 'valid bearer token required' }))
  } else {
    response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ total: 0, [request.url.endsWith('problems') ? 'problems' : 'contests']: [] }))
  }
}

test('smoke accepts valid responses including the expected admin 401', async context => {
  const { server, url } = await listen(reply)
  context.after(() => close(server))
  const messages = []
  assert.equal(await smokeWeb(url, message => messages.push(message)), true)
  assert.equal(messages.length, 5)
  assert.ok(messages.every(message => message.startsWith('OK')))
})

test('smoke rejects HTML fallbacks, invalid JSON, broken proxies and auth bypasses', async context => {
  for (const mode of ['html', 'invalid-json', 'bad-list', 'unavailable', 'auth-bypass', 'redirect']) {
    await context.test(mode, async subcontext => {
      const { server, url } = await listen((request, response) => {
        if (request.url === '/api/v1/admin/users/me' && mode === 'auth-bypass') {
          response.writeHead(200, { 'Content-Type': 'application/json' }).end('{}')
        } else if (request.url === '/api/v1/problems' && mode !== 'auth-bypass') {
          const status = mode === 'unavailable' ? 503 : mode === 'redirect' ? 302 : 200
          response.writeHead(status, { 'Content-Type': mode === 'html' ? 'text/html' : 'application/json', Location: '/' })
          response.end(mode === 'html' ? '<div id="app"></div>' : mode === 'invalid-json' ? '{' : '{}')
        } else {
          reply(request, response)
        }
      })
      subcontext.after(() => close(server))
      const messages = []
      assert.equal(await smokeWeb(url, message => messages.push(message)), false)
      assert.equal(messages.length, 5)
      assert.ok(messages.some(message => message.startsWith('FAIL')))
    })
  }
})

test('frontend lifecycle and real Vite proxies work without backend services', async context => {
  const runtime = await mkdtemp(path.join(tmpdir(), 'gcoj-dev-web-'))
  const appRequests = []
  const adminRequests = []
  const upstream = await listen((request, response) => {
    appRequests.push(request.url)
    reply(request, response)
  })
  const admin = await listen((request, response) => {
    adminRequests.push(request.url)
    reply(request, response)
  })
  const reserved = await listen(reply)
  const env = { ...process.env, RUNTIME_DIR: runtime, WEB_PORT: String(reserved.port), GCOJ_APP_UPSTREAM: upstream.url, GCOJ_ADMIN_UPSTREAM: admin.url }
  const run = command => execute('bash', [script, command], { cwd: root, env, timeout: 45000 })
  context.after(async () => {
    try { await run('stop') } finally {
      await close(upstream.server)
      await close(admin.server)
      await rm(runtime, { recursive: true, force: true })
    }
  })
  try {
    await assert.rejects(run('start'), error => /already in use/.test(error.stderr))
  } finally {
    await close(reserved.server)
  }
  assert.match((await run('start')).stdout, /frontend ready/)
  const originalPID = await readFile(path.join(runtime, 'pids/web.pid'), 'utf8')
  assert.match((await run('status')).stdout, /frontend running/)
  await assert.rejects(run('start'), error => /already running/.test(error.stderr))
  assert.equal(await smokeWeb(`http://127.0.0.1:${env.WEB_PORT}`, () => {}), true)
  assert.deepEqual(appRequests, ['/api/v1/problems', '/api/v1/contests'])
  assert.deepEqual(adminRequests, ['/api/v1/admin/users/me'])
  await writeFile(path.join(runtime, 'pids/web.pid'), `${process.pid}\n`)
  try {
    await assert.rejects(run('stop'), error => /unmanaged PID/.test(error.stderr))
  } finally {
    await writeFile(path.join(runtime, 'pids/web.pid'), originalPID)
  }
  assert.match((await run('stop')).stdout, /frontend stopped/)
  assert.match((await run('stop')).stdout, /not running/)
})
