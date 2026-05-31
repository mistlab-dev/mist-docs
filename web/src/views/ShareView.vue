<template>
  <div class="share-page">
    <!-- 加载 -->
    <div v-if="loading" class="state-center">
      <div class="loader-ring" />
      <p class="loader-text">加载中...</p>
    </div>

    <!-- 错误 -->
    <div v-else-if="error" class="state-center">
      <div class="error-icon">🔒</div>
      <h2 class="error-title">{{ error }}</h2>
      <p class="error-desc">请确认链接是否正确，或联系分享者</p>
    </div>

    <!-- 密码验证 -->
    <div v-else-if="needPassword" class="state-center">
      <div class="password-card">
        <div class="pw-icon">🔐</div>
        <h3 class="pw-title">{{ shareInfo?.title || '受保护的文档' }}</h3>
        <p class="pw-desc">此文档需要密码才能访问</p>
        <el-input v-model="password" type="password" placeholder="输入访问密码" size="large" show-password @keyup.enter="accessWithPassword" />
        <el-button type="primary" size="large" class="pw-btn" @click="accessWithPassword" :loading="verifying">
          访问文档
        </el-button>
      </div>
    </div>

    <!-- 文档内容 -->
    <div v-else class="share-content">
      <div class="content-inner">
        <div class="share-header">
          <div class="header-badge">
            <div class="badge-icon" :class="doc.type === 'sheet' ? 'sheet' : 'doc'">
              <el-icon :size="16"><Document v-if="doc.type !== 'sheet'" /><Grid v-else /></el-icon>
            </div>
            <span>{{ doc.type === 'sheet' ? '表格' : '文档' }}</span>
          </div>
          <h1 class="doc-title">{{ doc.title }}</h1>
          <div class="share-meta">
            <el-icon :size="14"><Link /></el-icon>
            <span>通过分享链接访问</span>
          </div>
        </div>
        <el-divider />
        <div class="doc-body" v-html="doc.content"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'

const route = useRoute()
const token = route.params.token as string

const loading = ref(true)
const verifying = ref(false)
const error = ref('')
const needPassword = ref(false)
const shareInfo = ref<any>(null)
const doc = ref<any>({})
const password = ref('')

async function loadShare() {
  try {
    const { data } = await axios.get(`/api/s/${token}`)
    doc.value = data
  } catch (e: any) {
    const resp = e.response?.data
    if (resp?.need_password) {
      needPassword.value = true
      shareInfo.value = { title: '受保护的文档' }
      try {
        const { data: info } = await axios.get(`/api/s/${token}/info`)
        shareInfo.value = info
      } catch {}
    } else if (e.response?.status === 410) {
      error.value = '分享链接已过期'
    } else {
      error.value = resp?.error || '分享链接不存在'
    }
  } finally {
    loading.value = false
  }
}

async function accessWithPassword() {
  if (!password.value) return
  verifying.value = true
  try {
    const { data } = await axios.get(`/api/s/${token}?password=${encodeURIComponent(password.value)}`)
    doc.value = data
    needPassword.value = false
  } catch (e: any) {
    error.value = e.response?.data?.error || '密码错误'
  } finally {
    verifying.value = false
  }
}

onMounted(loadShare)
</script>

<style scoped>
.share-page { min-height: 100vh; background: #f5f7fa; }

/* 居中状态 */
.state-center {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; min-height: 100vh; padding: 40px 20px;
}

/* 加载动画 */
.loader-ring {
  width: 40px; height: 40px; border-radius: 50%;
  border: 3px solid #e8ecf0; border-top-color: #4f6ef7;
  animation: spin 0.8s linear infinite;
}
.loader-text { margin-top: 16px; color: #909399; font-size: 14px; }
@keyframes spin { to { transform: rotate(360deg); } }

/* 错误 */
.error-icon { font-size: 56px; margin-bottom: 16px; }
.error-title { font-size: 18px; color: #303133; margin: 0 0 8px; }
.error-desc { font-size: 14px; color: #909399; margin: 0; }

/* 密码卡片 */
.password-card {
  width: 380px; padding: 36px; background: #fff;
  border-radius: 16px; box-shadow: 0 8px 32px rgba(0,0,0,0.08);
  display: flex; flex-direction: column; align-items: center;
}
.pw-icon { font-size: 40px; margin-bottom: 12px; }
.pw-title { font-size: 18px; color: #303133; margin: 0 0 6px; text-align: center; }
.pw-desc { font-size: 13px; color: #909399; margin: 0 0 20px; text-align: center; }
.password-card :deep(.el-input) { width: 100%; }
.password-card :deep(.el-input__wrapper) { border-radius: 10px; }
.pw-btn { width: 100%; margin-top: 14px; border-radius: 10px; font-size: 15px; }

/* 文档内容 */
.share-content {
  display: flex; justify-content: center; padding: 32px 20px;
}
.content-inner {
  width: 100%; max-width: 800px; background: #fff;
  border-radius: 16px; padding: 40px 48px; min-height: calc(100vh - 64px);
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.share-header { margin-bottom: 8px; }
.header-badge {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 4px 12px; background: #f5f7fa; border-radius: 20px;
  font-size: 12px; color: #606266; margin-bottom: 12px;
}
.badge-icon {
  width: 22px; height: 22px; border-radius: 6px;
  display: flex; align-items: center; justify-content: center;
}
.badge-icon.doc { background: #e8f0fe; color: #4f6ef7; }
.badge-icon.sheet { background: #e6f7f0; color: #36b37e; }

.doc-title { font-size: 26px; font-weight: 700; color: #1a1a2e; margin: 0 0 10px; }
.share-meta {
  display: flex; align-items: center; gap: 6px;
  color: #909399; font-size: 13px;
}
.share-header + :deep(.el-divider) { margin: 16px 0 24px; }

/* 富文本样式 */
.doc-body { line-height: 1.8; color: #333; }
.doc-body :deep(h1) { font-size: 24px; margin: 32px 0 16px; }
.doc-body :deep(h2) { font-size: 20px; margin: 28px 0 12px; }
.doc-body :deep(h3) { font-size: 17px; margin: 20px 0 10px; }
.doc-body :deep(p) { margin: 0 0 12px; }
.doc-body :deep(img) { max-width: 100%; border-radius: 8px; margin: 8px 0; }
.doc-body :deep(table) { border-collapse: collapse; width: 100%; margin: 12px 0; }
.doc-body :deep(th), .doc-body :deep(td) { border: 1px solid #e8ecf0; padding: 10px 14px; }
.doc-body :deep(th) { background: #fafbfc; font-weight: 600; }
.doc-body :deep(pre) { background: #f6f8fa; padding: 16px; border-radius: 10px; overflow-x: auto; font-size: 13px; }
.doc-body :deep(code) { background: #f0f2f5; padding: 2px 6px; border-radius: 4px; font-size: 13px; }
.doc-body :deep(blockquote) { border-left: 4px solid #4f6ef7; padding-left: 16px; color: #606266; margin: 12px 0; }
.doc-body :deep(ul), .doc-body :deep(ol) { padding-left: 24px; }
.doc-body :deep(a) { color: #4f6ef7; text-decoration: none; }
.doc-body :deep(a:hover) { text-decoration: underline; }

@media (max-width: 768px) {
  .content-inner { padding: 24px 16px; border-radius: 0; }
  .doc-title { font-size: 20px; }
  .share-content { padding: 0; }
}
</style>
