import axios from 'axios'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

const http = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

// Refresh token state (prevent concurrent refresh requests)
let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []

function onRefreshed(token: string) {
  refreshSubscribers.forEach(cb => cb(token))
  refreshSubscribers = []
}

function addRefreshSubscriber(cb: (token: string) => void) {
  refreshSubscribers.push(cb)
}

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
        return new Promise((resolve) => {
          addRefreshSubscriber((token: string) => {
            originalRequest.headers.Authorization = `Bearer ${token}`
            resolve(http(originalRequest))
          })
        })
      }

      originalRequest._retry = true
      isRefreshing = true

      try {
        // Call Portal refresh endpoint (API subdomain)
        const apiBase = import.meta.env.VITE_API_URL || 'https://api.mistlab.dev/v1'
        const resp = await axios.post(`${apiBase}/auth/refresh`, {
          refresh_token: refreshToken,
        })

        if (resp.data?.access_token) {
          const newAccessToken = resp.data.access_token
          const newRefreshToken = resp.data.refresh_token || refreshToken

          // Update stored tokens
          auth.token = newAccessToken
          localStorage.setItem('mist-docs-token', newAccessToken)
          if (resp.data.refresh_token) {
            localStorage.setItem('mist-docs-refresh-token', newRefreshToken)
          }

          // Retry queued requests
          onRefreshed(newAccessToken)

          // Retry original request
          originalRequest.headers.Authorization = `Bearer ${newAccessToken}`
          return http(originalRequest)
        } else {
          throw new Error('No access_token in refresh response')
        }
      } catch (refreshErr) {
        // Refresh failed → clear everything and redirect to Portal login
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
