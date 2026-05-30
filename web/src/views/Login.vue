<template>
  <div class="login-page">
    <div class="login-card">
      <h1 class="title">MistDocs</h1>
      <p class="subtitle">企业文档协作平台</p>
      <el-form :model="form" @submit.prevent="handleLogin">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" prefix-icon="User" size="large" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" placeholder="密码" prefix-icon="Lock" size="large" show-password />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" native-type="submit">
          登录
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const form = ref({ username: '', password: '' })

async function handleLogin() {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(form.value.username, form.value.password)
    router.push('/')
  } catch {
    ElMessage.error('登录失败，请检查用户名和密码')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1d1e2c 0%, #2a3a5c 50%, #1d1e2c 100%);
  background-size: 200% 200%;
  animation: gradientShift 8s ease infinite;
}
@keyframes gradientShift {
  0%, 100% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
}
.login-card {
  width: 380px;
  padding: 40px;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}
.title {
  text-align: center;
  font-size: 28px;
  margin: 0 0 4px;
  color: #1d1e2c;
  letter-spacing: -0.5px;
}
.subtitle {
  text-align: center;
  color: #999;
  margin: 0 0 32px;
  font-size: 14px;
}
.login-card :deep(.el-input__wrapper) {
  border-radius: 8px;
}
.login-card :deep(.el-button) {
  border-radius: 8px;
  font-size: 15px;
}
</style>
