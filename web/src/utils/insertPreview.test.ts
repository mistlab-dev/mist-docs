import { describe, it, expect } from 'vitest'
import { conclusionText, conclusionType, shortDate, flagKey, type PreviewResult } from './insertPreview'
import zh from '@/i18n/locales/zh-CN'
import en from '@/i18n/locales/en-US'

const item = (order_no: string) => ({ id: order_no, order_no, title: order_no, priority: 'normal', due_date: '2026-09-30', new_finish: '2026-09-30', delay_days: 0, late_days: 0, outcome: 'unaffected' as const })
const res = (b: number, d: number): PreviewResult => ({
  insert: item('N'), new_breaches: Array.from({ length: b }, (_, i) => item('B' + i)),
  delayed: Array.from({ length: d }, (_, i) => item('D' + i)), unaffected: [], started: [],
  conclusion: b ? 'breach' : d ? 'delay' : 'none', per_day: 1,
})

function lookup(msgs: any, key: string) {
  return key.split('.').reduce((o, k) => (o ? o[k] : undefined), msgs)
}

describe('insert preview helpers', () => {
  it('summarises the three outcomes', () => {
    expect(conclusionText(res(1, 2))).toEqual({ key: 'insertPreview.sumBreachDelay', args: { breach: 1, delay: 2 } })
    expect(conclusionText(res(2, 0)).key).toBe('insertPreview.sumBreach')
    expect(conclusionText(res(0, 3)).key).toBe('insertPreview.sumDelay')
    expect(conclusionText(res(0, 0)).key).toBe('insertPreview.sumNone')
    expect(conclusionType('breach')).toBe('danger')
    expect(conclusionType('delay')).toBe('warning')
    expect(conclusionType('none')).toBe('success')
  })

  it('shortens dates within the year', () => {
    expect(shortDate('2026-09-28', 2026)).toBe('09-28')
    expect(shortDate('2027-01-02', 2026)).toBe('2027-01-02')
    expect(shortDate(undefined)).toBe('—')
  })

  it('has zh and en text for every key it uses', () => {
    const keys = ['insertPreview.sumBreachDelay', 'insertPreview.sumBreach', 'insertPreview.sumDelay', 'insertPreview.sumNone',
      flagKey('penalty')!, flagKey('key_customer')!, flagKey('already_late')!]
    for (const k of keys) {
      expect(lookup(zh, k), k).toBeTypeOf('string')
      expect(lookup(en, k), k).toBeTypeOf('string')
    }
    expect(flagKey('other')).toBeNull()
  })

  it('never promises a date', () => {
    for (const msgs of [zh, en]) {
      const all = JSON.stringify((msgs as any).insertPreview)
      expect(all).not.toMatch(/保证|承诺|guarantee|promise/i)
    }
  })
})
