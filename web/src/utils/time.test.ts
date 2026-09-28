import { describe, expect, it } from 'vitest'
import { formatDateTime, formatRelative, parseTime } from './time'

const t = (key: string, args?: unknown[]) => (args ? `${key}:${args.join(',')}` : key)

describe('formatDateTime', () => {
  it('converts UTC server time to the viewer zone', () => {
    expect(formatDateTime('2026-09-27T14:54:03Z', { timeZone: 'Asia/Shanghai' })).toBe('2026-09-27 22:54:03')
    expect(formatDateTime('2026-09-27T14:54:03Z', { timeZone: 'America/Los_Angeles' })).toBe('2026-09-27 07:54:03')
  })

  it('honours an explicit offset', () => {
    expect(formatDateTime('2026-09-27T22:54:03+08:00', { timeZone: 'UTC' })).toBe('2026-09-27 14:54:03')
  })

  it('crosses midnight correctly', () => {
    expect(formatDateTime('2026-09-27T17:30:00Z', { timeZone: 'Asia/Shanghai', seconds: false })).toBe('2026-09-28 01:30')
  })

  it('leaves unparseable input visible and empty input empty', () => {
    expect(formatDateTime('not a time')).toBe('not a time')
    expect(formatDateTime('')).toBe('')
    expect(formatDateTime(null)).toBe('')
  })
})

describe('formatRelative', () => {
  const now = Date.parse('2026-09-27T14:00:00Z')
  it('uses relative buckets', () => {
    expect(formatRelative('2026-09-27T13:59:30Z', t, { now })).toBe('common.justNow')
    expect(formatRelative('2026-09-27T13:15:00Z', t, { now })).toBe('common.minutesAgo:45')
    expect(formatRelative('2026-09-27T09:00:00Z', t, { now })).toBe('common.hoursAgo:5')
    expect(formatRelative('2026-09-24T14:00:00Z', t, { now })).toBe('common.daysAgo:3')
  })
  it('falls back to the local date after a week', () => {
    expect(formatRelative('2026-09-01T20:00:00Z', t, { now, locale: 'zh-CN', timeZone: 'Asia/Shanghai' })).toBe('2026/9/2')
  })
  it('handles empty input', () => {
    expect(formatRelative('', t)).toBe('')
    expect(parseTime(undefined)).toBeNull()
  })
})
