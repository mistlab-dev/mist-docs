import { describe, expect, it } from 'vitest'
import { toneSheetColor } from './sheet-color'

describe('toneSheetColor', () => {
  it('leaves colours untouched in light mode', () => {
    expect(toneSheetColor('#eef3ff', false, 'bg')).toBe('#eef3ff')
    expect(toneSheetColor('#006100', false, 'fg')).toBe('#006100')
  })

  it('mixes pale fills into the dark surface', () => {
    const out = toneSheetColor('#eef3ff', true, 'bg')
    expect(out).toContain('color-mix')
    expect(out).toContain('var(--md-surface)')
    expect(out).toContain('#eef3ff')
  })

  it('keeps already-dark fills such as header blue', () => {
    expect(toneSheetColor('#4472c4', true, 'bg')).toBe('#4472c4')
  })

  it('lifts dark ink and keeps light ink', () => {
    expect(toneSheetColor('#006100', true, 'fg')).toContain('var(--md-text)')
    expect(toneSheetColor('#ffffff', true, 'fg')).toBe('#ffffff')
  })

  it('tones colour stops inside gradients without rewriting the gradient syntax', () => {
    const src = 'linear-gradient(90deg, #c6efce 40%, transparent 40%)'
    const out = toneSheetColor(src, true, 'bg')
    expect(out.startsWith('linear-gradient(90deg,')).toBe(true)
    expect(out).toContain('color-mix')
    expect(out).toContain('transparent 40%')
  })

  it('parses rgb() fills from the colour scale', () => {
    const out = toneSheetColor('rgb(238, 243, 255)', true, 'bg')
    expect(out).toContain('color-mix')
  })
})