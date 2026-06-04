<template>
  <div class="docs-page">
    <!-- 顶部工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="sidebarOpen = !sidebarOpen" class="menu-btn" circle size="small">
          <el-icon><Operation /></el-icon>
        </el-button>
        <el-button type="primary" @click="showNewDoc = true">
          <el-icon><Plus /></el-icon> 新建文档
        </el-button>
        <el-button @click="showNewSheet = true">
          <el-icon><Grid /></el-icon> 表格
        </el-button>
        <el-button @click="showImportDialog = true">
          <el-icon><Upload /></el-icon>
        </el-button>
      </div>
      <div class="toolbar-right">
        <el-input v-model="search" placeholder="搜索文档..." class="search-box" clearable @input="debounceSearch" @clear="clearSearch" size="default">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="searchTagId" placeholder="全部标签" clearable class="tag-filter" size="default" @change="doSearch">
          <el-option v-for="t in allTags" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
        <el-select v-model="sortBy" size="default" class="sort-select" @change="sortDocs">
          <el-option label="更新时间" value="updated" />
          <el-option label="创建时间" value="created" />
          <el-option label="标题" value="title" />
        </el-select>
        <el-button-group>
          <el-button :type="layoutMode === 'grid' ? 'primary' : 'default'" @click="layoutMode = 'grid'" size="default">
            <el-icon><Monitor /></el-icon>
          </el-button>
          <el-button :type="layoutMode === 'list' ? 'primary' : 'default'" @click="layoutMode = 'list'" size="default">
            <el-icon><List /></el-icon>
          </el-button>
        </el-button-group>
      </div>
    </div>

    <!-- 批量操作栏 -->
    <transition name="slide-down">
      <div v-if="selectedDocs.length" class="batch-bar">
        <div class="batch-info">
          <div class="batch-dot" />
          <span>已选 <strong>{{ selectedDocs.length }}</strong> 项</span>
        </div>
        <el-button size="small" @click="showBatchMove = true">移动</el-button>
        <el-button size="small" @click="batchExport">导出</el-button>
        <el-button size="small" type="danger" @click="batchDelete">删除</el-button>
        <el-button size="small" link @click="selectedDocs = []">取消</el-button>
      </div>
    </transition>

    <div class="content">
      <!-- 移动端遮罩 -->
      <div class="sidebar-overlay" :class="{ open: sidebarOpen }" @click="sidebarOpen = false" />

      <!-- 左侧栏 -->
      <div class="sidebar" :class="{ open: sidebarOpen }">
        <div class="sidebar-inner">
          <!-- 快捷入口 -->
          <div class="sidebar-section">
            <div class="nav-item" :class="{ active: viewMode === 'all' }" @click="switchView('all')">
              <svg viewBox="0 0 20 20" fill="currentColor" class="nav-icon"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
              全部文档
            </div>
            <div class="nav-item" :class="{ active: viewMode === 'recent' }" @click="switchView('recent')">
              <el-icon class="nav-icon"><Clock /></el-icon>
              最近打开
            </div>
            <div class="nav-item" :class="{ active: viewMode === 'favorites' }" @click="switchView('favorites')">
              <el-icon class="nav-icon"><Star /></el-icon>
              我的收藏
            </div>
          </div>

          <!-- 标签 -->
          <div v-if="sidebarTags.length" class="sidebar-section">
            <div class="section-title">标签</div>
            <div v-for="tag in sidebarTags" :key="tag.id" class="nav-item" @click="filterByTag(tag.id)">
              <span class="tag-dot" :style="{ background: tag.color }" />
              {{ tag.name }}
              <span class="tag-count">{{ tag.doc_count }}</span>
            </div>
          </div>

          <!-- 文件夹树 -->
          <div class="sidebar-section">
            <div class="section-title">
              文件夹
              <el-button size="small" text @click="newFolderParentId = null; showNewFolder = true" class="section-add">+ 新建</el-button>
            </div>
            <el-tree
              :data="treeData"
              :props="{ label: 'name', children: 'children' }"
              node-key="id"
              highlight-current
              default-expand-all
              @node-click="onFolderClick"
              @node-contextmenu="onFolderContextMenu"
              class="folder-tree"
            >
              <template #default="{ data }">
                <span class="tree-node">
                  <svg viewBox="0 0 20 20" fill="currentColor" class="folder-icon"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
                  <span>{{ data.name }}<span v-if="data.doc_count > 0" class="doc-count-badge">{{ data.doc_count }}</span></span>
                </span>
              </template>
            </el-tree>
          </div>
        </div>
      </div>

      <!-- 右侧文档区 -->
      <div class="doc-area">
        <!-- 搜索结果头 -->
        <div v-if="searchMode" class="search-header">
          搜索「{{ search }}」— 找到 {{ docs.length }} 个结果
        </div>

        <!-- 加载 -->
        <div v-if="loading" class="loading-state">
          <el-skeleton :rows="5" animated />
        </div>

        <!-- 空状态 -->
        <div v-else-if="!docs.length" class="empty-state">
          <div class="empty-icon"><svg viewBox="0 0 20 20" fill="currentColor" width="48" height="48"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 3h6v2H7V7zm0 4h4v2H7v-2z"/></svg></div>
          <p v-if="viewMode === 'recent'">还没有打开过文档</p>
          <p v-else-if="viewMode === 'favorites'">还没有收藏文档</p>
          <p v-else>暂无文档，点击「新建文档」开始</p>
        </div>

        <!-- 网格视图 -->
        <div v-else-if="layoutMode === 'grid'" class="doc-grid">
          <div
            v-for="doc in docs"
            :key="doc.id"
            class="doc-card"
            :class="{ selected: selectedDocs.includes(doc.id) }"
            @click="openDoc(doc)"
          >
            <div class="card-check" @click.stop="toggleSelect(doc.id)">
              <el-checkbox :model-value="selectedDocs.includes(doc.id)" />
            </div>
            <div class="card-preview" :class="doc.type">
              <el-icon :size="36">
                <Document v-if="doc.type === 'doc'" />
                <Grid v-else />
              </el-icon>
            </div>
            <div class="card-body">
              <div class="card-title" v-html="doc.titleHtml || doc.title"></div>
              <div v-if="doc.snippetHtml || doc.snippet" class="card-snippet" v-html="doc.snippetHtml || doc.snippet"></div>
              <div class="card-meta">
                <el-tag :type="doc.type === 'doc' ? '' : 'success'" size="small" effect="light" round>
                  {{ doc.type === 'doc' ? '文档' : '表格' }}
                </el-tag>
                <span class="card-version">v{{ doc.version }}</span>
              </div>
              <div class="card-footer">
                <span class="card-author">{{ doc.created_by_name || '未知' }}</span>
                <span class="card-time">{{ formatTime(doc.updated_at) }}</span>
              </div>
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
                <el-icon :size="18" class="more-icon"><MoreFilled /></el-icon>
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

        <!-- 列表视图 -->
        <div v-else class="doc-table-wrap">
          <el-table
            :data="docs"
            @row-click="openDoc"
            :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
            :cell-style="{ fontSize: '14px' }"
            @selection-change="(rows: any[]) => selectedDocs = rows.map((r: any) => r.id)"
          >
            <el-table-column type="selection" width="40" />
            <el-table-column label="标题" min-width="260">
              <template #default="{ row }">
                <div class="table-title">
                  <div class="type-dot" :class="row.type">
                    <el-icon :size="14"><Document v-if="row.type==='doc'" /><Grid v-else /></el-icon>
                  </div>
                  <span v-html="row.titleHtml || row.title"></span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.type === 'doc' ? '' : 'success'" size="small" effect="light" round>
                  {{ row.type === 'doc' ? '文档' : '表格' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建者" width="100">
              <template #default="{ row }">
                <span class="author-text">{{ row.created_by_name || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="version" label="版本" width="70" align="center">
              <template #default="{ row }">
                <span class="version-text">v{{ row.version }}</span>
              </template>
            </el-table-column>
            <el-table-column label="更新时间" width="140">
              <template #default="{ row }">
                <span class="time-text">{{ formatTime(row.updated_at) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="" width="80" fixed="right" align="center">
              <template #default="{ row }">
                <div @click.stop class="table-actions">
                  <el-icon :size="16" :class="{ 'fav-active': row.is_favorite }" @click="toggleFavorite(row)" style="cursor:pointer">
                    <StarFilled v-if="row.is_favorite" /><Star v-else />
                  </el-icon>
                  <el-dropdown trigger="click">
                    <el-icon :size="16" style="cursor:pointer"><MoreFilled /></el-icon>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="showRename(row)">重命名</el-dropdown-item>
                        <el-dropdown-item @click="showMove(row)">移动</el-dropdown-item>
                        <el-dropdown-item @click="deleteDoc(row)" divided><span style="color:#f56c6c">删除</span></el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </div>

    <!-- 新建文件夹 -->
    <el-dialog v-model="showNewFolder" :title="newFolderParentId ? '新建子文件夹' : '新建文件夹'" width="400" destroy-on-close>
      <el-input v-model="newFolderName" placeholder="文件夹名称" size="large" />
      <template #footer>
        <el-button @click="showNewFolder = false">取消</el-button>
        <el-button type="primary" @click="createFolder">创建</el-button>
      </template>
    </el-dialog>

    <!-- 文件夹右键菜单 -->
    <div v-if="folderCtxMenu.show" :style="{ position: 'fixed', left: folderCtxMenu.x + 'px', top: folderCtxMenu.y + 'px', zIndex: 9999 }" class="folder-ctx-menu">
      <div class="folder-ctx-item" @click="createSubFolder">新建子文件夹</div>
      <div class="folder-ctx-item" @click="startRenameFolder">重命名</div>
      <div class="folder-ctx-item danger" @click="deleteFolder">删除</div>
    </div>

    <!-- 重命名文件夹 -->
    <el-dialog v-model="showRenameFolder" title="重命名文件夹" width="400" destroy-on-close>
      <el-input v-model="renameFolderName" placeholder="文件夹名称" size="large" />
      <template #footer>
        <el-button @click="showRenameFolder = false">取消</el-button>
        <el-button type="primary" @click="doRenameFolder">确定</el-button>
      </template>
    </el-dialog>

    <!-- 新建文档 -->
    <el-dialog v-model="showNewDoc" title="新建文档" width="520" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="文档标题">
          <el-input v-model="newDocTitle" placeholder="输入文档标题" size="large" />
        </el-form-item>
        <el-form-item label="选择模板">
          <div class="template-grid">
            <div v-for="t in templateList" :key="t.key" class="tpl-card" :class="{ active: newDocTemplate === t.key }" @click="newDocTemplate = t.key">
              <div class="tpl-icon" v-html="t.icon"></div>
              <div class="tpl-label">{{ t.name }}</div>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showNewDoc = false">取消</el-button>
        <el-button type="primary" @click="createDoc('doc')">创建</el-button>
      </template>
    </el-dialog>

    <!-- 新建表格 -->
    <el-dialog v-model="showNewSheet" title="新建表格" width="400" destroy-on-close>
      <el-input v-model="newDocTitle" placeholder="表格标题" size="large" />
      <template #footer>
        <el-button @click="showNewSheet = false">取消</el-button>
        <el-button type="primary" @click="createDoc('sheet')">创建</el-button>
      </template>
    </el-dialog>

    <!-- 导入 -->
    <el-dialog v-model="showImportDialog" title="批量导入" width="500">
      <p class="import-hint">支持 .txt、.md、.html、.docx、.xlsx 文件，最多20个，每个不超过10MB</p>
      <el-upload ref="importUpload" :auto-upload="false" :limit="20" multiple accept=".txt,.md,.html,.htm,.docx,.xlsx" :on-change="onImportFileChange" drag>
        <el-icon :size="32" color="#c0c4cc"><Upload /></el-icon>
        <div class="upload-text">拖拽文件到此处，或 <em>点击上传</em></div>
      </el-upload>
      <template #footer>
        <el-button @click="showImportDialog = false">取消</el-button>
        <el-button type="primary" @click="doImport" :loading="importing" :disabled="!importFiles.length">
          导入 {{ importFiles.length ? importFiles.length + ' 个文件' : '' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 重命名 -->
    <el-dialog v-model="renameDialog" title="重命名" width="400" destroy-on-close>
      <el-input v-model="renameTitle" placeholder="新标题" size="large" />
      <template #footer>
        <el-button @click="renameDialog = false">取消</el-button>
        <el-button type="primary" @click="doRename">确定</el-button>
      </template>
    </el-dialog>

    <!-- 移动 -->
    <el-dialog v-model="moveDialog" title="移动到文件夹" width="400">
      <el-tree :data="treeData" :props="{ label: 'name', children: 'children' }" node-key="id" highlight-current default-expand-all @node-click="selectMoveTarget">
        <template #default="{ data }">
          <span class="tree-node">
            <svg viewBox="0 0 20 20" fill="currentColor" class="folder-icon"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
            {{ data.name }}
          </span>
        </template>
      </el-tree>
      <template #footer>
        <el-button @click="moveDialog = false">取消</el-button>
        <el-button type="primary" @click="doMove" :disabled="!moveTargetFolder">移动</el-button>
      </template>
    </el-dialog>

    <!-- 批量移动 -->
    <el-dialog v-model="showBatchMove" title="批量移动" width="400">
      <p style="color:#909399;margin-bottom:16px">将 {{ selectedDocs.length }} 个文档移动到：</p>
      <el-tree :data="treeData" :props="{ label: 'name', children: 'children' }" node-key="id" highlight-current default-expand-all @node-click="batchMoveTarget = $event.id">
        <template #default="{ data }">
          <span class="tree-node">
            <svg viewBox="0 0 20 20" fill="currentColor" class="folder-icon"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
            {{ data.name }}
          </span>
        </template>
      </el-tree>
      <template #footer>
        <el-button @click="showBatchMove = false">取消</el-button>
        <el-button type="primary" @click="doBatchMove" :disabled="!batchMoveTarget">移动</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Star, StarFilled, Clock, Files, MoreFilled, Operation, Monitor, List } from '@element-plus/icons-vue'
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
const loading = ref(false)
const search = ref('')
const searchMode = ref(false)
const searchTagId = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

function debounceSearch() {
  clearTimeout(searchTimer!)
  if (!search.value && !searchTagId.value) {
    clearSearch()
    return
  }
  searchTimer = setTimeout(doSearch, 400)
}
const allTags = ref<any[]>([])
const favoriteIds = ref<Set<string>>(new Set())
const sidebarOpen = ref(false)

const showNewFolder = ref(false)
const newFolderParentId = ref<string | null>(null)
const showNewDoc = ref(false)
const showNewSheet = ref(false)
const showImportDialog = ref(false)
const importFiles = ref<any[]>([])
const importing = ref(false)
const renameDialog = ref(false)
const renameTitle = ref('')
const renameDoc = ref<any>(null)
const moveDialog = ref(false)
const moveDoc = ref<any>(null)
const moveTargetFolder = ref('')
const newFolderName = ref('')
const showRenameFolder = ref(false)
const renameFolderName = ref('')
const renameFolderId = ref('')
const folderCtxMenu = reactive({ show: false, x: 0, y: 0, nodeId: '', nodeName: '' })
const newDocTitle = ref('')
const newDocTemplate = ref('')
const layoutMode = ref<'grid'|'list'>('grid')
const sortBy = ref('updated')

const templateList = [
  { key: '', icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 3h6v2H7V7zm0 4h4v2H7v-2z"/></svg>', name: '空白文档' },
  { key: 'meeting', icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M2 5a2 2 0 012-2h8a2 2 0 012 2v3H6a2 2 0 00-2 2v5H4a2 2 0 01-2-2V5zm4 6a2 2 0 012-2h8a2 2 0 012 2v5a2 2 0 01-2 2H8a2 2 0 01-2-2v-5zm2 0v5h8v-5H8z"/></svg>', name: '会议纪要' },
  { key: 'weekly', icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M3 3a2 2 0 012-2h10a2 2 0 012 2v14a2 2 0 01-2 2H5a2 2 0 01-2-2V3zm2 0v14h10V3H5zm1 3h8v2H6V6zm0 4h6v2H6v-2z"/></svg>', name: '周报' },
  { key: 'requirement', icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 2h6v1H7V6zm0 2h6v1H7V8zm0 2h4v1H7v-1z"/><path d="M8 12l2 2 4-4" stroke=\'currentColor\' fill=\'none\' stroke-width=\'1.5\'/></svg>', name: '需求文档' },
  { key: 'api', icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M7 3a1 1 0 00-.894.553L5.382 5H4a1 1 0 000 2h12a1 1 0 100-2h-1.382l-.724-1.447A1 1 0 0013 3H7zm0 2h6l.724 1.447A1 1 0 0014.618 7H5.382a1 1 0 00.894-.553L7 5zM5 9h10v6a2 2 0 01-2 2H7a2 2 0 01-2-2V9z"/></svg>', name: 'API 文档' },
  { key: 'readme', icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6zm0 2.5L17.5 8H14V4.5zM6 4h6v6h6v10H6V4z"/></svg>', name: 'README' },
]

const templates: Record<string, string> = {
  meeting: '<h2>会议纪要</h2><p><strong>日期：</strong>' + new Date().toLocaleDateString() + '</p><h3>讨论内容</h3><ul><li></li></ul><h3>决议</h3><ul><li></li></ul><h3>待办事项</h3><table><thead><tr><th>任务</th><th>负责人</th><th>截止日期</th><th>状态</th></tr></thead><tbody><tr><td></td><td></td><td></td><td></td></tr></tbody></table>',
  weekly: '<h2>周报 - ' + new Date().toLocaleDateString() + '</h2><h3>本周完成</h3><ul><li></li></ul><h3>进行中</h3><ul><li></li></ul><h3>下周计划</h3><ul><li></li></ul><h3>风险/问题</h3><ul><li></li></ul>',
  requirement: '<h2>需求文档</h2><h3>1. 背景与目标</h3><p></p><h3>2. 用户故事</h3><p><strong>作为</strong> ___ <strong>我希望</strong> ___ <strong>以便</strong> ___</p><h3>3. 功能需求</h3><table><thead><tr><th>编号</th><th>功能</th><th>优先级</th><th>描述</th></tr></thead><tbody><tr><td>F-001</td><td></td><td></td><td></td></tr></tbody></table><h3>4. 非功能需求</h3><ul><li>性能：</li><li>安全：</li><li>兼容性：</li></ul><h3>5. 验收标准</h3><ul><li></li></ul>',
  api: '<h2>API 文档</h2><h3>接口信息</h3><table><thead><tr><th>项目</th><th>内容</th></tr></thead><tbody><tr><td>Method</td><td>GET/POST/PUT/DELETE</td></tr><tr><td>Path</td><td>/api/v1/resource</td></tr><tr><td>认证</td><td>Bearer Token</td></tr></tbody></table><h3>请求参数</h3><table><thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead><tbody><tr><td></td><td></td><td></td><td></td></tr></tbody></table><h3>响应示例</h3><pre><code>{"code":200,"data":{}}</code></pre>',
  readme: '<h1>项目名称</h1><p>项目简介描述</p><h2>快速开始</h2><pre><code>npm install\nnpm run dev</code></pre><h2>功能特性</h2><ul><li></li></ul><h2>技术栈</h2><ul><li></li></ul><h2>许可证</h2><p>MIT</p>',
}

function sortDocs() {
  if (sortBy.value === 'title') {
    docs.value.sort((a, b) => a.title.localeCompare(b.title))
  } else if (sortBy.value === 'created') {
    docs.value.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  } else {
    docs.value.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
  }
}

function highlightText(text: string, keyword: string): string {
  if (!keyword || !text) return text
  const escaped = keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return text.replace(new RegExp(`(${escaped})`, 'gi'), '<mark class="search-hl">$1</mark>')
}

function setDocs(list: any[]) {
  docs.value = list.map((d: any) => ({
    ...d,
    is_favorite: favoriteIds.value.has(d.id),
    titleHtml: searchMode.value ? highlightText(d.title, search.value) : '',
    snippetHtml: d.snippet ? highlightText(d.snippet, search.value) : '',
  }))
  sortDocs()
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
  // 后端已返回树形结构，直接使用；兼容旧版扁平数据
  const raw = data.data || []
  treeData.value = raw.length > 0 && raw[0].children !== undefined ? raw : buildTree(raw)
}

async function loadDocs(folderId?: string) {
  loading.value = true
  try {
    const params: any = {}
    if (folderId) params.folder_id = folderId
    const { data } = await http.get('/docs/documents', { params })
    setDocs(data.data || [])
  } finally { loading.value = false }
}

async function loadRecent() {
  loading.value = true
  try {
    const { data } = await http.get('/docs/documents/recent')
    setDocs(data.data || [])
  } finally { loading.value = false }
}

async function loadFavorites() {
  loading.value = true
  try {
    const { data } = await http.get('/docs/favorites')
    setDocs(data.data || [])
  } finally { loading.value = false }
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
  if (!search.value && !searchTagId.value) return
  searchMode.value = true
  const params: any = { q: search.value || '' }
  if (searchTagId.value) params.tag_id = searchTagId.value
  const { data } = await http.get('/docs/documents/search', { params })
  setDocs(data.data || [])
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
  const parentId = newFolderParentId.value ?? currentFolder.value
  await http.post('/docs/folders', { name: newFolderName.value, parent_id: parentId })
  ElMessage.success('文件夹已创建')
  showNewFolder.value = false
  newFolderName.value = ''
  newFolderParentId.value = null
  loadTree()
}

function onFolderContextMenu(event: MouseEvent, node: any) {
  event.preventDefault()
  folderCtxMenu.show = true
  folderCtxMenu.x = event.clientX
  folderCtxMenu.y = event.clientY
  folderCtxMenu.nodeId = node.id
  folderCtxMenu.nodeName = node.name
}

function createSubFolder() {
  folderCtxMenu.show = false
  newFolderParentId.value = folderCtxMenu.nodeId
  newFolderName.value = ''
  showNewFolder.value = true
}

function startRenameFolder() {
  folderCtxMenu.show = false
  renameFolderId.value = folderCtxMenu.nodeId
  renameFolderName.value = folderCtxMenu.nodeName
  showRenameFolder.value = true
}

async function doRenameFolder() {
  if (!renameFolderName.value) return
  await http.put(`/docs/folders/${renameFolderId.value}`, { name: renameFolderName.value })
  ElMessage.success('文件夹已重命名')
  showRenameFolder.value = false
  loadTree()
}

async function deleteFolder() {
  folderCtxMenu.show = false
  try {
    await ElMessageBox.confirm(`确定删除文件夹「${folderCtxMenu.nodeName}」吗？`, '删除文件夹', { type: 'warning' })
    try {
      await http.delete(`/docs/folders/${folderCtxMenu.nodeId}`)
      ElMessage.success('文件夹已删除')
      if (currentFolder.value === folderCtxMenu.nodeId) {
        currentFolder.value = null
        loadDocs()
      }
      loadTree()
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.error || '删除文件夹失败')
    }
  } catch {}
}

async function createDoc(type: string) {
  if (!newDocTitle.value.trim()) return ElMessage.warning('请输入文档标题')
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

async function batchExport() {
  if (!selectedDocs.value.length) return
  ElMessage.info('正在导出...')
  let ok = 0
  for (const id of selectedDocs.value) {
    try {
      const resp = await http.get(`/docs/documents/${id}/export`, { params: { format: 'markdown' }, responseType: 'blob' })
      const url = URL.createObjectURL(resp.data)
      const a = document.createElement('a'); a.href = url
      a.download = resp.headers['content-disposition']?.match(/"([^"]+)"/)?.[1] || `${id}.md`
      a.click(); URL.revokeObjectURL(url)
      ok++
    } catch {}
  }
  ElMessage.success(`已导出 ${ok} 个文档`)
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

function onImportFileChange(_file: any, fileList: any[]) {
  importFiles.value = fileList
}

async function doImport() {
  if (!importFiles.value.length) return
  importing.value = true
  const fd = new FormData()
  for (const f of importFiles.value) fd.append('files', f.raw)
  if (currentFolder.value) fd.append('folder_id', currentFolder.value)
  try {
    const { data } = await http.post('/docs/import', fd)
    ElMessage.success(data.message || '导入完成')
    showImportDialog.value = false
    importFiles.value = []
    loadDocs()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || '导入失败')
  }
  importing.value = false
}

onMounted(async () => {
  await loadFavoriteIds()
  loadTree()
  loadDocs()
  loadSidebarTags()
  document.addEventListener('click', closeFolderCtxMenu)
})
onUnmounted(() => {
  document.removeEventListener('click', closeFolderCtxMenu)
})

function closeFolderCtxMenu() { folderCtxMenu.show = false }

async function loadSidebarTags() {
  try {
    const { data } = await http.get('/docs/tags')
    sidebarTags.value = data || []
    allTags.value = data?.data || data || []
  } catch {}
}

async function filterByTag(tagId: string) {
  try {
    searchMode.value = true
    const { data } = await http.get(`/docs/tags/${tagId}/documents`)
    setDocs(data || [])
  } catch {}
}
</script>

<style scoped>
.docs-page { height: 100%; display: flex; flex-direction: column; padding: 16px 20px; background: #f5f7fa; }

/* 工具栏 */
.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 16px; flex-wrap: wrap; gap: 12px;
}
.toolbar-left { display: flex; align-items: center; gap: 8px; }
.toolbar-right { display: flex; align-items: center; gap: 8px; }
.search-box { width: 220px; }
.tag-filter { width: 140px; }
.sort-select { width: 120px; }

/* 批量操作 */
.batch-bar {
  display: flex; align-items: center; gap: 16px;
  padding: 10px 20px; margin-bottom: 12px;
  background: linear-gradient(135deg, #e6f7ff, #f0f5ff);
  border: 1px solid #91d5ff; border-radius: 10px;
  font-size: 14px; color: #1890ff;
}
.batch-info { display: flex; align-items: center; gap: 8px; }
.batch-dot { width: 8px; height: 8px; border-radius: 50%; background: #1890ff; }

.slide-down-enter-active, .slide-down-leave-active { transition: all 0.25s ease; }
.slide-down-enter-from, .slide-down-leave-to { opacity: 0; transform: translateY(-8px); margin-top: -44px; }

/* 主内容区 */
.content { flex: 1; display: flex; gap: 16px; overflow: hidden; }

/* 左侧栏 */
.sidebar {
  width: 230px; border-radius: 16px; overflow-y: auto;
  background: #fff; box-shadow: 0 2px 12px rgba(0,0,0,0.04);
  flex-shrink: 0;
}
.sidebar-overlay { display: none; }
.sidebar-inner { padding: 8px 0; }
.sidebar-section { border-bottom: 1px solid #f0f0f0; }

.nav-item {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 16px; cursor: pointer; font-size: 14px; color: #606266;
  transition: all 0.15s;
}
.nav-item:hover { background: #f0f5ff; color: #4f6ef7; }
.nav-item.active { background: #e8f0fe; color: #4f6ef7; font-weight: 600; }
.nav-icon { width: 18px; height: 18px; color: inherit; }

.section-title {
  padding: 10px 16px; font-size: 12px; font-weight: 600;
  color: #909399; text-transform: uppercase; letter-spacing: 0.05em;
  display: flex; justify-content: space-between; align-items: center;
}
.section-add { font-size: 12px; color: #4f6ef7; }

.tag-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 2px; }
.tag-count { font-size: 12px; color: #c0c4cc; margin-left: auto; }

.folder-tree :deep(.el-tree-node__content) { height: 36px; border-radius: 6px; margin: 1px 8px; }
.folder-tree :deep(.el-tree-node.is-current > .el-tree-node__content) { background: #e8f0fe; color: #4f6ef7; }
.tree-node { display: flex; align-items: center; gap: 6px; font-size: 13px; }
.folder-icon { width: 16px; height: 16px; color: #fa8c16; }
.doc-count-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 20px; padding: 0 6px;
  background: #e8f0fe; color: #4f6ef7; border-radius: 10px;
  font-size: 11px; font-weight: 600; margin-left: 6px;
}

/* 右侧文档区 */
.doc-area { flex: 1; overflow-y: auto; min-width: 0; }

.search-header {
  padding: 12px 16px; margin-bottom: 12px;
  background: #fff; border-radius: 12px; font-size: 14px; color: #606266;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.search-hl { background: #fef08a; color: inherit; padding: 0 2px; border-radius: 2px; }

/* 空状态 */
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; height: 400px; color: #909399;
}
.empty-icon { font-size: 56px; margin-bottom: 16px; opacity: 0.5; }
.empty-state p { font-size: 15px; }
.loading-state { padding: 40px 20px; }

/* 网格视图 */
.doc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}
.doc-card {
  background: #fff; border-radius: 14px; padding: 16px;
  cursor: pointer; position: relative;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
  transition: all 0.2s ease;
}
.doc-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(0,0,0,0.08);
}
.doc-card.selected { background: #f0f5ff; border: 2px solid #4f6ef7; padding: 14px; }

.card-check { position: absolute; top: 10px; left: 10px; z-index: 1; }

.card-preview {
  width: 100%; height: 80px; border-radius: 10px; margin-bottom: 12px;
  display: flex; align-items: center; justify-content: center;
}
.card-preview.doc { background: linear-gradient(135deg, #e8f0fe, #f0f5ff); color: #4f6ef7; }
.card-preview:not(.doc) { background: linear-gradient(135deg, #e6f7f0, #f0fff7); color: #36b37e; }

.card-body { min-width: 0; }
.card-title {
  font-size: 14px; font-weight: 600; color: #1a1a2e; margin-bottom: 8px;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.card-snippet { font-size: 12px; color: #909399; margin-bottom: 4px; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; line-height: 1.5; }
.card-meta { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.card-version { color: #909399; font-size: 12px; }
.card-footer { display: flex; align-items: center; justify-content: space-between; margin-top: 4px; }
.card-author { color: #909399; font-size: 12px; display: flex; align-items: center; gap: 4px; }
.card-author::before { content: ''; display: inline-block; width: 14px; height: 14px; border-radius: 50%; background: #e0e4ea; }
.card-time { color: #909399; font-size: 12px; }

.card-actions {
  position: absolute; top: 10px; right: 10px;
  display: flex; gap: 2px; opacity: 0; transition: opacity 0.2s;
}
.doc-card:hover .card-actions { opacity: 1; }
.card-actions .el-icon { cursor: pointer; color: #909399; padding: 4px; border-radius: 6px; }
.card-actions .el-icon:hover { color: #4f6ef7; background: #f0f5ff; }
.more-icon { cursor: pointer; }
.fav-active { color: #f7ba2a !important; }

/* 列表视图 */
.doc-table-wrap {
  background: #fff; border-radius: 16px; overflow: hidden;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.doc-table-wrap :deep(.el-table__row) { height: 56px; cursor: pointer; }
.doc-table-wrap :deep(.el-table__row:hover) { background: #f9fbff !important; }
.doc-table-wrap :deep(.el-table__cell) { padding: 12px 0; }

.table-title { display: flex; align-items: center; gap: 10px; }
.type-dot {
  width: 28px; height: 28px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.type-dot.doc { background: #e8f0fe; color: #4f6ef7; }
.type-dot:not(.doc) { background: #e6f7f0; color: #36b37e; }

.version-text { font-size: 12px; color: #909399; }
.author-text { font-size: 13px; color: #606266; }
.time-text { font-size: 13px; color: #909399; }

.table-actions { display: flex; align-items: center; gap: 8px; }

/* 弹窗 */
:deep(.el-dialog) { border-radius: 16px; }

/* 模板 */
.template-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; width: 100%; }
.tpl-card {
  border: 2px solid #edf0f4; border-radius: 12px; padding: 14px 8px;
  text-align: center; cursor: pointer; transition: all 0.2s;
}
.tpl-card:hover { border-color: #4f6ef7; background: #f9fbff; }
.tpl-card.active { border-color: #4f6ef7; background: #f0f5ff; }
.tpl-icon { font-size: 28px; margin-bottom: 6px; }
.tpl-label { font-size: 13px; color: #606266; }

/* 导入 */
.import-hint { color: #909399; font-size: 13px; margin-bottom: 12px; }
.upload-text { color: #909399; font-size: 14px; margin-top: 8px; }
.upload-text em { color: #4f6ef7; font-style: normal; }

/* 移动端 */
@media (max-width: 768px) {
  .docs-page { padding: 12px; }
  .toolbar { gap: 8px; }
  .search-box { width: 100% !important; order: 10; }
  .sort-select { width: 100% !important; order: 11; }

  .sidebar {
    position: fixed; top: 0; left: 0;
    width: 280px; height: calc(var(--vh, 1vh) * 100); z-index: 200;
    border-radius: 0; box-shadow: none;
    transform: translateX(-100%); transition: transform 0.25s ease;
  }
  .sidebar.open { transform: translateX(0); box-shadow: 4px 0 24px rgba(0,0,0,0.15); }
  .sidebar-overlay.open {
    display: block; position: fixed; top: 0; left: 0;
    width: 100vw; height: calc(var(--vh, 1vh) * 100); background: rgba(0,0,0,0.3); z-index: 199;
  }

  .doc-grid { grid-template-columns: 1fr; }
  .doc-card { display: flex; align-items: center; gap: 12px; }
  .card-preview { width: 48px; height: 48px; margin-bottom: 0; flex-shrink: 0; }
  .card-actions { opacity: 1; position: static; }
  .card-check { position: static; }
}
@media (min-width: 769px) { .menu-btn { display: none !important; } }
.folder-ctx-menu { background: #fff; border-radius: 8px; box-shadow: 0 4px 16px rgba(0,0,0,.12); padding: 4px 0; min-width: 140px; }
.folder-ctx-item { padding: 8px 16px; cursor: pointer; font-size: 13px; color: #333; transition: background .15s; }
.folder-ctx-item:hover { background: #f0f5ff; color: #409eff; }
.folder-ctx-item.danger { color: #f56c6c; }
.folder-ctx-item.danger:hover { background: #fef0f0; }
</style>
