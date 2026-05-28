<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">部门管理</h3>
      <div>
        <el-button @click="showImport = true">
          <el-icon><Upload /></el-icon> 批量导入
        </el-button>
        <el-button type="primary" @click="openForm()">
          <el-icon><Plus /></el-icon> 新建部门
        </el-button>
      </div>
    </div>

    <el-table :data="flatDepts" row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }" stripe>
      <el-table-column prop="name" label="部门名称" min-width="200" />
      <el-table-column prop="status" label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button link size="small" @click="openForm(row)">编辑</el-button>
          <el-button link type="danger" size="small" @click="del(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

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
      <p>CSV 格式：name,parent_name,sort_order</p>
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
