// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import {
  findMermaidBlocks,
  isMermaidCodeBlock,
  isPngDataURI,
  replaceMermaidBlocks,
} from './mermaidExport'

const PNG_URI = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=='

function doc(html: string): Document {
  return new DOMParser().parseFromString(html, 'text/html')
}

describe('isMermaidCodeBlock', () => {
  it('accepts <code class="language-mermaid">', () => {
    const el = doc('<code class="language-mermaid">graph TD;</code>').querySelector('code')!
    expect(isMermaidCodeBlock(el)).toBe(true)
  })

  it('rejects other languages', () => {
    const el = doc('<code class="language-go">package main</code>').querySelector('code')!
    expect(isMermaidCodeBlock(el)).toBe(false)
  })

  it('rejects a class list that merely contains the word', () => {
    const el = doc('<code class="not-language-mermaid">x</code>').querySelector('code')!
    expect(isMermaidCodeBlock(el)).toBe(false)
  })
})

describe('findMermaidBlocks', () => {
  it('finds mermaid blocks and their source', () => {
    const d = doc('<pre><code class="language-mermaid">graph TD;\nA--&gt;B;</code></pre>')
    const blocks = findMermaidBlocks(d)
    expect(blocks).toHaveLength(1)
    expect(blocks[0].source).toContain('graph TD;')
  })

  it('ignores a mermaid <code> that is not inside <pre>', () => {
    const d = doc('<code class="language-mermaid">inline</code>')
    expect(findMermaidBlocks(d)).toHaveLength(0)
  })
})

describe('isPngDataURI', () => {
  it('accepts a png data uri', () => {
    expect(isPngDataURI(PNG_URI)).toBe(true)
  })

  it('rejects other types and garbage', () => {
    expect(isPngDataURI('data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=')).toBe(false)
    expect(isPngDataURI('data:image/jpeg;base64,/9j/4AAQ')).toBe(false)
    expect(isPngDataURI('https://example.com/a.png')).toBe(false)
    expect(isPngDataURI('')).toBe(false)
  })
})

describe('replaceMermaidBlocks', () => {
  it('replaces a mermaid block with an image when rendering succeeds', async () => {
    const html = '<p>hi</p><pre><code class="language-mermaid">graph TD;</code></pre>'
    const out = await replaceMermaidBlocks(html, async () => PNG_URI)
    expect(out.rendered).toBe(1)
    expect(out.failed).toBe(0)
    expect(out.html).toContain('<img')
    expect(out.html).toContain(PNG_URI)
    expect(out.html).not.toContain('language-mermaid')
  })

  it('falls back to the code block when rendering fails, keeping the source', async () => {
    const html = '<pre><code class="language-mermaid">graph TD;\nA--&gt;B;</code></pre>'
    const out = await replaceMermaidBlocks(html, async () => null)
    expect(out.rendered).toBe(0)
    expect(out.failed).toBe(1)
    expect(out.html).toContain('language-mermaid')
    expect(out.html).toContain('graph TD;')
    expect(out.html).not.toContain('<img')
  })

  it('falls back when the renderer throws', async () => {
    const html = '<pre><code class="language-mermaid">boom</code></pre>'
    const out = await replaceMermaidBlocks(html, async () => {
      throw new Error('render exploded')
    })
    expect(out.failed).toBe(1)
    expect(out.html).toContain('boom')
  })

  it('falls back when the renderer returns a non-png data uri', async () => {
    const html = '<pre><code class="language-mermaid">x</code></pre>'
    const out = await replaceMermaidBlocks(html, async () => 'data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=')
    expect(out.rendered).toBe(0)
    expect(out.failed).toBe(1)
    expect(out.html).toContain('language-mermaid')
  })

  it('leaves non-mermaid code blocks alone', async () => {
    const html = '<pre><code class="language-go">package main</code></pre>'
    const out = await replaceMermaidBlocks(html, async () => PNG_URI)
    expect(out.rendered).toBe(0)
    expect(out.failed).toBe(0)
    expect(out.html).toContain('package main')
  })

  it('renders every mermaid block and keeps a mixed document intact', async () => {
    const html =
      '<pre><code class="language-mermaid">A</code></pre>' +
      '<p>text</p>' +
      '<pre><code class="language-mermaid">B</code></pre>'
    const out = await replaceMermaidBlocks(html, async () => PNG_URI)
    expect(out.rendered).toBe(2)
    expect(out.failed).toBe(0)
    expect(out.html).toContain('<p>text</p>')
    expect(out.html.match(/<img/g)).toHaveLength(2)
  })

  it('returns the input untouched when there is no mermaid block', async () => {
    const html = '<p>plain</p>'
    const out = await replaceMermaidBlocks(html, async () => PNG_URI)
    expect(out.html).toBe(html)
    expect(out.rendered).toBe(0)
  })
})
