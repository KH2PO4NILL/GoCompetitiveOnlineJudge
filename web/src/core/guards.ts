import type { Router } from 'vue-router'
import { isAdminUser, type Scope } from '../api/client'
import { getRuntime } from './runtime'

export function installGuards(router: Router): void {
  router.beforeEach(async (to) => {
    const scope = (to.meta.scope as Scope | undefined) ?? 'app'
    await getRuntime(scope).api.restore()
    if (to.meta.requiresAuth !== true) return true
    const session = getRuntime(scope).api.snapshot()
    if (!session) {
      return { path: scope === 'admin' ? '/admin/login' : '/login', query: { returnTo: to.fullPath } }
    }
    if (scope === 'admin' && !isAdminUser(session.user)) return { path: '/' }
    return true
  })

  router.afterEach((to) => {
    const title = to.meta.title as string | undefined
    document.title = title ? `${title} · GCOJ` : 'GCOJ · 算法竞赛在线评测'
  })
}
