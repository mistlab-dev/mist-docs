<template>
  <el-container class="main-layout">
    <!-- 侧边栏 -->
    <el-aside :width="collapsed ? '64px' : '220px'" class="sidebar">
      <div class="logo" @click="collapsed = !collapsed">
        <span v-if="!collapsed" class="logo-text">MistDocs</span>
        <span v-else class="logo-icon">📄</span>
      </div>
      <el-menu
        :default-active="route.path"
        :collapse="collapsed"
        router
        background-color="#1d1e2c"
        text-color="#a0a4b8"
        active-text-color="#409eff"
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
        <div class="breadcrumb">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/docs' }">文档</el-breadcrumb-item>
            <el-breadcrumb-item v-if="route.name === 'DocEditor'">
              {{ route.params.id }}
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="user-area">
          <el-dropdown @command="handleCommand">
            <span class="user-name">
              {{ auth.user?.name || auth.user?.username }}
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
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
  </el-container>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import http from '@/utils/http'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const collapsed = ref(false)

const showPasswordDialog = ref(false)
const passwordForm = ref({ old: '', new_: '' })

function handleCommand(cmd: string) {
  if (cmd === 'logout') {
    auth.logout()
    router.push('/login')
  } else if (cmd === 'password') {
    showPasswordDialog.value = true
  }
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
  if (auth.token && !auth.user) {
    auth.fetchMe()
  }
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
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e8e8e8;
  background: #fff;
}
.user-name {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 4px;
}
.el-divider { margin: 8px 16px; border-color: #2a2b3d; }
</style>
