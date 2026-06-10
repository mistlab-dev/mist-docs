<template>
  <el-container class="main-layout">
    <!-- 移动端遮罩 -->
    <div class="sidebar-overlay" :class="{ open: mobileMenu }" @click="mobileMenu = false"></div>

    <!-- 侧边栏 -->
    <el-aside v-show="!sidebarHidden" :width="collapsed ? '64px' : '220px'" class="sidebar" :class="{ open: mobileMenu }">
      <div class="sidebar-top">
        <div class="logo" @click="collapsed = !collapsed">
          <span v-if="!collapsed" class="logo-text">MistDocs</span>
          <svg v-else class="logo-svg" viewBox="0 0 24 24" fill="#4f6ef7" stroke="none"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8" fill="#fff" stroke="none"/></svg>
        </div>
        <button class="sidebar-hide-btn" @click="sidebarHidden = true" :title="t('mainLayout.hideSidebar')">«</button>
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
          <template #title>{{ t('mainLayout.docs') }}</template>
        </el-menu-item>
        <el-menu-item index="/trash">
          <el-icon><Delete /></el-icon>
          <template #title>{{ t('mainLayout.trash') }}</template>
        </el-menu-item>
        <el-menu-item index="/help">
          <el-icon><QuestionFilled /></el-icon>
          <template #title>{{ t('mainLayout.help') }}</template>
        </el-menu-item>

        <el-divider v-if="auth.isAdmin" />

        <template v-if="auth.isAdmin">
          <el-menu-item index="/admin/dashboard">
            <el-icon><DataAnalysis /></el-icon>
            <template #title>{{ t('mainLayout.dashboard') }}</template>
          </el-menu-item>
          <el-menu-item index="/admin/audits">
            <el-icon><List /></el-icon>
            <template #title>{{ t('mainLayout.audits') }}</template>
          </el-menu-item>
          <el-menu-item index="/admin/storage">
            <el-icon><Monitor /></el-icon>
            <template #title>{{ t('mainLayout.storage') }}</template>
          </el-menu-item>
        </template>
      </el-menu>
      <div class="sidebar-bottom">
        <div class="help-btn" @click="showHelp = true" :title="collapsed ? t('mainLayout.help') : ''">
          <el-icon><QuestionFilled /></el-icon>
          <span v-if="!collapsed">{{ t('mainLayout.help') }}</span>
        </div>
      </div>
    </el-aside>

    <!-- 主内容 -->
    <el-container>
      <el-header class="topbar">
        <el-button v-if="sidebarHidden" class="menu-btn" @click="sidebarHidden = false" text :title="t('mainLayout.showSidebar')">
          <el-icon :size="20"><Operation /></el-icon>
        </el-button>
        <el-button v-else class="menu-btn" @click="mobileMenu = !mobileMenu" text>
          <el-icon :size="20"><Operation /></el-icon>
        </el-button>
        <div class="breadcrumb">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/docs' }">{{ t('mainLayout.docs') }}</el-breadcrumb-item>
            <el-breadcrumb-item v-if="route.name === 'DocEditor'">
              {{ t('mainLayout.breadcrumbEdit') }}
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="user-area">
          <!-- 语言切换 -->
          <LangSwitch />

          <!-- 通知铃铛 -->
          <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99" class="notif-badge">
            <el-button :icon="Bell" circle size="small" @click="showNotifications = true" />
          </el-badge>

          <el-dropdown @command="handleCommand">
            <span class="user-name">
              <el-avatar :size="28" class="user-avatar">{{ (auth.user?.display_name || auth.user?.username || '?')[0] }}</el-avatar>
              <span class="user-name-text">{{ auth.user?.display_name || auth.user?.username }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="theme">
                  <el-icon><Sunny v-if="isDark" /><Moon v-else /></el-icon>
                  {{ isDark ? t('mainLayout.lightMode') : t('mainLayout.darkMode') }}
                </el-dropdown-item>
                <el-dropdown-item command="password">{{ t('mainLayout.changePassword') }}</el-dropdown-item>
                <el-dropdown-item command="logout" divided>{{ t('mainLayout.logout') }}</el-dropdown-item>
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
    <el-dialog v-model="showPasswordDialog" :title="t('mainLayout.changePassword')" width="400">
      <el-form :model="passwordForm" label-width="80px">
        <el-form-item :label="t('mainLayout.oldPassword')">
          <el-input v-model="passwordForm.old" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('mainLayout.newPassword')">
          <el-input v-model="passwordForm.new_" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPasswordDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="changePassword">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 通知面板 -->
    <el-drawer v-model="showNotifications" :title="t('mainLayout.notifications')" size="360px">
      <div class="notif-header">
        <el-button link size="small" @click="markAllRead">{{ t('mainLayout.markAllRead') }}</el-button>
      </div>
      <div class="notif-list">
        <div v-for="n in notifications" :key="n.id" class="notif-item" :class="{ unread: !n.is_read }">
          <el-tag size="small" :type="notifType(n.type)">{{ notifLabel(n.type) }}</el-tag>
          <div class="notif-title">{{ n.title }}</div>
          <div class="notif-time">{{ formatTime(n.created_at) }}</div>
          <div class="notif-actions">
            <el-button v-if="!n.is_read" link size="small" @click="markRead(n.id)">{{ t('mainLayout.markRead') }}</el-button>
            <el-button v-if="n.document_id" link size="small" type="primary" @click="goDoc(n.document_id)">{{ t('mainLayout.viewDoc') }}</el-button>
          </div>
        </div>
        <div v-if="!notifications.length" class="no-data">{{ t('mainLayout.noNotifications') }}</div>
      </div>
    </el-drawer>

    <!-- 帮助 -->
    <el-dialog v-model="showHelp" :title="t('mainLayout.helpDialogTitle')" width="520">
      <div class="help-content">
        <h4>{{ t('mainLayout.helpSectionDocMgmt') }}</h4>
        <ul>
          <li>{{ t('mainLayout.helpDocList') }}</li>
          <li>{{ t('mainLayout.helpNewDoc') }}</li>
          <li>{{ t('mainLayout.helpNewSheet') }}</li>
          <li>{{ t('mainLayout.helpFolderContext') }}</li>
          <li>{{ t('mainLayout.helpDragDoc') }}</li>
        </ul>
        <h4>{{ t('mainLayout.helpSectionCollab') }}</h4>
        <ul>
          <li>{{ t('mainLayout.helpShareBtn') }}</li>
          <li>{{ t('mainLayout.helpSetPerm') }}</li>
          <li>{{ t('mainLayout.helpLinkShare') }}</li>
        </ul>
        <h4>{{ t('mainLayout.helpSectionShortcuts') }}</h4>
        <ul>
          <li><kbd>Ctrl</kbd>+<kbd>S</kbd> {{ t('mainLayout.helpCtrlS') }}</li>
          <li><kbd>Ctrl</kbd>+<kbd>/</kbd> {{ t('mainLayout.helpCtrlSlash') }}</li>
        </ul>
        <h4>{{ t('mainLayout.helpSectionFolders') }}</h4>
        <ul>
          <li>{{ t('mainLayout.helpFolderCount') }}</li>
          <li>{{ t('mainLayout.helpNestedFolder') }}</li>
          <li>{{ t('mainLayout.helpFolderInherit') }}</li>
        </ul>
      </div>
    </el-dialog>
  </el-container>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Sunny, Moon, Folder, Delete, DataAnalysis, User, OfficeBuilding, List, Monitor, Operation, ArrowDown, QuestionFilled } from '@element-plus/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { Bell } from '@element-plus/icons-vue'
