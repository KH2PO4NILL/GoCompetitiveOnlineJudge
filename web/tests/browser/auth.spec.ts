import { expect, test } from '@playwright/test'

test('登录页模式切换保持字段正确', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('heading', { level: 1, name: '登录' })).toBeVisible()
  // 用户端登录页不暴露管理端入口，管理端只能直接访问 /admin/login
  await expect(page.getByRole('link', { name: '管理员入口' })).toHaveCount(0)

  // 默认：用户名密码
  await expect(page.getByLabel('用户名 *')).toBeVisible()
  await expect(page.getByLabel('密码 *')).toHaveAttribute('type', 'password')

  // 验证码登录：无密码字段，有验证码
  await page.getByRole('tab', { name: '验证码登录' }).click()
  await expect(page.getByLabel('邮箱 *')).toBeVisible()
  await expect(page.getByLabel('密码 *')).toHaveCount(0)
  await expect(page.getByLabel('邮箱验证码 *')).toBeVisible()
  await expect(page.getByRole('button', { name: '发送验证码' })).toBeVisible()

  // 注册
  await page.getByRole('tab', { name: '注册' }).click()
  await expect(page.getByRole('heading', { level: 1, name: '注册账号' })).toBeVisible()
  await expect(page.getByLabel('用户名 *')).toBeVisible()
  await expect(page.getByRole('button', { name: '创建账号' })).toBeVisible()

  // 找回密码
  await page.getByRole('tab', { name: '找回密码' }).click()
  await expect(page.getByLabel('新密码 *')).toBeVisible()
  await expect(page.getByRole('button', { name: '重置密码' })).toBeVisible()
})

test('管理端登录不提供注册入口', async ({ page }) => {
  await page.goto('/admin/login')
  await expect(page.getByRole('heading', { level: 1, name: '管理员登录' })).toBeVisible()
  await expect(page.getByRole('tab', { name: '注册' })).toHaveCount(0)
  await expect(page.getByRole('tab', { name: '找回密码' })).toBeVisible()
})

test('非法 returnTo 被拒绝', async ({ page }) => {
  await page.goto('/login?returnTo=https%3A%2F%2Fevil.example')
  await page.getByLabel('用户名 *').fill('x')
  await page.getByLabel('密码 *').fill('y')
  // 后端不在线时登录必然失败，这里只验证表单可交互且不跳转外部
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\/login/)
})

test('真实认证与角色边界（需要隔离后端）', async ({ page, request }) => {
  test.skip(process.env.GCOJ_AUTH_E2E !== '1', 'requires GCOJ_AUTH_E2E=1 with isolated backend')
  const password = process.env.GCOJ_AUTH_E2E_PASSWORD!
  const appUpstream = process.env.GCOJ_APP_UPSTREAM ?? 'http://127.0.0.1:8002'
  const adminUpstream = process.env.GCOJ_ADMIN_UPSTREAM ?? 'http://127.0.0.1:8001'

  await page.goto('/login')
  await page.getByLabel('用户名 *').fill('e2e-user')
  await page.getByLabel('密码 *').fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\//)
  await expect(page.getByRole('link', { name: 'e2e-user' })).toBeVisible()

  // 用户 token 不能访问管理端 API
  const login = await request.post(`${appUpstream}/api/v1/auth/login/username`, { data: { username: 'e2e-user', password } })
  expect(login.status()).toBe(200)
  const tokens = (await login.json()).tokens
  const denied = await request.get(`${adminUpstream}/api/v1/admin/users`, { headers: { Authorization: `Bearer ${tokens.access_token}` } })
  expect(denied.status()).toBe(403)
  await request.post(`${appUpstream}/api/v1/auth/logout`, { data: tokens })

  // 非管理员从管理端登录被拒绝
  await page.goto('/admin/login')
  await page.getByLabel('用户名 *').fill('e2e-user')
  await page.getByLabel('密码 *').fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByText('此账号没有管理权限')).toBeVisible()

  // 管理员登录进入工作台
  await page.getByLabel('用户名 *').fill('e2e-admin')
  await page.getByLabel('密码 *').fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '管理工作台' })).toBeVisible()

  // 刷新后通过 HttpOnly Cookie 恢复，且脚本存储无凭据残留
  await page.reload()
  await expect(page.getByRole('heading', { name: '管理工作台' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'e2e-admin' })).toBeVisible()
  expect(await page.evaluate(() => localStorage.length + sessionStorage.length)).toBe(0)
  expect(await page.evaluate(() => document.cookie)).not.toContain('refresh')
  await page.goto('/admin/login')
  await page.getByLabel('用户名 *').fill('e2e-user')
  await page.getByLabel('密码 *').fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByText('此账号没有管理权限')).toBeVisible()
  await page.reload()
  await expect(page.getByRole('link', { name: 'e2e-admin', exact: true })).toBeVisible()

})

