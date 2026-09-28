// Helpers for the 401 → refresh → retry flow in http.ts, kept free of axios
// and the store so they can be unit tested.

export interface RefreshedTokens {
  access: string
  refresh?: string
}

/**
 * Reads the Portal refresh response. Accepts both a bare body
 * ({access_token, refresh_token}) and the {data: {...}} envelope.
 */
export function pickTokens(body: unknown): RefreshedTokens | null {
  if (!body || typeof body !== 'object') return null
  const b = body as Record<string, unknown>
  const src = (b.access_token ? b : (b.data && typeof b.data === 'object' ? b.data : null)) as
    | Record<string, unknown>
    | null
  if (!src || typeof src.access_token !== 'string' || !src.access_token) return null
  const refresh = typeof src.refresh_token === 'string' && src.refresh_token ? src.refresh_token : undefined
  return { access: src.access_token, refresh }
}

/**
 * Requests that hit 401 while a refresh is already running wait here.
 * Every waiter is settled exactly once: resolved with the new token or
 * rejected when the refresh fails (they used to hang forever).
 */
export function createRefreshQueue() {
  let waiting: Array<{ resolve: (t: string) => void; reject: (e: unknown) => void }> = []
  return {
    wait(): Promise<string> {
      return new Promise((resolve, reject) => waiting.push({ resolve, reject }))
    },
    resolve(token: string) {
      const w = waiting
      waiting = []
      w.forEach(x => x.resolve(token))
    },
    reject(err: unknown) {
      const w = waiting
      waiting = []
      w.forEach(x => x.reject(err))
    },
    get size() {
      return waiting.length
    },
  }
}
