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
                v{{ v.version }} - {{ v.created_at }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <!-- 在线用户 -->
    <div v-if="cursors.length" class="cursors-bar">
      <span v-for="c in cursors" :key="c.id" class="cursor-badge" :style="{ background: c.color }">
        {{ c.name }}
      </span>
    </div>

    <!-- TipTap 工具栏（仅文档） -->
    <div v-if="editor" class="toolbar">
      <el-button-group>
        <el-button size="small" @click="editor.chain().focus().toggleBold().run()" :type="editor.isActive('bold') ? 'primary' : ''">
          <strong>B</strong>
        </el-button>
        <el-button size="small" @click="editor.chain().focus().toggleItalic().run()" :type="editor.isActive('italic') ? 'primary' : ''">
          <em>I</em>
        </el-button>
        <el-button size="small" @click="editor.chain().focus().toggleStrike().run()" :type="editor.isActive('strike') ? 'primary' : ''">
          <s>S</s>
        </el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 1 }).run()" :type="editor.isActive('heading', { level: 1 }) ? 'primary' : ''">H1</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 2 }).run()" :type="editor.isActive('heading', { level: 2 }) ? 'primary' : ''">H2</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleBulletList().run()" :type="editor.isActive('bulletList') ? 'primary' : ''">• 列表</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleOrderedList().run()" :type="editor.isActive('orderedList') ? 'primary' : ''">1. 有序</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleCodeBlock().run()" :type="editor.isActive('codeBlock') ? 'primary' : ''">代码</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleBlockquote().run()" :type="editor.isActive('blockquote') ? 'primary' : ''">引用</el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().setHorizontalRule().run()">分割线</el-button>
        <el-button size="small" @click="editor.chain().focus().undo().run()">撤销</el-button>
        <el-button size="small" @click="editor.chain().focus().redo().run()">重做</el-button>
      </el-button-group>
    </div>

    <!-- 文档编辑器（TipTap） -->
    <div v-if="doc?.type === 'doc' && editor" class="editor-body">
      <editor-content :editor="editor" class="tiptap-editor" />
    </div>

    <!-- 表格编辑器（Univer） -->
    <div v-else-if="doc?.type === 'sheet'" class="editor-body sheet-body">
      <SheetEditor ref="sheetRef" :initial-data="sheetData" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Editor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'
import Placeholder from '@tiptap/extension-placeholder'
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'
import http from '@/utils/http'
import SheetEditor from '@/components/SheetEditor.vue'

const route = useRoute()
const router = useRouter()
const docId = route.params.id as string

const doc = ref<any>(null)
const title = ref('')
const versions = ref<any[]>([])
const onlineCount = ref(0)
const cursors = ref<any[]>([])
const editor = ref<Editor | null>(null)
const sheetData = ref('{}')
const sheetRef = ref<InstanceType<typeof SheetEditor> | null>(null)
let ydoc: Y.Doc | null = null
let wsProvider: WebsocketProvider | null = null
let saveTimer: any = null

const userName = localStorage.getItem('user_name') || '用户'
const userColors = ['#e06c75', '#e5c07b', '#98c379', '#56b6c2', '#61afef', '#c678dd', '#d19a66', '#be5046']
const userColor = userColors[Math.floor(Math.random() * userColors.length)]

async function loadDoc() {
  const { data } = await http.get(`/docs/documents/${docId}/content`)
  doc.value = data.data
  title.value = doc.value?.title || ''
  // 表格数据
  if (doc.value?.type === 'sheet') {
    sheetData.value = doc.value?.content || '{}'
  }
}

async function loadVersions() {
  const { data } = await http.get(`/docs/documents/${docId}/versions`)
  versions.value = (data.data || []).reverse()
}

function initEditor(initialContent?: string) {
  ydoc = new Y.Doc()

  const token = localStorage.getItem('token')
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  wsProvider = new WebsocketProvider(
    `${proto}://${location.host}/ws`,
    `docs/${docId}?token=${token}`,
    ydoc,
    { connect: true }
  )

  wsProvider.awareness.setLocalStateField('user', { name: userName, color: userColor })
  wsProvider.awareness.on('change', () => {
    const states = wsProvider!.awareness.getStates()
    cursors.value = Array.from(states.entries()).map(([id, state]) => ({
      id: String(id),
      name: state.user?.name || 'Unknown',
      color: state.user?.color || '#999',
    }))
    onlineCount.value = cursors.value.length
  })

  editor.value = new Editor({
    content: initialContent || '',
    extensions: [
      StarterKit.configure({ history: false }),
      Collaboration.configure({ document: ydoc, field: 'content' }),
      CollaborationCursor.configure({
        provider: wsProvider,
        user: { name: userName, color: userColor },
      }),
      Placeholder.configure({ placeholder: '开始编辑文档...' }),
    ],
    editorProps: {
      attributes: { class: 'prose prose-lg focus:outline-none max-w-none' },
    },
  })

  ydoc.on('update', () => {
    clearTimeout(saveTimer)
    saveTimer = setTimeout(saveContent, 3000)
  })
}

