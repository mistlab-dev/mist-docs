import mermaid from 'mermaid'

let initialized = false

export function initMermaid() {
  if (initialized) return
  mermaid.initialize({
    startOnLoad: false,
    theme: 'default',
    securityLevel: 'loose',
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
