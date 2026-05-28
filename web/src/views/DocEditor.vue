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
        <el-button type="primary" size="small" @click="manualSave">
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

    <!-- TipTap 工具栏 -->
    <div v-if="editor" class="toolbar">
      <!-- 文本格式 -->
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
        <el-button size="small" @click="editor.chain().focus().toggleUnderline().run()" :type="editor.isActive('underline') ? 'primary' : ''">
          <u>U</u>
        </el-button>
      </el-button-group>

      <!-- 标题 -->
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 1 }).run()" :type="editor.isActive('heading', { level: 1 }) ? 'primary' : ''">H1</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 2 }).run()" :type="editor.isActive('heading', { level: 2 }) ? 'primary' : ''">H2</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 3 }).run()" :type="editor.isActive('heading', { level: 3 }) ? 'primary' : ''">H3</el-button>
      </el-button-group>

      <!-- 列表/引用 -->
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().toggleBulletList().run()" :type="editor.isActive('bulletList') ? 'primary' : ''">• 列表</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleOrderedList().run()" :type="editor.isActive('orderedList') ? 'primary' : ''">1. 有序</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleTaskList().run()" :type="editor.isActive('taskList') ? 'primary' : ''">☑ 任务</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleBlockquote().run()" :type="editor.isActive('blockquote') ? 'primary' : ''">引用</el-button>
      </el-button-group>

      <!-- 代码 -->
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="toggleCodeBlock" :type="editor.isActive('codeBlock') ? 'primary' : ''">代码块</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleCode().run()" :type="editor.isActive('code') ? 'primary' : ''">行内代码</el-button>
      </el-button-group>

      <!-- 插入 -->
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="insertLink" :type="editor.isActive('link') ? 'primary' : ''">🔗 链接</el-button>
        <el-button size="small" @click="triggerImageUpload">🖼 图片</el-button>
        <el-button size="small" @click="insertTable">📋 表格</el-button>
        <el-button size="small" @click="editor.chain().focus().setHorizontalRule().run()">— 线</el-button>
      </el-button-group>

      <!-- 表格操作（表格激活时显示） -->
      <el-button-group v-if="editor.isActive('table')" style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().addRowBefore().run()">+ 行上</el-button>
        <el-button size="small" @click="editor.chain().focus().addRowAfter().run()">+ 行下</el-button>
        <el-button size="small" @click="editor.chain().focus().addColumnBefore().run()">+ 列左</el-button>
        <el-button size="small" @click="editor.chain().focus().addColumnAfter().run()">+ 列右</el-button>
        <el-button size="small" type="danger" @click="editor.chain().focus().deleteRow().run()">删行</el-button>
        <el-button size="small" type="danger" @click="editor.chain().focus().deleteColumn().run()">删列</el-button>
        <el-button size="small" type="danger" @click="editor.chain().focus().deleteTable().run()">删表</el-button>
        <el-button size="small" @click="editor.chain().focus().mergeCells().run()">合并</el-button>
        <el-button size="small" @click="editor.chain().focus().splitCell().run()">拆分</el-button>
      </el-button-group>

      <!-- 撤销重做 -->
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().undo().run()">↩ 撤销</el-button>
        <el-button size="small" @click="editor.chain().focus().redo().run()">↪ 重做</el-button>
      </el-button-group>
    </div>

    <!-- 隐藏的文件上传 -->
    <input type="file" ref="imageInput" style="display:none" accept="image/*" @change="handleImageUpload" />

    <!-- 文档编辑器 -->
    <div v-if="doc?.type === 'doc' && editor" class="editor-body">
      <editor-content :editor="editor" class="tiptap-editor" />
    </div>

    <!-- 表格编辑器 -->
    <div v-else-if="doc?.type === 'sheet'" class="editor-body sheet-body">
      <SheetEditor ref="sheetRef" :initial-data="sheetData" @change="onSheetChange" />
    </div>

    <!-- 链接弹窗 -->
    <el-dialog v-model="linkDialog.show" title="插入链接" width="420px">
      <el-form label-width="60px">
        <el-form-item label="文本">
          <el-input v-model="linkDialog.text" placeholder="显示文字" />
        </el-form-item>
        <el-form-item label="链接">
          <el-input v-model="linkDialog.url" placeholder="https://" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="linkDialog.show = false">取消</el-button>
        <el-button v-if="editor?.isActive('link')" type="danger" @click="removeLink">移除链接</el-button>
        <el-button type="primary" @click="confirmLink">确定</el-button>
      </template>
    </el-dialog>

    <!-- 代码语言选择弹窗 -->
    <el-dialog v-model="codeLangDialog.show" title="代码块语言" width="320px">
      <el-select v-model="codeLangDialog.lang" placeholder="选择语言" style="width:100%">
        <el-option-group label="常用">
          <el-option v-for="l in popularLangs" :key="l" :label="l" :value="l" />
        </el-option-group>
        <el-option-group label="其他">
          <el-option v-for="l in otherLangs" :key="l" :label="l" :value="l" />
        </el-option-group>
      </el-select>
      <template #footer>
        <el-button @click="codeLangDialog.show = false">取消</el-button>
        <el-button type="primary" @click="confirmCodeLang">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Editor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Underline from '@tiptap/extension-underline'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import Placeholder from '@tiptap/extension-placeholder'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableCell } from '@tiptap/extension-table-cell'
