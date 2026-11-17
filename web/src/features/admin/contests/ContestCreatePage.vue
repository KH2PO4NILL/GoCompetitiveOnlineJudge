<script setup lang="ts">
import { requireId } from '../../../lib/ids'
import { ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import type { ContestEditableFields, CreateContestResp } from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { contestValues } from '../../../lib/forms'
import { freezePolicy } from '../../../lib/freeze'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import ContestForm from './ContestForm.vue'

const api = useApi()
const router = useRouter()
const form = ref<InstanceType<typeof ContestForm> | null>(null)

const { busy, error, notice, uncertain, run } = useAction()

async function create() {
  await run(async () => {
    const values = form.value?.snapshot()
    if (!values) throw new Error('表单尚未就绪')
    const fields = contestValues(values)
    const freeze = freezePolicy(values)
    const contest: ContestEditableFields = { ...fields, ...freeze.fields }
    const resp = await api.request<CreateContestResp>('/contests', { method: 'POST', body: { contest } })
    const id = requireId(resp?.contest?.contest_id, '新建编号')
    await router.push(`/admin/contests/${id}`)
  })
}
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-contests' }">比赛管理</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>创建比赛</span>
    </nav>
    <div class="page-head">
      <h1>创建比赛</h1>
    </div>
    <div class="panel">
      <div class="panel-body">
        <form @submit.prevent="create">
          <fieldset :disabled="busy">
            <ContestForm ref="form" />
            <div class="row submit-row">
              <button type="submit" :disabled="busy">
                {{ busy ? '创建中…' : uncertain ? '结果未知 · 核对后可重试' : '创建比赛' }}
              </button>
              <RouterLink class="small" :to="{ name: 'admin-contests' }">取消</RouterLink>
            </div>
          </fieldset>
        </form>
        <p v-if="notice" class="notice notice-success" role="status">{{ notice }}</p>
        <ErrorNotice :error="error" />
        <p v-if="uncertain" class="notice notice-warning" role="alert">
          创建结果未知：请先到比赛管理列表核对是否已创建，不要盲目重试。
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.submit-row { margin-top: var(--space-4); }
</style>
