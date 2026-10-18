import mermaid from 'mermaid'

let initialized = false

export function initMermaid() {
  if (initialized) return
  mermaid.initialize({
    startOnLoad: false,
    theme: 'default',
    // strict: 图表内容来自协作文档，队友可写入。strict 禁止脚本并过滤
    // javascript: 链接，避免图表源码携带的 XSS 影响其他协作者。
    securityLevel: 'strict',
    fontFamily: 'inherit',
  })
  initialized = true
}

let renderIdCounter = 0

/**
 * Render mermaid source text to SVG string.
 */
export async function renderMermaidSvg(source: string): Promise<string> {
  initMermaid()
  const cleanCode = source.trim()
  if (!cleanCode) return ''
  const id = `mermaid-svg-${Date.now()}-${++renderIdCounter}`
  try {
    const { svg } = await mermaid.render(id, cleanCode)
    return svg
  } catch (err: any) {
    // mermaid creates an error element in document body on failure, clean it up
    const el = document.getElementById(id)
    if (el) el.remove()
    const errorEl = document.getElementById('d' + id)
    if (errorEl) errorEl.remove()
    throw err
  }
}
