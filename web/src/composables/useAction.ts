import { ref } from 'vue'
import { ApiError } from '../api/client'

/**
 * 表单/动作执行器：
 * - busy 防重复点击；
 * - 结果未知（uncertain）后锁死动作，必须先核对服务端结果，不得自动重复非幂等操作；
 * - 统一收集 notice（成功提示）与 error。
 */
export function useAction() {
  const busy = ref(false)
  const error = ref<unknown>()
  const notice = ref('')
  const uncertain = ref(false)

  /**
   * 执行动作。结果未知（uncertain）后默认锁死，禁止再次触发；
   * 仅当调用方自带幂等保障（如复用 client_request_id）时可传 allowUncertain 显式解锁。
   */
  async function run(fn: () => Promise<void | string>, options: { allowUncertain?: boolean } = {}): Promise<boolean> {
    if (busy.value || (uncertain.value && !options.allowUncertain)) return false
    busy.value = true
    error.value = undefined
    notice.value = ''
    try {
      const result = await fn()
      if (typeof result === 'string') notice.value = result
      return true
    } catch (failure) {
      error.value = failure
      if (failure instanceof ApiError && failure.uncertain) uncertain.value = true
      return false
    } finally {
      busy.value = false
    }
  }

  function reset() {
    error.value = undefined
    notice.value = ''
    uncertain.value = false
  }

  return { busy, error, notice, uncertain, run, reset }
}
