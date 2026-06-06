<template>
  <div class="login-page">
    <div class="login-card">
      <h1 class="title">MistDocs</h1>
      <p class="subtitle">企业文档协作平台</p>
      <div class="sso-hint">
        <p>请通过 MistLab 主站登录</p>
        <el-button type="primary" size="large" style="width: 100%" @click="goPortal" :loading="loading">
          前往登录
        </el-button>
        <p class="tip">登录后将自动跳转回文档系统</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const loading = ref(false)

function goPortal() {
  loading.value = true
  auth.redirectToPortalLogin()
}
</script>

<style scoped>
.login-page {
  height: calc(var(--vh, 1vh) * 100);
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
.sso-hint {
  text-align: center;
}
.sso-hint > p {
  color: #666;
  margin-bottom: 16px;
}
.tip {
  margin-top: 16px;
  font-size: 12px;
  color: #999;
}
.login-card :deep(.el-button) {
  border-radius: 8px;
  font-size: 15px;
}
</style>