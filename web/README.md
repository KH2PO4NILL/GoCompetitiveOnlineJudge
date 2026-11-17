# GCOJ Web

GoCompetitiveOnlineJudge 的浏览器前端，使用 **Vue 3 + TypeScript + Vite** 重建，生产页面直连现有 App / Admin HTTP API，不提供 Mock 业务后端，不虚构题目、通过率或排行榜数据。

## 技术栈

| 分类 | 依赖 |
|---|---|
| 框架 | Vue 3.5（Composition API + `<script setup lang="ts">`） |
| 路由 | vue-router 5（用户端与管理端独立路由树 + 权限守卫） |
| 服务端状态 | @tanstack/vue-query 5（缓存、轮询、失效） |
| 客户端状态 | Pinia 4（仅最小化 UI 状态，不复制服务端缓存） |
| 会话 / 请求 | 框架无关的 `ApiClient`（Access/Refresh 轮换、刷新单飞、epoch、精确 int64） |
| 题面渲染 | markdown-it 15 + markdown-it-katex（GFM + KaTeX，禁原始 HTML） |
| 精确 JSON | lossless-json（64 位 ID 不经不安全 `number` 转换） |
| 构建 / 测试 | Vite 8、vue-tsc、Vitest 5、Playwright |

依赖锁定版本并提交 `package-lock.json`。

## 目录结构

```text
web/
├── scripts/contracts.mjs        # 从 .api 生成 TS DTO，--check 检测漂移
├── src/
│   ├── api/
│   │   ├── client.ts / json.ts  # 框架无关：会话、刷新单飞、epoch、精确 int64
│   │   ├── json.ts              # 精确数字 JSON
│   │   └── contracts/           # 生成的 DTO（app / admin）
│   ├── core/
│   │   ├── runtime.ts           # 按 scope 创建 ApiClient + QueryClient
│   │   ├── ScopeProvider.vue    # 注入 runtime 与 QueryClient，按 scope 重挂载
│   │   ├── routes.ts            # 用户端 / 管理端两棵路由树
│   │   ├── guards.ts            # 登录守卫与 returnTo
│   │   └── keys.ts              # inject key
│   ├── components/
│   │   ├── layout/              # SiteLayout / AdminLayout
│   │   ├── ui/                  # Badge / StateBlock / ConfirmDialog / Pager / MarkdownView …
│   │   └── data/                # 领域展示组件
│   ├── composables/             # useApi / useSession / useAction / useCountdown / useCodeDraft
│   ├── lib/                     # ids / time / labels / judge / errors / markdown / upload / forms
│   ├── stores/                  # Pinia（ui.ts，仅导航等客户端状态）
│   ├── features/                # home / auth / problems / submit / submissions / contests / admin
│   ├── styles/                  # tokens / base / layout / ui
│   ├── main.ts / App.vue        # 入口
│   └── env.d.ts / shims.d.ts    # 环境与第三方声明
└── tests/browser/               # Playwright 浏览器回归
```

## 开发与检查

需要 Node `>=22.13 <23` 或 `>=24`。

```sh
cd web
npm ci
npm run dev               # 开发服务 http://127.0.0.1:5173，仅监听 localhost
npm run typecheck         # vue-tsc 严格类型检查
npm run lint              # eslint（typescript-eslint + eslint-plugin-vue）
npm test                  # Vitest 单元测试
npm run contracts:check   # 契约漂移检查
npm run check             # 契约 + 类型 + lint + 单测 + 构建，全量门禁
npx playwright install chromium
npm run test:browser      # Playwright 浏览器回归
```

Vite 优先代理 `/api/v1/admin`（8001）再代理 `/api/v1`（8002）；前端不读取根 `.env`，不用 `VITE_*` 保存密钥。`preview` 仅检查生产静态产物，不提供 API 代理。

## 页面与路由

用户端与管理端共用一个 SPA 外壳，但路由、会话、查询缓存严格隔离（各自持独立 `ApiClient` + `QueryClient`）。