import http from '@/utils/http'
import teamApi from '@/utils/team-api'
import LangSwitch from '@/components/LangSwitch.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const isDark = ref(false)
const collapsed = ref(false)
const mobileMenu = ref(false)
const sidebarHidden = ref(false)
const showHelp = ref(false)

function onMenuSelect() {
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
    const { data } = await teamApi.get('/notifications')
    notifications.value = data.data || []
    unreadCount.value = data.unread_count || 0
  } catch {}
}

async function markRead(id: string) {
  await teamApi.put(`/notifications/${id}/read`)
  loadNotifications()
}

async function markAllRead() {
  await teamApi.put('/notifications/read-all')
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
  const map: any = {
    comment: t('mainLayout.notifComment'),
    reply: t('mainLayout.notifReply'),
    share: t('mainLayout.notifShare'),
  }
  return map[type] || type
}

function formatTime(time: string) {
  if (!time) return ''
  const d = new Date(time)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return t('common.justNow')
  if (diff < 3600000) return t('common.minutesAgo', [Math.floor(diff / 60000)])
  if (diff < 86400000) return t('common.hoursAgo', [Math.floor(diff / 3600000)])
  return d.toLocaleDateString()
}

function handleCommand(cmd: string) {
  if (cmd === 'logout') {
    auth.logout()
    auth.redirectToPortalLogin()
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
    ElMessage.success(t('mainLayout.passwordChanged'))
    showPasswordDialog.value = false
    passwordForm.value = { old: '', new_: '' }
  } catch {
    ElMessage.error(t('mainLayout.passwordChangeFailed'))
  }
}

