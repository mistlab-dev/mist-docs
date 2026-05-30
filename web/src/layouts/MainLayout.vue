<template>
  <el-container class="main-layout">
    <!-- 移动端遮罩 -->
    <div class="sidebar-overlay" :class="{ open: mobileMenu }" @click="mobileMenu = false"></div>

    <!-- 侧边栏 -->
    <el-aside :width="collapsed ? '64px' : '220px'" class="sidebar" :class="{ open: mobileMenu }">
      <div class="logo" @click="collapsed = !collapsed">
        <span v-if="!collapsed" class="logo-text">MistDocs</span>
        <svg v-else class="logo-svg" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
      </div>
      <el-menu
        :default-active="route.path"
        :collapse="collapsed"
        router
        background-color="#1d1e2c"
        text-color="#a0a4b8"
        active-text-color="#409eff"
        @select="onMenuSelect"
      >
        <el-menu-item index="/docs">
          <el-icon><Folder /></el-icon>
          <template #title>文档</template>
        </el-menu-item>
        <el-menu-item index="/trash">
          <el-icon><Delete /></el-icon>
          <template #title>回收站</template>
        </el-menu-item>

        <el-divider v-if="auth.isAdmin" />

        <template v-if="auth.isAdmin">
          <el-menu-item index="/admin/dashboard">
            <el-icon><DataAnalysis /></el-icon>
            <template #title>系统概览</template>
          </el-menu-item>
          <el-menu-item index="/admin/users">
            <el-icon><User /></el-icon>
            <template #title>用户管理</template>
          </el-menu-item>
          <el-menu-item index="/admin/departments">
            <el-icon><OfficeBuilding /></el-icon>
            <template #title>部门管理</template>
          </el-menu-item>
          <el-menu-item index="/admin/permissions">
            <el-icon><Lock /></el-icon>
            <template #title>权限管理</template>
          </el-menu-item>
          <el-menu-item index="/admin/audits">
            <el-icon><List /></el-icon>
            <template #title>审计日志</template>
          </el-menu-item>
          <el-menu-item index="/admin/storage">
            <el-icon><Monitor /></el-icon>
            <template #title>存储监控</template>
          </el-menu-item>
        </template>
      </el-menu>
    </el-aside>

    <!-- 主内容 -->
    <el-container>
      <el-header class="topbar">
        <el-button class="menu-btn" @click="mobileMenu = !mobileMenu" text>
          <el-icon :size="20"><Operation /></el-icon>
        </el-button>
        <div class="breadcrumb">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/docs' }">文档</el-breadcrumb-item>
            <el-breadcrumb-item v-if="route.name === 'DocEditor'">
              编辑
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="user-area">
          <!-- 通知铃铛 -->
          <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99" class="notif-badge">
            <el-button :icon="Bell" circle size="small" @click="showNotifications = true" />
          </el-badge>

          <el-dropdown @command="handleCommand">
            <span class="user-name">
              <el-avatar :size="28" class="user-avatar">{{ (auth.user?.name || auth.user?.username || '?')[0] }}</el-avatar>
              <span class="user-name-text">{{ auth.user?.name || auth.user?.username }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="theme">
                  <el-icon><Sunny v-if="isDark" /><Moon v-else /></el-icon>
                  {{ isDark ? '浅色模式' : '深色模式' }}
                </el-dropdown-item>
                <el-dropdown-item command="password">修改密码</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>

    <!-- 修改密码对话框 -->
    <el-dialog v-model="showPasswordDialog" title="修改密码" width="400">
      <el-form :model="passwordForm" label-width="80px">
        <el-form-item label="旧密码">
          <el-input v-model="passwordForm.old" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="passwordForm.new_" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPasswordDialog = false">取消</el-button>
        <el-button type="primary" @click="changePassword">确定</el-button>
      </template>
    </el-dialog>

    <!-- 通知面板 -->
    <el-drawer v-model="showNotifications" title="通知" size="360px">
      <div class="notif-header">
        <el-button link size="small" @click="markAllRead">全部已读</el-button>
      </div>
      <div class="notif-list">
        <div v-for="n in notifications" :key="n.id" class="notif-item" :class="{ unread: !n.is_read }">
          <el-tag size="small" :type="notifType(n.type)">{{ notifLabel(n.type) }}</el-tag>
          <div class="notif-title">{{ n.title }}</div>
          <div class="notif-time">{{ formatTime(n.created_at) }}</div>
          <div class="notif-actions">
            <el-button v-if="!n.is_read" link size="small" @click="markRead(n.id)">已读</el-button>
            <el-button v-if="n.document_id" link size="small" type="primary" @click="goDoc(n.document_id)">查看</el-button>
          </div>
        </div>
        <div v-if="!notifications.length" class="no-data">暂无通知</div>
      </div>
    </el-drawer>
  </el-container>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Sunny, Moon } from '@element-plus/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { Bell } from '@element-plus/icons-vue'
