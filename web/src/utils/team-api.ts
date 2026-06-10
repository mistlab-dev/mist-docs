import http from './http'
import { useAuthStore } from '@/stores/auth'

/**
 * Team-scoped API helper.
 * Automatically prepends /teams/:teamId to all paths.
 *
 * Usage:
 *   import teamApi from '@/utils/team-api'
 *   teamApi.get('/documents')
 *   teamApi.post('/documents', { title: 'xxx' })
 *   teamApi.put(`/documents/${id}`, { title: 'new' })
 *   teamApi.delete(`/documents/${id}`)
 */
function teamPath(path: string): string {
  const auth = useAuthStore()
  const teamId = auth.currentTeamId
  if (!teamId) {
    console.warn('[teamApi] No team selected, API call may fail')
    return path
  }
  return `/teams/${teamId}${path}`
}

export default {
  get(path: string, config?: Parameters<typeof http.get>[1]) {
    return http.get(teamPath(path), config)
  },
  post(path: string, data?: any, config?: Parameters<typeof http.post>[2]) {
    return http.post(teamPath(path), data, config)
  },
  put(path: string, data?: any, config?: Parameters<typeof http.put>[2]) {
    return http.put(teamPath(path), data, config)
  },
  delete(path: string, config?: Parameters<typeof http.delete>[1]) {
    return http.delete(teamPath(path), config)
  },
}
