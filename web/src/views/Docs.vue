<template>
  <div class="docs-page" @click="closeContextMenu">
    <div class="top-nav">
      <div class="nav-left">
        <el-button @click="sidebarOpen = !sidebarOpen" class="menu-btn" text><el-icon :size="18"><Operation /></el-icon></el-button>
        <h1 class="page-title">MistDocs</h1>
      </div>
      <div class="nav-center">
        <div class="search-box" :class="{ focused: searchFocused }">
          <el-icon class="search-icon"><Search /></el-icon>
          <input v-model="search" :placeholder="searchFocused ? '搜索文档标题...' : '搜索'" class="search-input"
            @focus="searchFocused = true" @blur="searchFocused = false" @keyup.enter="doSearch" />
          <el-icon v-if="search" class="search-clear" @click="clearSearch"><Close /></el-icon>
        </div>
      </div>
      <div class="nav-right">
        <el-button type="primary" @click="showNewDoc = true"><el-icon><Plus /></el-icon><span>新建文档</span></el-button>
        <el-dropdown @command="handleNewCommand" trigger="click">
          <el-button class="more-create-btn"><el-icon><ArrowDown /></el-icon></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="doc"><el-icon><Document /></el-icon> 新建文档</el-dropdown-item>
              <el-dropdown-item command="sheet"><el-icon><Grid /></el-icon> 新建表格</el-dropdown-item>
              <el-dropdown-item command="folder"><el-icon><FolderAdd /></el-icon> 新建文件夹</el-dropdown-item>
              <el-dropdown-item command="import" divided><el-icon><Upload /></el-icon> 导入文件</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <transition name="slide-down">
      <div v-if="selectedDocs.length" class="batch-bar">
        <div class="batch-info">
          <el-button size="small" text @click="selectedDocs = []"><el-icon><Close /></el-icon></el-button>
          <span>已选 <strong>{{ selectedDocs.length }}</strong> 项</span>
        </div>
        <div class="batch-actions">
          <el-button size="small" @click="showBatchMove = true"><el-icon><Rank /></el-icon> 移动</el-button>
          <el-button size="small" @click="batchExport"><el-icon><Download /></el-icon> 导出</el-button>
          <el-button size="small" type="danger" plain @click="batchDelete"><el-icon><Delete /></el-icon> 删除</el-button>
        </div>
      </div>
    </transition>

    <div class="content">
      <div class="sidebar-overlay" :class="{ open: sidebarOpen }" @click="sidebarOpen = false"></div>
      <div class="sidebar" :class="{ open: sidebarOpen }">
        <div class="sidebar-inner">
          <div class="nav-section">
            <div class="nav-item" :class="{ active: viewMode === 'all' }" @click="switchView('all')">
              <svg class="nav-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v12a1 1 0 01-1 1H4a1 1 0 01-1-1V4zm2 1v10h10V5H5z"/></svg>
              <span>全部文档</span>
              <span class="nav-count" v-if="viewMode === 'all'">{{ docs.length }}</span>
            </div>
            <div class="nav-item" :class="{ active: viewMode === 'recent' }" @click="switchView('recent')">
              <svg class="nav-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a8 8 0 100 16 8 8 0 000-16zm1 8.414l-3.293 3.293-1.414-1.414L9 9.586V5h2v5.414z"/></svg>
              <span>最近打开</span>
            </div>
            <div class="nav-item" :class="{ active: viewMode === 'favorites' }" @click="switchView('favorites')">
              <svg class="nav-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.286 3.957a1 1 0 00.95.69h4.162c.969 0 1.371 1.24.588 1.81l-3.37 2.448a1 1 0 00-.364 1.118l1.287 3.957c.3.921-.755 1.688-1.54 1.118l-3.37-2.448a1 1 0 00-1.176 0l-3.37 2.448c-.784.57-1.838-.197-1.54-1.118l1.287-3.957a1 1 0 00-.364-1.118L2.063 9.384c-.783-.57-.38-1.81.588-1.81h4.162a1 1 0 00.95-.69l1.286-3.957z"/></svg>
              <span>我的收藏</span>
            </div>
          </div>
          <div v-if="sidebarTags.length" class="sidebar-group">
            <div class="group-header" @click="tagsExpanded = !tagsExpanded">
              <svg class="chevron" :class="{ rotated: tagsExpanded }" viewBox="0 0 20 20" fill="currentColor"><path d="M6 4l8 6-8 6V4z"/></svg>
              <span>标签</span>
            </div>
            <div v-show="tagsExpanded" class="group-content">
              <div v-for="tag in sidebarTags" :key="tag.id" class="tag-item" @click="filterByTag(tag.id)">
                <span class="tag-dot" :style="{ background: tag.color }"></span>
                <span class="tag-name">{{ tag.name }}</span>
                <span class="tag-count">{{ tag.doc_count }}</span>
              </div>
            </div>
          </div>
          <div class="sidebar-group">
            <div class="group-header" @click="foldersExpanded = !foldersExpanded">
              <svg class="chevron" :class="{ rotated: foldersExpanded }" viewBox="0 0 20 20" fill="currentColor"><path d="M6 4l8 6-8 6V4z"/></svg>
              <span>文件夹</span>
              <el-button size="small" text class="group-action" @click.stop="showNewFolder = true"><el-icon :size="14"><Plus /></el-icon></el-button>
            </div>
            <div v-show="foldersExpanded" class="group-content">
              <el-tree :data="treeData" :props="{ label: 'name', children: 'children' }" node-key="id"
                highlight-current default-expand-all :indent="12" @node-click="onFolderClick">
                <template #default="{ data }">
                  <div class="tree-node" :class="{ active: currentFolder === data.id }">
                    <svg class="tree-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                    <span class="tree-label">{{ data.name }}</span>
                  </div>
                </template>
              </el-tree>
            </div>
          </div>
        </div>
      </div>

      <div class="main-content">
        <div class="content-header">
          <div class="header-left">
            <h2 class="view-title">
              <template v-if="searchMode">搜索「{{ search }}」</template>
              <template v-else-if="viewMode === 'recent'">最近打开</template>
              <template v-else-if="viewMode === 'favorites'">我的收藏</template>
              <template v-else-if="currentFolder">文件夹</template>
              <template v-else>全部文档</template>
            </h2>
            <span class="doc-count" v-if="!loading">{{ docs.length }} 个</span>
          </div>
          <div class="header-right">
            <el-select v-model="sortBy" size="small" class="sort-select" @change="sortDocs">
              <el-option label="最近更新" value="updated" />
              <el-option label="最近创建" value="created" />
              <el-option label="标题排序" value="title" />
            </el-select>
            <div class="view-toggle">
              <button class="toggle-btn" :class="{ active: layoutMode === 'grid' }" @click="layoutMode = 'grid'" title="网格视图">
                <svg viewBox="0 0 20 20" fill="currentColor"><path d="M5 3a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2V5a2 2 0 00-2-2H5zm8 0a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2V5a2 2 0 00-2-2h-2zm-8 8a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2v-2a2 2 0 00-2-2H5zm8 0a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2v-2a2 2 0 00-2-2h-2z"/></svg>
              </button>
              <button class="toggle-btn" :class="{ active: layoutMode === 'list' }" @click="layoutMode = 'list'" title="列表视图">
                <svg viewBox="0 0 20 20" fill="currentColor"><path d="M2 4.75A.75.75 0 012.75 4h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 4.75zm0 5A.75.75 0 012.75 9h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 9.75zm0 5a.75.75 0 01.75-.75h14.5a.75.75 0 010 1.5H2.75a.75.75 0 01-.75-.75z"/></svg>
              </button>
            </div>
          </div>
        </div>

        <div v-if="loading" class="loading-state">
          <div v-for="i in 6" :key="i" class="skeleton-card">
            <div class="sk-preview"></div>
            <div class="sk-lines"><div class="sk-line sk-title"></div><div class="sk-line sk-meta"></div></div>
          </div>
        </div>

        <div v-else-if="!docs.length" class="empty-state">
          <svg class="empty-svg" viewBox="0 0 200 160" fill="none">
            <rect x="30" y="20" width="140" height="120" rx="8" fill="#f0f2f5" stroke="#dcdfe6" stroke-width="1.5"/>
            <rect x="45" y="38" width="70" height="6" rx="3" fill="#dcdfe6"/>
            <rect x="45" y="52" width="110" height="4" rx="2" fill="#e8e8e8"/>
            <rect x="45" y="62" width="90" height="4" rx="2" fill="#e8e8e8"/>
            <rect x="45" y="72" width="100" height="4" rx="2" fill="#e8e8e8"/>
            <circle cx="155" cy="105" r="25" fill="#ecf5ff" stroke="#b3d8ff" stroke-width="1.5"/>
            <path d="M147 105h16M155 97v16" stroke="#409eff" stroke-width="2" stroke-linecap="round"/>
          </svg>
          <p class="empty-title">
            <template v-if="searchMode">没有找到匹配的文档</template>
            <template v-else-if="viewMode === 'recent'">还没有打开过文档</template>
            <template v-else-if="viewMode === 'favorites'">还没有收藏文档</template>
            <template v-else>还没有文档</template>
          </p>
          <p class="empty-desc" v-if="!searchMode">点击「新建文档」开始创建</p>
          <el-button v-if="viewMode === 'all' && !searchMode" type="primary" @click="showNewDoc = true"><el-icon><Plus /></el-icon> 新建文档</el-button>
        </div>

        <div v-else-if="layoutMode === 'grid'" class="doc-grid">
          <div v-for="doc in docs" :key="doc.id" class="doc-card"
            :class="{ selected: selectedDocs.includes(doc.id) }"
            @click="openDoc(doc)" @contextmenu.prevent="showContextMenu($event, doc)">
            <div class="card-preview" :class="doc.type === 'sheet' ? 'preview-sheet' : 'preview-doc'">
              <div class="preview-inner">
                <template v-if="doc.type === 'sheet'">
                  <div class="sheet-lines"><div class="sheet-row" v-for="r in 4" :key="r">
                    <div class="sheet-cell" v-for="c in 3" :key="c" :style="{ width: (25+c*12)+'px' }"></div>
                  </div></div>
                </template>
                <template v-else>
                  <div class="doc-lines">
                    <div class="doc-line" style="width:60%"></div>
                    <div class="doc-line" style="width:90%"></div>
                    <div class="doc-line" style="width:75%"></div>
                    <div class="doc-line" style="width:85%"></div>
                  </div>
                </template>
              </div>
            </div>
            <div class="card-body">
              <div class="card-title-row">
                <div class="card-title">{{ doc.title }}</div>
                <el-dropdown trigger="click" @command="(cmd: string) => handleDocAction(cmd, doc)">
                  <el-icon class="card-menu" @click.stop><MoreFilled /></el-icon>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="open">打开</el-dropdown-item>
                      <el-dropdown-item command="rename">重命名</el-dropdown-item>
                      <el-dropdown-item command="move">移动到...</el-dropdown-item>
                      <el-dropdown-item command="favorite">{{ doc.is_favorite ? '取消收藏' : '收藏' }}</el-dropdown-item>
                      <el-dropdown-item command="delete" divided><span style="color:#f56c6c">删除</span></el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
              <div class="card-meta">
                <el-tag :type="doc.type === 'doc' ? 'info' : 'success'" size="small" effect="plain" round>{{ doc.type === 'doc' ? '文档' : '表格' }}</el-tag>
                <span class="card-version">v{{ doc.version }}</span>
                <span class="card-spacer"></span>
                <el-icon class="card-fav" :class="{ active: doc.is_favorite }" @click.stop="toggleFavorite(doc)">
                  <svg viewBox="0 0 20 20" fill="currentColor"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.286 3.957a1 1 0 00.95.69h4.162c.969 0 1.371 1.24.588 1.81l-3.37 2.448a1 1 0 00-.364 1.118l1.287 3.957c.3.921-.755 1.688-1.54 1.118l-3.37-2.448a1 1 0 00-1.176 0l-3.37 2.448c-.784.57-1.838-.197-1.54-1.118l1.287-3.957a1 1 0 00-.364-1.118L2.063 9.384c-.783-.57-.38-1.81.588-1.81h4.162a1 1 0 00.95-.69l1.286-3.957z"/></svg>
                </el-icon>
              </div>
              <div class="card-time">{{ formatTime(doc.updated_at) }}</div>
            </div>
            <div class="card-check" @click.stop="toggleSelect(doc.id)"><el-checkbox :model-value="selectedDocs.includes(doc.id)" /></div>
          </div>
        </div>

        <div v-else class="doc-table">
          <div class="table-row table-header-row">
            <div class="table-col col-check"></div>
            <div class="table-col col-title">标题</div>
            <div class="table-col col-type">类型</div>
            <div class="table-col col-version">版本</div>
            <div class="table-col col-time">更新时间</div>
            <div class="table-col col-actions"></div>
          </div>
          <div v-for="doc in docs" :key="doc.id" class="table-row"
            :class="{ selected: selectedDocs.includes(doc.id) }"
            @click="openDoc(doc)" @contextmenu.prevent="showContextMenu($event, doc)">
            <div class="table-col col-check" @click.stop="toggleSelect(doc.id)"><el-checkbox :model-value="selectedDocs.includes(doc.id)" /></div>
            <div class="table-col col-title">
              <div class="title-icon" :class="doc.type">
                <svg v-if="doc.type==='doc'" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L2.414 12.586A2 2 0 014 12h4.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg>
                <svg v-else viewBox="0 0 20 20" fill="currentColor"><path d="M3 3h14a1 1 0 011 1v12a1 1 0 01-1 1H3a1 1 0 01-1-1V4a1 1 0 011-1zm1 2v2h4V5H4zm6 0v2h4V5h-4zm-6 4v2h4V9H4zm6 0v2h4V9h-4z"/></svg>
              </div>
              <span class="title-text">{{ doc.title }}</span>
            </div>
            <div class="table-col col-type"><el-tag :type="doc.type==='doc'?'info':'success'" size="small" effect="plain" round>{{ doc.type === 'doc' ? '文档' : '表格' }}</el-tag></div>
            <div class="table-col col-version">v{{ doc.version }}</div>
            <div class="table-col col-time">{{ formatTime(doc.updated_at) }}</div>
            <div class="table-col col-actions" @click.stop>
              <el-icon class="row-fav" :class="{ active: doc.is_favorite }" @click="toggleFavorite(doc)">
                <svg viewBox="0 0 20 20" fill="currentColor"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.286 3.957a1 1 0 00.95.69h4.162c.969 0 1.371 1.24.588 1.81l-3.37 2.448a1 1 0 00-.364 1.118l1.287 3.957c.3.921-.755 1.688-1.54 1.118l-3.37-2.448a1 1 0 00-1.176 0l-3.37 2.448c-.784.57-1.838-.197-1.54-1.118l1.287-3.957a1 1 0 00-.364-1.118L2.063 9.384c-.783-.57-.38-1.81.588-1.81h4.162a1 1 0 00.95-.69l1.286-3.957z"/></svg>
              </el-icon>
              <el-dropdown trigger="click" @command="(cmd: string) => handleDocAction(cmd, doc)">
                <el-icon class="row-more"><MoreFilled /></el-icon>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="open">打开</el-dropdown-item>
                    <el-dropdown-item command="rename">重命名</el-dropdown-item>
                    <el-dropdown-item command="move">移动到...</el-dropdown-item>
                    <el-dropdown-item command="favorite">{{ doc.is_favorite ? '取消收藏' : '收藏' }}</el-dropdown-item>
                    <el-dropdown-item command="delete" divided><span style="color:#f56c6c">删除</span></el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="contextMenu.show" class="context-menu" :style="{ left: contextMenu.x+'px', top: contextMenu.y+'px' }" @click.stop>
      <div class="ctx-item" @click="handleDocAction('open', contextMenu.doc); contextMenu.show = false">打开</div>
      <div class="ctx-item" @click="handleDocAction('rename', contextMenu.doc); contextMenu.show = false">重命名</div>
      <div class="ctx-item" @click="handleDocAction('move', contextMenu.doc); contextMenu.show = false">移动到...</div>
      <div class="ctx-item" @click="handleDocAction('favorite', contextMenu.doc); contextMenu.show = false">{{ contextMenu.doc?.is_favorite ? '取消收藏' : '收藏' }}</div>
      <div class="ctx-sep"></div>
      <div class="ctx-item danger" @click="handleDocAction('delete', contextMenu.doc); contextMenu.show = false">删除</div>
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

    <!-- 批量导入 -->
    <el-dialog v-model="showImportDialog" title="批量导入" width="500">
      <p style="color:#999;font-size:13px;margin-bottom:12px">支持 .txt、.md、.html、.docx、.xlsx 文件，最多20个，每个不超过10MB</p>
      <el-upload
        ref="importUpload"
        :auto-upload="false"
        :limit="20"
        multiple
        accept=".txt,.md,.html,.htm,.docx,.xlsx"
        :on-change="onImportFileChange"
      >
        <el-button type="primary" size="small">选择文件</el-button>
      </el-upload>
      <template #footer>
        <el-button @click="showImportDialog = false">取消</el-button>
        <el-button type="primary" @click="doImport" :loading="importing" :disabled="!importFiles.length">
          导入 {{ importFiles.length ? importFiles.length + ' 个文件' : '' }}
        </el-button>
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
import { Star, StarFilled, Clock, Files, MoreFilled, Operation, Monitor, List } from '@element-plus/icons-vue'
import { Close, Plus, ArrowDown, Delete, Download, Rank, FolderAdd, Search } from '@element-plus/icons-vue'
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
const favoriteIds = ref<Set<string>>(new Set())
const sidebarOpen = ref(false)
const searchFocused = ref(false)
const tagsExpanded = ref(true)
const foldersExpanded = ref(true)

