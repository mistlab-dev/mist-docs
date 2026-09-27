import { describe, expect, it } from 'vitest'
import { confidenceKey, evidenceKey, isInsufficient, verdictType, worthExplaining } from './explain'

describe('explain helpers', () => {
  it('maps verdicts to tag colours', () => {
    expect(verdictType('late')).toBe('danger')
    expect(verdictType('overdue')).toBe('danger')
    expect(verdictType('tight')).toBe('warning')
    expect(verdictType('started')).toBe('warning')
    expect(verdictType('on_track')).toBe('success')
    expect(verdictType('done')).toBe('success')
    expect(verdictType('no_signal')).toBe('info')
  })
  it('maps confidence and evidence types to i18n keys with safe fallbacks', () => {
    expect(confidenceKey('high')).toBe('explain.confHigh')
    expect(confidenceKey('medium')).toBe('explain.confMedium')
    expect(confidenceKey('weird')).toBe('explain.confLow')
    expect(evidenceKey('event')).toBe('explain.evEvent')
    expect(evidenceKey('load')).toBe('explain.evLoad')
    expect(evidenceKey('unknown')).toBe('explain.evHistory')
  })
  it('shows the card shortcut only for open orders at risk', () => {
    expect(worthExplaining('overdue', 'overdue')).toBe(true)
    expect(worthExplaining('critical', 'running')).toBe(true)
    expect(worthExplaining('ok', 'pending')).toBe(false)
    expect(worthExplaining('critical', 'done')).toBe(false)
  })
  it('detects the data-insufficient conclusion', () => {
    expect(isInsufficient({ conclusion: '数据不足：按当前排队可按期' })).toBe(true)
    expect(isInsufficient({ conclusion: '按当前排队可能晚 2 天' })).toBe(false)
  })
})
