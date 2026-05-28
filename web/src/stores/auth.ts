import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import http from '@/utils/http'

interface User {
  id: string
  username: string
  name: string
  role: string
  department_id: string
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref<User | null>(null)

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'super_admin' || user.value?.role === 'dept_admin')

  async function login(username: string, password: string) {
    const { data } = await http.post('/auth/login', { username, password })
    token.value = data.token
    user.value = data.user
    localStorage.setItem('token', data.token)
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }

  async function fetchMe() {
    try {
      const { data } = await http.get('/auth/me')
      user.value = data.data
    } catch {
      logout()
    }
  }

  return { token, user, isLoggedIn, isAdmin, login, logout, fetchMe }
})
