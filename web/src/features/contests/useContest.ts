import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { useRoute } from 'vue-router'
import type { GetContestResp, GetMyContestStatusResp } from '../../api/contracts/app'
import { useApi } from '../../composables/useApi'
import { useSession } from '../../composables/useSession'
import { isValidId } from '../../lib/ids'

/**
 * 比赛上下文：contest detail 与（登录时）my status 的共享查询。
 * ContestLayout 与各子页面使用相同 queryKey，vue-query 自动共享缓存并合并在途请求。
 */
export function useContestContext() {
  const api = useApi()
  const route = useRoute()
  const session = useSession()

  const contestId = computed(() => String(route.params.contestId ?? ''))
  const idValid = computed(() => isValidId(contestId.value))
  // 64 位 ID 一律 String() 参与 key，避免 bigint 精度/类型问题
  const userId = computed(() => (session.value ? String(session.value.user.user_id) : ''))
  const loggedIn = computed(() => session.value !== null)

  const contest = useQuery({
    queryKey: computed(() => ['contest', contestId.value]),
    enabled: idValid,
    queryFn: ({ signal }) => api.request<GetContestResp>(`/contests/${contestId.value}`, { signal }),
  })

  const myStatus = useQuery({
    queryKey: computed(() => ['contest', contestId.value, 'my-status', userId.value]),
    enabled: computed(() => idValid.value && userId.value !== ''),
    queryFn: ({ signal }) => api.request<GetMyContestStatusResp>(`/contests/${contestId.value}/registrations/me`, { signal }),
  })

  const detail = computed(() => contest.data.value?.contest)
  const my = computed(() => myStatus.data.value?.status)

  return { session, contestId, idValid, loggedIn, contest, myStatus, detail, my }
}
