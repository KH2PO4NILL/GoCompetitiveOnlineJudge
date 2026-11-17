import { computed, onScopeDispose, onMounted, ref } from 'vue'

/** 秒级倒计时，用于验证码重发间隔。 */
export function useCountdown() {
  const deadline = ref(0)
  const now = ref(Date.now())
  let timer: ReturnType<typeof setInterval> | undefined

  onMounted(() => { timer = setInterval(() => { now.value = Date.now() }, 1000) })
  onScopeDispose(() => { if (timer) clearInterval(timer) })

  const remaining = computed(() => Math.max(0, Math.ceil((deadline.value - now.value) / 1000)))

  function start(seconds: number) {
    deadline.value = Date.now() + seconds * 1000
    now.value = Date.now()
  }

  return { remaining, start }
}
