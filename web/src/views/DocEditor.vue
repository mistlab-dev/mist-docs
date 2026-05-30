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
        <el-button size="small" text @click="loadAndShowStats">
          <el-icon><DataLine /></el-icon>
        </el-button>
        <el-tag v-if="collabUsers.length" size="small" type="success">{{ collabUsers.length + 1 }} 人在线</el-tag>
        <el-tag v-if="collabStatus === 'connected'" size="small" type="success">协同中</el-tag>
        <el-button type="primary" size="small" @click="manualSave" :loading="saving">
          <el-icon><Check /></el-icon> 保存
        </el-button>
        <span v-if="saveStatus === 'saving'" class="save-indicator saving">⏳ 保存中...</span>
        <span v-else-if="saveStatus === 'saved'" class="save-indicator saved">✅ 已保存</span>
        <span v-else-if="saveStatus === 'error'" class="save-indicator error">❌ 保存失败</span>
        <el-button v-if="doc?.locked_by && doc?.locked_by !== currentUserId" size="small" type="warning" disabled>
          🔒 已锁定
        </el-button>
        <el-button v-else-if="doc?.locked_by === currentUserId" size="small" type="warning" @click="unlockDoc">
          🔓 解锁
        </el-button>
        <el-button v-else size="small" @click="lockDoc">🔒 锁定</el-button>
        <el-dropdown @command="handleExport">
          <el-button size="small">导出 <el-icon><ArrowDown /></el-icon></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="html">HTML</el-dropdown-item>
              <el-dropdown-item command="markdown">Markdown</el-dropdown-item>
              <el-dropdown-item command="txt">纯文本</el-dropdown-item>
              <el-dropdown-item command="pdf" divided>PDF</el-dropdown-item>
              <el-dropdown-item command="docx">Word (.doc)</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button size="small" @click="showMoveDialog = true">
          <el-icon><FolderOpened /></el-icon> 移动
        </el-button>
        <el-button size="small" @click="showShareDialog = true">
          <el-icon><Share /></el-icon> 分享
        </el-button>
        <el-button size="small" @click="toggleWatermark" :type="showWatermark ? 'warning' : ''">
          <el-icon><Stamp /></el-icon> 水印
        </el-button>
        <el-badge :value="commentCount" :hidden="commentCount === 0" :max="99">
          <el-button size="small" @click="showComments = true">
            <el-icon><ChatDotRound /></el-icon> 评论
          </el-button>
        </el-badge>
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
    <div v-if="doc?.type === 'doc' && editor" class="editor-body with-outline">
      <editor-content :editor="editor as any" class="tiptap-editor" />
      <!-- 大纲导航 -->
      <div v-if="outlineItems.length" class="outline-panel">
        <div class="outline-title">大纲</div>
        <div
          v-for="(item, i) in outlineItems" :key="i"
          class="outline-item"
          :class="'outline-h' + item.level"
          @click="scrollToHeading(item.id)"
        >{{ item.text }}</div>
      </div>
    </div>

    <!-- 表格编辑器 -->
    <div v-else-if="doc?.type === 'sheet'" class="editor-body sheet-body">
      <SheetEditor ref="sheetRef" :initial-data="sheetData" @change="onSheetChange" />
    </div>

    <!-- 水印层 -->
    <div v-if="showWatermark" class="watermark-layer">
      <div v-for="i in 40" :key="i" class="watermark-text">{{ currentUser }} · {{ formatTime(new Date().toISOString()) }}</div>
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
          <el-button size="small" text type="primary" @click="openDiff(v.version)" style="margin-left:8px">对比</el-button>
        </el-timeline-item>
      </el-timeline>
      <template #footer>
        <el-button @click="versionDialog.show = false">取消</el-button>
        <el-button type="primary" @click="confirmRestore" :loading="versionDialog.loading">
          恢复到 v{{ versionDialog.version }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 统计弹窗 -->
    <el-dialog v-model="showStats" title="文档统计" width="500px">
      <div v-if="stats" class="stats-grid">
        <div class="stat-item">
          <div class="stat-value">{{ stats.word_count }}</div>
          <div class="stat-label">字数</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ stats.char_count }}</div>
          <div class="stat-label">字符数</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ stats.edit_count }}</div>
          <div class="stat-label">编辑次数</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ stats.contributors }}</div>
          <div class="stat-label">贡献者</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ stats.comment_count }}</div>
          <div class="stat-label">评论数</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ stats.edit_count + 1 }}</div>
          <div class="stat-label">版本数</div>
        </div>
      </div>
      <div v-if="stats?.daily_edits?.length" style="margin-top:16px">
        <p style="color:#999;font-size:13px;margin-bottom:8px">编辑活动（近30天）</p>
        <div class="activity-chart">
          <div v-for="(d, i) in stats.daily_edits" :key="i" class="activity-bar"
               :style="{ height: Math.min(d.count * 20, 60) + 'px' }"
               :title="d.date + ': ' + d.count + ' 次'">
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 版本对比弹窗 -->
    <el-dialog v-model="showDiff" title="版本对比" width="700px" :fullscreen="windowWidth < 768">
      <div class="diff-toolbar">
        <span>版本 </span>
        <el-select v-model="diffOld" size="small" style="width:120px">
          <el-option v-for="v in versions" :key="v.version" :label="'v' + v.version" :value="v.version" />
        </el-select>
        <span style="margin:0 8px">→</span>
        <el-select v-model="diffNew" size="small" style="width:120px">
          <el-option v-for="v in versions" :key="v.version" :label="'v' + v.version" :value="v.version" />
        </el-select>
        <el-button size="small" type="primary" @click="loadDiff" :loading="diffLoading">对比</el-button>
      </div>
      <div v-if="diffHtml" class="diff-content" v-html="diffHtml" />
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

    <!-- 标签 -->
    <div v-if="docTags.length" class="doc-tags-bar">
      <el-tag
        v-for="tag in docTags" :key="tag.id"
        :color="tag.color" effect="dark" size="small"
        closable @close="removeTag(tag.id)"
        style="margin-right:6px"
      >{{ tag.name }}</el-tag>
      <el-button size="small" text @click="showTagDialog = true">+ 标签</el-button>
    </div>

    <!-- 标签管理弹窗 -->
    <el-dialog v-model="showTagDialog" title="管理标签" width="400px">
      <div v-if="allTags.length" style="margin-bottom:12px">
        <p style="color:#999;font-size:13px;margin-bottom:8px">点击添加已有标签：</p>
        <el-tag
          v-for="tag in allTags" :key="tag.id"
          :color="tag.color" effect="dark" size="small"
          :class="{ 'tag-disabled': docTagIds.includes(tag.id) }"
          style="margin:0 6px 6px 0;cursor:pointer"
          @click="addTag(tag.id)"
        >{{ tag.name }}</el-tag>
      </div>
      <div style="display:flex;gap:8px">
        <el-input v-model="newTagName" placeholder="新标签名" size="small" style="flex:1" />
        <el-color-picker v-model="newTagColor" size="small" />
        <el-button size="small" type="primary" @click="createAndAddTag">创建</el-button>
      </div>
      <template #footer>
        <el-button @click="showTagDialog = false">完成</el-button>
      </template>
    </el-dialog>

    <!-- 移动文档弹窗 -->
    <el-dialog v-model="showMoveDialog" title="移动文档" width="400px">
      <p style="color:#999;margin-bottom:12px">选择目标文件夹：</p>
      <el-tree
        :data="folderTree"
        :props="{ label: 'name', children: 'children', value: 'id' }"
        node-key="id"
        highlight-current
        default-expand-all
        @node-click="moveTarget = $event.id"
      >
        <template #default="{ node, data }">
          <span :style="{ color: moveTarget === data.id ? '#409eff' : '' }">
            📁 {{ data.name }}
          </span>
        </template>
      </el-tree>
      <template #footer>
        <el-button @click="showMoveDialog = false">取消</el-button>
        <el-button type="primary" @click="moveDoc" :disabled="!moveTarget">移动</el-button>
      </template>
    </el-dialog>

    <!-- 分享弹窗 -->
    <el-dialog v-model="showShareDialog" title="分享文档" width="500">
      <el-form label-width="80px">
        <el-form-item label="访问密码">
          <el-input v-model="shareForm.password" placeholder="留空则无需密码" />
        </el-form-item>
        <el-form-item label="有效期">
          <el-select v-model="shareForm.expiresIn" style="width:100%">
            <el-option label="永久" :value="0" />
            <el-option label="1 小时" :value="1" />
            <el-option label="24 小时" :value="24" />
            <el-option label="7 天" :value="168" />
            <el-option label="30 天" :value="720" />
          </el-select>
        </el-form-item>
      </el-form>
      <div v-if="shareResult" style="margin-top:12px">
        <el-alert type="success" :closable="false" style="margin-bottom:12px">
          分享链接已生成
        </el-alert>
        <el-input :model-value="shareResult.share_url" readonly>
          <template #append>
            <el-button @click="copyShareUrl">复制</el-button>
          </template>
        </el-input>
      </div>
      <template #footer>
        <el-button @click="showShareDialog = false; shareResult = null">关闭</el-button>
        <el-button type="primary" @click="createShare">生成链接</el-button>
      </template>
    </el-dialog>

    <!-- 已有分享列表 -->
    <el-dialog v-model="showShareList" title="分享记录" width="500">
      <el-table :data="shares" stripe>
        <el-table-column label="链接" prop="token" width="120" />
        <el-table-column label="密码保护" width="80">
          <template #default="{ row }">{{ row.has_password ? '是' : '否' }}</template>
        </el-table-column>
        <el-table-column label="访问次数" prop="access_count" width="80" />
        <el-table-column label="过期" width="60">
          <template #default="{ row }">{{ row.expired ? '是' : '否' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="deleteShare(row.id)">取消</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 评论面板 -->
    <el-drawer v-model="showComments" title="评论" size="400px">
      <div class="comment-input">
        <div style="position:relative">
          <el-input v-model="newComment" type="textarea" :rows="3" placeholder="写评论... @提及用户" @input="onCommentInput" />
          <div v-if="mentionList.length" class="mention-dropdown">
            <div v-for="u in mentionList" :key="u.id" class="mention-item" @click="selectMention(u)">
              {{ u.name }} ({{ u.username }})
            </div>
          </div>
        </div>
        <el-button type="primary" size="small" @click="submitComment" :disabled="!newComment.trim()" style="margin-top:8px">发送</el-button>
      </div>
      <div class="comment-list">
        <div v-for="c in comments" :key="c.id" class="comment-item">
          <div class="comment-header">
            <strong>{{ c.user_name }}</strong>
            <span class="comment-time">{{ formatTime(c.created_at) }}</span>
          </div>
          <div class="comment-content">{{ c.content }}</div>
          <div class="comment-actions">
            <el-button link size="small" @click="replyTo(c)">回复</el-button>
            <el-button v-if="c.user_id === currentUserId" link type="danger" size="small" @click="deleteComment(c.id)">删除</el-button>
          </div>
          <!-- 回复 -->
          <div v-for="r in getReplies(c.id)" :key="r.id" class="comment-reply">
            <div class="comment-header">
              <strong>{{ r.user_name }}</strong>
              <span class="comment-time">{{ formatTime(r.created_at) }}</span>
            </div>
            <div class="comment-content">{{ r.content }}</div>
            <div class="comment-actions">
              <el-button v-if="r.user_id === currentUserId" link type="danger" size="small" @click="deleteComment(r.id)">删除</el-button>
            </div>
          </div>
        </div>
        <div v-if="!comments.length" class="no-data">暂无评论</div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
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
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'
import { common, createLowlight } from 'lowlight'
import * as Y from 'yjs'
import { MistWSProvider, type CollabUser } from '@/utils/collab'
import http from '@/utils/http'
import SheetEditor from '@/components/SheetEditor.vue'

