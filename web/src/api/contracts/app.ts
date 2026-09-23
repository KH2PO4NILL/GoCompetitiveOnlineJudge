export type Int64 = number | bigint

export type EmptyResp = {

}

export type HealthResp = {
  status: string
}

export type UserInfo = {
  user_id: Int64
  username: string
  email: string
  role: string
  status: string
  school_id: Int64
}

export type TokenPair = {
  access_token: string
  refresh_token: string
  expires_in: Int64
}

export type UsernameLoginReq = {
  username: string
  password: string
}

export type EmailPasswordLoginReq = {
  email: string
  password: string
}

export type EmailCodeLoginReq = {
  email: string
  code: string
}

export type LoginResp = {
  tokens: TokenPair
  user: UserInfo
}

export type EmailRegisterReq = {
  email: string
  username: string
  password: string
  code: string
  school_id?: Int64
}

export type RegisterResp = {
  user: UserInfo
}

export type SendEmailCodeReq = {
  email: string
  purpose: string
}

export type SendEmailCodeResp = {
  expires_in: number
  retry_after: number
}

export type RefreshTokenReq = {
  refresh_token: string
}

export type RefreshTokenResp = {
  tokens: TokenPair
}

export type LogoutReq = {
  access_token?: string
  refresh_token?: string
}

export type ChangePasswordReq = {
  current_password: string
  new_password: string
}

export type ResetPasswordReq = {
  email: string
  code: string
  new_password: string
}

export type GetCurrentUserResp = {
  user: UserInfo
}

export type UpdateProfileReq = {
  username?: string
  school_id?: Int64
}

export type UpdateProfileResp = {
  user: UserInfo
}

export type ChangeEmailReq = {
  new_email: string
  code: string
}

export type ChangeEmailResp = {
  user: UserInfo
}

export type ProblemSample = {
  input: string
  output: string
  explanation: string
}

export type ProblemStatement = {
  title: string
  description: string
  input_description: string
  output_description: string
  samples: ProblemSample[]
  hint: string
}

export type ProblemMetadata = {
  difficulty: string
  tags: string[]
  source: string
  author_id: Int64
}

export type JudgeSettings = {
  time_limit_ms: number
  memory_limit_mb: number
  allowed_languages: string[]
  checker_type: string
}

export type PracticeProblemInfo = {
  problem_id: Int64
  statement: ProblemStatement
  metadata: ProblemMetadata
  judge_settings: JudgeSettings
  revision: Int64
}

export type ProblemSummary = {
  problem_id: Int64
  title: string
  difficulty: string
  tags: string[]
  status: string
  visibility: string
  author_id: Int64
  updated_at?: string
  revision: Int64
}

export type GetProblemReq = {
  problem_id: Int64
  contest_id?: Int64
}

export type GetProblemResp = {
  problem: PracticeProblemInfo
}

export type ListProblemsReq = {
  page?: number
  page_size?: number
  keyword?: string
  tags?: string[]
  difficulty?: string
}

export type ListProblemsResp = {
  total: Int64
  problems: ProblemSummary[]
}

export type ContestSummary = {
  contest_id: Int64
  title: string
  mode: string
  status: string
  start_at?: string
  end_at?: string
  registration_policy: string
  visibility: string
  registration_open: boolean
  participant_count: Int64
}

export type ContestDetail = {
  contest_id: Int64
  title: string
  description: string
  mode: string
  status: string
  start_at?: string
  end_at?: string
  registration_start_at?: string
  registration_end_at?: string
  registration_policy: string
  visibility: string
  scoreboard_visibility: string
  registration_open: boolean
  scoreboard_frozen: boolean
  participant_count: Int64
}

export type ContestProblemSummary = {
  problem_id: Int64
  label: string
  display_order: number
  max_score: Int64
}

export type ContestRegistration = {
  registration_id: Int64
  contest_id: Int64
  user_id: Int64
  status: string
  application_note: string
  reviewer_id: Int64
  review_note: string
  revision: Int64
  created_at?: string
  reviewed_at?: string
  cancelled_at?: string
  updated_at?: string
}

