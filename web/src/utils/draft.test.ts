// @vitest-environment jsdom
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  DRAFT_MAX_AGE_MS,
  draftKey,
  isDraftRecoverable,
  parseServerTime,
  readDraft,
  removeDraft,
  saveDraft,
  type DraftRecord,
} from './draft'

// Minimal process typing: @types/node is not a dependency of this package.
declare const process: { env: Record<string, string | undefined> }

// Pin the timezone: the API serves offset-less "YYYY-MM-DD HH:MM:SS" values
// (MySQL parseTime=true&loc=Local) that must be read as local wall clock.
// That distinction is only observable outside UTC, so force a zone where a
// naive "+Z" parse would skew by 8 hours.
const prevTZ = process.env.TZ
process.env.TZ = 'Asia/Shanghai'
afterAll(() => {
  if (prevTZ === undefined) delete process.env.TZ
  else process.env.TZ = prevTZ
})

function record(overrides: Partial<DraftRecord> = {}): DraftRecord {
  // Times must be relative to Date.now(): readDraft drops drafts older than
  // DRAFT_MAX_AGE_MS (7 days). Hardcoded 2026-10-19 dates made every round-
  // trip test start failing once real time passed that mark. The absolute-
  // timestamp cases below use explicit overrides and are unaffected.
  return {
    content: '<p>draft body</p>',
    baseVersion: 3,
    baseUpdatedAt: serverClock(5 * 60 * 1000),
    savedAt: Date.now(),
    ...overrides,
  }
}

