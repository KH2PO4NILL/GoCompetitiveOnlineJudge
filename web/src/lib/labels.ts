export const difficulties: Record<string, string> = {
  easy: '入门', easy_plus: '基础', medium: '进阶', medium_plus: '挑战', hard: '困难',
}

export const difficultyTones: Record<string, string> = {
  easy: 'success', easy_plus: 'accent', medium: 'warning', medium_plus: 'warning', hard: 'danger',
}

export const verdicts: Record<string, string> = {
  accepted: 'Accepted',
  partial_accepted: 'Partial Accepted',
  wrong_answer: 'Wrong Answer',
  time_limit_exceeded: 'Time Limit Exceeded',
  memory_limit_exceeded: 'Memory Limit Exceeded',
  runtime_error: 'Runtime Error',
  compile_error: 'Compile Error',
  output_limit_exceeded: 'Output Limit Exceeded',
  system_error: 'System Error',
}

export const judgeStatuses: Record<string, string> = {
  queued: '等待评测', judging: '正在评测', finished: '评测完成',
}

export const problemStatuses: Record<string, string> = {
  draft: '草稿', published: '已发布', archived: '已归档', deleted: '已删除',
}

export const visibilities: Record<string, string> = {
  public: '公开', private: '私有', contest_only: '仅比赛',
}

export const checkerTypes: Record<string, string> = {
  standard: '标准比较', special_judge: 'Special Judge', interactive: '交互题',
}

export const contestStatuses: Record<string, string> = {
  draft: '草稿', published: '未开始', running: '进行中', finished: '已结束',
  cancelled: '已取消', archived: '已归档', deleted: '已删除',
}

export const contestModes: Record<string, string> = {
  icpc: 'ICPC', ioi: 'IOI', oi: 'OI',
}

export const registrationPolicies: Record<string, string> = {
  open: '公开报名', approval_required: '审核报名', invite_only: '邀请码报名',
}

export const registrationStatuses: Record<string, string> = {
  pending: '等待审核', approved: '报名通过', rejected: '报名被拒绝', cancelled: '已取消报名',
}

export const participantStatuses: Record<string, string> = {
  normal: '有效', disqualified: '已取消资格', withdrawn: '已退出',
}

export const userRoles: Record<string, string> = {
  user: '普通用户', admin: '管理员', super_admin: '超级管理员',
}

export const userStatuses: Record<string, string> = {
  normal: '正常', disabled: '禁用', banned: '封禁', deleted: '已删除',
}

export function labelOf(map: Record<string, string>, value: string | undefined): string {
  if (!value) return '—'
  return map[value] ?? value
}
