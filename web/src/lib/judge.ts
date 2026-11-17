import type { SubmissionSummary } from '../api/contracts/app'
import { judgeStatuses, verdicts } from './labels'

export function isPending(submission: SubmissionSummary): boolean {
  return submission.status === 'queued' || submission.status === 'judging'
}

/** 终态且结果完全可见时展示 verdict，否则展示状态文案 */
export function resultLabel(submission: SubmissionSummary): string {
  if (submission.status !== 'finished' || submission.result_visibility !== 'full') {
    return judgeStatuses[submission.status] ?? submission.status
  }
  return verdicts[submission.verdict] ?? (submission.verdict === 'unspecified' ? '结果未提供' : submission.verdict)
}

/** 结果 → 徽章色调：绿=通过，红=错误，灰=等待，蓝(动效)=运行中 */
export function verdictTone(submission: SubmissionSummary): 'success' | 'danger' | 'warning' | 'neutral' | 'accent' | 'muted' {
  if (submission.status === 'queued') return 'neutral'
  if (submission.status === 'judging') return 'accent'
  if (submission.result_visibility !== 'full') return 'muted'
  if (submission.verdict === 'accepted') return 'success'
  if (submission.verdict === 'partial_accepted') return 'warning'
  if (submission.verdict === 'system_error') return 'muted'
  return 'danger'
}
