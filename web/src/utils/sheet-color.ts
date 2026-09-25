/**
 * Tone user-chosen spreadsheet colours for dark mode without changing stored data.
 * Pale fills are mixed into the dark surface so they don't glare; dark ink is
 * lifted so it stays readable on those fills. Already-dark fills and already-light
 * text are left alone (header blues, white type).
 */

export type SheetColorRole = 'bg' | 'fg'

const HEX_OR_RGB = /#(?:[0-9a-fA-F]{3,8})|rgba?\([^)]+\)/g

function channel(c: number): number {
  const s = c / 255
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
}

function parseRGB(color: string): [number, number, number] | null {
  const raw = color.trim()
  const hex = raw.match(/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/)
  if (hex) {
    let h = hex[1]
    if (h.length === 3) h = h.split('').map((ch) => ch + ch).join('')
    if (h.length === 8) h = h.slice(0, 6)
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)]
  }
  const rgb = raw.match(/^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)/i)
  if (rgb) return [Number(rgb[1]), Number(rgb[2]), Number(rgb[3])]
  return null
}

function luminance(color: string): number | null {
  const rgb = parseRGB(color)
  if (!rgb) return null
  const [r, g, b] = rgb
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function toneOne(color: string, role: SheetColorRole): string {
  const L = luminance(color)
  if (L === null) return color
  if (role === 'bg') {
    if (L <= 0.28) return color
    const pct = L >= 0.72 ? 24 : 42
    return `color-mix(in srgb, ${color} ${pct}%, var(--md-surface))`
  }
  if (L >= 0.55) return color
  return `color-mix(in srgb, ${color} 42%, var(--md-text))`
}

export function toneSheetColor(color: string, dark: boolean, role: SheetColorRole = 'bg'): string {
  if (!dark || !color) return color
  if (/gradient\(/i.test(color)) {
    return color.replace(HEX_OR_RGB, (m) => toneOne(m, role))
  }
  return toneOne(color, role)
}
