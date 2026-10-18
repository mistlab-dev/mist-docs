import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import { renderMermaidSvg } from '../utils/mermaid'

const mermaidPluginKey = new PluginKey('mermaid-preview')

// Cache rendered SVG per source text to avoid re-rendering untouched diagrams
const svgCache = new Map<string, string>()

function createMermaidWidget(source: string): HTMLElement {
  const container = document.createElement('div')
  container.className = 'mermaid-preview-widget'
  container.setAttribute('contenteditable', 'false')

  if (!source.trim()) {
    container.innerHTML = '<div class="mermaid-empty">Mermaid: 请输入图表语法</div>'
    return container
  }

  const cached = svgCache.get(source)
  if (cached) {
    container.innerHTML = `<div class="mermaid-svg-wrapper">${cached}</div>`
    return container
  }

  container.innerHTML = '<div class="mermaid-loading">正在渲染图表...</div>'

  renderMermaidSvg(source)
    .then((svg) => {
      svgCache.set(source, svg)
      container.innerHTML = `<div class="mermaid-svg-wrapper">${svg}</div>`
    })
    .catch((err) => {
      container.innerHTML = `<div class="mermaid-error">图表渲染失败: ${err?.message || err}</div>`
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
            key: `mermaid-${pos}-${textContent.length}`,
          })
        )
      }
    }
  })

  return DecorationSet.create(doc, decos)
}