export type ContestParticipant = {
  contest_id: Int64
  user_id: Int64
  status: string
  qualified_at?: string
  updated_at?: string
}

export type MyContestStatus = {
  contest_id: Int64
  registration?: ContestRegistration
  participant?: ContestParticipant
  can_view_problems: boolean
  can_submit: boolean
}

export type IcpcProblemResult = {
  solved: boolean
  wrong_attempts: number
  penalty_seconds: Int64
  accepted_at?: string
  first_to_solve: boolean
}

export type IoiProblemResult = {
  score: Int64
  max_score: Int64
  submission_count: number
  last_effective_submission_at?: string
}

export type OiProblemResult = {
  score: Int64
  max_score: Int64
  submission_count: number
  last_effective_submission_at?: string
}

export type ScoreboardProblemResult = {
  problem_id: Int64
  label: string
  icpc?: IcpcProblemResult
  ioi?: IoiProblemResult
  oi?: OiProblemResult
}

export type IcpcScore = {
  solved_count: number
  total_penalty_seconds: Int64
  last_solved_at?: string
}

export type IoiScore = {
  total_score: Int64
  last_effective_submission_at?: string
}

export type OiScore = {
  total_score: Int64
  last_effective_submission_at?: string
}

export type ScoreboardRow = {
  rank: Int64
  ordinal: Int64
  user_id: Int64
  display_name: string
  problem_results: ScoreboardProblemResult[]
  icpc?: IcpcScore
  ioi?: IoiScore
  oi?: OiScore
}

export type Scoreboard = {
  contest_id: Int64
  mode: string
  generated_at?: string
  frozen: boolean
  view: string
  total: Int64
  rows: ScoreboardRow[]
}

export type ListContestsReq = {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
  mode?: string
}

export type ListContestsResp = {
  total: Int64
  contests: ContestSummary[]
}

export type GetContestReq = {
  contest_id: Int64
}

export type GetContestResp = {
  contest: ContestDetail
}

export type RegisterContestReq = {
  contest_id: Int64
  invite_code?: string
  application_note?: string
}

export type RegisterContestResp = {
  registration: ContestRegistration
}

export type CancelRegistrationReq = {
  contest_id: Int64
  expected_revision: Int64
}

export type CancelRegistrationResp = {
  registration: ContestRegistration
}

export type GetMyContestStatusReq = {
  contest_id: Int64
}

export type GetMyContestStatusResp = {
  status: MyContestStatus
}

export type ListContestProblemsReq = {
  contest_id: Int64
  page?: number
  page_size?: number
}

export type ListContestProblemsResp = {
  total: Int64
  problems: ContestProblemSummary[]
}

export type GetScoreboardReq = {
  contest_id: Int64
  page?: number
  page_size?: number
}

export type GetScoreboardResp = {
  scoreboard: Scoreboard
}

export type SubmissionSummary = {
  submission_id: Int64
  user_id: Int64
  problem_id: Int64
  contest_id?: Int64
  language: string
  status: string
  verdict: string
  score: Int64
  max_score: Int64
  problem_revision: Int64
  attempt_count: number
  result_visibility: string
  submitted_at?: string
  judging_started_at?: string
  finished_at?: string
}

export type TestCaseResult = {
  test_case_id: Int64
  ordinal: number
  group_name: string
  verdict: string
  score: Int64
  time_used_ms: Int64
  memory_used_kb: Int64
}

export type SubmissionDetail = {
  summary: SubmissionSummary
  source_code: string
  compile_output: string
  case_results: TestCaseResult[]
}

export type CreateSubmissionReq = {
  problem_id: Int64
  contest_id?: Int64
  language: string
  source_code: string
  client_request_id: string
}

export type CreateSubmissionResp = {
  submission: SubmissionDetail
  deduplicated: boolean
}

export type GetSubmissionReq = {
  submission_id: Int64
}

export type GetSubmissionResp = {
  submission: SubmissionDetail
}

export type ListSubmissionsReq = {
  page?: number
  page_size?: number
  problem_id?: Int64
  contest_id?: Int64
  status?: string
  language?: string
}

export type ListSubmissionsResp = {
  total: Int64
  submissions: SubmissionSummary[]
}
