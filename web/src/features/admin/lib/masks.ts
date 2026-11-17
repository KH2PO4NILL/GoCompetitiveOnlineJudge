import type {
  Contest,
  ContestEditableFields,
  ContestPatch,
  Problem,
  ProblemEditableFields,
  ProblemPatch,
  ProblemSample,
} from '../../../api/contracts/admin'

export function samplesEqual(a: ProblemSample[], b: ProblemSample[]): boolean {
  if (a.length !== b.length) return false
  return a.every((item, index) => {
    const other = b[index]
    return item.input === other.input
      && item.output === other.output
      && (item.explanation ?? '') === (other.explanation ?? '')
  })
}

export function stringArrayEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false
  return a.every((value, index) => value === b[index])
}

/**
 * 计算题目改动：只把用户改动过的顶层字段放入 patch 与 update_mask。
 * 顶层字段：statement / difficulty / tags / source / judge_settings / visibility。
 */
export function problemDiff(next: ProblemEditableFields, server: Problem): { patch: ProblemPatch; updateMask: string[] } {
  const patch: ProblemPatch = {}
  const updateMask: string[] = []
  const statement = server.statement
  const nextStatement = next.statement
  if (
    statement.title !== nextStatement.title
    || statement.description !== nextStatement.description
    || statement.input_description !== nextStatement.input_description
    || statement.output_description !== nextStatement.output_description
    || (statement.hint ?? '') !== (nextStatement.hint ?? '')
    || !samplesEqual(statement.samples ?? [], nextStatement.samples ?? [])
  ) {
    patch.statement = nextStatement
    updateMask.push('statement')
  }
  if (server.metadata.difficulty !== next.difficulty) {
    patch.difficulty = next.difficulty
    updateMask.push('difficulty')
  }
  if (!stringArrayEqual(server.metadata.tags ?? [], next.tags)) {
    patch.tags = next.tags
    updateMask.push('tags')
  }
  if ((server.metadata.source ?? '') !== (next.source ?? '')) {
    patch.source = next.source
    updateMask.push('source')
  }
  const settings = server.judge_config.settings
  const nextSettings = next.judge_settings
  if (
    settings.time_limit_ms !== nextSettings.time_limit_ms
    || settings.memory_limit_mb !== nextSettings.memory_limit_mb
    || settings.checker_type !== nextSettings.checker_type
    || !stringArrayEqual(settings.allowed_languages ?? [], nextSettings.allowed_languages ?? [])
  ) {
    patch.judge_settings = nextSettings
    updateMask.push('judge_settings')
  }
  if (server.visibility !== next.visibility) {
    patch.visibility = next.visibility
    updateMask.push('visibility')
  }
  return { patch, updateMask }
}

const contestStringFields = [
  'title',
  'description',
  'mode',
  'start_at',
  'end_at',
  'registration_start_at',
  'registration_end_at',
  'registration_policy',
  'visibility',
  'scoreboard_visibility',
  'invite_code',
] as const satisfies readonly (keyof ContestEditableFields)[]

/**
 * 计算比赛常规字段改动（不含封榜配置，封榜由 lib/freeze freezePolicy 单独处理）。
 * 仅把改动过的字段放入 patch 与 update_mask。
 * 可选字段清空时只下发 mask、不下发空串：后端以字段缺省表示清空，
 * 空串会被当作非法时间/邀请码拒绝。
 */
export function contestDiff(next: ContestEditableFields, server: Contest): { patch: ContestPatch; updateMask: string[] } {
  const patch: ContestPatch = {}
  const updateMask: string[] = []
  const current = server.editable
  const nextRecord = next as unknown as Record<string, string | undefined>
  const currentRecord = current as unknown as Record<string, string | undefined>
  for (const field of contestStringFields) {
    const nextValue = nextRecord[field] ?? ''
    const currentValue = currentRecord[field] ?? ''
    if (nextValue === currentValue) continue
    updateMask.push(field)
    if (nextValue === '' && (field === 'registration_start_at' || field === 'registration_end_at' || field === 'invite_code')) continue
    ;(patch as unknown as Record<string, string>)[field] = nextValue
  }
  return { patch, updateMask }
}
