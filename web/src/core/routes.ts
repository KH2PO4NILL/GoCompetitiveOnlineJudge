import type { RouteRecordRaw } from 'vue-router'
import SiteLayout from '../components/layout/SiteLayout.vue'
import AdminLayout from '../components/layout/AdminLayout.vue'
// 首屏外壳与入口页静态导入，避免懒加载造成首帧导航缺失
import HomePage from '../features/home/HomePage.vue'
import LoginPage from '../features/auth/LoginPage.vue'
import NotFoundPage from '../features/NotFoundPage.vue'

const appChildren: RouteRecordRaw[] = [
  { path: '', name: 'home', component: HomePage, meta: { title: '首页' } },
  { path: 'login', name: 'login', component: LoginPage, meta: { title: '登录' } },
  { path: 'problems', name: 'problems', component: () => import('../features/problems/ProblemListPage.vue'), meta: { title: '题库' } },
  { path: 'problems/:problemId', name: 'problem', component: () => import('../features/problems/ProblemDetailPage.vue'), meta: { title: '题目' } },
  { path: 'problems/:problemId/submit', name: 'problem-submit', component: () => import('../features/submit/SubmitPage.vue'), meta: { title: '提交代码', requiresAuth: true } },
  { path: 'submit', name: 'submit', component: () => import('../features/submit/SubmitPage.vue'), meta: { title: '提交代码', requiresAuth: true } },
  { path: 'submissions', name: 'submissions', component: () => import('../features/submissions/SubmissionListPage.vue'), meta: { title: '评测记录', requiresAuth: true } },
  { path: 'submissions/:submissionId', name: 'submission', component: () => import('../features/submissions/SubmissionDetailPage.vue'), meta: { title: '评测详情', requiresAuth: true } },
  { path: 'contests', name: 'contests', component: () => import('../features/contests/ContestListPage.vue'), meta: { title: '比赛' } },
  {
    path: 'contests/:contestId',
    component: () => import('../features/contests/ContestLayout.vue'),
    children: [
      { path: '', name: 'contest-overview', component: () => import('../features/contests/ContestOverviewPage.vue'), meta: { title: '比赛概览' } },
      { path: 'problems', name: 'contest-problems', component: () => import('../features/contests/ContestProblemsPage.vue'), meta: { title: '比赛题目' } },
      { path: 'problems/:problemId', name: 'contest-problem', component: () => import('../features/contests/ContestWorkspacePage.vue'), meta: { title: '比赛工作区', requiresAuth: true } },
      { path: 'scoreboard', name: 'contest-scoreboard', component: () => import('../features/contests/ContestScoreboardPage.vue'), meta: { title: '排行榜' } },
    ],
  },
  { path: 'account', name: 'account', component: () => import('../features/auth/AccountPage.vue'), meta: { title: '个人信息', requiresAuth: true } },
  { path: ':pathMatch(.*)*', name: 'app-not-found', component: NotFoundPage, meta: { title: '页面不存在' } },
]

const adminChildren: RouteRecordRaw[] = [
  { path: '', name: 'admin-home', component: () => import('../features/admin/AdminHomePage.vue'), meta: { title: '管理工作台', requiresAuth: true } },
  { path: 'login', name: 'admin-login', component: LoginPage, meta: { title: '管理员登录' } },
  { path: 'account', name: 'admin-account', component: () => import('../features/auth/AccountPage.vue'), meta: { title: '我的账号', requiresAuth: true } },
  { path: 'problems', name: 'admin-problems', component: () => import('../features/admin/problems/ProblemManageListPage.vue'), meta: { title: '题目管理', requiresAuth: true } },
  { path: 'problems/new', name: 'admin-problem-new', component: () => import('../features/admin/problems/ProblemCreatePage.vue'), meta: { title: '创建题目', requiresAuth: true } },
  { path: 'problems/:problemId', name: 'admin-problem', component: () => import('../features/admin/problems/ProblemWorkbenchPage.vue'), meta: { title: '题目工作台', requiresAuth: true } },
  { path: 'problems/:problemId/test-data', name: 'admin-problem-test-data', component: () => import('../features/admin/testdata/TestDataPage.vue'), meta: { title: '测试数据', requiresAuth: true } },
  { path: 'submissions', name: 'admin-submissions', component: () => import('../features/admin/submissions/SubmissionManageListPage.vue'), meta: { title: '提交与重判', requiresAuth: true } },
  { path: 'submissions/:submissionId', name: 'admin-submission', component: () => import('../features/admin/submissions/SubmissionManageDetailPage.vue'), meta: { title: '提交详情', requiresAuth: true } },
  { path: 'contests', name: 'admin-contests', component: () => import('../features/admin/contests/ContestManageListPage.vue'), meta: { title: '比赛管理', requiresAuth: true } },
  { path: 'contests/new', name: 'admin-contest-new', component: () => import('../features/admin/contests/ContestCreatePage.vue'), meta: { title: '创建比赛', requiresAuth: true } },
  { path: 'contests/:contestId', name: 'admin-contest', component: () => import('../features/admin/contests/ContestWorkbenchPage.vue'), meta: { title: '比赛工作台', requiresAuth: true } },
  { path: 'contests/:contestId/registrations', name: 'admin-contest-registrations', component: () => import('../features/admin/contests/RegistrationReviewPage.vue'), meta: { title: '报名审核', requiresAuth: true } },
  { path: 'contests/:contestId/participants', name: 'admin-contest-participants', component: () => import('../features/admin/contests/ParticipantListPage.vue'), meta: { title: '参赛名单', requiresAuth: true } },
  { path: 'users', name: 'admin-users', component: () => import('../features/admin/users/UserListPage.vue'), meta: { title: '用户管理', requiresAuth: true } },
  { path: 'users/:userId', name: 'admin-user', component: () => import('../features/admin/users/UserDetailPage.vue'), meta: { title: '用户详情', requiresAuth: true } },
  { path: ':pathMatch(.*)*', name: 'admin-not-found', component: NotFoundPage, meta: { title: '管理页面不存在' } },
]

export const routes: RouteRecordRaw[] = [
  { path: '/', component: SiteLayout, meta: { scope: 'app' }, children: appChildren },
  { path: '/admin', component: AdminLayout, meta: { scope: 'admin' }, children: adminChildren },
]
