import { computed, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { useApi } from './useApi'
import { useSession } from './useSession'

// Drafts stay in memory and are scoped to the authenticated account and surface.
const drafts = new Map<string, { source: string; language: string }>()
export function clearCodeDrafts(scope: string) {
  for (const key of drafts.keys()) if (key.startsWith(`${scope}:`)) drafts.delete(key)
}
export function useCodeDraft(key: MaybeRefOrGetter<string>, languages: string[]) {
  const api = useApi()
  const session = useSession()
  const draftKey = computed(() => `${api.scope}:${session.value?.user.user_id ?? 'anonymous'}:${toValue(key)}`)
  const source = ref('')
  const language = ref('')
  let loading = false
  let discarded = false
  watch(draftKey, (value) => {
    loading = true
    const saved = drafts.get(value)
    source.value = saved?.source ?? ''
    language.value = saved?.language ?? languages[0] ?? ''
    discarded = false
    loading = false
  }, { immediate: true, flush: 'sync' })
  watch([source, language], () => {
    if (!loading && !discarded) drafts.set(draftKey.value, { source: source.value, language: language.value })
  }, { flush: 'sync' })
  function discard() { discarded = true; drafts.delete(draftKey.value) }
  return { source, language, discard }
}
