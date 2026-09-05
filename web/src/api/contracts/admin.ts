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

export type ResetPasswordReq = {
  email: string
  code: string
  new_password: string
}

export type ChangePasswordReq = {
  current_password: string
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

export type ListUsersReq = {
  page?: number
  page_size?: number
  status?: string
  role?: string
  school_id?: Int64
  keyword?: string
}

export type ListUsersResp = {
  total: Int64
  users: UserInfo[]
}

export type GetUserReq = {
  user_id: Int64
}

export type GetUserResp = {
  user: UserInfo
}

export type AdminUpdateUserReq = {
  user_id: Int64
  role?: string
  status?: string
}

export type AdminUpdateUserResp = {
  user: UserInfo
}

export type DeleteUserReq = {
  user_id: Int64
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

export type JudgeConfig = {
  settings: JudgeSettings
  test_data_version: string
  checker_file_key: string
  interactor_file_key: string
}

export type ProblemEditableFields = {
  statement: ProblemStatement
  difficulty: string
  tags: string[]
  source: string
  judge_settings: JudgeSettings
  visibility: string
}

export type ProblemStatementPatch = {
  title?: string
  description?: string
  input_description?: string
  output_description?: string
  samples?: ProblemSample[]
  hint?: string
}

export type JudgeSettingsPatch = {
  time_limit_ms?: number
  memory_limit_mb?: number
  allowed_languages?: string[]
  checker_type?: string
}

export type ProblemPatch = {
  statement?: ProblemStatementPatch
  difficulty?: string
  tags?: string[]
  source?: string
  judge_settings?: JudgeSettingsPatch
  visibility?: string
}

export type Problem = {
  problem_id: Int64
  statement: ProblemStatement
  metadata: ProblemMetadata
  judge_config: JudgeConfig
  status: string
  visibility: string
  revision: Int64
  created_at?: string
  updated_at?: string
  published_at?: string
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

export type TestCaseInfo = {
  test_case_id: Int64
  name: string
  score: number
  ordinal: number
  enabled: boolean
  group_name: string
}

export type ListProblemsReq = {
  page?: number
  page_size?: number
  keyword?: string
  tags?: string[]
  difficulty?: string
  status?: string
  visibility?: string
  author_id?: Int64
}

export type ListProblemsResp = {
  total: Int64
  problems: ProblemSummary[]
}

export type CreateProblemReq = {
  problem: ProblemEditableFields
}

export type CreateProblemResp = {
  problem: Problem
}

export type GetProblemForEditReq = {
  problem_id: Int64
}

export type GetProblemForEditResp = {
  problem: Problem
  test_cases: TestCaseInfo[]
}

export type UpdateProblemReq = {
  problem_id: Int64
  problem: ProblemPatch
  update_mask: string[]
  expected_revision: Int64
}

export type UpdateProblemResp = {
  problem: Problem
}

export type PublishProblemReq = {
  problem_id: Int64
  expected_revision: Int64
}

export type PublishProblemResp = {
  problem: Problem
}

export type ArchiveProblemReq = {
  problem_id: Int64
  expected_revision: Int64
}

export type ArchiveProblemResp = {
  problem: Problem
}

export type DeleteProblemReq = {
  problem_id: Int64
  expected_revision: Int64
}

export type DeleteProblemResp = {
  revision: Int64
}

export type RestoreProblemReq = {
  problem_id: Int64
  expected_revision: Int64
}

export type RestoreProblemResp = {
  problem: Problem
}

export type CreateTestDataUploadReq = {
  problem_id: Int64
  file_name: string
  content_length: Int64
  sha256: string
}

export type CreateTestDataUploadResp = {
  upload_id: string
  upload_url: string
  expires_at?: string
}

export type FinalizeTestDataUploadReq = {
  problem_id: Int64
  upload_id: string
  sha256: string
  expected_revision: Int64
}

export type FinalizeTestDataUploadResp = {
  test_data_version: string
  revision: Int64
  test_cases: TestCaseInfo[]
}

export type ContestEditableFields = {
  title: string
  description: string
  mode: string
  start_at: string
  end_at: string
  registration_start_at?: string
  registration_end_at?: string
  registration_policy: string
  visibility: string
  scoreboard_visibility: string
  scoreboard_freeze_at?: string
  scoreboard_freeze_before_end_seconds?: Int64
  invite_code?: string
}

export type ContestPatch = {
  title?: string
  description?: string
  mode?: string
  start_at?: string
  end_at?: string
  registration_start_at?: string
  registration_end_at?: string
  registration_policy?: string
  visibility?: string
  scoreboard_visibility?: string
  scoreboard_freeze_at?: string
  scoreboard_freeze_before_end_seconds?: Int64
  invite_code?: string
}

export type Contest = {
  contest_id: Int64
  creator_id: Int64
  editable: ContestEditableFields
  status: string
  revision: Int64
  created_at?: string
  updated_at?: string
  published_at?: string
  cancelled_at?: string
  archived_at?: string
  deleted_at?: string
  scoreboard_frozen: boolean
  scoreboard_frozen_at?: string
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

export type ContestProblem = {
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

export type CreateContestReq = {
  contest: ContestEditableFields
}

export type CreateContestResp = {
  contest: Contest
}

export type GetContestForEditReq = {
  contest_id: Int64
}

export type GetContestForEditResp = {
  contest: Contest
  problems: ContestProblem[]
}

export type UpdateContestReq = {
  contest_id: Int64
  contest: ContestPatch
  update_mask: string[]
  expected_revision: Int64
}

export type UpdateContestResp = {
  contest: Contest
}

export type PublishContestReq = {
  contest_id: Int64
  expected_revision: Int64
}

export type PublishContestResp = {
  contest: Contest
}

export type CancelContestReq = {
  contest_id: Int64
  expected_revision: Int64
  reason: string
}

export type CancelContestResp = {
  contest: Contest
}

export type ArchiveContestReq = {
  contest_id: Int64
  expected_revision: Int64
}

export type ArchiveContestResp = {
  contest: Contest
}

export type DeleteContestReq = {
  contest_id: Int64
  expected_revision: Int64
}

export type DeleteContestResp = {
  revision: Int64
}

export type RestoreContestReq = {
  contest_id: Int64
  expected_revision: Int64
}

export type RestoreContestResp = {
  contest: Contest
}

export type SetContestProblemsReq = {
  contest_id: Int64
  problems: ContestProblem[]
  expected_revision: Int64
}

export type SetContestProblemsResp = {
  contest: Contest
  problems: ContestProblem[]
}

export type ListRegistrationsReq = {
  contest_id: Int64
  page?: number
  page_size?: number
  status?: string
}

export type ListRegistrationsResp = {
  total: Int64
  registrations: ContestRegistration[]
}

export type ReviewRegistrationReq = {
  contest_id: Int64
  registration_id: Int64
  decision: string
  review_note?: string
  expected_revision: Int64
}

export type ReviewRegistrationResp = {
  registration: ContestRegistration
  participant?: ContestParticipant
}

export type ListParticipantsReq = {
  contest_id: Int64
  page?: number
  page_size?: number
  status?: string
}

export type ListParticipantsResp = {
  total: Int64
  participants: ContestParticipant[]
}

export type FreezeScoreboardReq = {
  contest_id: Int64
  expected_revision: Int64
}

export type FreezeScoreboardResp = {
  contest: Contest
}

export type UnfreezeScoreboardReq = {
  contest_id: Int64
  expected_revision: Int64
}

export type UnfreezeScoreboardResp = {
  contest: Contest
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

export type ListSubmissionsReq = {
  page?: number
  page_size?: number
  user_id?: Int64
  problem_id?: Int64
  contest_id?: Int64
  status?: string
  verdict?: string
  language?: string
}

export type ListSubmissionsResp = {
  total: Int64
  submissions: SubmissionSummary[]
}

export type GetSubmissionReq = {
  submission_id: Int64
}

export type GetSubmissionResp = {
  submission: SubmissionDetail
}

export type RejudgeSubmissionReq = {
  submission_id: Int64
  reason?: string
}

export type RejudgeSubmissionResp = {
  submission: SubmissionSummary
  attempt_id: Int64
}

export type BatchRejudgeSubmissionsReq = {
  submission_ids: Int64[]
  reason?: string
}

export type BatchRejudgeItem = {
  submission_id: Int64
  attempt_id: Int64
}

export type BatchRejudgeSubmissionsResp = {
  accepted: BatchRejudgeItem[]
}
