<template>
  <div class="editor-page">
    <div class="editor-header">
      <el-button @click="router.push('/docs')" text>
        <el-icon><ArrowLeft /></el-icon> 返回
      </el-button>
      <el-input v-model="title" class="title-input" @blur="saveTitle" />
      <div class="header-actions">
        <el-tag size="small">{{ doc?.type === 'sheet' ? '表格' : '文档' }}</el-tag>
        <el-tag size="small">v{{ doc?.version || 1 }}</el-tag>
        <el-button type="primary" size="small" @click="manualSave" :loading="saving">
          <el-icon><Check /></el-icon> 保存
        </el-button>
        <el-dropdown @command="handleVersion">
          <el-button size="small">
            版本 <el-icon><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="v in versions" :key="v.version" :command="v.version">
                v{{ v.version }} - {{ formatTime(v.created_at) }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <!-- TipTap 工具栏 -->
    <div v-if="editor" class="toolbar">
      <el-button-group>
        <el-button size="small" @click="editor.chain().focus().toggleBold().run()" :type="editor.isActive('bold') ? 'primary' : ''"><strong>B</strong></el-button>
        <el-button size="small" @click="editor.chain().focus().toggleItalic().run()" :type="editor.isActive('italic') ? 'primary' : ''"><em>I</em></el-button>
        <el-button size="small" @click="editor.chain().focus().toggleStrike().run()" :type="editor.isActive('strike') ? 'primary' : ''"><s>S</s></el-button>
        <el-button size="small" @click="editor.chain().focus().toggleUnderline().run()" :type="editor.isActive('underline') ? 'primary' : ''"><u>U</u></el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 1 }).run()" :type="editor.isActive('heading', { level: 1 }) ? 'primary' : ''">H1</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 2 }).run()" :type="editor.isActive('heading', { level: 2 }) ? 'primary' : ''">H2</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleHeading({ level: 3 }).run()" :type="editor.isActive('heading', { level: 3 }) ? 'primary' : ''">H3</el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().toggleBulletList().run()" :type="editor.isActive('bulletList') ? 'primary' : ''">• 列表</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleOrderedList().run()" :type="editor.isActive('orderedList') ? 'primary' : ''">1. 有序</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleTaskList().run()" :type="editor.isActive('taskList') ? 'primary' : ''">☑ 任务</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleBlockquote().run()" :type="editor.isActive('blockquote') ? 'primary' : ''">引用</el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="toggleCodeBlock" :type="editor.isActive('codeBlock') ? 'primary' : ''">代码块</el-button>
        <el-button size="small" @click="editor.chain().focus().toggleCode().run()" :type="editor.isActive('code') ? 'primary' : ''">行内代码</el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="insertLink" :type="editor.isActive('link') ? 'primary' : ''">🔗 链接</el-button>
        <el-button size="small" @click="triggerImageUpload">🖼 图片</el-button>
        <el-button size="small" @click="insertTable">📋 表格</el-button>
        <el-button size="small" @click="editor.chain().focus().setHorizontalRule().run()">— 线</el-button>
      </el-button-group>
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
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="editor.chain().focus().undo().run()">↩ 撤销</el-button>
        <el-button size="small" @click="editor.chain().focus().redo().run()">↪ 重做</el-button>
      </el-button-group>
    </div>

    <input type="file" ref="imageInput" style="display:none" accept="image/*" @change="handleImageUpload" />

    <!-- 文档编辑器 -->
    <div v-if="doc?.type === 'doc' && editor" class="editor-body">
      <editor-content :editor="editor" class="tiptap-editor" />
    </div>

    <!-- 表格编辑器 -->
    <div v-else-if="doc?.type === 'sheet'" class="editor-body sheet-body">
      <SheetEditor ref="sheetRef" :initial-data="sheetData" @change="onSheetChange" />
    </div>

    <!-- 版本回退确认 -->
    <el-dialog v-model="versionDialog.show" title="恢复版本" width="480px">
      <p style="margin-bottom:12px;color:#666">
        将恢复到 <strong>v{{ versionDialog.version }}</strong>，当前内容将被保存为新版本。
      </p>
      <el-timeline>
        <el-timeline-item
          v-for="v in versions.slice(0, 10)"
          :key="v.version"
          :timestamp="formatTime(v.created_at)"
          :type="v.version === versionDialog.version ? 'primary' : ''"
          placement="top"
        >
          版本 v{{ v.version }}
          <span v-if="v.version === doc?.version" style="color:#409eff;font-size:12px">（当前）</span>
        </el-timeline-item>
      </el-timeline>
      <template #footer>
        <el-button @click="versionDialog.show = false">取消</el-button>
        <el-button type="primary" @click="confirmRestore" :loading="versionDialog.loading">
          恢复到 v{{ versionDialog.version }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 链接弹窗 -->
    <el-dialog v-model="linkDialog.show" title="插入链接" width="420px">
      <el-form label-width="60px">
        <el-form-item label="文本"><el-input v-model="linkDialog.text" placeholder="显示文字" /></el-form-item>
        <el-form-item label="链接"><el-input v-model="linkDialog.url" placeholder="https://" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="linkDialog.show = false">取消</el-button>
        <el-button v-if="editor?.isActive('link')" type="danger" @click="removeLink">移除链接</el-button>
        <el-button type="primary" @click="confirmLink">确定</el-button>
      </template>
    </el-dialog>

    <!-- 代码语言弹窗 -->
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
import http from '@/utils/http'
import SheetEditor from '@/components/SheetEditor.vue'

const lowlight = createLowlight(common)

const route = useRoute()
const router = useRouter()
const docId = route.params.id as string

const doc = ref<any>(null)
const title = ref('')
const versions = ref<any[]>([])
const editor = ref<Editor | null>(null)
const sheetData = ref('{}')
const sheetRef = ref<InstanceType<typeof SheetEditor> | null>(null)
const imageInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
let autoSaveTimer: any = null

// 链接弹窗
const linkDialog = reactive({ show: false, text: '', url: '' })
// 代码语言弹窗
const codeLangDialog = reactive({ show: false, lang: 'plaintext' })
// 版本回退弹窗
const versionDialog = reactive({ show: false, version: 0, loading: false })
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

function initEditor(initialContent: string) {
  editor.value = new Editor({
    content: initialContent || '',
    extensions: [
      StarterKit.configure({ codeBlock: false }),
      Underline,
      TaskList,
      TaskItem.configure({ nested: true }),
      Placeholder.configure({ placeholder: '开始输入内容...' }),
      Image.configure({ inline: false, allowBase64: true, HTMLAttributes: { class: 'editor-image' } }),
      Link.configure({ openOnClick: false, HTMLAttributes: { class: 'editor-link', target: '_blank', rel: 'noopener' } }),
      Table.configure({ resizable: true }),
      TableRow,
      TableCell,
      TableHeader,
      CodeBlockLowlight.configure({ lowlight }),
    ],
    editorProps: {
      attributes: { class: 'prose prose-lg focus:outline-none max-w-none' },
      handlePaste: (_view: any, event: ClipboardEvent) => {
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
      handleDrop: (_view: any, event: DragEvent) => {
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
    onUpdate: () => {
      scheduleAutoSave()
    },
  })
}

// 图片上传
function triggerImageUpload() { imageInput.value?.click() }

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
    const { data } = await http.post('/docs/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    const url = data.data?.url || data.data?.path || data.data
    editor.value?.chain().focus().setImage({ src: url }).run()
    ElMessage.success('图片已上传')
  } catch {
    const reader = new FileReader()
    reader.onload = () => { editor.value?.chain().focus().setImage({ src: reader.result as string }).run() }
    reader.readAsDataURL(file)
  }
}

// 链接
function insertLink() {
  linkDialog.text = editor.value?.state.selection.content()?.content.firstChild?.text || ''
  linkDialog.url = editor.value?.getAttributes('link').href || ''
  linkDialog.show = true
}
function confirmLink() {
  if (!linkDialog.url) { linkDialog.show = false; return }
  editor.value?.chain().focus().extendMarkRange('link').setLink({ href: linkDialog.url }).run()
  linkDialog.show = false
}
function removeLink() {
  editor.value?.chain().focus().unsetLink().run()
  linkDialog.show = false
}

// 表格
function insertTable() { editor.value?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run() }

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
  if (!content) return
  saving.value = true
  try {
    await http.put(`/docs/documents/${docId}/content`, { content })
    await loadDoc()
    await loadVersions()
  } catch (e) { console.error('保存失败', e) }
  saving.value = false
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

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function handleVersion(ver: number) {
  versionDialog.version = ver
  versionDialog.show = true
}

async function confirmRestore() {
  versionDialog.loading = true
  try {
    await http.post(`/docs/documents/${docId}/restore`, { version: versionDialog.version })
    ElMessage.success(`已恢复到 v${versionDialog.version}`)
    versionDialog.show = false
    await loadDoc()
    await loadVersions()
    if (doc.value?.type === 'doc' && editor.value) {
      const content = doc.value?.content || ''
      editor.value.commands.setContent(content === '{}' ? '' : content)
    } else if (doc.value?.type === 'sheet') {
      sheetData.value = doc.value?.content || '{}'
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || '恢复失败')
  }
  versionDialog.loading = false
}

function onSheetChange() { scheduleAutoSave() }

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
})
</script>

<style scoped>
.editor-page { height: 100%; display: flex; flex-direction: column; }
.editor-header {
  display: flex; align-items: center; gap: 12px;
  padding: 8px 16px; border-bottom: 1px solid #e8e8e8; background: #fff;
}
.title-input { flex: 1; font-size: 18px; font-weight: bold; }
.title-input :deep(.el-input__wrapper) { box-shadow: none !important; background: transparent; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.toolbar {
  display: flex; align-items: center; padding: 6px 16px;
  border-bottom: 1px solid #e8e8e8; background: #fafafa;
  flex-wrap: wrap; gap: 4px;
}
.editor-body { flex: 1; overflow-y: auto; background: #fff; }
.sheet-body { overflow: hidden; }
.tiptap-editor { padding: 24px 48px; min-height: 100%; }
.tiptap-editor :deep(.ProseMirror) { outline: none; min-height: 60vh; }
.tiptap-editor :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  color: #adb5bd; content: attr(data-placeholder);
  float: left; height: 0; pointer-events: none;
}
.tiptap-editor :deep(.ProseMirror h1) { font-size: 2em; margin: 1em 0 0.5em; border-bottom: 1px solid #eee; padding-bottom: 0.3em; }
.tiptap-editor :deep(.ProseMirror h2) { font-size: 1.5em; margin: 1em 0 0.5em; border-bottom: 1px solid #eee; padding-bottom: 0.3em; }
.tiptap-editor :deep(.ProseMirror h3) { font-size: 1.25em; margin: 1em 0 0.5em; }
.tiptap-editor :deep(.ProseMirror p) { margin: 0.5em 0; line-height: 1.7; }
.tiptap-editor :deep(.ProseMirror ul), .tiptap-editor :deep(.ProseMirror ol) { padding-left: 1.5em; margin: 0.5em 0; }
.tiptap-editor :deep(.ProseMirror ul[data-type="taskList"]) { list-style: none; padding-left: 0; }
.tiptap-editor :deep(.ProseMirror ul[data-type="taskList"] li) { display: flex; align-items: flex-start; gap: 6px; margin: 4px 0; }
.tiptap-editor :deep(.ProseMirror ul[data-type="taskList"] li label) { margin-top: 4px; }
.tiptap-editor :deep(.ProseMirror blockquote) { border-left: 4px solid #409eff; padding: 8px 16px; margin: 0.5em 0; background: #f0f7ff; border-radius: 0 4px 4px 0; color: #555; }
.tiptap-editor :deep(.ProseMirror code) { background: #f0f0f0; padding: 2px 6px; border-radius: 3px; font-family: 'SF Mono', Monaco, monospace; font-size: 0.9em; color: #c7254e; }
.tiptap-editor :deep(.ProseMirror pre) { background: #1e1e2e; color: #cdd6f4; padding: 16px 20px; border-radius: 8px; overflow-x: auto; margin: 1em 0; font-size: 14px; line-height: 1.6; font-family: 'SF Mono', Monaco, monospace; }
.tiptap-editor :deep(.ProseMirror pre code) { background: none; color: inherit; padding: 0; font-size: inherit; }
.tiptap-editor :deep(.editor-link) { color: #409eff; text-decoration: underline; cursor: pointer; }
.tiptap-editor :deep(.editor-image) { max-width: 100%; height: auto; border-radius: 6px; margin: 1em 0; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
.tiptap-editor :deep(.ProseMirror img) { max-width: 100%; height: auto; border-radius: 6px; margin: 1em 0; }
.tiptap-editor :deep(.ProseMirror table) { border-collapse: collapse; width: 100%; margin: 1em 0; }
.tiptap-editor :deep(.ProseMirror table td), .tiptap-editor :deep(.ProseMirror table th) { border: 1px solid #d0d3d8; padding: 8px 12px; min-width: 80px; vertical-align: top; }
.tiptap-editor :deep(.ProseMirror table th) { background: #f5f7fa; font-weight: 600; text-align: left; }
.tiptap-editor :deep(.ProseMirror table .selectedCell) { background: #e8f0fe; }
.tiptap-editor :deep(.ProseMirror hr) { border: none; border-top: 2px solid #e8e8e8; margin: 1.5em 0; }

/* 移动端适配 */
@media (max-width: 768px) {
  .editor-header { gap: 6px; padding: 6px 10px; }
  .title-input { font-size: 15px; }
  .header-actions .el-tag { display: none; }
  .header-actions .el-button span { display: none; }
  .toolbar {
    padding: 4px 8px;
    gap: 2px;
    overflow-x: auto;
    flex-wrap: nowrap;
  }
  .toolbar .el-button-group { flex-shrink: 0; }
  .toolbar .el-button { padding: 4px 8px; font-size: 12px; }
  .tiptap-editor { padding: 12px 16px; }
  .editor-body { -webkit-overflow-scrolling: touch; }
}
</style>
