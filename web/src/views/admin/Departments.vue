<template>
  <div class="admin-page">
    <div class="page-header">
      <h2 class="page-title">部门管理</h2>
      <div class="header-actions">
        <el-input v-model="searchKey" placeholder="搜索部门..." size="small" style="width:180px" clearable>
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button size="small" @click="showImport = true">
          <el-icon><Upload /></el-icon> 批量导入
        </el-button>
        <el-button type="primary" size="small" @click="openForm()">
          <el-icon><Plus /></el-icon> 新建部门
        </el-button>
      </div>
    </div>

    <div class="table-card">
      <el-table :data="flatDepts" row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }" stripe>
        <el-table-column prop="name" label="部门名称" min-width="200">
          <template #default="{ row }">
            <div class="dept-name">
              <svg class="dept-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
              {{ row.name }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <span class="status-dot" :class="row.status === 1 ? 'active' : 'disabled'" />
            <span style="margin-left:6px;font-size:13px;color:#666">{{ row.status === 1 ? '正常' : '禁用' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <el-button link size="small" @click="openForm(row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="del(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showForm" :title="editing ? '编辑部门' : '新建部门'" width="450">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="上级部门">
          <el-tree-select v-model="form.parent_id" :data="deptTree" check-strictly :props="{ label: 'name', value: 'id' }" clearable placeholder="无（顶级）" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" />
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
import { ref, onMounted } from 'vue'
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

async function load() {
  const { data } = await http.get('/departments')
  deptTree.value = buildTree(data.data || [])
  flatDepts.value = deptTree.value
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

function openForm(row?: any) {
  editing.value = row || null
  form.value = row ? { ...row } : { name: '', parent_id: '', sort_order: 0 }
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
  await ElMessageBox.confirm(`删除部门「${row.name}」？`, '确认', { type: 'warning' })
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
.admin-page { height: 100%; display: flex; flex-direction: column; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; flex-wrap: wrap; gap: 8px; }
.page-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }
.header-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.table-card { background: #fff; border-radius: 10px; border: 1px solid #e8ecf0; flex: 1; overflow: auto; padding: 4px; }

.dept-name { display: flex; align-items: center; gap: 6px; }
.dept-icon { width: 16px; height: 16px; color: #e6a23c; flex-shrink: 0; }

.status-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; }
.status-dot.active { background: #67c23a; box-shadow: 0 0 0 2px rgba(103,194,58,0.2); }
.status-dot.disabled { background: #f56c6c; box-shadow: 0 0 0 2px rgba(245,108,108,0.2); }

@media (max-width: 768px) {
  .page-header { flex-direction: column; align-items: stretch; }
  .header-actions { width: 100%; }
  .header-actions .el-input { width: 100% !important; }
  .header-actions .el-button span { display: none; }
}
</style>