const lowlight = createLowlight(common)

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const docId = route.params.id as string

const doc = ref<any>(null)
const title = ref('')
const versions = ref<any[]>([])
const editor = ref<Editor | null>(null)
const sheetData = ref('{}')
const sheetRef = ref<InstanceType<typeof SheetEditor> | null>(null)
const imageInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const saveStatus = ref('') // '', 'saving', 'saved', 'error'
let saveTimer: any = null
let autoSaveTimer: any = null

// 分享
const showShareDialog = ref(false)
const showWatermark = ref(false)

function toggleWatermark() { showWatermark.value = !showWatermark.value }
const showShareList = ref(false)
const shareForm = reactive({ password: '', expiresIn: 0 })
const shareResult = ref<any>(null)
const shares = ref<any[]>([])

// 评论
const showComments = ref(false)
const comments = ref<any[]>([])
const newComment = ref('')
const mentionList = ref<any[]>([])
const allUsers = ref<any[]>([])

// 加载用户列表用于 @提及
async function loadMentionUsers() {
  if (allUsers.value.length) return
  try {
    const { data } = await http.get('/users')
    allUsers.value = data.data || []
  } catch {}
}

function onCommentInput() {
  const text = newComment.value
  const atIdx = text.lastIndexOf('@')
  if (atIdx < 0) { mentionList.value = []; return }
  const query = text.slice(atIdx + 1).toLowerCase()
  if (query.includes(' ') || query.includes('\n')) { mentionList.value = []; return }
  mentionList.value = allUsers.value.filter((u: any) =>
    u.name?.toLowerCase().includes(query) || u.username?.toLowerCase().includes(query)
  ).slice(0, 5)
  loadMentionUsers()
}