**用户端（App）**：首页 `/`、题库 `/problems`、题面 `/problems/:problemId`、提交 `/submit` 与 `/problems/:problemId/submit`、评测记录 `/submissions`、评测详情 `/submissions/:submissionId`、个人中心 `/account`、登录 `/login`，以及比赛 `/contests`（概览 / 题目 / 工作区 / 排行榜）。

**管理端（Admin）**：`/admin` 工作台、题目管理 `/admin/problems`（新建 / 工作台 / 测试数据）、提交与重判 `/admin/submissions`、比赛管理 `/admin/contests`（新建 / 工作台 / 报名审核 / 参赛名单）、用户管理 `/admin/users`。

## 状态管理边界

- **服务端数据**全部走 TanStack Query for Vue；筛选 / 分页状态进 URL，可刷新与前进后退。
- **会话**由框架无关的 `ApiClient` 管理（Access/Refresh Token 在内存，刷新单飞 + epoch），不放 Pinia、不落 localStorage / sessionStorage。浏览器另用两端隔离的 HttpOnly、SameSite=Strict Cookie 保存独立的可撤销浏览器会话凭据；HTTPS 下设置 Secure，登录与刷新要求同源，页面刷新后先签发新 Token 再加载当前用户。
- **Pinia** 只承载最小化客户端 UI 状态；用户端与管理端不得复用会话、缓存或权限状态。

## 保留的安全与数据约束

- 精确 `int64`：JSON 解析用 lossless-json，序列化仍发 JSON 数字，不改后端契约。
- revision 乐观锁：编辑冲突（409）保留未保存内容，显式重新加载才丢弃。
- 非幂等写请求：结果未知时不自动重试；提交用 `client_request_id` 幂等，仅允许同 id 手动安全重试。
- 判题轮询：最多自动刷新五分钟，可手动恢复；终态 / 出错 / 页面卸载即停，后台标签不持续轮询。
- Markdown：`html: false` 禁原始 HTML，图片仅显示替代文本（不加载远程），链接 `rel="noopener noreferrer"` + `referrerpolicy="no-referrer"`。
- 危险操作（删除、发布、重判、权限修改）保留确认；批量重判只作用于当前页勾选项，切换条件清空选择。
- 不在日志、持久化或错误界面泄露 Token、验证码、完整源码与预签名地址。

## 测试

- **Vitest 单测**：覆盖 ApiClient 契约、Markdown 安全渲染等，`npm test`。
- **Playwright 浏览器回归**：`tests/browser/*.spec.ts`，覆盖认证、题库、提交、比赛与管理端关键路径及 320 / 390 / 640 / 768 / 1024 / 1440 像素无核心横向溢出。

## 迁移状态

React 前端已完整重写为 Vue 3：React 源码、依赖与构建配置均已移除，`web/` 现在只包含 Vue 工程。单测、类型检查、lint、构建与 Playwright 浏览器回归均通过。真实后端联调需按 `playwright.config.ts` 的说明单独启动隔离环境（`GCOJ_AUTH_E2E=1`），默认浏览器回归使用请求级 mock，不依赖本地后端。

## 认证与账户管理

仅 `access_token_invalid` 会触发刷新并重放一次；业务 401 保持会话。刷新网络故障或 5xx 保留现有会话，确认为 `refresh_token_invalid`、`session_invalid` 或 `account_unavailable` 才清理并显示重新登录提示。登录切换成功后才替换原会话。支持 Web Locks 的浏览器会跨标签串行刷新，浏览器凭据独立于一次性 Refresh Token，因此导航中断或多标签刷新不会使 Cookie 作废；API Refresh Token 仍原子轮换并检测重放。

超级管理员可以在普通用户详情修改用户名、邮箱、学校或重置密码；留空密码不修改，学校 0 清空。保存前确认，结果未知时阻止直接重试。后端独立检查权限并在事务中锁定双方、更新账户和撤销会话。

`bash scripts/test-web-auth.sh` 启动一次性 MySQL/Redis 和 HTTP/RPC 服务，执行真实 Auth 数据层及浏览器回归，包含短期令牌过期、三种登录、页面刷新恢复、修改邮箱错误和管理员密码重置。退出时删除临时数据与凭据。
