import { onScopeDispose, ref, type Ref } from 'vue'
import type { Session } from '../api/client'
import { useApi } from './useApi'

export function useSession(): Ref<Session> {
  const api = useApi()
  const session = ref(api.snapshot()) as Ref<Session>
  const unsubscribe = api.subscribe(() => { session.value = api.snapshot() })
  onScopeDispose(unsubscribe)
  return session
}
