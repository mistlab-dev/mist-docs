<template>
  <div class="sso-callback">
    <div class="loading">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p>{{ t('sso.loggingIn') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { Loading } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { t } = useI18n()

onMounted(async () => {
  const token = (route.query.token as string) || ''
  const refreshToken = (route.query.refresh_token as string) || ''
  if (!token) {
    auth.redirectToPortalLogin()
    return
  }

  const ok = await auth.handleSSOCallback(token, refreshToken || undefined)
  if (ok) {
    // Clean URL
    window.history.replaceState({}, '', '/')
    router.push('/docs')
  } else {
    auth.redirectToPortalLogin()
  }
})
</script>

<style scoped>
.sso-callback {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
}
.loading {
  text-align: center;
  color: #666;
}
</style>