onMounted(() => {
  const setVh = () => document.documentElement.style.setProperty('--vh', `${window.innerHeight * 0.01}px`)
  setVh()
  window.addEventListener('resize', setVh)

  const saved = localStorage.getItem('mistdocs-theme')
  if (saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }

  if (auth.token && !auth.user) {
    auth.fetchMe()
  }
  loadNotifications()
  setInterval(loadNotifications, 60000)
})
</script>

<style scoped>
.main-layout { height: calc(var(--vh, 1vh) * 100); }
.sidebar {
  background: #1d1e2c;
  transition: width 0.3s, margin-left 0.3s;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}
.sidebar :deep(.el-menu) {
  flex: 1;
  border-right: none;
}
.sidebar-top { display: flex; align-items: center; border-bottom: 1px solid #2a2b3d; height: 56px; }
.sidebar-top .logo { flex: 1; height: 56px; }
.sidebar-hide-btn {
  width: 24px; height: 24px; border: none; background: transparent;
  color: #a0a4b8; font-size: 14px; cursor: pointer; border-radius: 4px;
  margin-right: 8px; display: flex; align-items: center; justify-content: center;
}
.sidebar-hide-btn:hover { background: #2a2b3d; color: #fff; }
.sidebar-bottom {
  margin-top: auto;
  padding: 8px 0 12px;
  border-top: 1px solid #2a2b3d;
}
.help-btn {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 20px; cursor: pointer;
  color: #a0a4b8; font-size: 14px;
  transition: all 0.2s;
}
.help-btn:hover { background: #2a2b3d; color: #fff; }
.help-btn .el-icon { font-size: 18px; }
.logo-text { letter-spacing: 2px; }
.logo-icon { font-size: 24px; }
.logo-svg { width: 28px; height: 28px; }
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

.help-content h4 { margin: 16px 0 8px; color: #303133; }
.help-content h4:first-child { margin-top: 0; }
.help-content ul { padding-left: 20px; margin: 4px 0; }
.help-content li { margin: 6px 0; color: #606266; font-size: 14px; line-height: 1.6; }
.help-content kbd { background: #f5f5f5; border: 1px solid #dcdfe6; border-radius: 3px; padding: 1px 6px; font-size: 12px; }

/* Mobile */
.menu-btn { display: none; color: #fff !important; }
.sidebar-overlay { display: none; }

@media (max-width: 768px) {
  .menu-btn { display: inline-flex !important; }
  .sidebar-overlay {
    display: none;
    position: fixed; top: 0; left: 0;
    width: 100vw; height: calc(var(--vh, 1vh) * 100);
    background: rgba(0,0,0,0.4);
    z-index: 199;
  }
  .sidebar-overlay.open { display: block; }
  .el-aside.sidebar {
    position: fixed !important;
    top: 0; left: -260px;
    height: calc(var(--vh, 1vh) * 100);
    z-index: 200;
    transition: left 0.3s;
  }
  .el-aside.sidebar.open { left: 0; }
  .breadcrumb { display: none; }
  .user-name-text { display: none; }
}
</style>