function selectMention(u: any) {
  const text = newComment.value
  const atIdx = text.lastIndexOf('@')
  newComment.value = text.slice(0, atIdx) + '@' + u.name + ' '
  mentionList.value = []
}
const commentCount = ref(0)
const outlineItems = ref<{text: string; level: number; id: string}[]>([])

// 提取大纲
function updateOutline() {
  if (!editor.value) return
  const items: {text: string; level: number; id: string}[] = []
  const doc = editor.value.state.doc
  doc.descendants((node: any, pos: number) => {
    if (node.type.name === 'heading') {
      const id = 'heading-' + pos
      items.push({ text: node.textContent, level: node.attrs.level, id })
    }
  })
  outlineItems.value = items
}

function scrollToHeading(id: string) {
  const el = document.querySelector(`[data-heading="${id}"]`) ||
    [...document.querySelectorAll('.tiptap-editor h1, .tiptap-editor h2, .tiptap-editor h3')]
      .find(el => el.textContent === outlineItems.value.find(o => o.id === id)?.text)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
const replyParent = ref('')
const currentUserId = ref('')
const currentUser = ref('')

// 协同编辑
const collabStatus = ref<'connecting' | 'connected' | 'disconnected'>('disconnected')
const collabUsers = ref<CollabUser[]>([])
let ydoc: Y.Doc | null = null
let wsProvider: MistWSProvider | null = null

// 移动文档
const showMoveDialog = ref(false)
const moveTarget = ref('')
const folderTree = ref<any[]>([])

// 标签
const docTags = ref<any[]>([])
const allTags = ref<any[]>([])
const docTagIds = ref<string[]>([])
const showTagDialog = ref(false)
const newTagName = ref('')
const newTagColor = ref('#409eff')

async function loadDocTags() {
  try {
    const { data } = await http.get(`/docs/documents/${docId}/tags`)
    docTags.value = data || []
    docTagIds.value = docTags.value.map((t: any) => t.id)
  } catch {}
}

async function loadAllTags() {
  try {
    const { data } = await http.get('/docs/tags')
    allTags.value = data || []
  } catch {}
}

async function addTag(tagId: string) {
  if (docTagIds.value.includes(tagId)) return
  const newIds = [...docTagIds.value, tagId]
  await http.put(`/docs/documents/${docId}/tags`, { tag_ids: newIds })
  await loadDocTags()
}

async function removeTag(tagId: string) {
  const newIds = docTagIds.value.filter(id => id !== tagId)
  await http.put(`/docs/documents/${docId}/tags`, { tag_ids: newIds })
  await loadDocTags()
}

async function createAndAddTag() {
  if (!newTagName.value.trim()) return
  try {
    await http.post('/docs/tags', { name: newTagName.value, color: newTagColor.value })
    await loadAllTags()
    // Find the new tag and add it
    const created = allTags.value.find((t: any) => t.name === newTagName.value.trim())
    if (created) await addTag(created.id)
    newTagName.value = ''
  } catch {}
}

watch(showTagDialog, (v) => { if (v) loadAllTags() })

// 文档统计
const showStats = ref(false)
const stats = ref<any>(null)

async function loadAndShowStats() {
  try {
    const { data } = await http.get(`/docs/documents/${docId}/stats`)
    stats.value = data
    showStats.value = true
  } catch {}
}

// 版本对比
const showDiff = ref(false)
const diffOld = ref(0)
const diffNew = ref(0)
const diffHtml = ref('')
const diffLoading = ref(false)
const windowWidth = ref(window.innerWidth)

// 锁定
async function lockDoc() {
  try {
    await http.post(`/docs/documents/${docId}/lock`)
    if (doc.value) doc.value.locked_by = currentUserId.value
    ElMessage.success('已锁定')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || '锁定失败')
  }
}

