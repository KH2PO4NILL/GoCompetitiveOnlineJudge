import { expect, test } from '@playwright/test'

test('公共导航与认证守卫', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'GCOJ 在线评测' })).toBeVisible()
  await expect(page.getByRole('navigation', { name: '用户端导航' })).toBeVisible()

  // 未登录时受保护路由重定向到登录页并携带 returnTo
  for (const path of ['/account', '/submit', '/submissions', '/submissions/1', '/problems/1/submit']) {
    await page.goto(path)
    await expect(page).toHaveURL((url) => url.pathname === '/login' && url.searchParams.get('returnTo') === path)
  }

  // 管理端守卫与 scope 隔离
  await page.goto('/admin/account')
  await expect(page).toHaveURL(/\/admin\/login\?returnTo=/)
  await expect(page.getByRole('heading', { name: '管理员登录' })).toBeVisible()
})

test('会话不落地存储', async ({ page }) => {
  await page.goto('/')
  await page.goto('/login')
  await page.goto('/admin/login')
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length }))).toEqual({ local: 0, session: 0 })
})

test('键盘操作：skip-link 与焦点可见', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeAttached()
  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('main#main')).toBeFocused()
})

test('移动端导航可展开收起', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  const nav = page.getByRole('navigation', { name: '用户端导航' })
  await expect(nav).toBeHidden()
  await page.getByRole('button', { name: '菜单' }).click()
  await expect(nav).toBeVisible()
  await page.getByRole('button', { name: '收起导航' }).click()
  await expect(nav).toBeHidden()
})

for (const width of [320, 390, 640, 768, 1024, 1440]) {
  test(`核心页面在 ${width}px 宽度无横向溢出`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    // 拦截 API，避免本地无后端时请求挂起影响布局断言
    await page.route('**/api/v1/**', (route) => route.abort('connectionrefused'))
    for (const path of ['/', '/login', '/admin/login', '/problems', '/contests']) {
      await page.goto(path)
      await expect(page.locator('main#main'), `${path} @${width}px`).toBeVisible({ timeout: 10_000 })
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), {
        message: `${path} @${width}px`,
        timeout: 10_000,
      }).toBe(true)
    }
  })
}
