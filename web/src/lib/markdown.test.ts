import { describe, expect, it } from 'vitest'
import { renderMarkdown } from './markdown'

describe('renderMarkdown 安全渲染', () => {
  it('不渲染原始 HTML', () => {
    const html = renderMarkdown('<script>alert(1)</script><img src=x onerror=alert(1)>')
    expect(html).not.toContain('<script>')
    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;script&gt;')
    expect(html).toContain('&lt;img')
  })

  it('净化 javascript: 协议链接但保留元素', () => {
    const html = renderMarkdown('[click](javascript:alert(1))')
    expect(html).toContain('<a')
    expect(html).not.toContain('javascript:')
    expect(html).toContain('href=""')
  })

  it('净化大小写与 URL 编码绕过的危险协议', () => {
    expect(renderMarkdown('[x](JaVaScRiPt:alert(1))')).not.toMatch(/href="javascript/i)
    expect(renderMarkdown('[x](%6a%61vascript:alert(1))')).not.toMatch(/href=".*javascript/i)
    expect(renderMarkdown('[x](data:text/html,<script>)')).toContain('href=""')
    expect(renderMarkdown('[x](vbscript:msgbox)')).toContain('href=""')
    expect(renderMarkdown('[x](file:///etc/passwd)')).toContain('href=""')
  })

  it('正常链接加 target=_blank 与 noopener', () => {
    const html = renderMarkdown('[doc](https://example.com)')
    expect(html).toContain('href="https://example.com"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer"')
    expect(html).toContain('referrerpolicy="no-referrer"')
  })

  it('图片不自动加载，仅显示说明', () => {
    const html = renderMarkdown('![tracker](https://evil.example/pixel.png)')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('evil.example')
    expect(html).toContain('图片：tracker')
  })

  it('渲染 GFM 表格与代码块', () => {
    const html = renderMarkdown('| a | b |\n|---|---|\n| 1 | 2 |\n\n```cpp\nint main() {}\n```')
    expect(html).toContain('<table>')
    expect(html).toContain('<code')
    expect(html).toContain('int main()')
  })

  it('渲染 KaTeX 公式且非法公式不抛错', () => {
    const html = renderMarkdown('$E = mc^2$')
    expect(html).toContain('katex')
    expect(() => renderMarkdown('$\\invalid{$')).not.toThrow()
  })

  it('HTML 实体在代码块中转义', () => {
    const html = renderMarkdown('`<b>x</b>`')
    expect(html).toContain('&lt;b&gt;')
    expect(html).not.toContain('<b>x</b>')
  })
})
