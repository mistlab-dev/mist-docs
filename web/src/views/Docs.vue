<template>
  <div class="docs-page">
    <div class="toolbar">
      <el-button @click="sidebarOpen = !sidebarOpen" class="menu-btn">
        <el-icon><Operation /></el-icon>
      </el-button>
      <el-button type="primary" size="small" @click="showNewDoc = true">
        <el-icon><Document /></el-icon><span class="btn-text"> 文档</span>
      </el-button>
      <el-button type="success" size="small" @click="showNewSheet = true">
        <el-icon><Grid /></el-icon><span class="btn-text"> 表格</span>
      </el-button>
      <div style="flex:1" />
      <el-input v-model="search" placeholder="搜索..." style="width:200px" clearable @keyup.enter="doSearch" @clear="clearSearch" size="small">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>

    <!-- 批量操作栏 -->
    <div v-if="selectedDocs.length" class="batch-bar">
      <span>已选 {{ selectedDocs.length }} 项</span>
      <el-button size="small" @click="showBatchMove = true">批量移动</el-button>
      <el-button size="small" type="danger" @click="batchDelete">批量删除</el-button>
      <el-button size="small" text @click="selectedDocs = []">取消选择</el-button>
    </div>

    <div class="content">
      <!-- 遮罩层（移动端） -->
      <div class="sidebar-overlay" :class="{ open: sidebarOpen }" @click="sidebarOpen = false"></div>
      <!-- 左侧：文件夹 + 快捷入口 -->
      <div class="sidebar" :class="{ open: sidebarOpen }" @click.self="sidebarOpen = false">
        <div class="sidebar-inner">
        <!-- 快捷入口 -->
        <div class="sidebar-section">
          <div class="section-item" :class="{ active: viewMode === 'all' }" @click="switchView('all')">
            <el-icon><Files /></el-icon> 全部文档
          </div>
          <div class="section-item" :class="{ active: viewMode === 'recent' }" @click="switchView('recent')">
            <el-icon><Clock /></el-icon> 最近打开
          </div>
          <div class="section-item" :class="{ active: viewMode === 'favorites' }" @click="switchView('favorites')">
            <el-icon><Star /></el-icon> 我的收藏
          </div>
        </div>

        <!-- 标签 -->
        <div v-if="sidebarTags.length" class="sidebar-section">
          <div class="tree-header">标签</div>
          <div v-for="tag in sidebarTags" :key="tag.id" class="section-item" @click="filterByTag(tag.id)">
            <span class="tag-dot" :style="{ background: tag.color }"></span>
            {{ tag.name }} <span style="color:#999">({{ tag.doc_count }})</span>
          </div>
        </div>

        <!-- 文件夹树 -->
        <div class="tree-header">文件夹</div>
        <el-tree
          :data="treeData"
          :props="{ label: 'name', children: 'children' }"
          node-key="id"
          highlight-current
          default-expand-all
          @node-click="onFolderClick"
        >
          <template #default="{ data }">
            <span class="tree-node">
              <el-icon><Folder /></el-icon>
              <span>{{ data.name }}</span>
            </span>
          </template>
        </el-tree>
        </div>
      </div>

      <!-- 右侧：文档列表 -->
      <div class="doc-list">
        <!-- 搜索结果 -->
        <div v-if="searchMode" class="list-header">
          搜索「{{ search }}」的结果（{{ docs.length }} 条）
        </div>

        <!-- 空状态 -->
        <div v-if="!docs.length" class="empty-state">
          <el-icon :size="48" color="#ccc"><Document /></el-icon>
          <p v-if="viewMode === 'recent'">还没有打开过文档</p>
          <p v-else-if="viewMode === 'favorites'">还没有收藏文档</p>
          <p v-else>暂无文档，点击上方按钮创建</p>
        </div>

        <!-- 文档卡片网格 -->
        <div v-else class="doc-grid">
          <div
            v-for="doc in docs"
            :key="doc.id"
            class="doc-card"
            :class="{ selected: selectedDocs.includes(doc.id) }"
            @click="openDoc(doc)"
          >
            <div class="card-checkbox" @click.stop="toggleSelect(doc.id)">
              <el-checkbox :model-value="selectedDocs.includes(doc.id)" />
            </div>
            <div class="card-icon">
              <el-icon :size="32">
                <Document v-if="doc.type === 'doc'" />
                <Grid v-else />
              </el-icon>
            </div>
            <div class="card-body">
              <div class="card-title">{{ doc.title }}</div>
              <div class="card-meta">
                <el-tag :type="doc.type === 'doc' ? '' : 'success'" size="small">
                  {{ doc.type === 'doc' ? '文档' : '表格' }}
                </el-tag>
                <span class="card-version">v{{ doc.version }}</span>
              </div>
              <div class="card-time">{{ formatTime(doc.updated_at) }}</div>
            </div>
            <div class="card-actions" @click.stop>
              <el-icon
                :size="18"
                :class="{ 'fav-active': doc.is_favorite }"
                @click="toggleFavorite(doc)"
              >
                <StarFilled v-if="doc.is_favorite" />
                <Star v-else />
              </el-icon>
              <el-dropdown trigger="click">
                <el-icon :size="18" class="more-btn"><MoreFilled /></el-icon>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item @click="openDoc(doc)">打开</el-dropdown-item>
                    <el-dropdown-item @click="showRename(doc)">重命名</el-dropdown-item>
                    <el-dropdown-item @click="showMove(doc)">移动到...</el-dropdown-item>
                    <el-dropdown-item @click="deleteDoc(doc)" divided>
                      <span style="color:#f56c6c">删除</span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 新建文件夹 -->
    <el-dialog v-model="showNewFolder" title="新建文件夹" width="400">
      <el-input v-model="newFolderName" placeholder="文件夹名称" />
      <template #footer>
        <el-button @click="showNewFolder = false">取消</el-button>
        <el-button type="primary" @click="createFolder">创建</el-button>
      </template>
    </el-dialog>

    <!-- 新建文档 -->
    <el-dialog v-model="showNewDoc" title="新建文档" width="460">
      <el-input v-model="newDocTitle" placeholder="文档标题" />
      <div style="margin-top:12px">
        <p style="color:#999;font-size:13px;margin-bottom:8px">选择模板：</p>
        <div class="template-grid">
          <div class="tpl-card" :class="{ active: newDocTemplate === '' }" @click="newDocTemplate = ''">
            <div class="tpl-icon">📝</div>
            <div class="tpl-label">空白文档</div>
          </div>
          <div class="tpl-card" :class="{ active: newDocTemplate === 'meeting' }" @click="newDocTemplate = 'meeting'">
            <div class="tpl-icon">📋</div>
            <div class="tpl-label">会议纪要</div>
          </div>
          <div class="tpl-card" :class="{ active: newDocTemplate === 'weekly' }" @click="newDocTemplate = 'weekly'">
            <div class="tpl-icon">📊</div>
            <div class="tpl-label">周报</div>
          </div>
          <div class="tpl-card" :class="{ active: newDocTemplate === 'requirement' }" @click="newDocTemplate = 'requirement'">
            <div class="tpl-icon">📐</div>
            <div class="tpl-label">需求文档</div>
          </div>
          <div class="tpl-card" :class="{ active: newDocTemplate === 'api' }" @click="newDocTemplate = 'api'">
            <div class="tpl-icon">🔌</div>
            <div class="tpl-label">API 文档</div>
          </div>
          <div class="tpl-card" :class="{ active: newDocTemplate === 'readme' }" @click="newDocTemplate = 'readme'">
            <div class="tpl-icon">📖</div>
            <div class="tpl-label">README</div>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="showNewDoc = false">取消</el-button>
        <el-button type="primary" @click="createDoc('doc')">创建</el-button>
      </template>
    </el-dialog>

    <!-- 新建表格 -->
    <el-dialog v-model="showNewSheet" title="新建表格" width="400">
      <el-input v-model="newDocTitle" placeholder="表格标题" />
      <template #footer>
        <el-button @click="showNewSheet = false">取消</el-button>
        <el-button type="primary" @click="createDoc('sheet')">创建</el-button>
      </template>
    </el-dialog>

    <!-- 重命名 -->
    <el-dialog v-model="renameDialog" title="重命名" width="400">
      <el-input v-model="renameTitle" placeholder="新标题" />
      <template #footer>
        <el-button @click="renameDialog = false">取消</el-button>
        <el-button type="primary" @click="doRename">确定</el-button>
      </template>
    </el-dialog>

    <!-- 移动文档 -->
    <el-dialog v-model="moveDialog" title="移动到文件夹" width="400">
      <el-tree
        :data="treeData"
        :props="{ label: 'name', children: 'children' }"
        node-key="id"
        highlight-current
        default-expand-all
        @node-click="selectMoveTarget"
      >
        <template #default="{ data }">
          <span style="display:flex;align-items:center;gap:4px">
            <el-icon><Folder /></el-icon> {{ data.name }}
          </span>
        </template>
      </el-tree>
      <template #footer>
        <el-button @click="moveDialog = false">取消</el-button>
        <el-button type="primary" @click="doMove" :disabled="!moveTargetFolder">移动</el-button>
      </template>
    </el-dialog>

    <!-- 批量移动弹窗 -->
    <el-dialog v-model="showBatchMove" title="批量移动" width="400px">
      <p style="color:#999;margin-bottom:12px">将 {{ selectedDocs.length }} 个文档移动到：</p>
      <el-tree
        :data="treeData"
        :props="{ label: 'name', children: 'children', value: 'id' }"
        node-key="id"
        highlight-current
        default-expand-all
        @node-click="batchMoveTarget = $event.id"
      />
      <template #footer>
        <el-button @click="showBatchMove = false">取消</el-button>
        <el-button type="primary" @click="doBatchMove" :disabled="!batchMoveTarget">移动</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Star, StarFilled, Clock, Files, MoreFilled, Operation } from '@element-plus/icons-vue'