import http from '@/utils/http'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const isDark = ref(false)
const collapsed = ref(false)
const mobileMenu = ref(false)

function onMenuSelect() {
  // 移动端点击菜单后自动关闭
  if (window.innerWidth <= 768) mobileMenu.value = false
}

const showPasswordDialog = ref(false)
const passwordForm = ref({ old: '', new_: '' })

// Notifications
const showNotifications = ref(false)
const notifications = ref<any[]>([])
const unreadCount = ref(0)

async function loadNotifications() {
  try {
    const { data } = await http.get('/notifications')
    notifications.value = data.data || []
    unreadCount.value = data.unread_count || 0
  } catch {}
}

async function markRead(id: string) {
  await http.put(`/notifications/${id}/read`)
  loadNotifications()
}

async function markAllRead() {
  await http.put('/notifications/read-all')
  loadNotifications()
}

function goDoc(docId: string) {
  showNotifications.value = false
  router.push(`/docs/${docId}`)
}

function notifType(type: string) {
  if (type === 'comment') return ''
  if (type === 'reply') return 'success'
  if (type === 'share') return 'warning'
  return 'info'
}

function notifLabel(type: string) {
  const map: any = { comment: '评论', reply: '回复', share: '分享' }
  return map[type] || type
}

function formatTime(t: string) {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + '分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + '小时前'
  return d.toLocaleDateString()
}

function handleCommand(cmd: string) {
  if (cmd === 'logout') {
    auth.logout()
    router.push('/login')
  } else if (cmd === 'password') {
    showPasswordDialog.value = true
  } else if (cmd === 'theme') {
    toggleDark()
  }
}

function toggleDark() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('mistdocs-theme', isDark.value ? 'dark' : 'light')
}

async function changePassword() {
  try {
    await http.put('/auth/password', {
      old_password: passwordForm.value.old,
      new_password: passwordForm.value.new_,
    })
    ElMessage.success('密码已修改')
    showPasswordDialog.value = false
    passwordForm.value = { old: '', new_: '' }
  } catch {
    ElMessage.error('修改失败')
  }
}

onMounted(() => {
  // Restore theme preference
  const saved = localStorage.getItem('mistdocs-theme')
  if (saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }

  if (auth.token && !auth.user) {
    auth.fetchMe()
  }
  loadNotifications()
  // Poll notifications every 60s
  setInterval(loadNotifications, 60000)
})
</script>

<style scoped>
.main-layout { height: 100vh; }
.sidebar {
  background: #1d1e2c;
  transition: width 0.3s;
  overflow-y: auto;
}
.logo {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 18px;
  font-weight: bold;
  cursor: pointer;
  border-bottom: 1px solid #2a2b3d;
}
.logo-text { letter-spacing: 2px; }
.logo-icon { font-size: 24px; }
.logo-svg { width: 24px; height: 24px; }
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e8e8e8;
  background: #fff;
}
.user-area { display: flex; align-items: center; gap: 16px; }
.user-name {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
}
.user-avatar { background: #409eff; color: #fff; font-size: 13px; flex-shrink: 0; }
.notif-badge { margin-right: 4px; }
.el-divider { margin: 8px 16px; border-color: #2a2b3d; }

.notif-header { display: flex; justify-content: flex-end; margin-bottom: 8px; }
.notif-item { padding: 12px; border-bottom: 1px solid #f0f0f0; }
.notif-item.unread { background: #f0f7ff; }
.notif-title { margin: 4px 0; font-size: 14px; }
.notif-time { font-size: 12px; color: #c0c4cc; }
.notif-actions { margin-top: 4px; display: flex; gap: 8px; }
.no-data { text-align: center; padding: 40px; color: #c0c4cc; }

/* Mobile */
.menu-btn { display: none; color: #fff !important; }
.sidebar-overlay { display: none; }

@media (max-width: 768px) {
  .menu-btn { display: inline-flex !important; }
  .sidebar-overlay {
    display: none;
    position: fixed; top: 0; left: 0;
    width: 100vw; height: 100vh;
    background: rgba(0,0,0,0.4);
    z-index: 199;
  }
  .sidebar-overlay.open { display: block; }
  .el-aside.sidebar {
    position: fixed !important;
    top: 0; left: -260px;
    height: 100vh;
    z-index: 200;
    transition: left 0.3s;
  }
  .el-aside.sidebar.open { left: 0; }
  .breadcrumb { display: none; }
  .user-name-text { display: none; }
}
</style>
