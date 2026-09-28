import { describe, it, expect } from 'vitest'
import { roleRank, canEditRole, isAdminRole, canManageRecord, docListActions } from './roles'

describe('roles', () => {
  it('ranks roles like the API', () => {
    expect(roleRank('owner')).toBe(3)
    expect(roleRank('admin')).toBe(3)
    expect(roleRank('editor')).toBe(2)
    expect(roleRank('member')).toBe(2)
    expect(roleRank('viewer')).toBe(1)
    expect(roleRank('guest')).toBe(0)
    expect(roleRank(undefined)).toBe(0)
  })
  it('owner counts as admin', () => {
    expect(isAdminRole('owner')).toBe(true)
    expect(isAdminRole('editor')).toBe(false)
    expect(canEditRole('viewer')).toBe(false)
    expect(canEditRole('editor')).toBe(true)
  })
  it('record ownership', () => {
    expect(canManageRecord('editor', 'u1', 'u1')).toBe(true)
    expect(canManageRecord('editor', 'u2', 'u1')).toBe(false)
    expect(canManageRecord('viewer', 'u1', 'u1')).toBe(false)
    expect(canManageRecord('admin', 'u2', 'u1')).toBe(true)
    expect(canManageRecord('editor', '', '')).toBe(false)
  })
})

describe('docListActions', () => {
  it('viewers get no rename/move/delete', () => {
    expect(docListActions('viewer')).toEqual({ rename: false, move: false, delete: false })
    expect(docListActions(undefined)).toEqual({ rename: false, move: false, delete: false })
  })
  it('editors rename and move but do not delete', () => {
    expect(docListActions('editor')).toEqual({ rename: true, move: true, delete: false })
    expect(docListActions('member')).toEqual({ rename: true, move: true, delete: false })
  })
  it('admins and owners get everything', () => {
    expect(docListActions('admin')).toEqual({ rename: true, move: true, delete: true })
    expect(docListActions('owner')).toEqual({ rename: true, move: true, delete: true })
  })
})