import http from '@/utils/http'

const router = useRouter()
const treeData = ref<any[]>([])
const docs = ref<any[]>([])
const selectedDocs = ref<string[]>([])
const showBatchMove = ref(false)
const batchMoveTarget = ref('')
const sidebarTags = ref<any[]>([])
const currentFolder = ref<string | null>(null)
const viewMode = ref('all')
const search = ref('')
const searchMode = ref(false)
const favoriteIds = ref<Set<string>>(new Set())
const sidebarOpen = ref(false)

const showNewFolder = ref(false)
const showNewDoc = ref(false)
const showNewSheet = ref(false)
const renameDialog = ref(false)
const renameTitle = ref('')
const renameDoc = ref<any>(null)
const moveDialog = ref(false)
const moveDoc = ref<any>(null)
const moveTargetFolder = ref('')
const newFolderName = ref('')
const newDocTitle = ref('')
const newDocTemplate = ref('')

const templates: Record<string, string> = {
  meeting: '<h2>会议纪要</h2><p><strong>日期：</strong>' + new Date().toLocaleDateString() + '</p><h3>讨论内容</h3><ul><li></li></ul><h3>决议</h3><ul><li></li></ul><h3>待办事项</h3><table><thead><tr><th>任务</th><th>负责人</th><th>截止日期</th><th>状态</th></tr></thead><tbody><tr><td></td><td></td><td></td><td></td></tr></tbody></table>',
  weekly: '<h2>周报 - ' + new Date().toLocaleDateString() + '</h2><h3>本周完成</h3><ul><li></li></ul><h3>进行中</h3><ul><li></li></ul><h3>下周计划</h3><ul><li></li></ul><h3>风险/问题</h3><ul><li></li></ul>',
  requirement: '<h2>需求文档</h2><h3>1. 背景与目标</h3><p></p><h3>2. 用户故事</h3><p><strong>作为</strong> ___ <strong>我希望</strong> ___ <strong>以便</strong> ___</p><h3>3. 功能需求</h3><table><thead><tr><th>编号</th><th>功能</th><th>优先级</th><th>描述</th></tr></thead><tbody><tr><td>F-001</td><td></td><td></td><td></td></tr></tbody></table><h3>4. 非功能需求</h3><ul><li>性能：</li><li>安全：</li><li>兼容性：</li></ul><h3>5. 验收标准</h3><ul><li></li></ul>',
  api: '<h2>API 文档</h2><h3>接口信息</h3><table><thead><tr><th>项目</th><th>内容</th></tr></thead><tbody><tr><td>Method</td><td>GET/POST/PUT/DELETE</td></tr><tr><td>Path</td><td>/api/v1/resource</td></tr><tr><td>认证</td><td>Bearer Token</td></tr></tbody></table><h3>请求参数</h3><table><thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead><tbody><tr><td></td><td></td><td></td><td></td></tr></tbody></table><h3>响应示例</h3><pre><code>{"code":200,"data":{}}</code></pre>',
  readme: '<h1>项目名称</h1><p>项目简介描述</p><h2>快速开始</h2><pre><code>npm install\nnpm run dev</code></pre><h2>功能特性</h2><ul><li></li></ul><h2>技术栈</h2><ul><li></li></ul><h2>许可证</h2><p>MIT</p>',
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前'
  if (diff < 604800000) return Math.floor(diff / 86400000) + ' 天前'
  return d.toLocaleDateString('zh-CN')
}

