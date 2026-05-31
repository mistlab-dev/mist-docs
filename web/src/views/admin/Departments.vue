<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">部门管理</h2>
        <span class="header-count">共 {{ flatDepts.length }} 个部门</span>
      </div>
      <div class="header-actions">
        <el-input v-model="searchKey" placeholder="搜索部门..." size="default" class="search-input" clearable>
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button size="default" @click="showImport = true">
          <el-icon><Upload /></el-icon>
        </el-button>
        <el-button type="primary" size="default" @click="openForm()">
          <el-icon><Plus /></el-icon> 新建
        </el-button>
      </div>
    </div>

    <div class="table-card">
      <el-table
        :data="filteredDepts"
        row-key="id"
        :tree-props="{ children: 'children' }"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
        :cell-style="{ fontSize: '14px' }"
        default-expand-all
      >
        <el-table-column label="部门名称" min-width="280">
          <template #default="{ row }">
            <div class="dept-name">
              <div class="dept-icon" :style="{ background: deptColor(row.id) }">
                <svg viewBox="0 0 20 20" fill="currentColor"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
              </div>
              <span class="dept-label">{{ row.name }}</span>
              <el-tag v-if="row.children?.length" size="small" type="info" effect="plain" round>
                {{ row.children.length }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.status === 1"
              size="small"
              inline-prompt
              active-text="启"
              inactive-text="停"
              @change="toggleStatus(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="排序" width="80" align="center">
          <template #default="{ row }">
            <span class="sort-text">{{ row.sort_order }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right" align="center">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="(cmd: string) => handleCmd(cmd, row)">
              <el-button link size="small" class="more-btn">
                <el-icon><MoreFilled /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit">
                    <el-icon><Edit /></el-icon> 编辑
                  </el-dropdown-item>
                  <el-dropdown-item command="addSub">
                    <el-icon><Plus /></el-icon> 添加子部门
                  </el-dropdown-item>
                  <el-dropdown-item command="delete" divided>
                    <span style="color:#f56c6c"><el-icon><Delete /></el-icon> 删除</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showForm" :title="editing ? '编辑部门' : '新建部门'" width="440" destroy-on-close>
      <el-form :model="form" label-position="top" class="dept-form">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="部门名称" />
        </el-form-item>
        <el-form-item label="上级部门">
          <el-tree-select
            v-model="form.parent_id"
            :data="deptTree"
            check-strictly
            :props="{ label: 'name', value: 'id' }"
            clearable
            placeholder="无（顶级部门）"
            class="full-width"
          />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForm = false">取消</el-button>
        <el-button type="primary" @click="submit">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showImport" title="批量导入部门" width="500">
      <p style="color:#999;font-size:13px;margin-bottom:8px">CSV 格式：name,parent_name,sort_order</p>
      <el-input v-model="csvData" type="textarea" :rows="6" placeholder="粘贴 CSV 数据" />
      <template #footer>
        <el-button @click="showImport = false">取消</el-button>
        <el-button type="primary" @click="doImport">导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import http from '@/utils/http'

const flatDepts = ref<any[]>([])
const deptTree = ref<any[]>([])
const showForm = ref(false)
const showImport = ref(false)
const editing = ref<any>(null)
const form = ref<any>({ name: '', parent_id: '', sort_order: 0 })
const csvData = ref('')
const searchKey = ref('')

const filteredDepts = computed(() => {
  if (!searchKey.value) return flatDepts.value
  const k = searchKey.value.toLowerCase()
  function filterTree(nodes: any[]): any[] {
    return nodes.reduce((acc: any[], node: any) => {
      const match = node.name?.toLowerCase().includes(k)
      const filteredChildren = filterTree(node.children || [])
      if (match || filteredChildren.length) {
        acc.push({ ...node, children: filteredChildren })
      }
      return acc
    }, [])
  }
  return filterTree(flatDepts.value)
})

function deptColor(id: string) {
  const colors = ['linear-gradient(135deg,#667eea,#764ba2)', 'linear-gradient(135deg,#36b37e,#00875a)', 'linear-gradient(135deg,#ff991f,#ff5630)', 'linear-gradient(135deg,#00b8d9,#0065ff)', 'linear-gradient(135deg,#eb5286,#ff5630)']
  let hash = 0
  for (let i = 0; i < id.length; i++) hash = id.charCodeAt(i) + ((hash << 5) - hash)
  return colors[Math.abs(hash) % colors.length]
}

function handleCmd(cmd: string, row: any) {
  if (cmd === 'edit') openForm(row)
  else if (cmd === 'addSub') openForm(null, row.id)
  else if (cmd === 'delete') del(row)
}

async function toggleStatus(row: any) {
  const newStatus = row.status === 1 ? 0 : 1
  await http.put(`/departments/${row.id}`, { ...row, status: newStatus })
  row.status = newStatus
  ElMessage.success(newStatus === 1 ? '已启用' : '已禁用')
}

async function load() {
  try {
    const res = await http.get('/departments')
    console.log('[DEBUG] departments res.data:', JSON.stringify(res.data)?.substring(0, 200))
    const arr = res.data?.data
    console.log('[DEBUG] arr type:', typeof arr, 'isArray:', Array.isArray(arr), 'length:', arr?.length)
    flatDepts.value = Array.isArray(arr) ? arr : []
    deptTree.value = flatDepts.value
  } catch(e) {
    console.error('[DEBUG] load error:', e)
  }
}

function buildTree(items: any[]): any[] {
  const map: any = {}
  const roots: any[] = []
  items.forEach((i: any) => { map[i.id] = { ...i, children: [] } })
  items.forEach((i: any) => {
    if (i.parent_id && map[i.parent_id]) map[i.parent_id].children.push(map[i.id])
    else roots.push(map[i.id])
  })
  return roots
}

function openForm(row?: any, parentId?: string) {
  editing.value = row || null
  form.value = row ? { ...row } : { name: '', parent_id: parentId || '', sort_order: 0 }
  showForm.value = true
}

async function submit() {
  if (editing.value) {
    await http.put(`/departments/${editing.value.id}`, form.value)
  } else {
    await http.post('/departments', form.value)
  }
  ElMessage.success('已保存')
  showForm.value = false
  load()
}

async function del(row: any) {
  await ElMessageBox.confirm(`删除部门「${row.name}」？子部门也会被删除。`, '确认', { type: 'warning' })
  await http.delete(`/departments/${row.id}`)
  ElMessage.success('已删除')
  load()
}

async function doImport() {
  await http.post('/departments/import', { csv: csvData.value })
  ElMessage.success('导入完成')
  showImport.value = false
  csvData.value = ''
  load()
}

onMounted(load)
</script>

<style scoped>
.admin-page { height: 100%; display: flex; flex-direction: column; padding: 20px; background: #f5f7fa; }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; flex-wrap: wrap; gap: 16px;
  padding-bottom: 16px; border-bottom: 1px solid #e8ecf0;
}
.header-left { display: flex; align-items: baseline; gap: 16px; }
.page-title { font-size: 22px; font-weight: 600; color: #1a1a2e; margin: 0; letter-spacing: -0.02em; }
.header-count { font-size: 14px; color: #909399; }

.header-actions { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.search-input { width: 220px; }

.table-card {
  background: #fff; border-radius: 16px;
  border: none; flex: 1; overflow: auto;
  padding: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.table-card :deep(.el-table__row) { height: 56px; }
.table-card :deep(.el-table__cell) { padding: 12px 0; }
.table-card :deep(.el-table__header-cell) { padding: 14px 0; background: #fafbfc !important; }

.dept-name { display: flex; align-items: center; gap: 10px; }
.dept-icon {
  width: 30px; height: 30px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0;
}
.dept-icon svg { width: 16px; height: 16px; color: #fff; }
.dept-label { font-weight: 500; color: #1a1a2e; font-size: 14px; }

.sort-text { font-size: 13px; color: #909399; }

.more-btn {
  width: 32px; height: 32px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  font-size: 18px; color: #909399; transition: all 0.2s ease;
}
.more-btn:hover { color: #409eff; background: #f0f5ff; }

.dept-form :deep(.el-form-item__label) { font-weight: 500; color: #606266; }
.dept-form :deep(.el-input__wrapper) { border-radius: 8px; }
.full-width { width: 100%; }

:deep(.el-dialog) { border-radius: 16px; }

@media (max-width: 768px) {
  .admin-page { padding: 12px; }
  .page-header { flex-direction: column; align-items: stretch; padding-bottom: 12px; }
  .header-actions { width: 100%; }
  .search-input { width: 100% !important; }
}
</style>
