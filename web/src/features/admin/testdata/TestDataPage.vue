<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { RouterLink, useRoute } from 'vue-router'
import { ApiError } from '../../../api/client'
import type {
  CreateTestDataUploadResp,
  FinalizeTestDataUploadResp,
  GetProblemForEditResp,
} from '../../../api/contracts/admin'
import { useAction } from '../../../composables/useAction'
import { useApi } from '../../../composables/useApi'
import { isValidId, parseIdParam } from '../../../lib/ids'
import { archiveDigest, uploadArchive } from '../../../lib/upload'
import { prepareTestData } from '../../../lib/testdata'
import ErrorNotice from '../../../components/ui/ErrorNotice.vue'
import StateBlock from '../../../components/ui/StateBlock.vue'

const api = useApi()
const route = useRoute()

const problemIdText = computed(() => String(route.params.problemId ?? ''))
const idValid = computed(() => isValidId(problemIdText.value))
const problemId = computed(() => parseIdParam(route.params.problemId))

const problem = useQuery({
  queryKey: computed(() => ['admin', 'problem', problemIdText.value]),
  enabled: idValid,
  queryFn: ({ signal }) => api.request<GetProblemForEditResp>(`/problems/${problemIdText.value}`, { signal }),
})

const testCases = computed(() => problem.data.value?.test_cases ?? [])
const version = computed(() => problem.data.value?.problem.judge_config.test_data_version ?? '')

type Phase = 'idle' | 'preparing' | 'hashing' | 'creating' | 'uploading' | 'finalizing'
const uploadAction = useAction()
const phase = ref<Phase>('idle')
const progress = ref(0)
const conflict = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
let controller: AbortController | null = null

const phaseText: Record<Phase, string> = {
  idle: '',
  preparing: '正在配对文件并生成数据包…',
  hashing: '正在计算 SHA-256 校验值…',
  creating: '正在申请上传地址…',
  uploading: '正在上传数据包…',
  finalizing: '正在提交入库…',
}

const busy = computed(() => uploadAction.busy.value)
const locked = computed(() => busy.value || uploadAction.uncertain.value)

async function startUpload(files: File[]) {
  const data = problem.data.value
  if (!data) return
  const expectedRevision = data.problem.revision
  controller = new AbortController()
  const signal = controller.signal
  conflict.value = false
  progress.value = 0
  await uploadAction.run(async () => {
    phase.value = 'preparing'
    const file = await prepareTestData(files, signal)
    phase.value = 'hashing'
    const sha256 = await archiveDigest(file, signal)
    phase.value = 'creating'
    // 预签名地址仅用于上传，绝不渲染或记录到日志。
    const created = await api.request<CreateTestDataUploadResp>(`/problems/${problemIdText.value}/test-data/uploads`, {
      method: 'POST',
      body: { problem_id: problemId.value, file_name: file.name, content_length: BigInt(file.size), sha256 },
      signal,
    })
    phase.value = 'uploading'
    await uploadArchive(created.upload_url, file, signal, (percent) => { progress.value = percent })
    phase.value = 'finalizing'
    try {
      const finalized = await api.request<FinalizeTestDataUploadResp>(
        `/problems/${problemIdText.value}/test-data/uploads/${created.upload_id}/finalize`,
        {
          method: 'POST',
          body: { problem_id: problemId.value, upload_id: created.upload_id, sha256, expected_revision: expectedRevision },
          signal,
        },
      )
      await problem.refetch()
      return `测试数据已更新，版本 ${finalized.test_data_version}。`
    } catch (failure) {
      if (failure instanceof ApiError && failure.status === 409) {
        conflict.value = true
        await problem.refetch()
      }
      throw failure
    }
  })
  phase.value = 'idle'
  progress.value = 0
  controller = null
  if (fileInput.value) fileInput.value.value = ''
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  if (files.length) void startUpload(files)
}

function cancel() {
  controller?.abort()
}