async function loadTree() {
  const { data } = await http.get('/docs/tree')
  treeData.value = buildTree(data.data || [])
}

async function loadDocs(folderId?: string) {
  const params: any = {}
  if (folderId) params.folder_id = folderId
  const { data } = await http.get('/docs/documents', { params })
  docs.value = (data.data || []).map((d: any) => ({
    ...d,
    is_favorite: favoriteIds.value.has(d.id),
  }))
}

async function loadRecent() {
  const { data } = await http.get('/docs/documents/recent')
  docs.value = (data.data || []).map((d: any) => ({
    ...d,
    is_favorite: favoriteIds.value.has(d.id),
  }))
}

async function loadFavorites() {
  const { data } = await http.get('/docs/favorites')
  docs.value = (data.data || []).map((d: any) => ({
    ...d,
    is_favorite: true,
  }))
}

async function loadFavoriteIds() {
  const { data } = await http.get('/docs/favorites')
  favoriteIds.value = new Set((data.data || []).map((d: any) => d.id))
}

function switchView(mode: string) {
  viewMode.value = mode
  searchMode.value = false
  sidebarOpen.value = false
  if (mode === 'recent') loadRecent()
  else if (mode === 'favorites') loadFavorites()
  else loadDocs()
}

function onFolderClick(node: any) {
  currentFolder.value = node.id
  viewMode.value = 'all'
  searchMode.value = false
  sidebarOpen.value = false
  loadDocs(node.id)
}