import { TableHeader } from '@tiptap/extension-table-header'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'
import http from '@/utils/http'
import SheetEditor from '@/components/SheetEditor.vue'

const lowlight = createLowlight(common)

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
const imageInput = ref<HTMLInputElement | null>(null)
let ydoc: Y.Doc | null = null
let wsProvider: WebsocketProvider | null = null
let autoSaveTimer: any = null
let lastSavedContent = ''

const userName = localStorage.getItem('user_name') || '用户'
const userColors = ['#e06c75', '#e5c07b', '#98c379', '#56b6c2', '#61afef', '#c678dd', '#d19a66', '#be5046']
const userColor = userColors[Math.floor(Math.random() * userColors.length)]

// 链接弹窗
const linkDialog = reactive({ show: false, text: '', url: '' })

// 代码语言弹窗
const codeLangDialog = reactive({ show: false, lang: 'plaintext' })
const popularLangs = ['plaintext', 'javascript', 'typescript', 'python', 'go', 'java', 'bash', 'sql', 'html', 'css', 'json', 'yaml', 'markdown']
const otherLangs = ['c', 'cpp', 'csharp', 'rust', 'ruby', 'php', 'swift', 'kotlin', 'scala', 'lua', 'perl', 'r', 'dockerfile', 'nginx', 'xml', 'diff']

async function loadDoc() {
  const { data } = await http.get(`/docs/documents/${docId}/content`)
  doc.value = data.data
  title.value = doc.value?.title || ''
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

  const yContent = ydoc.getText('content')
  const hasContent = yContent.length > 0

  editor.value = new Editor({
    content: hasContent ? undefined : (initialContent || ''),
    extensions: [
      StarterKit.configure({
        history: false,
        codeBlock: false, // 用 CodeBlockLowlight 替代
      }),
      Underline,
      TaskList,
      TaskItem.configure({ nested: true }),
      Placeholder.configure({ placeholder: '开始输入内容...' }),
      Image.configure({
        inline: false,
        allowBase64: true,
        HTMLAttributes: { class: 'editor-image' },
      }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: { class: 'editor-link', target: '_blank', rel: 'noopener' },
      }),
      Table.configure({ resizable: true }),
      TableRow,
      TableCell,
      TableHeader,
      CodeBlockLowlight.configure({ lowlight }),
      Collaboration.configure({ document: ydoc, field: 'content' }),
      CollaborationCursor.configure({
        provider: wsProvider,
        user: { name: userName, color: userColor },
      }),
    ],
    editorProps: {
      attributes: { class: 'prose prose-lg focus:outline-none max-w-none' },
      handlePaste: (view, event) => {
        // 粘贴图片
        const items = event.clipboardData?.items
        if (items) {
          for (let i = 0; i < items.length; i++) {
            if (items[i].type.indexOf('image') >= 0) {
              event.preventDefault()
              const file = items[i].getAsFile()
              if (file) uploadImageFile(file)
              return true
            }
          }
        }
        return false
      },
      handleDrop: (view, event) => {
        // 拖放图片
        const files = event.dataTransfer?.files
        if (files) {
          for (let i = 0; i < files.length; i++) {
            if (files[i].type.indexOf('image') >= 0) {
              event.preventDefault()
              uploadImageFile(files[i])
              return true
            }
          }
        }
        return false
      },
    },
  })

  ydoc.on('update', () => scheduleAutoSave())
  editor.value.on('update', () => scheduleAutoSave())
}

