import { clearCodeDrafts } from '../composables/useCodeDraft'
import { QueryClient } from '@tanstack/vue-query'
import { ApiClient, type Scope } from '../api/client'

export interface Runtime {
  api: ApiClient
  queryClient: QueryClient
}

function createRuntime(scope: Scope): Runtime {
  const api = new ApiClient(scope)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 15_000, gcTime: 60_000, refetchOnWindowFocus: true },
      mutations: { retry: false },
    },
  })
  // 会话用户变更时清空整个查询缓存，防止跨账号数据残留
  let currentUser = api.snapshot()?.user
  api.subscribe(() => {
    const user = api.snapshot()?.user
    if (user !== currentUser) queryClient.clear()
    if (String(user?.user_id) !== String(currentUser?.user_id)) clearCodeDrafts(scope)
    currentUser = user
  })
  return { api, queryClient }
}

// 用户端与管理端各自持有独立的 ApiClient 与 QueryClient，会话/缓存/权限完全隔离
const runtimes: Record<Scope, Runtime> = {
  app: createRuntime('app'),
  admin: createRuntime('admin'),
}

export function getRuntime(scope: Scope): Runtime {
  return runtimes[scope]
}