function resetLock() {
  uploadAction.reset()
  void problem.refetch()
}
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="面包屑">
      <RouterLink :to="{ name: 'admin-problems' }">题目管理</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <RouterLink :to="`/admin/problems/${problemIdText}`">#{{ problemIdText }}</RouterLink>
      <span class="sep" aria-hidden="true">/</span>
      <span>测试数据</span>
    </nav>
    <div class="page-head">
      <h1>测试数据</h1>
      <RouterLink class="small" :to="`/admin/problems/${problemIdText}`">← 返回题目工作台</RouterLink>
    </div>

    <p v-if="!idValid" class="notice notice-error" role="alert">题目编号无效。</p>
    <StateBlock v-else :pending="problem.isPending.value" :error="problem.error.value" @retry="problem.refetch()">
      <template #content>
        <div class="stack">
          <section class="panel">
            <div class="panel-head">
              <h2>上传数据包</h2>
              <span class="small muted mono">当前版本 {{ version || '—' }}</span>
            </div>
            <div class="panel-body">
              <p class="small muted">
                选择一个 ZIP，或同时多选同名的 .in / .out 文件（如 1.in、1.out、2.in、2.out）。系统会自动配对并打包，每个测试点默认 1 分、启用，按文件名自然排序。上传将替换当前测试数据，浏览器上传上限为 128 MiB。
              </p>
              <div class="row">
                <input
                  id="testdata-file" ref="fileInput" type="file" multiple accept=".zip,.in,.out,application/zip" aria-label="选择 ZIP 或成对的输入输出文件"
                  :disabled="locked" @change="onFileChange"
                >
                <button v-if="busy" class="danger-quiet sm" type="button" @click="cancel">取消上传</button>
              </div>
              <details class="small">
                <summary>合法 ZIP 格式与自定义分值</summary>
                <p>ZIP 根目录必须包含 manifest.json，不要在外面再套一层文件夹。输入输出文件可以放在子目录中，清单路径必须与 ZIP 内路径一致。</p>
                <pre>manifest.json
1.in
1.out</pre>
                <p>manifest.json 示例（sha256 需替换为对应文件真实的 64 位十六进制 SHA-256；size 是文件字节数）：</p>
                <pre>{{ JSON.stringify({ version: 1, files: [
                  { path: '1.in', sha256: '输入文件的 SHA-256', size: 4 },
                  { path: '1.out', sha256: '输出文件的 SHA-256', size: 2 },
                ], test_cases: [
                  { name: '1', input_file: '1.in', output_file: '1.out', score: 100, ordinal: 1, enabled: true, group_name: '' },
                ] }, null, 2) }}</pre>
                <p>所有数据文件都必须在 files 中声明，不能夹带其他文件；测试点序号不能重复，分值必须为非负整数。不允许绝对路径、.. 路径、重复路径或符号链接。服务端还会校验文件大小、校验值和解压限制。</p>
              </details>
              <div v-if="busy" class="upload-status">
                <span class="small" role="status">{{ phaseText[phase] }}</span>
                <div v-if="phase === 'uploading'" class="progress" aria-hidden="true">
                  <div :style="{ width: `${progress}%` }" />
                </div>
                <span v-if="phase === 'uploading'" class="small muted mono">{{ progress }}%</span>
              </div>
              <p v-if="uploadAction.notice.value" class="notice notice-success" role="status">{{ uploadAction.notice.value }}</p>
              <p v-if="conflict" class="notice notice-warning" role="alert">
                服务端题目已更新（revision 变化），入库被拒绝。请刷新页面确认最新状态后重新上传。
              </p>
              <ErrorNotice :error="uploadAction.error.value" />
              <p v-if="uploadAction.uncertain.value" class="notice notice-warning" role="alert">
                入库结果未知：上传按钮已锁定，请先到<RouterLink :to="`/admin/problems/${problemIdText}`">题目工作台</RouterLink>核对 test_data_version，确认未生效后再处理，系统不会自动重试。
                <button class="link sm" type="button" @click="resetLock">我已核对，解锁</button>
              </p>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h2>测试点</h2>
              <span class="small muted">{{ testCases.length }} 个</span>
            </div>
            <div v-if="testCases.length" class="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th scope="col" class="num">序号</th>
                    <th scope="col">名称</th>
                    <th scope="col">分组</th>
                    <th scope="col" class="num">分值</th>
                    <th scope="col">启用</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in testCases" :key="String(item.test_case_id)">
                    <td class="num">{{ item.ordinal }}</td>
                    <td class="mono small">{{ item.name }}</td>
                    <td>{{ item.group_name || '—' }}</td>
                    <td class="num">{{ item.score }}</td>
                    <td>{{ item.enabled ? '是' : '否' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="panel-body"><p class="small muted">尚无测试数据。</p></div>
          </section>
        </div>
      </template>
    </StateBlock>
  </div>
</template>

<style scoped>
.upload-status { display: grid; gap: var(--space-2); margin-top: var(--space-3); }
</style>
