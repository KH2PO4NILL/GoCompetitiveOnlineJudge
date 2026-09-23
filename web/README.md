# GCOJ Web

当前范围：React / TypeScript 工程、中文用户和管理布局、三个登录入口、邮箱注册、重置密码、个人资料、密码与邮箱修改。所有操作直连现有 API，不提供 Mock 业务后端。**前端构建和导航冒烟通过不等于 M1 真实认证或完整 GOAL 验收通过。**

## 开发与检查

使用 Node 22.13+（22 LTS 系列）或 24+，本次验证 Node 22.23.1 / npm 10.9.8。依赖固定版本、提交 package-lock；TypeScript 5.9.3 与 typescript-eslint 的 peer 范围匹配，不盲目升级至不兼容的大版本。

```sh
cd web
npm ci
npm run dev
npm run check
npx playwright install chromium
npm run test:browser
```

开发入口 `http://127.0.0.1:5173`；先按根 README 启动 App 8002 / Admin 8001 及其 RPC / 基础设施。Vite 优先代理 `/api/v1/admin`，再代理 `/api/v1`；前端不读取根 `.env`，不用 `VITE_*` 保存密钥。开发服务仅监听 localhost。`preview` 仅检查生产静态产物，不提供 API 代理；真实部署需配置同源反向代理。

`npm run contracts` 从两个 `.api` 的 type 声明生成 TypeScript DTO；修改后端契约后重新生成。`contracts:check` 检测漂移，不从 README 猜 DTO。新增语法未被生成器支持时必须显式扩展，不能跳过字段。

## 会话与安全取舍

- Access / Refresh Token 仅内存保存，无 localStorage、sessionStorage 或 Cookie。刷新页面、新标签页需要重新登录。不同标签页不共享 Token，所以不存在跨标签页共享 Refresh Token 竞争；关闭一个标签页不撤销另一个独立登录会话。
- App / Admin 使用独立 API 客户端和查询缓存；进入管理端不能复用用户端 Token。管理登录角色检查只改善界面，真实权限仍由 HTTP / RPC 执行。
- 同会话并发 401 共用一次刷新；旧 Token 的迟到 401 不重复轮换。刷新失败清空会话，退出立即清空本地缓存并取消请求，迟到响应不能恢复身份或前一用户数据。
- 所有请求有 30 秒超时，支持取消、不使用 Cookie、不跟随重定向；不自动重试网络失败和 5xx 写请求。鉴权明确拒绝的 401 最多在刷新后重发一次；结果未知要求核对而不是连续点击提交。
- `int64` 在 JSON 解析时保留为安全 number 或 bigint，序列化仍发送 JSON 数字，不修改后端契约；不要直接对接口响应使用原生 JSON.parse / JSON.stringify。
- 不记录 Token、密码、验证码或源码；浏览器测试默认关闭 trace / video / 自动截图，防止未来真实认证测试产物泄露凭据。
- `tests/browser/navigation.spec.ts` 仅验证真实前端导航、未登录管理守卫和移动布局，不 Mock API，也不声称验证真实登录。客户端竞争测试中的 transport fixture 仅用于单测。

## 待交付

题库 / 编辑器 / 提交、完整题目上传与管理、比赛与排行榜、用户角色管理、真实 SMTP 与浏览器业务 E2E、全栈启动和部署恢复均未在此基础项中验收。不得将本目录的存在或静态构建通过视为完整 GOAL 完成。