test('真实三种登录、过期刷新、业务错误和账户切换（隔离后端）', async ({ page, context }) => {
  test.skip(process.env.GCOJ_AUTH_E2E !== '1', 'requires isolated backend')
  const failures: string[] = []
  page.on('response', async (response) => {
    const path = new URL(response.url()).pathname
    if (path.startsWith('/api/v1/') && response.status() >= 400) {
      const code = await response.json().then((body: { code?: string }) => body.code).catch(() => 'unreadable')
      failures.push(`${path}: ${response.status()} ${code ?? 'unknown'}`)
    }
  })
  const password = process.env.GCOJ_AUTH_E2E_PASSWORD!
  for (const mode of ['username', 'email-password', 'email-code']) {
    await page.goto('/login')
    if (mode === 'email-password') await page.getByRole('button', { name: '邮箱密码登录', exact: true }).click()
    if (mode === 'email-code') await page.getByRole('tab', { name: '验证码登录' }).click()
    const name = mode === 'email-code' ? 'e2e-code' : 'e2e-user'
    if (mode === 'username') await page.getByLabel('用户名 *').fill(name)
    else await page.getByLabel('邮箱 *').fill(`${name}@example.invalid`)
    if (mode === 'email-code') await page.getByLabel('邮箱验证码 *').fill('482913')
    else await page.getByLabel('密码 *').fill(password)
    await page.getByRole('button', { name: '登录', exact: true }).click()
    await expect(page.getByRole('link', { name, exact: true })).toBeVisible()
    // Fixture uses the same token logic with a short access TTL to cross expiry.
    await page.waitForTimeout(3500)
    await page.getByRole('link', { name, exact: true }).click()
    await expect(page.getByRole('heading', { name: '个人信息', exact: true })).toBeVisible()
    await expect(page.getByRole('link', { name, exact: true })).toBeVisible()
    const cookie = (await context.cookies()).find((cookie) => cookie.name === 'gcoj_app_refresh')
    expect(cookie?.httpOnly).toBe(true)
    expect(cookie?.sameSite).toBe('Strict')
    await page.reload()
    try { await expect(page.getByRole('link', { name, exact: true })).toBeVisible() } catch (error) {
      console.info('Auth response codes:', failures)
      console.info('Session notices:', await page.getByRole('alert').allTextContents())
      throw error
    }
    // Incorrect change-email code must remain a business error without logout.
    const result = await page.evaluate(async () => {
      const refresh = await fetch('/api/v1/auth/token/refresh', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-GCOJ-Browser-Session': '1' }, body: '{"refresh_token":""}' })
      const tokens = (await refresh.json()).tokens
      const response = await fetch('/api/v1/users/me/email', { method: 'PUT', headers: { Authorization: `Bearer ${tokens.access_token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ new_email: 'unused@example.invalid', code: '000000' }) })
      return { status: response.status, code: (await response.json()).code }
    })
    expect(result).toEqual({ status: 400, code: 'invalidargument' })
    await page.reload()
    await expect(page.getByRole('link', { name, exact: true })).toBeVisible()
    // A failed switch must preserve the original account and cookie.
    await page.goto('/login')
    await page.getByLabel('用户名 *').fill('missing-account')
    await page.getByLabel('密码 *').fill('wrong-password')
    await page.getByRole('button', { name: '登录', exact: true }).click()
    await expect(page.getByRole('link', { name, exact: true })).toBeVisible()
    await page.reload()
    await expect(page.getByRole('link', { name, exact: true })).toBeVisible()
    await page.getByRole('button', { name: '退出', exact: true }).click()
    await expect(page.getByRole('link', { name: '登录 / 注册' })).toBeVisible()
  }
})

test('真实超级管理员修改账户并撤销旧会话（隔离后端）', async ({ page, request }) => {
  test.skip(process.env.GCOJ_AUTH_E2E !== '1', 'requires isolated backend')
  const password = process.env.GCOJ_AUTH_E2E_PASSWORD!
  const upstream = process.env.GCOJ_APP_UPSTREAM ?? 'http://127.0.0.1:8002'
  const login = await request.post(`${upstream}/api/v1/auth/login/username`, { data: { username: 'e2e-target', password } })
  expect(login.status()).toBe(200)
  const original = await login.json()
  await page.goto('/admin/login')
  await page.getByLabel('用户名 *').fill('e2e-super')
  await page.getByLabel('密码 *').fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '管理工作台', exact: true })).toBeVisible()
  await page.getByRole('navigation', { name: '管理端导航' }).getByRole('link', { name: '用户管理', exact: true }).click()
  await page.getByRole('link', { name: 'e2e-target', exact: true }).click()
  const account = page.getByRole('heading', { name: '账户资料与密码', exact: true }).locator('..').locator('..')
  await account.getByLabel('用户名', { exact: true }).fill('e2e-renamed')
  await account.getByLabel('邮箱', { exact: true }).fill('e2e-renamed@example.invalid')
  await account.getByLabel('新密码', { exact: true }).fill(`${password}-reset`)
  await account.getByRole('button', { name: '保存账户资料' }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: '确认保存' }).click()
  await expect(page.getByText('账户已更新，目标用户的原有登录会话已撤销。')).toBeVisible()
  await expect(account.getByLabel('新密码', { exact: true })).toHaveValue('')
  const oldSession = await request.get(`${upstream}/api/v1/users/me`, { headers: { Authorization: `Bearer ${original.tokens.access_token}` } })
  expect(oldSession.status()).toBe(401)
  const refresh = await request.post(`${upstream}/api/v1/auth/token/refresh`, { data: { refresh_token: original.tokens.refresh_token } })
  expect(refresh.status()).toBe(401)
  const oldPassword = await request.post(`${upstream}/api/v1/auth/login/username`, { data: { username: 'e2e-renamed', password } })
  expect(oldPassword.status()).toBe(401)
  const newPassword = await request.post(`${upstream}/api/v1/auth/login/username`, { data: { username: 'e2e-renamed', password: `${password}-reset` } })
  expect(newPassword.status()).toBe(200)
  const body = await newPassword.json()
  expect(body.user.username).toBe('e2e-renamed')
  expect(JSON.stringify(body.user)).not.toContain('password')
})