async function doSearch() {
  if (!search.value) return
  searchMode.value = true
  const { data } = await http.get('/docs/documents/search', { params: { q: search.value } })
  docs.value = (data.data || []).map((d: any) => ({
    ...d,
    is_favorite: favoriteIds.value.has(d.id),
  }))
}

function clearSearch() {
  searchMode.value = false
  switchView(viewMode.value)
}

function buildTree(items: any[]): any[] {
  const map: any = {}
  const roots: any[] = []
  items.forEach((item: any) => { map[item.id] = { ...item, children: [] } })
  items.forEach((item: any) => {
    if (item.parent_id && map[item.parent_id]) map[item.parent_id].children.push(map[item.id])
    else roots.push(map[item.id])
  })
  return roots
}

async function createFolder() {
  if (!newFolderName.value) return
  await http.post('/docs/folders', { name: newFolderName.value, parent_id: currentFolder.value })
  ElMessage.success('文件夹已创建')
  showNewFolder.value = false
  newFolderName.value = ''
  loadTree()
}

async function createDoc(type: string) {
  if (!newDocTitle.value) return
  const tplContent = type === 'doc' ? (templates[newDocTemplate.value] || '') : ''
  const { data } = await http.post('/docs/documents', {
    title: newDocTitle.value, type, folder_id: currentFolder.value,
    ...(tplContent ? { content: tplContent } : {}),
  })
  ElMessage.success('已创建')
  showNewDoc.value = false
  showNewSheet.value = false
  newDocTitle.value = ''
  newDocTemplate.value = ''
  loadDocs()
  router.push(`/docs/${data.data.id}`)
}

