import { ref } from 'vue'

// A stalled judge must not keep a tab polling indefinitely. The user can resume.
export function useJudgePolling(now: () => number = Date.now) {
  const paused = ref(false)
  let deadline = now() + 5 * 60_000
  function reset() { deadline = now() + 5 * 60_000; paused.value = false }
  function interval(pending: boolean, error: unknown, delay: number): number | false {
    if (error || !pending) return false
    if (now() >= deadline) { paused.value = true; return false }
    return delay
  }
  return { paused, reset, interval }
}