async function saveContent() {
  let content = ''
  if (doc.value?.type === 'sheet') {
    content = sheetRef.value?.getData() || '{}'
  } else if (editor.value) {
    content = editor.value.getHTML()
  }
  await http.put(`/docs/documents/${docId}/content`, { content })
  await loadDoc()
  await loadVersions()
}

async function saveTitle() {
  if (!title.value || title.value === doc.value?.title) return
  await http.put(`/docs/documents/${docId}`, { title: title.value })
  doc.value.title = title.value
}

function handleVersion(ver: number) {
  ElMessage.info(`恢复版本 v${ver}（需后端支持返回内容）`)
}

onMounted(async () => {
  await loadDoc()
  await loadVersions()
  if (doc.value?.type === 'doc') {
    const content = doc.value?.content || ''
    initEditor(content === '{}' ? '' : content)
  }
})

onUnmounted(() => {
  if (saveTimer) {
    // 退出前保存
    saveContent().catch(() => {})
  }
  clearTimeout(saveTimer)
  editor.value?.destroy()
  wsProvider?.disconnect()
  wsProvider?.destroy()
  ydoc?.destroy()
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
.title-input { flex: 1; font-size: 18px; font-weight: bold; }
.title-input :deep(.el-input__wrapper) { box-shadow: none !important; background: transparent; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.cursors-bar { display: flex; gap: 4px; padding: 4px 16px; background: #f9f9f9; }
.cursor-badge { padding: 2px 8px; border-radius: 10px; color: #fff; font-size: 12px; }
.toolbar {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  border-bottom: 1px solid #e8e8e8;
  background: #fafafa;
  flex-wrap: wrap;
  gap: 4px;
}
.editor-body { flex: 1; overflow-y: auto; background: #fff; }
.sheet-body { overflow: hidden; }
.tiptap-editor { padding: 24px 48px; min-height: 100%; }
.tiptap-editor :deep(.ProseMirror) { outline: none; min-height: 60vh; }
.tiptap-editor :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  color: #adb5bd;
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
}
.tiptap-editor :deep(.ProseMirror h1) { font-size: 2em; margin: 1em 0 0.5em; }
.tiptap-editor :deep(.ProseMirror h2) { font-size: 1.5em; margin: 1em 0 0.5em; }
.tiptap-editor :deep(.ProseMirror p) { margin: 0.5em 0; line-height: 1.6; }
.tiptap-editor :deep(.ProseMirror ul),
.tiptap-editor :deep(.ProseMirror ol) { padding-left: 1.5em; margin: 0.5em 0; }
.tiptap-editor :deep(.ProseMirror blockquote) { border-left: 3px solid #ddd; padding-left: 1em; color: #666; }
.tiptap-editor :deep(.ProseMirror pre) { background: #f5f5f5; padding: 12px 16px; border-radius: 4px; overflow-x: auto; }
.tiptap-editor :deep(.ProseMirror code) { background: #f5f5f5; padding: 2px 4px; border-radius: 3px; }
.tiptap-editor :deep(.ProseMirror hr) { border: none; border-top: 2px solid #e8e8e8; margin: 1.5em 0; }
.tiptap-editor :deep(.ProseMirror strong) { font-weight: bold; }
.tiptap-editor :deep(.ProseMirror em) { font-style: italic; }
.tiptap-editor :deep(.collaboration-cursor__caret) {
  position: relative;
  margin-left: -1px;
  margin-right: -1px;
  border-left: 1px solid;
  border-right: 1px solid;
  word-break: normal;
  pointer-events: none;
}
.tiptap-editor :deep(.collaboration-cursor__label) {
  position: absolute;
  top: -1.4em;
  left: -1px;
  font-size: 12px;
  font-weight: 600;
  line-height: normal;
  user-select: none;
  color: #fff;
  padding: 0.1em 0.3em;
  border-radius: 3px 3px 3px 0;
  white-space: nowrap;
}
</style>