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
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    children: [
      { path: '', redirect: '/docs' },
      { path: 'docs', name: 'Docs', component: () => import('@/views/Docs.vue') },
      { path: 'docs/:id', name: 'DocEditor', component: () => import('@/views/DocEditor.vue') },
      { path: 'trash', name: 'Trash', component: () => import('@/views/Trash.vue') },
      { path: 'admin/users', name: 'Users', component: () => import('@/views/admin/Users.vue'), meta: { admin: true } },
      { path: 'admin/departments', name: 'Departments', component: () => import('@/views/admin/Departments.vue'), meta: { admin: true } },
      { path: 'admin/permissions', name: 'Permissions', component: () => import('@/views/admin/Permissions.vue'), meta: { admin: true } },
      { path: 'admin/audits', name: 'Audits', component: () => import('@/views/admin/Audits.vue'), meta: { admin: true } },
      { path: 'admin/storage', name: 'Storage', component: () => import('@/views/admin/Storage.vue'), meta: { admin: true } },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.token) {
    next('/login')
  } else {
    next()
  }
})

export default router
