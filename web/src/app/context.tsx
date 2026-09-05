import { createContext, useContext, useSyncExternalStore, type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ApiClient, type Scope } from '../api/client'

function runtime(scope: Scope) {
  const api = new ApiClient(scope)
  const queries = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 15_000, gcTime: 60_000 }, mutations: { retry: false } } })
  let previous = api.snapshot()?.user
  api.subscribe(() => {
    const current = api.snapshot()?.user
    if (current !== previous) queries.clear()
    previous = current
  })
  return { api, queries }
}

const runtimes = { app: runtime('app'), admin: runtime('admin') }
const Context = createContext(runtimes.app.api)

export function ScopeProvider({ scope, children }: { scope: Scope; children: ReactNode }) {
  const { api, queries } = runtimes[scope]
  return <Context.Provider key={scope} value={api}><QueryClientProvider client={queries}>{children}</QueryClientProvider></Context.Provider>
}

export const useApi = () => useContext(Context)
export function useSession() {
  const api = useApi()
  return useSyncExternalStore(api.subscribe, api.snapshot)
}
