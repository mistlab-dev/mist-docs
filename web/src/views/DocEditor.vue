<template>
  <div class="editor-page">
    <div class="editor-header">
      <el-button @click="router.push('/docs')" text>
        <el-icon><ArrowLeft /></el-icon> 返回
      </el-button>
      <el-input v-model="title" class="title-input" @blur="saveTitle" />
      <div class="header-actions">
        <el-tag v-if="onlineCount > 0" type="success" size="small">
          {{ onlineCount }} 人在线
        </el-tag>
        <el-tag size="small">{{ doc?.type === 'sheet' ? '表格' : '文档' }}</el-tag>
        <el-tag size="small">v{{ doc?.version || 1 }}</el-tag>
        <el-button type="primary" size="small" @click="saveContent">
          <el-icon><Check /></el-icon> 保存
        </el-button>
        <el-dropdown @command="handleVersion">
          <el-button size="small">
            版本 <el-icon><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="v in versions" :key="v.version" :command="v.version">
                v{{ v.version }} - {{ v.created_at }} ({{ formatSize(v.file_size) }})
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <!-- 在线用户光标 -->
    <div v-if="cursors.length" class="cursors-bar">
      <span v-for="c in cursors" :key="c.id" class="cursor-badge" :style="{ background: c.color }">
        {{ c.name }}
      </span>
    </div>

    <!-- 编辑区 -->
    <div class="editor-body">
      <div v-if="doc?.type === 'doc'" class="doc-editor">
        <textarea
          ref="editorRef"
          v-model="content"
          class="editor-textarea"
          placeholder="开始编辑..."
          @input="onInput"
        />
      </div>
      <div v-else class="sheet-editor">
        <div class="sheet-placeholder">
          <el-icon :size="48"><Grid /></el-icon>
          <p>表格编辑器（Univer 集成中）</p>
          <textarea
            v-model="content"
            class="editor-textarea"
            placeholder="表格数据（JSON）..."
            @input="onInput"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import http from '@/utils/http'

const route = useRoute()
const router = useRouter()
const docId = route.params.id as string

const doc = ref<any>(null)
const title = ref('')
const content = ref('')
const versions = ref<any[]>([])
const onlineCount = ref(0)
const cursors = ref<any[]>([])
const editorRef = ref<HTMLTextAreaElement>()
let ws: WebSocket | null = null
let dirty = false
let saveTimer: any = null

async function loadDoc() {
  const { data } = await http.get(`/docs/documents/${docId}/content`)
  doc.value = data.data?.document
  title.value = doc.value?.title || ''
  if (data.data?.content) {
    try {
      const parsed = JSON.parse(data.data.content)
      content.value = parsed.content || data.data.content
    } catch {
      content.value = data.data.content
    }
  }
}

async function loadVersions() {
  const { data } = await http.get(`/docs/documents/${docId}/versions`)
  versions.value = (data.data || []).reverse()
}

async function saveContent() {
  try {
    await http.put(`/docs/documents/${docId}/content`, {
      content: JSON.stringify({ content: content.value }),
    })
    dirty = false
    await loadDoc()
    await loadVersions()
    ElMessage.success('已保存')
  } catch {
    ElMessage.error('保存失败')
  }
}

async function saveTitle() {
  if (!title.value || title.value === doc.value?.title) return
  await http.put(`/docs/documents/${docId}`, { title: title.value })
  doc.value.title = title.value
}

function onInput() {
  dirty = true
  // Auto-save after 3s idle
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    if (dirty) saveContent()
  }, 3000)
}

function handleVersion(ver: number) {
  ElMessage.info(`恢复到版本 v${ver}`)
  http.post(`/docs/documents/${docId}/restore`, { version: ver }).then(() => {
    loadDoc()
    loadVersions()
    ElMessage.success('已恢复')
  })
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(1)} KB`
}

function connectWS() {
  const token = localStorage.getItem('token')
  if (!token) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws/docs/${docId}?token=${token}`)

  ws.onopen = () => console.log('[WS] connected')
  ws.onmessage = (e) => {
    try {
      if (typeof e.data === 'string') {
        const msg = JSON.parse(e.data)
        if (msg.type === 'clients') {
          cursors.value = msg.users || []
          onlineCount.value = cursors.value.length
        } else if (msg.type === 'join') {
          cursors.value.push(msg.user)
          onlineCount.value = cursors.value.length
        } else if (msg.type === 'leave') {
          cursors.value = cursors.value.filter((c: any) => c.id !== msg.user.id)
          onlineCount.value = cursors.value.length
        }
      }
    } catch {}
  }
  ws.onclose = () => {
    onlineCount.value = 0
    cursors.value = []
  }
}

onMounted(() => {
  loadDoc()
  loadVersions()
  connectWS()
})

onUnmounted(() => {
  if (dirty) saveContent()
  ws?.close()
  clearTimeout(saveTimer)
})
</script>

<style scoped>
.editor-page { height: 100%; display: flex; flex-direction: column; }
.editor-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid #e8e8e8;
  background: #fff;
}
.title-input {
  flex: 1;
  font-size: 18px;
  font-weight: bold;
}
.title-input :deep(.el-input__wrapper) {
  box-shadow: none !important;
  background: transparent;
}
.header-actions { display: flex; align-items: center; gap: 8px; }
.cursors-bar { display: flex; gap: 4px; padding: 4px 16px; background: #f9f9f9; }
.cursor-badge {
  padding: 2px 8px;
  border-radius: 10px;
  color: #fff;
  font-size: 12px;
}
.editor-body { flex: 1; overflow: hidden; }
.editor-textarea {
  width: 100%;
  height: 100%;
  border: none;
  outline: none;
  resize: none;
  padding: 24px;
  font-size: 15px;
  line-height: 1.8;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}
.sheet-placeholder {
  padding: 40px;
  text-align: center;
  color: #999;
}
.sheet-placeholder .editor-textarea {
  height: 400px;
  border: 1px solid #e8e8e8;
  border-radius: 4px;
  margin-top: 16px;
}
</style>