async function deleteDoc(row: any) {
  await ElMessageBox.confirm(`确定删除「${row.title}」？`, '删除确认', { type: 'warning' })
  await http.delete(`/docs/documents/${row.id}`)
  ElMessage.success('已删除')
  switchView(viewMode.value)
}

function showRename(doc: any) {
  renameDoc.value = doc
  renameTitle.value = doc.title
  renameDialog.value = true
}

async function doRename() {
  if (!renameTitle.value.trim()) return ElMessage.warning('标题不能为空')
  await http.put(`/docs/documents/${renameDoc.value.id}`, { title: renameTitle.value })
  renameDoc.value.title = renameTitle.value
  renameDialog.value = false
  ElMessage.success('已重命名')
}

function showMove(doc: any) {
  moveDoc.value = doc
  moveTargetFolder.value = ''
  moveDialog.value = true
}

function selectMoveTarget(data: any) {
  moveTargetFolder.value = data.id
}

async function doMove() {
  if (!moveTargetFolder.value) return
  await http.put(`/docs/documents/${moveDoc.value.id}`, { folder_id: moveTargetFolder.value })
  moveDialog.value = false
  ElMessage.success('已移动')
  switchView(viewMode.value)
}

function openDoc(row: any) {
  router.push(`/docs/${row.id}`)
}

async function toggleFavorite(doc: any) {
  try {
    if (doc.is_favorite) {
      await http.delete(`/docs/favorites/${doc.id}`)
      doc.is_favorite = false
      favoriteIds.value.delete(doc.id)
      ElMessage.success('已取消收藏')
    } else {
      await http.post(`/docs/favorites/${doc.id}`)
      doc.is_favorite = true
      favoriteIds.value.add(doc.id)
      ElMessage.success('已收藏')
    }
    if (viewMode.value === 'favorites') loadFavorites()
  } catch { /* ignore */ }
}

function toggleSelect(id: string) {
  const idx = selectedDocs.value.indexOf(id)
  if (idx >= 0) selectedDocs.value.splice(idx, 1)
  else selectedDocs.value.push(id)
}

async function batchDelete() {
  try {
    await ElMessageBox.confirm(`确定删除 ${selectedDocs.value.length} 个文档？`, '批量删除', { type: 'warning' })
  } catch { return }
  let ok = 0
  for (const id of selectedDocs.value) {
    try { await http.delete(`/docs/documents/${id}`); ok++ } catch {}
  }
  ElMessage.success(`已删除 ${ok} 个文档`)
  selectedDocs.value = []
  loadDocs()
}

async function doBatchMove() {
  let ok = 0
  for (const id of selectedDocs.value) {
    try { await http.put(`/docs/documents/${id}`, { folder_id: batchMoveTarget.value }); ok++ } catch {}
  }
  ElMessage.success(`已移动 ${ok} 个文档`)
  selectedDocs.value = []
  showBatchMove.value = false
  loadDocs()
}

onMounted(async () => {
  await loadFavoriteIds()
  loadTree()
  loadDocs()
  loadSidebarTags()
})

async function loadSidebarTags() {
  try {
    const { data } = await http.get('/docs/tags')
    sidebarTags.value = data || []
  } catch {}
}

async function filterByTag(tagId: string) {
  try {
    searchMode.value = true
    const { data } = await http.get(`/docs/tags/${tagId}/documents`)
    docs.value = data || []
  } catch {}
}
</script>

<style scoped>
.docs-page { height: 100%; display: flex; flex-direction: column; }
.batch-bar {
  display: flex; align-items: center; gap: 12px;
  padding: 8px 16px; background: #ecf5ff; border-bottom: 1px solid #d9ecff;
  font-size: 14px; color: #409eff;
}
.toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.content { flex: 1; display: flex; gap: 16px; overflow: hidden; }

