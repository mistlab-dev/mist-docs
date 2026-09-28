import axios from 'axios'
import { useAuthStore } from '@/stores/auth'
import { createRefreshQueue, pickTokens } from '@/utils/tokenRefresh'

const http = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

// One refresh at a time; requests that 401 meanwhile wait in the queue and
// are retried with the new token, or rejected if the refresh fails.
let isRefreshing = false
const refreshQueue = createRefreshQueue()

http.interceptors.request.use((config) => {
  const auth = useAuthStore()
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  return config
})

http.interceptors.response.use(
  (res) => res,
  async (err) => {
    const originalRequest = err.config

    if (err.response?.status === 401 && !originalRequest._retry) {
      const auth = useAuthStore()
      const refreshToken = localStorage.getItem('mist-docs-refresh-token')

      // No refresh token available → full re-login
      if (!refreshToken) {
        auth.logout()
        auth.redirectToPortalLogin()
        return Promise.reject(err)
      }

      // If already refreshing, queue this request
      if (isRefreshing) {
        const token = await refreshQueue.wait()
        originalRequest._retry = true
        originalRequest.headers.Authorization = `Bearer ${token}`
        return http(originalRequest)
      }

      originalRequest._retry = true
      isRefreshing = true

      try {
        // Call Portal refresh endpoint (API subdomain)
        const apiBase = import.meta.env.VITE_API_URL || 'https://api.mistlab.dev/v1'
        const resp = await axios.post(`${apiBase}/auth/refresh`, {
          refresh_token: refreshToken,
        })

        const tokens = pickTokens(resp.data)
        if (tokens) {
          const newAccessToken = tokens.access

          // Update stored tokens (store and localStorage stay in sync)
          auth.token = newAccessToken
          localStorage.setItem('mist-docs-token', newAccessToken)
          if (tokens.refresh) {
            auth.refreshToken = tokens.refresh
            localStorage.setItem('mist-docs-refresh-token', tokens.refresh)
          }

          // Retry queued requests
          refreshQueue.resolve(newAccessToken)

          // Retry original request
          originalRequest.headers.Authorization = `Bearer ${newAccessToken}`
          return http(originalRequest)
        } else {
          throw new Error('No access_token in refresh response')
        }
      } catch (refreshErr) {
        // Refresh failed → fail the queued requests, clear everything and
        // redirect to Portal login
        refreshQueue.reject(refreshErr)
        auth.logout()
        localStorage.removeItem('mist-docs-refresh-token')
        auth.redirectToPortalLogin()
        return Promise.reject(refreshErr)
      } finally {
        isRefreshing = false
      }
    }

    return Promise.reject(err)
  },
)

export default http
