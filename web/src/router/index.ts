import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true },
  },
  {
    path: '/auth/callback',
    name: 'SSOCallback',
    component: () => import('@/views/SSOCallback.vue'),
    meta: { public: true },
  },
  {
    path: '/s/:token',
    name: 'ShareView',
    component: () => import('@/views/ShareView.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    children: [
      { path: '', redirect: '/docs' },
      { path: 'docs', name: 'Docs', component: () => import('@/views/Docs.vue') },
      { path: 'docs/:id', name: 'DocEditor', component: () => import('@/views/DocEditor.vue') },
      { path: 'trash', name: 'Trash', component: () => import('@/views/Trash.vue') },
      { path: 'help', name: 'Help', component: () => import('@/views/Help.vue') },
      // Admin pages now scoped to team
      { path: 'admin/dashboard', name: 'Dashboard', component: () => import('@/views/admin/Dashboard.vue'), meta: { admin: true } },
      { path: 'admin/folders', name: 'TeamFolders', component: () => import('@/views/admin/TeamFolders.vue'), meta: { admin: true } },
      { path: 'admin/audits', name: 'Audits', component: () => import('@/views/admin/Audits.vue'), meta: { admin: true } },
      { path: 'admin/storage', name: 'Storage', component: () => import('@/views/admin/Storage.vue'), meta: { admin: true } },
      { path: 'admin/permissions', name: 'Permissions', component: () => import('@/views/admin/Permissions.vue'), meta: { admin: true } },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/docs',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to, _from, next) => {
  const auth = useAuthStore()

  // Public pages
  if (to.meta.public) {
    return next()
  }

  // Not logged in → redirect to Portal
  if (!auth.token) {
    auth.redirectToPortalLogin()
    return
  }

  // Fetch user info if missing
  if (auth.token && !auth.user) {
    try {
      await auth.fetchMe()
    } catch {
      auth.logout()
      auth.redirectToPortalLogin()
      return
    }
  }

  // Admin pages: check team role
  if (to.meta.admin && !auth.isTeamAdmin) {
    next('/docs')
    return
  }

  next()
})

export default router