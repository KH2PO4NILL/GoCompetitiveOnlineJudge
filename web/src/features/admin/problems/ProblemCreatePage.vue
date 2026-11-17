<script setup lang="ts">
import { requireId } from '../../../lib/ids'
import { ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import type { CreateProblemResp } from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { problemValues } from '../../../lib/forms'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import ProblemForm from './ProblemForm.vue'

const api = useApi()
const router = useRouter()
const form = ref<InstanceType<typeof ProblemForm> | null>(null)

const { busy, error, notice, uncertain, run } = useAction()

async function create() {
  await run(async () => {
    const snapshot = form.value?.snapshot()
    if (!snapshot) throw new Error('表单尚未就绪')
    const problem = problemValues(snapshot.values, snapshot.samples)
    const resp = await api.request<CreateProblemResp>('/problems', { method: 'POST', body: { problem } })
    const id = requireId(resp?.problem?.problem_id, '新建编号')
    await router.push(`/admin/problems/${id}`)
  })
}
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-problems' }">题目管理</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>创建题目</span>
    </nav>
    <div class="page-head">
      <h1>创建题目</h1>
    </div>
    <div class="panel">
      <div class="panel-body">
        <form @submit.prevent="create">
          <fieldset :disabled="busy">
            <ProblemForm ref="form" />
            <p class="small muted">创建后可在题目工作台的“测试数据”中上传成对的 .in / .out 文件或 ZIP 数据包。</p>
            <div class="row submit-row">
              <button type="submit" :disabled="busy">
                {{ busy ? '创建中…' : uncertain ? '结果未知 · 核对后可重试' : '创建题目' }}
              </button>
              <RouterLink class="small" :to="{ name: 'admin-problems' }">取消</RouterLink>
            </div>
          </fieldset>
        </form>
        <p v-if="notice" class="notice notice-success" role="status">{{ notice }}</p>
        <ErrorNotice :error="error" />
        <p v-if="uncertain" class="notice notice-warning" role="alert">
          创建结果未知：请先到题目管理列表核对是否已创建，不要盲目重试。
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.submit-row { margin-top: var(--space-4); }
</style>
