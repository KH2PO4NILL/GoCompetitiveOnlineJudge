import MarkdownIt from 'markdown-it'
import markdownItKatex from 'markdown-it-katex'
import 'katex/dist/katex.min.css'

// 题面为不可信内容：禁用原始 HTML，链接协议统一净化，图片不自动加载（防远程跟踪）。
const md = new MarkdownIt({ html: false, linkify: true, breaks: false })

md.use(markdownItKatex, { throwOnError: false })

// 放行所有协议进入渲染，由 link_open 规则统一清空危险协议，
// 保证 javascript: 等链接仍渲染为无害的 <a>（href=""），而非整体缺失。
md.validateLink = () => true

const defaultLinkOpen = md.renderer.rules.link_open
  ?? ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch]!)
}

function isDangerousProtocol(href: string): boolean {
  let decoded = href.trim().toLowerCase()
  try { decoded = decodeURIComponent(decoded) } catch { /* 保持原样 */ }
  return /^(javascript|vbscript|file|data):/.test(decoded)
}

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const href = String(tokens[idx].attrGet('href') ?? '')
  if (isDangerousProtocol(href)) tokens[idx].attrSet('href', '')
  tokens[idx].attrSet('target', '_blank')
  tokens[idx].attrSet('rel', 'noopener noreferrer')
  tokens[idx].attrSet('referrerpolicy', 'no-referrer')
  return defaultLinkOpen(tokens, idx, options, env, self)
}

md.renderer.rules.image = (tokens, idx) => {
  const alt = escapeHtml(tokens[idx].content || '未提供说明')
  return `<span class="muted">[图片：${alt}；为避免远程跟踪，不自动加载]</span>`
}

export function renderMarkdown(content: string): string {
  return md.render(content)
}