async function unlockDoc() {
  try {
    await http.post(`/docs/documents/${docId}/unlock`)
    if (doc.value) doc.value.locked_by = ''
    ElMessage.success('已解锁')
  } catch { ElMessage.error('解锁失败') }
}

function openDiff(ver: number) {
  diffOld.value = Math.max(1, ver - 1)
  diffNew.value = ver
  showDiff.value = true
  loadDiff()
}

async function loadDiff() {
  if (!diffOld.value || !diffNew.value) return
  diffLoading.value = true
  try {
    const [oldResp, newResp] = await Promise.all([
      fetch(`/api/docs/documents/${docId}/versions/${diffOld.value}/content`, { headers: authHeader() }),
      fetch(`/api/docs/documents/${docId}/versions/${diffNew.value}/content`, { headers: authHeader() }),
    ])
    const oldText = await oldResp.text()
    const newText = await newResp.text()
    diffHtml.value = simpleDiff(oldText, newText)
  } catch { diffHtml.value = '<p style="color:#f56c6c">加载失败</p>' }
  diffLoading.value = false
}

function authHeader(): Record<string, string> {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

function simpleDiff(oldHtml: string, newHtml: string): string {
  const stripTags = (s: string) => s.replace(/<[^>]+>/g, '').split(/\s+/).filter(Boolean)
  const oldWords = stripTags(oldHtml)
  const newWords = stripTags(newHtml)
  const oldSet = new Set(oldWords)
  const newSet = new Set(newWords)
  let html = '<div style="font-size:14px;line-height:1.8">'
  // Removed (in old but not in new)
  const removed = oldWords.filter(w => !newSet.has(w))
  const added = newWords.filter(w => !oldSet.has(w))
  if (removed.length === 0 && added.length === 0) {
    html += '<p style="color:#67c23a">✅ 两个版本内容相同</p>'
  } else {
    if (removed.length) html += '<p><strong style="color:#f56c6c">删除（' + removed.length + ' 词）：</strong></p><p>' + removed.slice(0, 50).map(w => `<span style="background:#fde2e2;color:#f56c6c;padding:1px 3px;border-radius:3px">${w}</span>`).join(' ') + (removed.length > 50 ? ' ...' : '') + '</p>'
    if (added.length) html += '<p><strong style="color:#67c23a">新增（' + added.length + ' 词）：</strong></p><p>' + added.slice(0, 50).map(w => `<span style="background:#e1f3d8;color:#67c23a;padding:1px 3px;border-radius:3px">${w}</span>`).join(' ') + (added.length > 50 ? ' ...' : '') + '</p>'
  }
  html += '</div>'
  return html
}

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

// === 文档移动 ===
async function loadFolderTree() {
  try {
    const { data } = await http.get('/docs/tree')
    folderTree.value = data || []
  } catch {}
}

async function moveDoc() {
  if (!moveTarget.value) return
  try {
    await http.put(`/docs/documents/${docId}`, { title: title.value, folder_id: moveTarget.value })
    ElMessage.success('文档已移动')
    showMoveDialog.value = false
    if (doc.value) doc.value.folder_id = moveTarget.value
  } catch { ElMessage.error('移动失败') }
}

watch(showMoveDialog, (v) => { if (v) loadFolderTree() })

async function loadVersions() {
  const { data } = await http.get(`/docs/documents/${docId}/versions`)
  versions.value = (data.data || []).reverse()
}

function initEditor(initialContent: string) {
  // Try to use Yjs collaboration
  const token = localStorage.getItem('token')
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${wsProtocol}//${window.location.host}/ws/docs/${docId}?token=${token}`

  // Check if we already have Yjs state on the server
  // If yes, use collaboration mode. If no, use local mode.
  const useCollab = !!token && doc.value?.type === 'doc'

  if (useCollab) {
    try {
      ydoc = new Y.Doc()

      // Setup WS provider
      wsProvider = new MistWSProvider(wsUrl, ydoc)
      wsProvider.onStatus = (status) => { collabStatus.value = status }
      wsProvider.onUserJoin = (user) => { collabUsers.value = [...collabUsers.value, user] }
      wsProvider.onUserLeave = (userId) => { collabUsers.value = collabUsers.value.filter(u => u.id !== userId) }
      wsProvider.onClients = (users) => { collabUsers.value = users.filter((u: CollabUser) => u.id !== currentUserId.value) }
      wsProvider.bind()

      // Load existing HTML into Y.Doc if it's empty
      const yXmlFragment = ydoc.getXmlFragment('default')
      if (yXmlFragment.length === 0 && initialContent) {
        // Use a temp editor to convert HTML to Y.Doc format
        const tempDiv = document.createElement('div')
        tempDiv.innerHTML = initialContent
        // Let Collaboration extension handle initial content
      }

      const userColors = ['#e06c75', '#e5c07b', '#98c379', '#56b6c2', '#61afef', '#c678dd', '#d19a66']
      const userColorIdx = currentUserId.value.split('').reduce((a, c) => a + c.charCodeAt(0), 0) % userColors.length

      editor.value = new Editor({
        extensions: [
          StarterKit.configure({
            codeBlock: false,
            // history: false, // Yjs handles history — removed: not in StarterKitOptions
          }),
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
          Collaboration.configure({
            document: ydoc,
          }),
          CollaborationCursor.configure({
            provider: { awareness: null } as any,
            user: {
              name: auth.user?.name || auth.user?.username || '匿名',
              color: userColors[userColorIdx],
              fallbackColor: userColors[userColorIdx],
            },
          }),
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
          // In collab mode, Yjs handles sync via WS
          // Still auto-save HTML snapshot periodically
          scheduleAutoSave()
          updateOutline()
        },
      })
      return
    } catch (e) {
      console.warn('Collab mode failed, falling back to local:', e)
      ydoc?.destroy()
      ydoc = null
      wsProvider?.destroy()
      wsProvider = null
    }
  }

  // Fallback: local mode (no collab)
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
      updateOutline()
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
  saveStatus.value = 'saving'
  try {
    await http.put(`/docs/documents/${docId}/content`, { content })
    await loadDoc()
    await loadVersions()
    saveStatus.value = 'saved'
    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => { saveStatus.value = '' }, 3000)
  } catch (e) {
    console.error('保存失败', e)
    saveStatus.value = 'error'
  }
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

// === 分享 ===
async function createShare() {
  const { data } = await http.post(`/docs/documents/${docId}/share`, shareForm)
  shareResult.value = data
  ElMessage.success('分享链接已生成')
}

function copyShareUrl() {
  if (!shareResult.value) return
  const url = `${window.location.origin}${shareResult.value.share_url}`
  navigator.clipboard.writeText(url)
  ElMessage.success('已复制到剪贴板')
}

async function loadShares() {
  const { data } = await http.get(`/docs/documents/${docId}/shares`)
  shares.value = data.data || []
  showShareList.value = true
}

async function deleteShare(id: string) {
  await http.delete(`/docs/shares/${id}`)
  ElMessage.success('已取消分享')
  loadShares()
}

// === 导出 ===
async function handleExport(format: string) {
  try {
    if (format === 'pdf') {
      // PDF: 前端生成，支持中文
      const html2pdf = (await import('html2pdf.js')).default
      const editorEl = document.querySelector('.ProseMirror') as HTMLElement
      if (!editorEl) { ElMessage.error('导出失败'); return }
      // Clone and wrap for PDF
      const wrapper = document.createElement('div')
      wrapper.style.cssText = 'padding:20px;font-family:"Noto Sans SC",sans-serif;font-size:14px;line-height:1.8;color:#333'
      wrapper.innerHTML = `<h1 style="text-align:center;font-size:22px;margin-bottom:16px">${doc.value?.title || ''}</h1>` + editorEl.innerHTML
      const opt = {
        margin: [10, 10, 10, 10] as [number, number, number, number],
        filename: `${doc.value?.title || 'export'}.pdf`,
        image: { type: 'jpeg' as const, quality: 0.95 },
        html2canvas: { scale: 2, useCORS: true },
        jsPDF: { unit: 'mm', format: 'a4', orientation: 'portrait' as const },
      }
      await html2pdf().set(opt).from(wrapper).save()
      ElMessage.success('PDF 导出成功')
      return
    }
    const token = localStorage.getItem('token')
    const resp = await fetch(`/api/docs/documents/${docId}/export?format=${format}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (!resp.ok) { ElMessage.error('导出失败'); return }
    const blob = await resp.blob()
    const cd = resp.headers.get('Content-Disposition') || ''
    const match = cd.match(/filename="?([^"]+)"?/)
    const filename = match ? match[1] : `${doc.value?.title || 'export'}.${format === 'markdown' ? 'md' : format}`
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = filename; a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (e) { console.error(e); ElMessage.error('导出失败') }
}

// === 评论 ===
async function loadComments() {
  const { data } = await http.get(`/docs/documents/${docId}/comments`)
  comments.value = data.data || []
  commentCount.value = comments.value.length
}

async function submitComment() {
  if (!newComment.value.trim()) return
  await http.post(`/docs/documents/${docId}/comments`, {
    content: newComment.value,
    parent_id: replyParent.value,
  })
  newComment.value = ''
  replyParent.value = ''
  loadComments()
}

function replyTo(c: any) {
  replyParent.value = c.id
  newComment.value = `@${c.user_name} `
}

async function deleteComment(id: string) {
  await http.delete(`/docs/comments/${id}`)
  loadComments()
}

function getReplies(parentId: string) {
  return comments.value.filter(c => c.parent_id === parentId)
}

onMounted(async () => {
  await loadDoc()
  await loadVersions()
  await loadDocTags()
  // Get current user ID
  try {
    const { data: me } = await http.get('/auth/me')
    currentUserId.value = me.data?.id || ''
    currentUser.value = me.data?.name || ''
  } catch {}
  // Load comment count
  loadComments()
  if (doc.value?.type === 'doc') {
    const content = doc.value?.content || ''
    initEditor(content === '{}' ? '' : content)
  }
})

onUnmounted(() => {
  doSave().catch(() => {})
  clearTimeout(autoSaveTimer)
  wsProvider?.destroy()
  ydoc?.destroy()
  editor.value?.destroy()
  document.removeEventListener('keydown', handleGlobalKeydown)
})

function handleGlobalKeydown(e: KeyboardEvent) {
  const mod = e.ctrlKey || e.metaKey
  // Ctrl+S 保存
  if (mod && e.key === 's') {
    e.preventDefault()
    manualSave()
  }
  // Ctrl+P 导出 PDF
  if (mod && e.key === 'p') {
    e.preventDefault()
    handleExport('pdf')
  }
  // Ctrl+Shift+E 导出 HTML
  if (mod && e.shiftKey && e.key === 'E') {
    e.preventDefault()
    handleExport('html')
  }
  // Ctrl+Shift+S 分享
  if (mod && e.shiftKey && e.key === 'S') {
    e.preventDefault()
    showShareDialog.value = true
  }
  // Ctrl+K 插入链接（TipTap 已内置，这里处理无选中文本的情况）
  // Ctrl+/ 显示快捷键帮助
  if (mod && e.key === '/') {
    e.preventDefault()
    ElMessage({
      message: 'Ctrl+S 保存 · Ctrl+P PDF · Ctrl+Shift+E HTML · Ctrl+Shift+S 分享 · Ctrl+B 粗体 · Ctrl+I 斜体 · Ctrl+U 下划线',
      duration: 4000,
    })
  }
}

// Register global shortcut
document.addEventListener('keydown', handleGlobalKeydown)
</script>

<style scoped>
.editor-page { height: 100%; display: flex; flex-direction: column; }
.doc-tags-bar { padding: 4px 16px; background: #fafafa; border-bottom: 1px solid #eee; }
.tag-disabled { opacity: 0.4; cursor: default !important; }
.stats-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; }
.stat-item { text-align: center; padding: 12px; background: #f5f7fa; border-radius: 8px; }
.stat-value { font-size: 24px; font-weight: bold; color: #409eff; }
.stat-label { font-size: 12px; color: #999; margin-top: 4px; }
.activity-chart { display: flex; align-items: flex-end; gap: 2px; height: 60px; }
.activity-bar { flex: 1; background: #409eff; border-radius: 2px 2px 0 0; min-height: 2px; opacity: 0.7; }
.diff-toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
.diff-content { max-height: 400px; overflow-y: auto; padding: 12px; background: #fafafa; border-radius: 8px; }
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
.editor-body.with-outline { display: flex; }
.editor-body.with-outline .tiptap-editor { flex: 1; }
.outline-panel {
  width: 200px; min-width: 200px; border-left: 1px solid #e8e8e8;
  background: #fafafa; padding: 12px 0; overflow-y: auto; font-size: 13px;
}
.outline-title { font-weight: bold; padding: 0 12px 8px; color: #333; border-bottom: 1px solid #eee; margin-bottom: 4px; }
.outline-item {
  padding: 4px 12px; cursor: pointer; color: #666; white-space: nowrap;
  overflow: hidden; text-overflow: ellipsis; transition: all .15s;
}
.outline-item:hover { color: #1a73e8; background: #e8f0fe; }
.outline-h1 { font-weight: 600; }
.outline-h2 { padding-left: 20px; }
.outline-h3 { padding-left: 28px; font-size: 12px; }
.save-indicator { font-size: 12px; margin-left: 8px; }
.save-indicator.saving { color: #E6A23C; }
.save-indicator.saved { color: #67C23A; }
.save-indicator.error { color: #F56C6C; }
.watermark-layer {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  pointer-events: none; z-index: 9999; overflow: hidden;
}
.watermark-text {
  display: inline-block; width: 280px; text-align: center;
  transform: rotate(-30deg); font-size: 14px; color: rgba(0,0,0,.06);
  padding: 40px 20px; user-select: none; white-space: nowrap;
}
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

/* Collaboration cursors */
.tiptap-editor :deep(.collaboration-cursor__caret) {
  border-left: 1px solid #0d0d0d;
  border-right: 1px solid #0d0d0d;
  margin-left: -1px;
  margin-right: -1px;
  pointer-events: none;
  position: relative;
  word-break: normal;
}
.tiptap-editor :deep(.collaboration-cursor__label) {
  border-radius: 3px 3px 3px 0;
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  left: -1px;
  line-height: normal;
  padding: 2px 6px;
  position: absolute;
  top: -1.4em;
  user-select: none;
  white-space: nowrap;
}

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

/* 评论 */
.comment-input { margin-bottom: 16px; }
.mention-dropdown {
  position: absolute; top: 100%; left: 0; z-index: 10;
  background: #fff; border: 1px solid #ddd; border-radius: 4px;
  max-height: 200px; overflow-y: auto; box-shadow: 0 4px 12px rgba(0,0,0,.1);
}
.mention-item { padding: 6px 12px; cursor: pointer; font-size: 13px; }
.mention-item:hover { background: #e8f0fe; }
.comment-item { padding: 12px 0; border-bottom: 1px solid #f0f0f0; }
.comment-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.comment-time { font-size: 12px; color: #c0c4cc; }
.comment-content { font-size: 14px; line-height: 1.6; }
.comment-actions { margin-top: 4px; display: flex; gap: 8px; }
.comment-reply { margin-left: 24px; padding: 8px 0; border-top: 1px dashed #eee; }
.no-data { text-align: center; padding: 40px; color: #c0c4cc; }
</style>
