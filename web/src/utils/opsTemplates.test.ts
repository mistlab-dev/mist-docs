import { describe, it, expect } from 'vitest'
import { opsTemplateKeys, opsTemplateHTML, isOpsTemplate } from './opsTemplates'

describe('ops templates', () => {
  it('has the five operations templates', () => {
    expect(opsTemplateKeys).toEqual(['release', 'rollback', 'dbchange', 'certrenew', 'onboarding'])
    expect(isOpsTemplate('release')).toBe(true)
    expect(isOpsTemplate('meeting')).toBe(false)
  })

  for (const key of opsTemplateKeys) {
    for (const locale of ['zh-CN', 'en-US']) {
      it(`${key} (${locale}) is usable editor HTML`, () => {
        const html = opsTemplateHTML(key, locale)
        expect(html.startsWith('<h2>')).toBe(true)
        expect(html).toContain('<h3>')
        // Checklists render as ticking task lists in the editor.
        expect(html).toContain('data-type="taskList"')
        // Only documentation addresses: example.com and 192.0.2.x.
        const ips = html.match(/\b\d{1,3}(\.\d{1,3}){3}\b/g) || []
        for (const ip of ips) expect(ip.startsWith('192.0.2.')).toBe(true)
        const hosts = html.match(/[a-z0-9.-]+\.(com|cn|net|dev|io)\b/gi) || []
        for (const h of hosts) expect(h.toLowerCase().endsWith('example.com')).toBe(true)
        // Plain wording, no jargon.
        for (const word of ['零信任', 'Vault', '租户', 'CA ', 'CA证书']) expect(html).not.toContain(word)
        // Code is escaped (no raw tags from shell/SQL snippets).
        expect(html).not.toMatch(/<(script|iframe)/i)
        if (locale.startsWith('zh')) expect(html).toMatch(/[\u4e00-\u9fa5]/)
        else expect(html).not.toMatch(/[\u4e00-\u9fa5]/)
      })
    }
  }
})
