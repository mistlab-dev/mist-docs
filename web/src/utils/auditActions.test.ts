import { describe, it, expect } from 'vitest'
import zh from '@/i18n/locales/zh-CN'
import en from '@/i18n/locales/en-US'
import { AUDIT_ACTIONS, LEGACY_ACTIONS, auditActionKey, auditActionColor } from './auditActions'

describe('audit actions', () => {
  it('every action has a zh and en label', () => {
    for (const a of [...AUDIT_ACTIONS, ...Object.keys(LEGACY_ACTIONS)]) {
      const key = auditActionKey(a).split('.').pop()!
      expect((zh as any).admin.audits.act[key], `zh ${a}`).toBeTruthy()
      expect((en as any).admin.audits.act[key], `en ${a}`).toBeTruthy()
    }
  })
  it('legacy restore shares the restore_doc label', () => {
    expect(auditActionKey('restore')).toBe(auditActionKey('restore_doc'))
  })
  it('colors by kind', () => {
    expect(auditActionColor('delete_share')).toBe('danger')
    expect(auditActionColor('create_comment')).toBe('primary')
    expect(auditActionColor('restore')).toBe('success')
    expect(auditActionColor('view')).toBe('info')
  })
})
