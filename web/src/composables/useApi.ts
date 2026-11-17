import { inject, toValue } from 'vue'
import type { ApiClient } from '../api/client'
import { runtimeKey } from '../core/keys'

export function useApi(): ApiClient {
  const runtime = inject(runtimeKey)
  if (!runtime) throw new Error('useApi 必须在应用作用域内使用')
  return toValue(runtime).api
}
