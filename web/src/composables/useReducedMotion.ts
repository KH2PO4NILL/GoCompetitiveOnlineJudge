import { onScopeDispose, ref } from 'vue'

export function useReducedMotion() {
  const reduced = ref(window.matchMedia('(prefers-reduced-motion: reduce)').matches)
  const query = window.matchMedia('(prefers-reduced-motion: reduce)')
  const onChange = (event: MediaQueryListEvent) => { reduced.value = event.matches }
  query.addEventListener('change', onChange)
  onScopeDispose(() => query.removeEventListener('change', onChange))
  return reduced
}
