<template>
  <div class="share-page">
    <div v-if="loading" class="loading">
      <el-icon :size="32" class="spin"><Loading /></el-icon>
      <p>加载中...</p>
    </div>

    <div v-else-if="error" class="error">
      <el-icon :size="48" color="#f56c6c"><CircleClose /></el-icon>
      <h2>{{ error }}</h2>
    </div>

    <div v-else-if="needPassword" class="password-form">
      <el-card style="max-width:400px;margin:100px auto">
        <h3 style="text-align:center;margin-top:0">{{ shareInfo?.title }}</h3>
        <p style="text-align:center;color:#909399">此文档需要密码访问</p>
        <el-input v-model="password" type="password" placeholder="输入访问密码" @keyup.enter="accessWithPassword" />
        <el-button type="primary" style="width:100%;margin-top:12px" @click="accessWithPassword" :loading="loading">访问文档</el-button>
      </el-card>
    </div>

    <div v-else class="share-content">
      <div class="share-header">
        <h1>{{ doc.title }}</h1>
        <div class="share-meta">
          <el-tag size="small">{{ doc.type === 'sheet' ? '表格' : '文档' }}</el-tag>
          <span>通过分享链接访问</span>
        </div>
      </div>
      <el-divider />
      <div class="doc-content" v-html="doc.content"></div>
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
      // Try getting info
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
  loading.value = true
  try {
    const { data } = await axios.get(`/api/s/${token}?password=${encodeURIComponent(password.value)}`)
    doc.value = data
    needPassword.value = false
  } catch (e: any) {
    error.value = e.response?.data?.error || '访问失败'
  } finally {
    loading.value = false
  }
}

onMounted(loadShare)
</script>

<style scoped>
.share-page { min-height: 100vh; background: #f5f7fa; }
.loading, .error { text-align: center; padding-top: 120px; }
.loading .spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.share-content { max-width: 800px; margin: 0 auto; padding: 40px 20px; background: #fff; min-height: 100vh; }
.share-header h1 { margin: 0; font-size: 24px; }
.share-meta { display: flex; align-items: center; gap: 8px; margin-top: 8px; color: #909399; font-size: 13px; }
.doc-content { line-height: 1.8; }
.doc-content :deep(img) { max-width: 100%; }
.doc-content :deep(table) { border-collapse: collapse; width: 100%; }
.doc-content :deep(th), .doc-content :deep(td) { border: 1px solid #ddd; padding: 8px; }
.doc-content :deep(pre) { background: #f4f4f4; padding: 12px; border-radius: 6px; overflow-x: auto; }
.doc-content :deep(blockquote) { border-left: 4px solid #ddd; padding-left: 1em; color: #666; }
</style>
