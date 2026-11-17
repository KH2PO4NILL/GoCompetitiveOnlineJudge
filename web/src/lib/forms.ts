import type { ContestEditableFields, ProblemEditableFields, ProblemSample } from '../api/contracts/admin'
import { difficulties, visibilities, checkerTypes } from './labels'
import { fromLocalInput } from './time'

export type StringRecord = Record<string, string>

export function positiveInteger(value: string, label: string, maximum = 2147483647): number {
  if (!/^[1-9]\d*$/.test(value) || !Number.isSafeInteger(Number(value)) || Number(value) > maximum) {
    throw new Error(`${label}必须是 1–${maximum} 的整数`)
  }
  return Number(value)
}

export function splitValues(value: string): string[] {
  return [...new Set(value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean))]
}

/** 校验并构造题目可编辑字段（创建/更新共用） */
export function problemValues(values: StringRecord, samples: ProblemSample[]): ProblemEditableFields {
  const title = values.title.trim()
  const languages = splitValues(values.allowed_languages)
  const tags = splitValues(values.tags)
  if (!title || [...title].length > 200) throw new Error('题目名称须为 1–200 个字符')
  if (!languages.length || languages.length > 64 || languages.some((language) => !/^[\x21-\x7e]{1,64}$/.test(language))) {
    throw new Error('请填写有效的允许语言标识，多个语言以逗号分隔')
  }
  if (tags.length > 50 || samples.length > 100) throw new Error('最多支持 50 个标签和 100 个公开样例')
  if (!Object.hasOwn(difficulties, values.difficulty) || !Object.hasOwn(visibilities, values.visibility) || !Object.hasOwn(checkerTypes, values.checker_type)) {
    throw new Error('无效的题目配置')
  }
  return {
    statement: {
      title,
      description: values.description,
      input_description: values.input_description,
      output_description: values.output_description,
      hint: values.hint,
      samples,
    },
    difficulty: values.difficulty,
    tags,
    source: values.source,
    visibility: values.visibility,
    judge_settings: {
      time_limit_ms: positiveInteger(values.time_limit_ms, '时间限制'),
      memory_limit_mb: positiveInteger(values.memory_limit_mb, '内存限制'),
      allowed_languages: languages,
      checker_type: values.checker_type,
    },
  }
}

const contestTimeKeys = ['start_at', 'end_at', 'registration_start_at', 'registration_end_at'] as const

/** 校验并把表单值转换为比赛可编辑字段；freeze_* 辅助键由 freezePolicy 单独处理 */
export function contestValues(values: StringRecord): ContestEditableFields {
  const fields: Record<string, string> = { ...values }
  for (const key of ['freeze_policy', 'freeze_at', 'freeze_seconds']) delete fields[key]
  for (const key of contestTimeKeys) {
    if (fields[key]) fields[key] = fromLocalInput(fields[key])
    else if (key in fields) fields[key] = ''
  }
  if (!fields.invite_code) delete fields.invite_code
  if (!fields.title?.trim()) throw new Error('请填写比赛标题。')
  if (!fields.start_at || !fields.end_at) throw new Error('请填写比赛开始与结束时间。')
  if (fields.start_at >= fields.end_at) throw new Error('结束时间必须晚于开始时间。')
  if (fields.registration_start_at && fields.registration_end_at && fields.registration_start_at >= fields.registration_end_at) {
    throw new Error('报名截止时间必须晚于报名开始时间。')
  }
  if (fields.registration_end_at && fields.registration_end_at > fields.start_at) throw new Error('报名截止时间不能晚于比赛开始时间。')
  return fields as unknown as ContestEditableFields
}

/** 管理端列表页：把 URL 查询参数转换为请求参数，并检测非法 ID */
export function listParams(search: URLSearchParams, fields: string[], isValidId: (value: string) => boolean, pageNumberFn: (value: string | null) => number) {
  const params = new URLSearchParams({ page: String(pageNumberFn(search.get('page'))), page_size: '20' })
  for (const field of fields) {
    const value = search.get(field)?.trim()
    if (!value) continue
    if (field === 'tags') for (const tag of splitValues(value)) params.append(field, tag)
    else params.set(field, value)
  }
  const invalid = fields.some((field) => field.endsWith('_id')
    && params.has(field)
    && !(field === 'school_id' && params.get(field) === '0')
    && !isValidId(params.get(field)!))
  return { params, invalid, page: pageNumberFn(search.get('page')) }
}
