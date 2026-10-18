import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'

export interface WikiLinkMatch {
  from: number
  to: number
  query: string
}

export interface WikiLinkOptions {
  onTrigger?: (match: WikiLinkMatch | null, coords?: { left: number; top: number; bottom: number }) => void
}

export const WikiLinkPluginKey = new PluginKey('wikilink-trigger')

/**
 * WikiLink extension detects typing `[[` and extracts the query string until closing `]]` or newline.
 * Notifies the UI layer to show/hide the suggestion dropdown at the cursor coords.
 */
export const WikiLink = Extension.create<WikiLinkOptions>({
  name: 'wikiLink',

  addOptions() {
    return {
      onTrigger: undefined,
    }
  },

  addProseMirrorPlugins() {
    const { onTrigger } = this.options

    return [
      new Plugin({
        key: WikiLinkPluginKey,
        view() {
          return {
            update(view) {
              if (!onTrigger) return

              const { state } = view
              const { selection } = state
              if (!selection.empty) {
                onTrigger(null)
                return
              }

              const pos = selection.from
              // Scan backwards up to 50 chars in the current text block
              const resolved = state.doc.resolve(pos)
              const textBefore = resolved.parent.textBetween(
                0,
                resolved.parentOffset,
                undefined,
                '\0'
              )

              const lastOpen = textBefore.lastIndexOf('[[')
              if (lastOpen === -1) {
                onTrigger(null)
                return
              }

              // If there's already a closing ]] after [[, don't trigger
              const afterOpen = textBefore.slice(lastOpen + 2)
              if (afterOpen.includes(']]') || afterOpen.includes('\n')) {
                onTrigger(null)
                return
              }

              const from = pos - afterOpen.length - 2
              const to = pos
              const coords = view.coordsAtPos(pos)

              onTrigger({ from, to, query: afterOpen.trim() }, coords)
            },
            destroy() {
              if (onTrigger) onTrigger(null)
            },
          }
        },
      }),
    ]
  },
})
