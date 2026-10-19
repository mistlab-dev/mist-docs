// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  DRAFT_MAX_AGE_MS,
  draftKey,
  isDraftRecoverable,
  readDraft,
  removeDraft,
  saveDraft,
  type DraftRecord,
} from './draft'

function record(overrides: Partial<DraftRecord> = {}): DraftRecord {
  return {
    content: '<p>draft body</p>',
    baseVersion: 3,
    baseUpdatedAt: '2026-10-19 10:00:00',
    savedAt: Date.parse('2026-10-19T10:05:00Z'),
    ...overrides,
  }
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
    const stale = record({ savedAt: Date.parse('2026-10-19T09:00:00Z') })
    expect(isDraftRecoverable(stale, server)).toBe(false)
  })

  it('accepts a draft written after the server write even when base moved on', () => {
    const bothChanged = record({ baseUpdatedAt: '2026-10-19 09:00:00', savedAt: Date.parse('2026-10-19T10:30:00Z') })
    expect(isDraftRecoverable(bothChanged, server)).toBe(true)
  })

  it('compares Z-suffixed server timestamps too', () => {
    const s = { content: '<p>server</p>', version: 1, updated_at: '2026-10-19T10:00:00Z' }
    const stale = record({ savedAt: Date.parse('2026-10-19T09:00:00Z') })
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
