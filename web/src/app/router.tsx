import { useState } from 'react'
import { BrowserRouter, Link, NavLink, Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import { ScopeProvider, useApi, useSession } from './context'
import { isAdmin } from '../api/client'
import { AccountPage, LoginPage } from '../features/auth/auth'
import { ErrorNotice } from '../components/shared'

function Layout() {
  const api = useApi()
  const session = useSession()
  const [error, setError] = useState<unknown>()
  const admin = api.scope === 'admin'
  const base = admin ? '/admin' : ''
  return <><a className="skip-link" href="#main">跳到主要内容</a><header>
    <Link className="brand" to={base || '/'}><span className="brand-mark">G</span> GCOJ <small>{admin ? '管理工作台' : '在线评测'}</small></Link>
    <nav aria-label="主要导航"><NavLink end to={base || '/'}>首页</NavLink>{session && <NavLink to={`${base}/account`}>我的账号</NavLink>}
      {!admin && isAdmin(session?.user) && <NavLink to="/admin">管理工作台</NavLink>}
      {session ? <button className="secondary" onClick={async () => { setError(undefined); try { await api.logout() } catch (failure) { setError(failure) } }}>退出 {session.user.username}</button> : <NavLink to={`${base}/login`}>登录 / 注册</NavLink>}
    </nav></header><main id="main"><ErrorNotice error={error} /><Outlet /></main><footer>Go Competitive Online Judge · 只展示真实接口支持的能力</footer></>
}

function Guard() {
  const api = useApi()
  const session = useSession()
  const location = useLocation()
  if (!session) return <Navigate replace to={`${api.scope === 'admin' ? '/admin' : ''}/login?returnTo=${encodeURIComponent(location.pathname + location.search)}`} />
  if (api.scope === 'admin' && !isAdmin(session.user)) return <section role="alert"><h1>没有管理权限</h1><p>此账号不能访问管理工作台。</p></section>
  return <Outlet />
}

function Home() {
  const api = useApi()
  const session = useSession()
  const admin = api.scope === 'admin'
  return <><section className="hero"><p className="eyebrow">{admin ? 'CONTROL CENTER' : 'THINK. CODE. IMPROVE.'}</p>
    <h1>{admin ? '专注于每一次可靠交付。' : <>让思考，<br />变成通过的代码。</>}</h1>
    <p>{admin ? '账号管理已接入真实 API。题目与比赛管理正在逐项建设，未完成能力不提供操作按钮。' : '从一道题开始，打磨算法与工程能力。支持 C++20 与 Go 的安全隔离评测。'}</p>
    <Link className="button" to={session ? (admin ? '/admin/account' : '/account') : (admin ? '/admin/login' : '/login')}>{session ? '管理我的账号' : '登录开始'}</Link>
  </section><div className="cards"><section className="panel"><span className="number">01</span><h2>真实身份</h2><p>统一认证，用户与管理员分离；个人信息随会话隔离。</p></section>
    <section className="panel"><span className="number">02</span><h2>可信评测</h2><p>C++20 / Go 官方语言镜像，测试数据版本与评测 Attempt 可追踪。</p></section>
    <section className="panel"><span className="number">03</span><h2>建设状态透明</h2><p>当前交付认证基础。做题与比赛页面尚未接入，不展示模拟结果或虚构统计。</p></section></div></>
}

export function Router() {
  return <BrowserRouter><Routes>
    <Route path="/admin" element={<ScopeProvider scope="admin"><Layout /></ScopeProvider>}>
      <Route path="login" element={<LoginPage />} /><Route element={<Guard />}><Route index element={<Home />} /><Route path="account" element={<AccountPage />} /></Route>
    </Route>
    <Route path="/" element={<ScopeProvider scope="app"><Layout /></ScopeProvider>}>
      <Route index element={<Home />} /><Route path="login" element={<LoginPage />} /><Route element={<Guard />}><Route path="account" element={<AccountPage />} /></Route>
      <Route path="*" element={<section><h1>页面不存在</h1><Link to="/">返回首页</Link></section>} />
    </Route>
  </Routes></BrowserRouter>
}