// 图片上传
function triggerImageUpload() {
  imageInput.value?.click()
}

async function handleImageUpload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  await uploadImageFile(file)
  ;(e.target as HTMLInputElement).value = ''
}

async function uploadImageFile(file: File) {
  try {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await http.post('/docs/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    const url = data.data?.url || data.data?.path || data.data
    editor.value?.chain().focus().setImage({ src: url }).run()
    ElMessage.success('图片已上传')
  } catch (e: any) {
    // 如果上传失败，转 base64 内联
    const reader = new FileReader()
    reader.onload = () => {
      editor.value?.chain().focus().setImage({ src: reader.result as string }).run()
    }
    reader.readAsDataURL(file)
  }
}

// 链接
function insertLink() {
  const existingUrl = editor.value?.getAttributes('link').href || ''
  const selectedText = editor.value?.state.selection.content()?.content.firstChild?.text || ''
  linkDialog.text = selectedText
  linkDialog.url = existingUrl
  linkDialog.show = true
}

function confirmLink() {
  if (!linkDialog.url) { linkDialog.show = false; return }
  if (linkDialog.text && linkDialog.text !== editor.value?.state.selection.content()?.content.firstChild?.text) {
    // 有自定义文本
    editor.value?.chain().focus().extendMarkRange('link').insertContent({
      type: 'text',
      text: linkDialog.text,
      marks: [{ type: 'link', attrs: { href: linkDialog.url } }],
    }).run()
  } else {
    editor.value?.chain().focus().extendMarkRange('link').setLink({ href: linkDialog.url }).run()
  }
  linkDialog.show = false
}

function removeLink() {
  editor.value?.chain().focus().unsetLink().run()
  linkDialog.show = false
}

// 表格
function insertTable() {
  editor.value?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()
}

// 代码块
function toggleCodeBlock() {
  if (editor.value?.isActive('codeBlock')) {
    editor.value.chain().focus().toggleCodeBlock().run()
  } else {
    codeLangDialog.lang = 'plaintext'
    codeLangDialog.show = true
  }
}

function confirmCodeLang() {
  editor.value?.chain().focus().toggleCodeBlock({ language: codeLangDialog.lang }).run()
  codeLangDialog.show = false
}

// 保存
function scheduleAutoSave() {
  clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(doSave, 3000)
}

async function doSave() {
  let content = ''
  if (doc.value?.type === 'sheet') {
    content = sheetRef.value?.getData() || '{}'
  } else if (editor.value) {
    content = editor.value.getHTML()
  }
  if (content === lastSavedContent) return
  lastSavedContent = content
  try {
    await http.put(`/docs/documents/${docId}/content`, { content })
    await loadDoc()
    await loadVersions()
  } catch (e) {
    console.error('保存失败', e)
  }
}

async function manualSave() {
  clearTimeout(autoSaveTimer)
  await doSave()
  ElMessage.success('已保存')
}

async function saveTitle() {
  if (!title.value || title.value === doc.value?.title) return
  await http.put(`/docs/documents/${docId}`, { title: title.value })
  doc.value.title = title.value
}

function handleVersion(ver: number) {
  ElMessage.info(`恢复版本 v${ver}（需后端支持返回内容）`)
}

function onSheetChange() {
  scheduleAutoSave()
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
  doSave().catch(() => {})
  clearTimeout(autoSaveTimer)
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
  padding: 6px 16px;
  border-bottom: 1px solid #e8e8e8;
  background: #fafafa;
  flex-wrap: wrap;
  gap: 4px;
}
.editor-body { flex: 1; overflow-y: auto; background: #fff; }
.sheet-body { overflow: hidden; }
.tiptap-editor { padding: 24px 48px; min-height: 100%; }

/* ProseMirror 基础样式 */
.tiptap-editor :deep(.ProseMirror) { outline: none; min-height: 60vh; }
.tiptap-editor :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  color: #adb5bd;
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
}

/* 标题 */
.tiptap-editor :deep(.ProseMirror h1) { font-size: 2em; margin: 1em 0 0.5em; border-bottom: 1px solid #eee; padding-bottom: 0.3em; }
.tiptap-editor :deep(.ProseMirror h2) { font-size: 1.5em; margin: 1em 0 0.5em; border-bottom: 1px solid #eee; padding-bottom: 0.3em; }
.tiptap-editor :deep(.ProseMirror h3) { font-size: 1.25em; margin: 1em 0 0.5em; }
.tiptap-editor :deep(.ProseMirror p) { margin: 0.5em 0; line-height: 1.7; }

/* 列表 */
.tiptap-editor :deep(.ProseMirror ul),
.tiptap-editor :deep(.ProseMirror ol) { padding-left: 1.5em; margin: 0.5em 0; }

/* 任务列表 */
.tiptap-editor :deep(.ProseMirror ul[data-type="taskList"]) { list-style: none; padding-left: 0; }
.tiptap-editor :deep(.ProseMirror ul[data-type="taskList"] li) {
  display: flex; align-items: flex-start; gap: 6px; margin: 4px 0;
}
.tiptap-editor :deep(.ProseMirror ul[data-type="taskList"] li label) { margin-top: 4px; }
.tiptap-editor :deep(.ProseMirror ul[data-type="taskList"] li label input[type="checkbox"]) {
  width: 16px; height: 16px; cursor: pointer;
}

/* 引用 */
.tiptap-editor :deep(.ProseMirror blockquote) {
  border-left: 4px solid #409eff; padding: 8px 16px; margin: 0.5em 0;
  background: #f0f7ff; border-radius: 0 4px 4px 0; color: #555;
}

/* 行内代码 */
.tiptap-editor :deep(.ProseMirror code) {
  background: #f0f0f0; padding: 2px 6px; border-radius: 3px;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace; font-size: 0.9em; color: #c7254e;
}

/* 代码块 */
.tiptap-editor :deep(.ProseMirror pre) {
  background: #1e1e2e; color: #cdd6f4; padding: 16px 20px; border-radius: 8px;
  overflow-x: auto; margin: 1em 0; font-size: 14px; line-height: 1.6;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Fira Code', monospace;
}
.tiptap-editor :deep(.ProseMirror pre code) {
  background: none; color: inherit; padding: 0; font-size: inherit;
}

/* 链接 */
.tiptap-editor :deep(.editor-link) {
  color: #409eff; text-decoration: underline; cursor: pointer;
}
.tiptap-editor :deep(.editor-link:hover) { color: #66b1ff; }

/* 图片 */
.tiptap-editor :deep(.editor-image) {
  max-width: 100%; height: auto; border-radius: 6px; margin: 1em 0;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1); cursor: default;
}
.tiptap-editor :deep(.ProseMirror img) {
  max-width: 100%; height: auto; border-radius: 6px; margin: 1em 0;
}

/* 表格 */
.tiptap-editor :deep(.ProseMirror table) {
  border-collapse: collapse; width: 100%; margin: 1em 0; overflow: hidden;
}
.tiptap-editor :deep(.ProseMirror table td),
.tiptap-editor :deep(.ProseMirror table th) {
  border: 1px solid #d0d3d8; padding: 8px 12px; min-width: 80px;
  vertical-align: top; position: relative;
}
.tiptap-editor :deep(.ProseMirror table th) {
  background: #f5f7fa; font-weight: 600; text-align: left;
}
.tiptap-editor :deep(.ProseMirror table .selectedCell) {
  background: #e8f0fe;
}
.tiptap-editor :deep(.ProseMirror table .column-resize-handle) {
  position: absolute; right: -2px; top: 0; bottom: -2px; width: 4px;
  background-color: #409eff; pointer-events: none;
}

/* 分割线 */
.tiptap-editor :deep(.ProseMirror hr) { border: none; border-top: 2px solid #e8e8e8; margin: 1.5em 0; }

/* 协同光标 */
.tiptap-editor :deep(.collaboration-cursor__caret) {
  position: relative; margin-left: -1px; margin-right: -1px;
  border-left: 1px solid; border-right: 1px solid;
  word-break: normal; pointer-events: none;
}
.tiptap-editor :deep(.collaboration-cursor__label) {
  position: absolute; top: -1.4em; left: -1px;
  font-size: 12px; font-weight: 600; line-height: normal;
  user-select: none; color: #fff; padding: 0.1em 0.3em;
  border-radius: 3px 3px 3px 0; white-space: nowrap;
}
</style>