const showNewFolder = ref(false)
const showNewDoc = ref(false)
const showNewSheet = ref(false)
const showImportDialog = ref(false)
const importFiles = ref<any[]>([])
const importing = ref(false)

function onImportFileChange(file: any, fileList: any[]) {
  importFiles.value = fileList
}

async function doImport() {
  if (!importFiles.value.length) return
  importing.value = true
  const fd = new FormData()
  for (const f of importFiles.value) {
    fd.append('files', f.raw)
  }
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
const renameDialog = ref(false)
const renameTitle = ref('')
const renameDoc = ref<any>(null)
const moveDialog = ref(false)
const moveDoc = ref<any>(null)
const moveTargetFolder = ref('')
const newFolderName = ref('')
const newDocTitle = ref('')
const newDocTemplate = ref('')
const layoutMode = ref<'grid'|'list'>('grid')
const contextMenu = ref<{ show: boolean; x: number; y: number; doc: any }>({ show: false, x: 0, y: 0, doc: null })
const sortBy = ref('updated')

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

function setDocs(list: any[]) {
  docs.value = list.map((d: any) => ({
    ...d,
    is_favorite: favoriteIds.value.has(d.id),
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
  treeData.value = buildTree(data.data || [])
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
  if (!search.value) return
  searchMode.value = true
  const { data } = await http.get('/docs/documents/search', { params: { q: search.value } })
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


function closeContextMenu() {
  contextMenu.value.show = false
}

function showContextMenu(e: MouseEvent, doc: any) {
  contextMenu.value = { show: true, x: e.clientX, y: e.clientY, doc }
  if (e.clientY + 200 > window.innerHeight) contextMenu.value.y = e.clientY - 200
  if (e.clientX + 160 > window.innerWidth) contextMenu.value.x = e.clientX - 160
}

function handleDocAction(cmd: string, doc: any) {
  if (!doc) return
  switch (cmd) {
    case 'open': openDoc(doc); break
    case 'rename': showRename(doc); break
    case 'move': showMove(doc); break
    case 'favorite': toggleFavorite(doc); break
    case 'delete': deleteDoc(doc); break
  }
}

function handleNewCommand(cmd: string) {
  switch (cmd) {
    case 'doc': showNewDoc.value = true; break
    case 'sheet': showNewSheet.value = true; break
    case 'folder': showNewFolder.value = true; break
    case 'import': showImportDialog.value = true; break
  }
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
    setDocs(data || [])
  } catch {}
}
</script>
<style scoped>
/* ── Page layout ── */
.docs-page { height: 100%; display: flex; flex-direction: column; background: #f8f9fb; }

/* ── Top navigation bar ── */
.top-nav {
  display: flex; align-items: center; height: 56px; padding: 0 20px;
  background: #fff; border-bottom: 1px solid #e8ecf0; flex-shrink: 0; gap: 16px;
}
.nav-left { display: flex; align-items: center; gap: 8px; }
.page-title { font-size: 18px; font-weight: 700; color: #1a1a2e; letter-spacing: -0.5px; margin: 0; }
.nav-center { flex: 1; display: flex; justify-content: center; }
.search-box {
  display: flex; align-items: center; gap: 6px; background: #f0f2f5;
  border-radius: 8px; padding: 0 12px; height: 36px; width: 320px;
  border: 2px solid transparent; transition: all 0.2s;
}
.search-box.focused { background: #fff; border-color: #409eff; box-shadow: 0 0 0 3px rgba(64,158,255,0.1); }
.search-icon { color: #999; font-size: 16px; }
.search-input { border: none; outline: none; background: transparent; flex: 1; font-size: 14px; color: #333; }
.search-input::placeholder { color: #aaa; }
.search-clear { color: #999; cursor: pointer; font-size: 14px; }
.search-clear:hover { color: #666; }
.nav-right { display: flex; align-items: center; gap: 6px; }
.more-create-btn { padding: 8px !important; }

/* ── Batch bar ── */
.batch-bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 20px; background: #ecf5ff; border-bottom: 1px solid #d9ecff;
}
.batch-info { display: flex; align-items: center; gap: 8px; font-size: 14px; color: #409eff; }
.batch-actions { display: flex; gap: 8px; }
.slide-down-enter-active, .slide-down-leave-active { transition: all 0.2s ease; }
.slide-down-enter-from, .slide-down-leave-to { transform: translateY(-100%); opacity: 0; }

/* ── Content layout ── */
.content { flex: 1; display: flex; overflow: hidden; }

/* ── Sidebar ── */
.sidebar {
  width: 240px; background: #fff; border-right: 1px solid #e8ecf0;
  overflow-y: auto; flex-shrink: 0;
}
.sidebar-inner { padding: 8px 0; }

.nav-section { padding: 4px 0; }
.nav-item {
  display: flex; align-items: center; gap: 10px; padding: 8px 16px;
  cursor: pointer; font-size: 14px; color: #555; transition: all 0.15s; border-radius: 0;
  position: relative;
}
.nav-item:hover { background: #f5f7fa; color: #1a1a2e; }
.nav-item.active { background: #e8f0fe; color: #1a73e8; }
.nav-item.active::before {
  content: ''; position: absolute; left: 0; top: 4px; bottom: 4px; width: 3px;
  background: #409eff; border-radius: 0 2px 2px 0;
}
.nav-icon { width: 18px; height: 18px; flex-shrink: 0; }
.nav-count {
  margin-left: auto; font-size: 11px; background: #f0f2f5; color: #999;
  padding: 1px 8px; border-radius: 10px;
}

.sidebar-group { border-top: 1px solid #f0f2f5; margin-top: 4px; padding-top: 4px; }
.group-header {
  display: flex; align-items: center; gap: 6px; padding: 8px 16px;
  font-size: 12px; font-weight: 600; color: #909399; text-transform: uppercase;
  letter-spacing: 0.5px; cursor: pointer; user-select: none;
}
.group-header:hover { color: #666; }
.chevron { width: 12px; height: 12px; transition: transform 0.2s; }
.chevron.rotated { transform: rotate(90deg); }
.group-action { margin-left: auto; padding: 0 !important; color: #909399 !important; }
.group-action:hover { color: #409eff !important; }

.group-content { padding: 2px 0; }
.tag-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 16px 6px 28px;
  font-size: 13px; color: #555; cursor: pointer; transition: background 0.15s;
}
.tag-item:hover { background: #f5f7fa; }
.tag-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.tag-name { flex: 1; }
.tag-count { font-size: 11px; color: #bbb; }

.tree-node {
  display: flex; align-items: center; gap: 6px; font-size: 13px; color: #555;
  padding: 2px 0; transition: color 0.15s;
}
.tree-node:hover { color: #1a1a2e; }
.tree-node.active { color: #409eff; font-weight: 500; }
.tree-icon { width: 16px; height: 16px; flex-shrink: 0; }
.tree-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* ── Main content ── */
.main-content { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 20px 24px; }

.content-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.header-left { display: flex; align-items: baseline; gap: 8px; }
.view-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }
.doc-count { font-size: 13px; color: #999; }
.header-right { display: flex; align-items: center; gap: 10px; }
.sort-select { width: 120px; }
.sort-select :deep(.el-input__wrapper) { box-shadow: none; background: #fff; border: 1px solid #e8ecf0; border-radius: 6px; }

.view-toggle { display: flex; border: 1px solid #e8ecf0; border-radius: 6px; overflow: hidden; background: #fff; }
.toggle-btn {
  display: flex; align-items: center; justify-content: center; width: 32px; height: 28px;
  border: none; background: transparent; cursor: pointer; color: #999; transition: all 0.15s;
}
.toggle-btn:hover { color: #666; background: #f5f7fa; }
.toggle-btn.active { background: #e8f0fe; color: #409eff; }
.toggle-btn svg { width: 16px; height: 16px; }

/* ── Loading skeleton ── */
.loading-state {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 16px;
}
.skeleton-card {
  border-radius: 10px; overflow: hidden; background: #fff; border: 1px solid #e8ecf0;
}
.sk-preview { height: 100px; background: linear-gradient(90deg, #f0f2f5 25%, #e8ecf0 50%, #f0f2f5 75%); background-size: 200% 100%; animation: shimmer 1.5s infinite; }
.sk-lines { padding: 12px; }
.sk-line { height: 10px; border-radius: 5px; background: #f0f2f5; margin-bottom: 8px; }
.sk-title { width: 70%; }
.sk-meta { width: 40%; }
@keyframes shimmer { 0% { background-position: 200% 0; } 100% { background-position: -200% 0; } }

/* ── Empty state ── */
.empty-state {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  flex: 1; padding: 60px 20px;
}
.empty-svg { width: 200px; height: 160px; margin-bottom: 20px; }
.empty-title { font-size: 16px; color: #555; margin: 0 0 4px; }
.empty-desc { font-size: 13px; color: #999; margin: 0 0 20px; }

/* ── Grid view ── */
.doc-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 16px; flex: 1; overflow-y: auto; align-content: start;
}
.doc-card {
  background: #fff; border: 1px solid #e8ecf0; border-radius: 10px;
  cursor: pointer; transition: all 0.2s; position: relative; overflow: hidden;
}
.doc-card:hover { border-color: #c8d8e8; box-shadow: 0 4px 16px rgba(0,0,0,0.06); transform: translateY(-2px); }
.doc-card.selected { border-color: #409eff; background: #f0f7ff; }

.card-preview { height: 100px; padding: 16px; }
.preview-doc .preview-inner { padding: 4px; }
.doc-lines { display: flex; flex-direction: column; gap: 6px; }
.doc-line { height: 4px; border-radius: 2px; background: #e8ecf0; }
.preview-sheet .preview-inner { padding: 4px; }
.sheet-lines { display: flex; flex-direction: column; gap: 4px; }
.sheet-row { display: flex; gap: 4px; }
.sheet-cell { height: 8px; border-radius: 2px; background: #e8ecf0; }

.card-body { padding: 0 14px 14px; }
.card-title-row { display: flex; align-items: center; gap: 4px; margin-bottom: 6px; }
.card-title { font-size: 14px; font-weight: 600; color: #1a1a2e; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.card-menu { color: #ccc; cursor: pointer; padding: 2px; border-radius: 4px; transition: all 0.15s; }
.card-menu:hover { color: #666; background: #f0f2f5; }
.card-meta { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
.card-version { font-size: 11px; color: #bbb; }
.card-spacer { flex: 1; }
.card-fav { color: #ddd; cursor: pointer; transition: color 0.15s; font-size: 14px; }
.card-fav:hover { color: #f7ba2a; }
.card-fav.active { color: #f7ba2a; }
.card-time { font-size: 12px; color: #bbb; }
.card-check {
  position: absolute; top: 6px; left: 6px; z-index: 1; opacity: 0; transition: opacity 0.15s;
}
.doc-card:hover .card-check, .doc-card.selected .card-check { opacity: 1; }

/* ── Table / List view ── */
.doc-table { flex: 1; overflow-y: auto; background: #fff; border-radius: 10px; border: 1px solid #e8ecf0; }
.table-row {
  display: flex; align-items: center; padding: 0 16px; height: 44px;
  border-bottom: 1px solid #f0f2f5; transition: background 0.15s; cursor: pointer;
}
.table-row:last-child { border-bottom: none; }
.table-row:hover { background: #f8f9fb; }
.table-row.selected { background: #f0f7ff; }
.table-header-row {
  background: #fafbfc; font-size: 12px; font-weight: 600; color: #909399;
  cursor: default; border-bottom: 1px solid #e8ecf0;
}
.table-header-row:hover { background: #fafbfc; }
.table-col { flex-shrink: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.col-check { width: 40px; }
.col-title { flex: 1; min-width: 0; display: flex; align-items: center; gap: 8px; font-size: 14px; color: #333; }
.col-type { width: 72px; text-align: center; }
.col-version { width: 52px; text-align: center; color: #999; font-size: 12px; }
.col-time { width: 120px; color: #999; font-size: 13px; }
.col-actions { width: 60px; display: flex; align-items: center; gap: 4px; justify-content: flex-end; }

.title-icon { width: 28px; height: 28px; border-radius: 6px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.title-icon svg { width: 16px; height: 16px; }
.title-icon.doc { background: #ecf5ff; color: #409eff; }
.title-icon.sheet { background: #f0f9eb; color: #67c23a; }
.title-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.row-fav { color: #ddd; cursor: pointer; transition: color 0.15s; font-size: 14px; }
.row-fav:hover { color: #f7ba2a; }
.row-fav.active { color: #f7ba2a; }
.row-more { color: #ccc; cursor: pointer; transition: color 0.15s; }
.row-more:hover { color: #666; }

/* ── Context menu ── */
.context-menu {
  position: fixed; z-index: 300; background: #fff; border-radius: 8px;
  border: 1px solid #e8ecf0; box-shadow: 0 8px 30px rgba(0,0,0,0.12);
  padding: 4px 0; min-width: 140px;
}
.ctx-item {
  padding: 8px 16px; font-size: 13px; color: #555; cursor: pointer; transition: all 0.1s;
}
.ctx-item:hover { background: #f5f7fa; color: #1a1a2e; }
.ctx-item.danger { color: #f56c6c; }
.ctx-item.danger:hover { background: #fef0f0; }
.ctx-sep { height: 1px; background: #f0f2f5; margin: 4px 0; }

/* ── Template grid ── */
.template-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.tpl-card {
  border: 2px solid #e8ecf0; border-radius: 10px; padding: 14px 8px;
  text-align: center; cursor: pointer; transition: all 0.2s;
}
.tpl-card:hover { border-color: #b3d8ff; background: #f5f7fa; }
.tpl-card.active { border-color: #409eff; background: #ecf5ff; }
.tpl-icon { width: 24px; height: 24px; margin: 0 auto 6px; color: #666; }
.tpl-card.active .tpl-icon { color: #409eff; }

/* ── Mobile ── */
.menu-btn { display: none !important; }
.sidebar-overlay { display: none; }

@media (max-width: 768px) {
  .menu-btn { display: inline-flex !important; }
  .page-title { display: none; }
  .search-box { width: 100%; }
  .nav-right span { display: none; }

  .sidebar-overlay.open {
    display: block; position: fixed; top: 0; left: 0;
    width: 100vw; height: 100vh; background: rgba(0,0,0,0.3); z-index: 199;
  }
  .sidebar {
    position: fixed; top: 0; left: 0; width: 280px; height: 100vh;
    z-index: 200; transform: translateX(-100%); transition: transform 0.25s ease;
    border-right: 1px solid #e8ecf0;
  }
  .sidebar.open { transform: translateX(0); box-shadow: 4px 0 20px rgba(0,0,0,0.1); }

  .main-content { padding: 12px; }
  .content-header { flex-wrap: wrap; gap: 8px; }
  .doc-grid { grid-template-columns: 1fr; }
  .card-check { opacity: 1; }
}
</style>
