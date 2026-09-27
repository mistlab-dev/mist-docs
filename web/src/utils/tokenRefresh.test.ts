import { describe, expect, it } from 'vitest'
import { createRefreshQueue, pickTokens } from './tokenRefresh'

describe('pickTokens', () => {
  it('reads a bare body', () => {
    expect(pickTokens({ access_token: 'a', refresh_token: 'r' })).toEqual({ access: 'a', refresh: 'r' })
  })
  it('reads the data envelope', () => {
    expect(pickTokens({ data: { access_token: 'a' } })).toEqual({ access: 'a', refresh: undefined })
  })
  it('rejects bodies without an access token', () => {
    expect(pickTokens(null)).toBeNull()
    expect(pickTokens('x')).toBeNull()
    expect(pickTokens({})).toBeNull()
    expect(pickTokens({ access_token: '' })).toBeNull()
    expect(pickTokens({ data: { refresh_token: 'r' } })).toBeNull()
  })
})

describe('createRefreshQueue', () => {
  it('resolves every waiter with the new token', async () => {
    const q = createRefreshQueue()
    const a = q.wait()
    const b = q.wait()
    expect(q.size).toBe(2)
    q.resolve('new')
    await expect(Promise.all([a, b])).resolves.toEqual(['new', 'new'])
    expect(q.size).toBe(0)
  })
  it('rejects waiters when the refresh fails instead of leaving them pending', async () => {
    const q = createRefreshQueue()
    const a = q.wait()
    q.reject(new Error('refresh failed'))
    await expect(a).rejects.toThrow('refresh failed')
    expect(q.size).toBe(0)
  })
  it('does not re-settle old waiters on the next round', async () => {
    const q = createRefreshQueue()
    const first = q.wait()
    q.resolve('t1')
    await first
    const second = q.wait()
    q.resolve('t2')
    await expect(second).resolves.toBe('t2')
  })
})
