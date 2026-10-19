// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import DOMPurify from 'dompurify'
import { sanitizeMermaidSvg } from './mermaid'

// 分享页正文消毒配置（与 ShareView.vue 中 SHARE_SANITIZE_OPTS 保持一致）。
// DOMPurify 的行为不依赖 Vue，所以这里直接复现同一份配置来锁住语义：
// 改配置必须同步两边，本测试会抓住差异。
const SHARE_SANITIZE_OPTS = {
  ADD_ATTR: ['target', 'data-type', 'data-checked', 'data-language'],
  ALLOW_DATA_ATTR: true,
} as any

function sanitize(html: string): string {
  return String(DOMPurify.sanitize(html, SHARE_SANITIZE_OPTS))
}

describe('share page content sanitize', () => {
  it('strips event handlers (onerror / onload / onclick)', () => {
    const out = sanitize('<p>hi</p><img src=x onerror="alert(document.cookie)">')
    expect(out).toContain('<p>hi</p>')
    expect(out).not.toContain('onerror')
    expect(out).not.toContain('alert(')
  })

  it('strips javascript: urls in href', () => {
    const out = sanitize('<a href="javascript:alert(1)">click</a>')
    expect(out).not.toContain('javascript:')
    expect(out).toContain('click')
  })

  it('strips javascript: urls in src (iframe/img)', () => {
    const out = sanitize('<iframe src="javascript:alert(1)"></iframe>')
    expect(out).not.toContain('iframe')
    expect(out).not.toContain('javascript:')
  })

  it('drops iframe entirely (share page does not embed third-party content)', () => {
    const out = sanitize('<iframe srcdoc="<script>alert(1)</script>"></iframe>')
    expect(out).not.toContain('iframe')
    expect(out).not.toContain('srcdoc')
    expect(out).not.toContain('script')
  })

  it('drops script and style blocks', () => {
    const out = sanitize('<script>steal()</script><style>@import "x";</style><p>ok</p>')
    expect(out).not.toContain('script')
    expect(out).not.toContain('style')
    expect(out).toContain('<p>ok</p>')
  })

  it('keeps mermaid code blocks so the renderer can find them', () => {
    const out = sanitize(
      '<pre><code class="language-mermaid">graph TD; A--&gt;B;</code></pre>'
    )
    expect(out).toContain('language-mermaid')
    expect(out).toContain('graph TD')
    expect(out).toContain('<pre>')
  })

  it('keeps the mermaid language class in class attributes', () => {
    const out = sanitize('<pre class="mermaid"><code class="language-mermaid">x</code></pre>')
    expect(out).toContain('language-mermaid')
  })

  it('keeps signed media urls and images', () => {
    const out = sanitize(
      '<img src="/api/media/t1/a.png?sig=abc" alt="pic">'
    )
    expect(out).toContain('/api/media/t1/a.png?sig=abc')
    expect(out).toContain('alt="pic"')
  })

  it('keeps TipTap link markup (target / rel) used by wikilinks', () => {
    const out = sanitize(
      '<a target="_blank" rel="noopener" class="editor-link" href="https://docs.mistlab.dev/docs/abc">title</a>'
    )
    expect(out).toContain('editor-link')
    expect(out).toContain('href="https://docs.mistlab.dev/docs/abc"')
    expect(out).toContain('target="_blank"')
    expect(out).toContain('title')
  })

  it('keeps task list and table structure', () => {
    const out = sanitize(
      '<ul data-type="taskList"><li data-checked="true">item</li></ul>' +
        '<table><tbody><tr><td>cell</td></tr></tbody></table>'
    )
    expect(out).toContain('taskList')
    expect(out).toContain('cell')
  })
})

describe('sanitizeMermaidSvg', () => {
  it('keeps foreignObject node labels but strips payloads', () => {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject width="100" height="50">
      <body xmlns="http://www.w3.org/1999/xhtml"><div>节点文字</div></body>
    </foreignObject><img src="x" onerror="alert(1)"/><a href="javascript:alert(1)">x</a></svg>`
    const out = sanitizeMermaidSvg(svg)
    expect(out).toContain('foreignObject')
    expect(out).toContain('节点文字')
    expect(out).not.toContain('onerror')
    expect(out).not.toContain('javascript:')
  })
})
