import { useEffect, useState } from 'react'
import { BrowserRouter, Link, NavLink, Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import { ScopeProvider, useApi, useSession } from './context'
import { isAdmin } from '../api/client'
import { AccountPage, LoginPage } from '../features/auth/auth'
import { ErrorNotice } from '../components/shared'
import { ServiceHallIllustration, ServiceIcon, type ServiceIconName } from '../components/service-icon'

function Layout() {
  const api = useApi()
  const session = useSession()
  const [error, setError] = useState<unknown>()
  const location = useLocation()
  const admin = api.scope === 'admin'
  const base = admin ? '/admin' : ''
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      if (location.hash) document.getElementById(location.hash.slice(1))?.scrollIntoView()
      else window.scrollTo(0, 0)
    })
    return () => cancelAnimationFrame(frame)
  }, [location.pathname, location.hash])
  return <div className={`site-shell${admin ? ' admin-shell' : ''}`}><a className="skip-link" href="#main">跳到主要内容</a>
    <div className="utility-bar"><div className="site-width"><span>欢迎访问 GCOJ 在线评测服务平台</span><span>规范 · 便捷 · 可信</span></div></div>
    <header className="site-header"><div className="site-width masthead">
      <Link className="brand" to={base || '/'}><span className="brand-mark"><ServiceIcon name="building" /></span><span><strong>GCOJ <span>{admin ? '管理服务中心' : '在线评测服务中心'}</span></strong><small>GO COMPETITIVE ONLINE JUDGE</small></span></Link>
      <div className="header-account"><ServiceIcon name="user" />{session ? <button className="text-button" onClick={async () => { setError(undefined); try { await api.logout() } catch (failure) { setError(failure) } }}>退出 {session.user.username}</button> : <Link to={`${base}/login`}>{admin ? '管理员登录' : '登录 / 注册'}</Link>}</div>
    </div><div className="navigation-bar"><nav className="site-width" aria-label="主要导航">
      <NavLink end to={base || '/'}>服务首页</NavLink><Link to={`${base}/#services`}>服务事项</Link><Link to={`${base}/#guide`}>办事指南</Link>
      {session && <NavLink to={`${base}/account`}>我的账号</NavLink>}
      {!admin && isAdmin(session?.user) && <NavLink to="/admin">管理工作台</NavLink>}
      {admin && <Link to="/">返回用户端</Link>}
      <span className="nav-caption">{admin ? '管理端 · 独立认证' : '一站式在线服务大厅'}</span>
    </nav></div></header>
    <main id="main" tabIndex={-1}><ErrorNotice error={error} /><Outlet /></main>
    <footer className="site-footer"><div className="site-width footer-content"><div className="footer-brand"><ServiceIcon name="building" /><div><strong>GCOJ 在线评测服务中心</strong><p>Go Competitive Online Judge</p></div></div><div><p>统一入口 · 安全认证 · 在线办理</p><p>本平台为在线评测项目，非政府机构网站。</p></div></div></footer>
  </div>
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
  const base = admin ? '/admin' : ''
  const accountPath = session ? `${base}/account` : `${base}/login?returnTo=${encodeURIComponent(`${base}/account`)}`
  const services: { title: string; description: string; icon: ServiceIconName; to?: string }[] = [
    { title: admin ? '管理账号' : '个人信息服务', description: '查看账号资料，维护个人信息', icon: 'user', to: accountPath },
    { title: '账号安全服务', description: '修改登录密码，验证并更换邮箱', icon: 'shield', to: accountPath },
    { title: admin ? '题目管理' : '题库浏览', description: admin ? '题目维护、测试数据与发布' : '浏览题目内容，开展算法练习', icon: 'book' },
    { title: admin ? '评测管理' : '代码评测', description: admin ? '提交查询与重判流程' : '提交程序代码，获取评测结果', icon: 'code' },
    { title: admin ? '比赛管理' : '赛事服务', description: admin ? '比赛编排、报名审核与榜单' : '了解赛事安排，报名参与比赛', icon: 'trophy' },
    { title: admin ? '提交管理' : '提交查询', description: admin ? '查询提交记录与评测详情' : '查询提交记录，查看评测详情', icon: 'list' },
  ]
  return <><section className="hero"><div className="hero-copy"><p className="eyebrow"><span />{admin ? '管理服务 · 统一工作平台' : '便捷服务 · 从这里开始'}</p>
    <h1>{admin ? '管理有序，服务有章。' : '一站式在线评测服务'}</h1>
    <p>{admin ? '集中办理账号事务，清晰掌握服务建设进度。' : '让每一次专注，都有清晰的起点。'}<br />{admin ? '规范管理流程，守护平台安全。' : '账号管理、算法练习、赛事参与，一个入口有序办理。'}</p>
    <div className="hero-actions"><Link className="button" to={accountPath}>{session ? '进入个人中心' : '登录服务大厅'}<ServiceIcon name="arrow" /></Link><a className="hero-guide" href="#guide">查看办事指南 <span aria-hidden="true">›</span></a></div>
    <div className="hero-meta"><span>统一身份认证</span><span>用户与管理权限分离</span></div>
  </div><ServiceHallIllustration /></section>
    <div className="announcement"><span className="announcement-label"><ServiceIcon name="notice" />服务提示</span><p>当前已开放账号与安全服务，题库、评测及赛事页面建设中，暂不提供办理入口。</p><a href="#services">查看事项 <span aria-hidden="true">›</span></a></div>
    <div className="portal-columns"><section id="services" className="service-section" aria-labelledby="services-title"><div className="section-heading"><h2 id="services-title">{admin ? '管理服务事项' : '在线服务事项'}</h2><span>按需选择 · 分类办理</span></div>
      <div className="service-grid">{services.map((service) => {
        const content = <><div className="service-card-top"><span className="service-icon-box"><ServiceIcon name={service.icon} /></span><span className={`service-status${service.to ? ' available' : ''}`}>{service.to ? '已开放' : '建设中'}</span></div><h3>{service.title}</h3><p>{service.description}</p><span className="service-card-action">{service.to ? '立即办理' : '暂未开放'}{service.to && <ServiceIcon name="arrow" />}</span></>
        return service.to ? <Link className="service-card" key={service.title} to={service.to}>{content}</Link> : <article className="service-card unavailable" key={service.title}>{content}</article>
      })}</div>
    </section><aside id="guide" className="guide-panel" aria-labelledby="guide-title"><div className="section-heading"><h2 id="guide-title">办事指南</h2><ServiceIcon name="book" /></div><p className="guide-intro">首次使用？按以下步骤开始办理。</p>
      <ol className="service-steps"><li><span>01</span><div><h3>{admin ? '登录管理账号' : '注册并登录'}</h3><p>{admin ? '使用具备管理权限的账号，独立登录管理端。' : '使用用户名、邮箱密码或邮箱验证码完成登录。'}</p></div></li><li><span>02</span><div><h3>选择服务事项</h3><p>查看开放状态，进入所需服务页面。</p></div></li><li><span>03</span><div><h3>填写并确认</h3><p>按提示填写信息，提交后查看办理结果。</p></div></li></ol>
      <div className="guide-note"><ServiceIcon name="shield" /><div><strong>账号安全提醒</strong><p>请勿向他人提供密码或验证码。公共设备使用完毕后，请及时退出。</p></div></div>
    </aside></div>
    <section className="service-standard"><div><span className="eyebrow">服务说明</span><h2>办事有指引，服务有边界</h2></div><p>仅开放已接入真实接口的服务事项。<br />未上线功能不提供模拟办理或虚构结果。</p><Link to={admin ? '/login' : '/admin/login'}>{admin ? '前往用户登录' : '管理人员入口'}<ServiceIcon name="arrow" /></Link></section>
  </>
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
