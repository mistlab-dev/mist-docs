import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import { renderMermaidSvg, sanitizeMermaidSvg } from '../utils/mermaid'

const mermaidPluginKey = new PluginKey('mermaid-preview')

// SVG 消毒配置统一在 utils/mermaid.ts（sanitizeMermaidSvg），编辑器与分享页
// 共用同一份，改配置不会漏一处（PR31 review 建议 3）。

// Cache rendered SVG per source text to avoid re-rendering untouched diagrams.
// Bounded so long editing sessions cannot grow it without limit.
const MAX_SVG_CACHE = 50
const svgCache = new Map<string, string>()

function cacheSvg(source: string, svg: string) {
  if (svgCache.has(source)) {
    svgCache.delete(source)
  }
  svgCache.set(source, svg)
  while (svgCache.size > MAX_SVG_CACHE) {
    const oldest = svgCache.keys().next()
    if (oldest.done) break
    svgCache.delete(oldest.value)
  }
}

function createMermaidWidget(source: string): HTMLElement {
  const container = document.createElement('div')
  container.className = 'mermaid-preview-widget'
  container.setAttribute('contenteditable', 'false')

  if (!source.trim()) {
    const empty = document.createElement('div')
    empty.className = 'mermaid-empty'
    empty.textContent = 'Mermaid: 请输入图表语法'
    container.appendChild(empty)
    return container
  }

  const cached = svgCache.get(source)
  if (cached) {
    const wrapper = document.createElement('div')
    wrapper.className = 'mermaid-svg-wrapper'
    wrapper.innerHTML = sanitizeMermaidSvg(cached)
    container.appendChild(wrapper)
    return container
  }

  const loading = document.createElement('div')
  loading.className = 'mermaid-loading'
  loading.textContent = '正在渲染图表...'
  container.appendChild(loading)

  renderMermaidSvg(source)
    .then((svg) => {
      cacheSvg(source, svg)
      container.innerHTML = ''
      const wrapper = document.createElement('div')
      wrapper.className = 'mermaid-svg-wrapper'
      wrapper.innerHTML = sanitizeMermaidSvg(svg)
      container.appendChild(wrapper)
    })
    .catch((err) => {
      // 错误信息用 textContent 渲染，避免异常文本本身携带的标记被当 HTML 解析
      container.innerHTML = ''
      const errorDiv = document.createElement('div')
      errorDiv.className = 'mermaid-error'
      errorDiv.textContent = `图表渲染失败: ${err?.message || err}`
      container.appendChild(errorDiv)
    })

  return container
}

export const MermaidPreview = Extension.create({
  name: 'mermaidPreview',

  addProseMirrorPlugins() {
    return [
      new Plugin({
        key: mermaidPluginKey,
        state: {
          init(_, state) {
            return buildDecorations(state.doc)
          },
          apply(tr, oldSet, oldState, newState) {
            // Recompute only when document content changes
            if (!tr.docChanged) return oldSet.map(tr.mapping, tr.doc)
            return buildDecorations(newState.doc)
          },
        },
        props: {
          decorations(state) {
            return this.getState(state)
          },
        },
      }),
    ]
  },
})

function contentHash(s: string): string {
  // FNV-1a 32-bit: cheap and good enough to distinguish same-length edits
  // in a decoration key. Collisions only cost one extra re-render.
  let h = 0x811c9dc5
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return (h >>> 0).toString(36)
}

function buildDecorations(doc: any): DecorationSet {
  const decos: Decoration[] = []

  doc.descendants((node: any, pos: number) => {
    if (node.type.name === 'codeBlock') {
      const lang = (node.attrs?.language || '').toLowerCase().trim()
      if (lang === 'mermaid') {
        const textContent = node.textContent || ''
        const widgetPos = pos + node.nodeSize
        decos.push(
          Decoration.widget(widgetPos, () => createMermaidWidget(textContent), {
            side: 1,
            key: `mermaid-${pos}-${contentHash(textContent)}`,
          })
        )
      }
    }
  })

  return DecorationSet.create(doc, decos)
}