// serverClock formats a moment `agoMs` in the past as the API's offset-less
// "YYYY-MM-DD HH:MM:SS" local wall-clock string.
function serverClock(agoMs: number): string {
  const d = new Date(Date.now() - agoMs)
  const p = (n: number) => String(n).padStart(2, '0')
  return (
    `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ` +
    `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
  )
}

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('saveDraft / readDraft', () => {
  it('round-trips a record', () => {
    expect(saveDraft('doc-1', record())).toBe(true)
    const got = readDraft('doc-1')
    expect(got?.content).toBe('<p>draft body</p>')
    expect(got?.baseVersion).toBe(3)
  })

  it('keys by doc id', () => {
    saveDraft('doc-a', record({ content: '<p>A</p>' }))
    saveDraft('doc-b', record({ content: '<p>B</p>' }))
    expect(readDraft('doc-a')?.content).toBe('<p>A</p>')
    expect(readDraft('doc-b')?.content).toBe('<p>B</p>')
    expect(draftKey('doc-a')).toBe('mist-docs-draft:doc-a')
  })

  it('returns null for missing or empty doc id', () => {
    expect(readDraft('nope')).toBeNull()
    expect(saveDraft('', record())).toBe(false)
    expect(readDraft('')).toBeNull()
  })

  it('drops drafts older than the max age', () => {
    saveDraft('doc-1', record({ savedAt: Date.now() - DRAFT_MAX_AGE_MS - 1000 }))
    expect(readDraft('doc-1')).toBeNull()
    // Stale record is cleaned up, not just hidden.
    expect(localStorage.getItem(draftKey('doc-1'))).toBeNull()
  })

  it('ignores corrupt JSON', () => {
    localStorage.setItem(draftKey('doc-1'), 'not-json{')
    expect(readDraft('doc-1')).toBeNull()
  })

  it('ignores records without content', () => {
    localStorage.setItem(draftKey('doc-1'), JSON.stringify({ content: '', savedAt: Date.now() }))
    expect(readDraft('doc-1')).toBeNull()
  })

  it('removeDraft deletes the record', () => {
    saveDraft('doc-1', record())
    removeDraft('doc-1')
    expect(readDraft('doc-1')).toBeNull()
  })

  it('evicts other documents when quota is exceeded', () => {
    // Fill storage until setItem throws, then verify saveDraft recovers by
    // pruning other drafts.
    saveDraft('old-1', record({ content: '<p>old</p>' }))
    saveDraft('old-2', record({ content: '<p>old</p>' }))
    const proto = Object.getPrototypeOf(localStorage)
    const original = proto.setItem
    let failForNewKey = true
    proto.setItem = function (this: Storage, key: string, value: string) {
      if (failForNewKey && key === draftKey('doc-1')) {
        failForNewKey = false
        throw new DOMException('QuotaExceededError')
      }
      return original.call(this, key, value)
    }
    try {
      expect(saveDraft('doc-1', record())).toBe(true)
    } finally {
      proto.setItem = original
    }
    expect(readDraft('doc-1')?.content).toBe('<p>draft body</p>')
    expect(localStorage.getItem(draftKey('old-1'))).toBeNull()
    expect(localStorage.getItem(draftKey('old-2'))).toBeNull()
  })
})

describe('isDraftRecoverable', () => {
  const server = {
    content: '<p>server body</p>',
    version: 3,
    updated_at: '2026-10-19 10:00:00',
  }

  it('accepts a newer differing draft', () => {
    expect(isDraftRecoverable(record(), server)).toBe(true)
  })

  it('rejects identical content', () => {
    expect(isDraftRecoverable(record({ content: server.content }), server)).toBe(false)
  })

  it('rejects a draft saved before the server write', () => {
    // Server wrote one minute AFTER the draft: draft is stale.
    const stale = record({ savedAt: Date.now() - 60_000 })
    const newerServer = { ...server, updated_at: serverClock(0) }
    expect(isDraftRecoverable(stale, newerServer)).toBe(false)
  })

  it('accepts a draft written after the server write even when base moved on', () => {
    const bothChanged = record({ baseUpdatedAt: '2026-10-19 09:00:00', savedAt: new Date(2026, 9, 19, 10, 30).getTime() })
    expect(isDraftRecoverable(bothChanged, server)).toBe(true)
  })

  it('compares Z-suffixed server timestamps too', () => {
    // Server (Z-form) wrote one minute after the draft: stale.
    const s = { content: '<p>server</p>', version: 1, updated_at: new Date(Date.now() + 60_000).toISOString() }
    const stale = record({ savedAt: Date.now() })
    expect(isDraftRecoverable(stale, s)).toBe(false)
  })

  it('falls back to content comparison when timestamps are absent', () => {
    const s = { content: '<p>server</p>' }
    expect(isDraftRecoverable(record(), s)).toBe(true)
    expect(isDraftRecoverable(record({ content: s.content }), s)).toBe(false)
  })

  it('never offers an empty draft', () => {
    expect(isDraftRecoverable(record({ content: '' }), { content: '<p>x</p>' })).toBe(false)
  })
})

describe('parseServerTime (timezone regression)', () => {
  it('parses offset-less timestamps as local wall clock', () => {
    // API values come from DSN parseTime=true&loc=Local: no offset, server
    // local wall clock. Must not be reinterpreted as UTC.
    const parsed = parseServerTime('2026-10-19 10:00:00')
    expect(parsed).toBe(new Date(2026, 9, 19, 10, 0, 0).getTime())
    // The old implementation appended "Z"; off UTC that shifts the instant.
    if (new Date().getTimezoneOffset() !== 0) {
      expect(parsed).not.toBe(Date.parse('2026-10-19 10:00:00Z'))
    }
  })

  it('keeps explicit offsets untouched', () => {
    expect(parseServerTime('2026-10-19T10:00:00Z')).toBe(Date.parse('2026-10-19T10:00:00Z'))
    expect(parseServerTime('2026-10-19T10:00:00+08:00')).toBe(Date.parse('2026-10-19T10:00:00+08:00'))
  })

  it('returns NaN for blank input', () => {
    expect(parseServerTime('')).toBeNaN()
    expect(parseServerTime('   ')).toBeNaN()
  })

  it('offers recovery for a no-Z updated_at with a later savedAt', () => {
    // Server wrote at 10:00 local; offline draft saved at 10:05 local.
    // Parsing 10:00 as UTC would make the draft look 8h stale in
    // Asia/Shanghai and silently drop the recovery prompt.
    const server = { content: '<p>server</p>', version: 2, updated_at: '2026-10-19 10:00:00' }
    const draft = record({
      content: '<p>draft newer than server</p>',
      baseUpdatedAt: '2026-10-19 10:00:00',
      savedAt: new Date(2026, 9, 19, 10, 5).getTime(),
    })
    expect(isDraftRecoverable(draft, server)).toBe(true)
  })
})
