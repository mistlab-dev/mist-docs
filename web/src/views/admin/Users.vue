<template>
  <div class="admin-page">
    <div class="page-header">
      <h2 class="page-title">用户管理</h2>
      <div class="header-actions">
        <el-input v-model="searchKey" placeholder="搜索用户..." size="small" style="width:180px" clearable>
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button size="small" @click="showImport = true">
          <el-icon><Upload /></el-icon> 批量导入
        </el-button>
        <el-button type="primary" size="small" @click="openForm()">
          <el-icon><Plus /></el-icon> 新建用户
        </el-button>
      </div>
    </div>

    <div class="table-card">
      <el-table :data="filteredUsers" stripe>
        <el-table-column prop="username" label="用户名" min-width="100">
          <template #default="{ row }">
            <div class="user-cell">
              <div class="avatar" :style="{ background: avatarColor(row.username) }">
                {{ row.name?.charAt(0) || '?' }}
              </div>
              <span>{{ row.username }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="姓名" width="100" />
        <el-table-column prop="email" label="邮箱" min-width="160" />
        <el-table-column prop="role" label="角色" width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'super_admin' ? 'danger' : row.role === 'dept_admin' ? 'warning' : 'info'" size="small" effect="plain" round>
              {{ roleMap[row.role] || row.role }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }">
            <span class="status-dot" :class="row.status === 1 ? 'active' : 'disabled'" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="openForm(row)">编辑</el-button>
            <el-button link size="small" @click="resetPwd(row)">重置密码</el-button>
            <el-button link type="danger" size="small" @click="del(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 用户表单 -->
    <el-dialog v-model="showForm" :title="editing ? '编辑用户' : '新建用户'" width="500">
      <el-form :model="form" label-width="80px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" :disabled="!!editing" />
        </el-form-item>
        <el-form-item label="姓名">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="密码" v-if="!editing">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item label="电话">
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-form-item label="部门">
          <el-tree-select v-model="form.department_id" :data="deptTree" check-strictly :props="{ label: 'name', value: 'id' }" clearable placeholder="选择部门" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role">
            <el-option label="普通成员" value="member" />
            <el-option label="部门管理员" value="dept_admin" />
            <el-option label="超级管理员" value="super_admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForm = false">取消</el-button>
        <el-button type="primary" @click="submitUser">确定</el-button>
      </template>
    </el-dialog>

    <!-- CSV 导入 -->
    <el-dialog v-model="showImport" title="批量导入用户" width="500">
      <p style="color:#999;font-size:13px;margin-bottom:8px">CSV 格式：username,name,password,email,phone,department_name,role</p>
      <el-input v-model="csvData" type="textarea" :rows="8" placeholder="粘贴 CSV 数据" />
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

const roleMap: any = { super_admin: '超管', dept_admin: '部门管理', member: '成员' }
const users = ref<any[]>([])
const deptTree = ref<any[]>([])
const showForm = ref(false)
const showImport = ref(false)
const editing = ref<any>(null)
const form = ref<any>({ username: '', name: '', password: '', email: '', phone: '', department_id: '', role: 'member' })
const csvData = ref('')
const searchKey = ref('')

const filteredUsers = computed(() => {
  if (!searchKey.value) return users.value
  const k = searchKey.value.toLowerCase()
  return users.value.filter((u: any) =>
    u.username?.toLowerCase().includes(k) ||
    u.name?.toLowerCase().includes(k) ||
    u.email?.toLowerCase().includes(k)
  )
})

function avatarColor(name: string) {
  const colors = ['#409eff', '#67c23a', '#e6a23c', '#f56c6c', '#909399', '#9b59b6', '#1abc9c']
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash)
  return colors[Math.abs(hash) % colors.length]
}

async function loadUsers() {
  const { data } = await http.get('/users')
  users.value = data.data || []
}

async function loadDepts() {
  const { data } = await http.get('/departments')
  deptTree.value = buildTree(data.data || [])
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
  if (row) {
    form.value = { ...row }
  } else {
    form.value = { username: '', name: '', password: '', email: '', phone: '', department_id: '', role: 'member' }
  }
  showForm.value = true
}

async function submitUser() {
  if (editing.value) {
    await http.put(`/users/${editing.value.id}`, form.value)
  } else {
    await http.post('/users', form.value)
  }
  ElMessage.success('已保存')
  showForm.value = false
  loadUsers()
}

async function resetPwd(row: any) {
  await ElMessageBox.confirm(`重置「${row.name}」的密码？`, '确认')
  await http.put(`/users/${row.id}/reset-password`)
  ElMessage.success('密码已重置为默认密码')
}

async function del(row: any) {
  await ElMessageBox.confirm(`删除用户「${row.name}」？`, '确认', { type: 'warning' })
  await http.delete(`/users/${row.id}`)
  ElMessage.success('已删除')
  loadUsers()
}

async function doImport() {
  await http.post('/users/import', { csv: csvData.value })
  ElMessage.success('导入完成')
  showImport.value = false
  csvData.value = ''
  loadUsers()
}

onMounted(() => { loadUsers(); loadDepts() })
</script>

<style scoped>
.admin-page { height: 100%; display: flex; flex-direction: column; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; flex-wrap: wrap; gap: 8px; }
.page-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }
.header-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.table-card { background: #fff; border-radius: 10px; border: 1px solid #e8ecf0; flex: 1; overflow: auto; padding: 4px; }

.user-cell { display: flex; align-items: center; gap: 8px; }
.avatar {
  width: 28px; height: 28px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 13px; font-weight: 600; flex-shrink: 0;
}

.status-dot {
  display: inline-block; width: 8px; height: 8px; border-radius: 50%;
}
.status-dot.active { background: #67c23a; box-shadow: 0 0 0 2px rgba(103,194,58,0.2); }
.status-dot.disabled { background: #f56c6c; box-shadow: 0 0 0 2px rgba(245,108,108,0.2); }

@media (max-width: 768px) {
  .page-header { flex-direction: column; align-items: stretch; }
  .header-actions { width: 100%; }
  .header-actions .el-input { width: 100% !important; }
  .header-actions .el-button span { display: none; }
}
</style>
