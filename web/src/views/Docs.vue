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
      <el-input v-model="search" placeholder="搜索文档..." style="width:240px" clearable>
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>

    <div class="content">
      <!-- 文件夹树 -->
      <div class="folder-tree">
        <div class="tree-header">文件夹</div>
        <el-tree
          :data="treeData"
          :props="{ label: 'name', children: 'children' }"
          node-key="id"
          highlight-current
          default-expand-all
          @node-click="onFolderClick"
        >
          <template #default="{ node, data }">
            <span class="tree-node">
              <el-icon><Folder /></el-icon>
              <span>{{ data.name }}</span>
            </span>
          </template>
        </el-tree>
      </div>

      <!-- 文档列表 -->
      <div class="doc-list">
        <el-table :data="filteredDocs" stripe @row-click="openDoc" style="cursor:pointer">
          <el-table-column prop="title" label="名称" min-width="300">
            <template #default="{ row }">
              <div class="doc-title">
                <el-icon :size="18">
                  <Document v-if="row.type === 'doc'" />
                  <Grid v-else />
                </el-icon>
                <span>{{ row.title }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="type" label="类型" width="80">
            <template #default="{ row }">
              <el-tag :type="row.type === 'doc' ? '' : 'success'" size="small">
                {{ row.type === 'doc' ? '文档' : '表格' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="version" label="版本" width="70" />
          <el-table-column prop="updated_at" label="更新时间" width="170" />
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button link type="danger" size="small" @click.stop="deleteDoc(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
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
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import http from '@/utils/http'

const router = useRouter()
const treeData = ref<any[]>([])
const docs = ref<any[]>([])
const currentFolder = ref<string | null>(null)
const search = ref('')

const showNewFolder = ref(false)
const showNewDoc = ref(false)
const showNewSheet = ref(false)
const newFolderName = ref('')
const newDocTitle = ref('')

const filteredDocs = computed(() => {
  let list = docs.value
  if (search.value) {
    list = list.filter((d: any) => d.title.includes(search.value))
  }
  return list
})

async function loadTree() {
  const { data } = await http.get('/docs/tree')
  treeData.value = buildTree(data.data || [])
}

async function loadDocs(folderId?: string) {
  const params: any = {}
  if (folderId) params.folder_id = folderId
  else if (currentFolder.value) params.folder_id = currentFolder.value
  const { data } = await http.get('/docs/documents', { params })
  docs.value = data.data || []
}

function onFolderClick(node: any) {
  currentFolder.value = node.id
  loadDocs(node.id)
}

function buildTree(items: any[]): any[] {
  const map: any = {}
  const roots: any[] = []
  items.forEach((item: any) => {
    map[item.id] = { ...item, children: [] }
  })
  items.forEach((item: any) => {
    if (item.parent_id && map[item.parent_id]) {
      map[item.parent_id].children.push(map[item.id])
    } else {
      roots.push(map[item.id])
    }
  })
  return roots
}

async function createFolder() {
  if (!newFolderName.value) return
  await http.post('/docs/folders', {
    name: newFolderName.value,
    parent_id: currentFolder.value,
  })
  ElMessage.success('文件夹已创建')
  showNewFolder.value = false
  newFolderName.value = ''
  loadTree()
}

async function createDoc(type: string) {
  if (!newDocTitle.value) return
  const { data } = await http.post('/docs/documents', {
    title: newDocTitle.value,
    type,
    folder_id: currentFolder.value,
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
  loadDocs()
}

function openDoc(row: any) {
  router.push(`/docs/${row.id}`)
}

onMounted(() => {
  loadTree()
  loadDocs()
})
</script>

<style scoped>
.docs-page { height: 100%; display: flex; flex-direction: column; }
.toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.content { flex: 1; display: flex; gap: 16px; overflow: hidden; }
.folder-tree {
  width: 240px;
  border: 1px solid #e8e8e8;
  border-radius: 6px;
  overflow-y: auto;
  background: #fafafa;
}
.tree-header {
  padding: 10px 16px;
  font-weight: bold;
  border-bottom: 1px solid #e8e8e8;
  background: #f5f5f5;
}
.doc-list { flex: 1; }
.doc-title { display: flex; align-items: center; gap: 8px; }
.tree-node { display: flex; align-items: center; gap: 6px; font-size: 14px; }
</style>
