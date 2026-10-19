import DOMPurify from 'dompurify'

let mermaidModule: any = null
let initialized = false

async function getMermaid(): Promise<any> {
  // 按需加载：没有 mermaid 图的文档不下载解析器（之前顶层 import 让
  // DocEditor chunk 从 389 kB 涨到 847 kB）。首次渲染图表时才加载。
  if (!mermaidModule) {
    const mod: any = await import('mermaid')
    mermaidModule = mod.default ?? mod
  }
  if (!initialized) {
    mermaidModule.initialize({
      startOnLoad: false,
      theme: 'default',
      // strict: 图表内容来自协作文档，队友可写入。strict 禁止脚本并过滤
      // javascript: 链接，避免图表源码携带的 XSS 影响其他协作者。
      securityLevel: 'strict',
      fontFamily: 'inherit',
    })
    initialized = true
  }
  return mermaidModule
}

let renderIdCounter = 0

// SVG sanitize 选项：保留 flowchart 的 <foreignObject> 节点文字，
// 同时 DOMPurify 默认仍会剥掉 onerror / javascript: 等载荷。
const SVG_SANITIZE_OPTS = {
  ADD_TAGS: ['foreignObject'],
  HTML_INTEGRATION_POINTS: { foreignobject: true },
} as any

// sanitizeMermaidSvg 把 mermaid 生成的 SVG 过一遍 DOMPurify。
// 导出供编辑器与分享页共用，改配置只改这一处。
export function sanitizeMermaidSvg(svg: string): string {
  // 配置了 HTML_INTEGRATION_POINTS 时 DOMPurify 返回 TrustedHTML，转回 string
  return String(DOMPurify.sanitize(svg, SVG_SANITIZE_OPTS))
}

// renderMermaidSafe 渲染 + 消毒一步到位，调用方拿到的是可安全 innerHTML 的 SVG。
export async function renderMermaidSafe(source: string): Promise<string> {
  return sanitizeMermaidSvg(await renderMermaidSvg(source))
}

export async function renderMermaidSvg(source: string): Promise<string> {
  const mermaid = await getMermaid()
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
