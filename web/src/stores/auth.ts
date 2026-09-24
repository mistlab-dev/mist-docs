import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import http from '@/utils/http'

interface Team {
  team_id: string
  team_name: string
  role: string
}

interface User {
  id: string
  username: string
  display_name: string
  email: string
  is_admin: boolean
  teams: Team[]
}

const TEAM_KEY = 'mist-docs-team'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('mist-docs-token') || '')
  const refreshToken = ref(localStorage.getItem('mist-docs-refresh-token') || '')
  const user = ref<User | null>(null)
  const currentTeamId = ref<string>('')
  const currentTeamRole = ref<string>('')

  function applyTeam(teams: Team[] | undefined) {
    if (!teams?.length) {
      currentTeamId.value = ''
      currentTeamRole.value = ''
      return
    }
    const saved = localStorage.getItem(TEAM_KEY) || currentTeamId.value
    const picked = teams.find(t => t.team_id === saved) || teams[0]
    currentTeamId.value = picked.team_id
    currentTeamRole.value = picked.role
    localStorage.setItem(TEAM_KEY, picked.team_id)
  }

  // 从 localStorage 恢复用户信息
  const savedUser = localStorage.getItem('mist-docs-user')
  if (savedUser && !user.value) {
    try {
      user.value = JSON.parse(savedUser)
      applyTeam(user.value?.teams)
    } catch { /* ignore */ }
  }

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.is_admin || currentTeamRole.value === 'admin')
  const isTeamAdmin = computed(() => currentTeamRole.value === 'admin')

  // DEPRECATED: local login no longer supported
  async function login(_username: string, _password: string) {
    throw new Error('请通过 mistlab.dev 登录')
  }

  // SSO: redirect to Portal login
  function redirectToPortalLogin() {
    const current = window.location.origin
    const portalUrl = import.meta.env.VITE_PORTAL_URL || 'https://mistlab.dev'
    window.location.href = `${portalUrl}/login?redirect=${encodeURIComponent(current)}`
  }

  // SSO: handle callback from Portal with token
  async function handleSSOCallback(ssoToken: string, ssoRefreshToken?: string) {
    if (!ssoToken) return false
    token.value = ssoToken
    localStorage.setItem('mist-docs-token', ssoToken)
    if (ssoRefreshToken) {
      refreshToken.value = ssoRefreshToken
      localStorage.setItem('mist-docs-refresh-token', ssoRefreshToken)
    }
    try {
      await fetchMe()
      return true
    } catch {
      logout()
      return false
    }
  }

  function logout() {
    token.value = ''
    refreshToken.value = ''
    user.value = null
    currentTeamId.value = ''
    currentTeamRole.value = ''
    localStorage.removeItem('mist-docs-token')
    localStorage.removeItem('mist-docs-refresh-token')
    localStorage.removeItem('mist-docs-user')
    localStorage.removeItem(TEAM_KEY)
  }

  async function fetchMe() {
    const { data } = await http.get('/auth/me')
    user.value = data.data as User
    localStorage.setItem('mist-docs-user', JSON.stringify(data.data))
    applyTeam(user.value?.teams)
  }

  function setTeam(teamId: string) {
    const team = user.value?.teams?.find(t => t.team_id === teamId)
    if (!team) return
    currentTeamId.value = team.team_id
    currentTeamRole.value = team.role
    localStorage.setItem(TEAM_KEY, team.team_id)
  }

  return {
    token, refreshToken, user, currentTeamId, currentTeamRole,
    isLoggedIn, isAdmin, isTeamAdmin,
    login, redirectToPortalLogin, handleSSOCallback, logout, fetchMe, setTeam
  }
})