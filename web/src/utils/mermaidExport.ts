// 客户端把 Mermaid 代码块渲染成图片，供 Word 导出使用（#56）。
//
// 为什么在浏览器里渲染：A1 选型（issue #54）实测三条路——服务端 headless
// chromium 要让镜像从 7MB 涨到 270MB+71MB 字体，内存 200~400MB；纯 Go
// 渲染器配色/箭头与 mermaid.js 肉眼可见不同（与编辑器预览不一致）。客户端
// 渲染与服务端零新依赖，且与编辑器预览共用同一份 mermaid.js + DOMPurify
// 配置，保真一致。
//
// 失败语义：任何一张图渲染失败（语法错、超时、非图产物）都只回退这一张——
// 保留原等宽代码块（源码完整），整篇导出仍然成功。
//
// 注意：SVG → PNG 必须用 data: URI 加载。blob: URL 在 mermaid 的
// securityLevel:'strict' 下会污染 canvas，toDataURL 会抛 SecurityError。

import { renderMermaidSvg, sanitizeMermaidSvg } from './mermaid'

// 单张图 PNG 的体积上限。data: URI 走的是 base64（体积 ×4/3），导出的
// HTML body 上限是 32MB（服务端 maxClientExportBytes）；留出余量，
// 超限就回退成代码块。
const MAX_DIAGRAM_PNG_BYTES = 2 * 1024 * 1024

// Mermaid 代码块在 TipTap HTML 里的形状：<pre><code class="language-mermaid">。
const MERMAID_LANG_CLASS = 'language-mermaid'

// renderer 注入点：把 Mermaid 源码渲染成 PNG data: URI；返回 null 表示
// 渲染失败（调用方回退成代码块）。测试用假实现，不依赖真实 canvas。
export type MermaidRenderer = (source: string) => Promise<string | null>

// isMermaidCodeBlock 报告一个元素是否是 Mermaid 代码块。
export function isMermaidCodeBlock(el: Element): boolean {
  if (el.tagName !== 'CODE') return false
  const cls = el.getAttribute('class') || ''
  return cls.split(/\s+/).includes(MERMAID_LANG_CLASS)
}

// findMermaidBlocks 找出 HTML 里的 Mermaid 代码块（<pre> 元素 + 源码）。
export function findMermaidBlocks(doc: Document): Array<{ pre: Element; source: string }> {
  const out: Array<{ pre: Element; source: string }> = []
  const codes = Array.from(doc.querySelectorAll('code'))
  for (const code of codes) {
    if (!isMermaidCodeBlock(code)) continue
    const pre = code.parentElement
    // 只要包在 <pre> 里的才认：散落的 <code> 不是代码块。
    if (!pre || pre.tagName !== 'PRE') continue
    out.push({ pre, source: (code.textContent || '') })
  }
  return out
}

// replaceMermaidBlocks 把 HTML 里的 Mermaid 代码块换成图片。
//
// renderer 返回 PNG data: URI → 替换成 <img src="...">（后端 #50 的
// docxImageFetcher 收 data: URI 的 png，直接嵌进 .docx）。
// renderer 返回 null → 保留原 <pre> 代码块（回退路径）。
export async function replaceMermaidBlocks(
  html: string,
  renderer: MermaidRenderer,
): Promise<{ html: string; rendered: number; failed: number }> {
  if (!html) return { html, rendered: 0, failed: 0 }
  const doc = new DOMParser().parseFromString(html, 'text/html')
  const blocks = findMermaidBlocks(doc)
  if (blocks.length === 0) return { html, rendered: 0, failed: 0 }

  let rendered = 0
  let failed = 0
  for (const { pre, source } of blocks) {
    let dataURI: string | null = null
    try {
      dataURI = await renderer(source)
    } catch {
      dataURI = null
    }
    if (!dataURI || !isPngDataURI(dataURI)) {
      failed++
      continue // 回退：原 <pre> 保留不动
    }
    const img = doc.createElement('img')
    img.setAttribute('src', dataURI)
    img.setAttribute('alt', 'mermaid')
    pre.replaceWith(img)
    rendered++
  }
  return { html: doc.body.innerHTML, rendered, failed }
}

// isPngDataURI 确认产物确实是 PNG data: URI：后端只收 png/jpeg/gif，
// 别的类型会被 docxImageFetcher 拒掉。
export function isPngDataURI(uri: string): boolean {
  return /^data:image\/png;base64,[A-Za-z0-9+/=]+$/.test(uri.trim())
}

// svgToPngDataURI 把 SVG 转成 PNG data: URI。
export function svgToPngDataURI(svg: string, scale = 2): Promise<string> {
  return new Promise((resolve, reject) => {
    try {
      // base64 + data: URI；blob: URL 会被 strict 安全模型污染 canvas。
      const b64 = btoa(unescape(encodeURIComponent(svg)))
      const url = 'data:image/svg+xml;base64,' + b64
      const img = new Image()
      img.onload = () => {
        const w = img.naturalWidth || img.width
        const h = img.naturalHeight || img.height
        if (!w || !h) {
          reject(new Error('SVG 尺寸无效'))
          return
        }
        const canvas = document.createElement('canvas')
        canvas.width = Math.max(1, Math.round(w * scale))
        canvas.height = Math.max(1, Math.round(h * scale))
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          reject(new Error('无法创建 canvas 上下文'))
          return
        }
        // 白底：mermaid 主题是透明底，直接嵌进 Word 会看不清深色文字。
        ctx.fillStyle = '#ffffff'
        ctx.fillRect(0, 0, canvas.width, canvas.height)
        ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
        const dataURI = canvas.toDataURL('image/png')
        if (dataURI.length > MAX_DIAGRAM_PNG_BYTES * 4 / 3 + 4) {
          reject(new Error('图表图片过大'))
          return
        }
        resolve(dataURI)
      }
      img.onerror = () => reject(new Error('SVG 载入失败'))
      img.src = url
    } catch (err) {
      reject(err instanceof Error ? err : new Error(String(err)))
    }
  })
}

// defaultMermaidRenderer 用编辑器同源的 mermaid.js 渲染：mermaid.js → SVG →
// DOMPurify 消毒 → PNG data: URI。消毒配置与编辑器预览共用
// utils/mermaid.ts，改一处两处同步。
export const defaultMermaidRenderer: MermaidRenderer = async (source) => {
  if (!source.trim()) return null
  let svg: string
  try {
    svg = await renderMermaidSvg(source)
  } catch {
    return null // 语法错等 → 回退代码块
  }
  if (!svg) return null
  const clean = sanitizeMermaidSvg(svg)
  if (!clean) return null
  try {
    return await svgToPngDataURI(clean)
  } catch {
    return null
  }
}

// renderMermaidForExport 是导出流程的入口：把 HTML 里的 Mermaid 代码块
// 换成图片，失败的保留代码块。返回值里的 rendered/failed 用于给用户提示。
export async function renderMermaidForExport(html: string): Promise<{
  html: string
  rendered: number
  failed: number
}> {
  return replaceMermaidBlocks(html, defaultMermaidRenderer)
}