/* 左侧栏 */
.sidebar {
  width: 220px;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  overflow-y: auto;
  background: #fafafa;
  flex-shrink: 0;
}
.sidebar-overlay {
  display: none;
}
.sidebar-section { padding: 8px 0; border-bottom: 1px solid #e8e8e8; }
.section-item {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 16px; cursor: pointer; font-size: 14px; color: #555;
  transition: all 0.2s;
}
.section-item:hover { background: #e8f0fe; color: #1a73e8; }
.section-item.active { background: #e8f0fe; color: #1a73e8; font-weight: 500; }
.tree-header {
  padding: 10px 16px; font-weight: bold;
  border-bottom: 1px solid #e8e8e8; background: #f5f5f5;
}
.tag-dot {
  display: inline-block; width: 8px; height: 8px; border-radius: 50%;
  margin-right: 6px; vertical-align: middle;
}
.tree-node { display: flex; align-items: center; gap: 6px; font-size: 14px; }

/* 右侧 */
.doc-list { flex: 1; overflow-y: auto; }
.list-header { padding: 8px 12px; color: #888; font-size: 13px; margin-bottom: 12px; }

/* 空状态 */
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; height: 300px; color: #999;
}
.empty-state p { margin-top: 12px; font-size: 14px; }

/* 文档卡片网格 */
.doc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}
.doc-card {
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  padding: 16px;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
  background: #fff;
}
.doc-card:hover {
  border-color: #409eff;
  box-shadow: 0 2px 12px rgba(64,158,255,0.15);
  transform: translateY(-1px);
}
.doc-card.selected {
  border-color: #409eff;
  background: #ecf5ff;
}
.card-checkbox {
  position: absolute;
  top: 8px;
  left: 8px;
  z-index: 1;
}
.card-icon { color: #409eff; margin-bottom: 8px; }
.card-body { min-width: 0; }
.card-title {
  font-size: 15px; font-weight: 500; margin-bottom: 6px;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.card-meta { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.card-version { color: #999; font-size: 12px; }
.card-time { color: #999; font-size: 12px; }
.card-actions {
  position: absolute; top: 8px; right: 8px;
  display: flex; gap: 4px; opacity: 0; transition: opacity 0.2s;
}
.doc-card:hover .card-actions { opacity: 1; }
.card-actions .el-icon { cursor: pointer; color: #999; padding: 4px; border-radius: 4px; }
.card-actions .el-icon:hover { color: #409eff; background: #f0f5ff; }
.fav-active { color: #f7ba2a !important; }
.more-btn { cursor: pointer; }

/* 移动端适配 */
@media (max-width: 768px) {
  .btn-text { display: none; }
  .toolbar { flex-wrap: wrap; gap: 6px; }
  .toolbar .el-input { width: 100% !important; order: 10; }
  .menu-btn { display: inline-flex !important; }

  .sidebar {
    position: fixed;
    top: 0; left: 0;
    width: 260px; height: 100vh;
    z-index: 200;
    border-radius: 0;
    border: none;
    border-right: 1px solid #e8e8e8;
    transform: translateX(-100%);
    transition: transform 0.25s ease;
    display: flex;
  }
  .sidebar.open {
    transform: translateX(0);
    box-shadow: 4px 0 16px rgba(0,0,0,0.15);
  }
  .sidebar-overlay.open {
    display: block;
    position: fixed; top: 0; left: 0;
    width: 100vw; height: 100vh;
    background: rgba(0,0,0,0.3);
    z-index: 199;
  }
  .sidebar-inner { width: 100%; height: 100%; overflow-y: auto; background: #fafafa; }

  .doc-grid { grid-template-columns: 1fr; }
  .doc-card { padding: 12px; }
  .card-icon .el-icon { font-size: 24px; }
  .card-actions { opacity: 1; }
}

/* PC端隐藏菜单按钮 */
@media (min-width: 769px) {
  .menu-btn { display: none !important; }
}

/* Template grid */
.template-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.tpl-card {
  border: 2px solid #e8e8e8; border-radius: 8px; padding: 12px 8px;
  text-align: center; cursor: pointer; transition: all 0.2s;
}
.tpl-card:hover { border-color: #409eff; background: #f5f7fa; }
.tpl-card.active { border-color: #409eff; background: #ecf5ff; }
.tpl-icon { font-size: 24px; margin-bottom: 4px; }
.tpl-label { font-size: 12px; color: #606266; }
</style>
