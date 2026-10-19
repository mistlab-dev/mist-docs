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
