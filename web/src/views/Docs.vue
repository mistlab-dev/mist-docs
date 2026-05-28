<template>
  <div class="docs-page">
    <div class="toolbar">
      <el-button type="primary" @click="showNewFolder = true">
        <el-icon><FolderAdd /></el-icon> 新建文件夹
      </el-button>
      <el-button type="primary" @click="showNewDoc = true">
        <el-icon><Document /></el-icon> 新建文档
      </el-button>
      <el-button type="success" @click="showNewSheet = true">
        <el-icon><Grid /></el-icon> 新建表格
      </el-button>
      <div style="flex:1" />
      <el-input v-model="search" placeholder="搜索文档..." style="width:260px" clearable @keyup.enter="doSearch" @clear="clearSearch">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>

    <div class="content">
      <!-- 左侧：文件夹 + 快捷入口 -->
      <div class="sidebar">
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
            @click="openDoc(doc)"
          >
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
    <el-dialog v-model="showNewDoc" title="新建文档" width="400">
      <el-input v-model="newDocTitle" placeholder="文档标题" />
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Star, StarFilled, Clock, Files, MoreFilled } from '@element-plus/icons-vue'
import http from '@/utils/http'

const router = useRouter()
const treeData = ref<any[]>([])
const docs = ref<any[]>([])
const currentFolder = ref<string | null>(null)
const viewMode = ref('all')
const search = ref('')
const searchMode = ref(false)
const favoriteIds = ref<Set<string>>(new Set())

const showNewFolder = ref(false)
const showNewDoc = ref(false)
const showNewSheet = ref(false)
const newFolderName = ref('')
const newDocTitle = ref('')

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
  if (mode === 'recent') loadRecent()
  else if (mode === 'favorites') loadFavorites()
  else loadDocs()
}

function onFolderClick(node: any) {
  currentFolder.value = node.id
  viewMode.value = 'all'
  searchMode.value = false
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
  const { data } = await http.post('/docs/documents', {
    title: newDocTitle.value, type, folder_id: currentFolder.value,
  })
  ElMessage.success('已创建')
  showNewDoc.value = false
  showNewSheet.value = false
  newDocTitle.value = ''
  loadDocs()
  router.push(`/docs/${data.data.id}`)
}

async function deleteDoc(row: any) {
  await ElMessageBox.confirm(`确定删除「${row.title}」？`, '删除确认', { type: 'warning' })
  await http.delete(`/docs/documents/${row.id}`)
  ElMessage.success('已删除')
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

onMounted(async () => {
  await loadFavoriteIds()
  loadTree()
  loadDocs()
})
</script>

<style scoped>
.docs-page { height: 100%; display: flex; flex-direction: column; }
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
</style>
