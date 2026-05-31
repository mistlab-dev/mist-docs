<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">用户管理</h2>
        <span class="user-count">共 {{ filteredUsers.length }} 位用户</span>
      </div>
      <div class="header-actions">
        <el-input v-model="searchKey" placeholder="搜索用户名、姓名、邮箱..." size="default" class="search-input" clearable>
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="roleFilter" placeholder="角色" size="default" clearable class="filter-select">
          <el-option label="普通成员" value="member" />
          <el-option label="部门管理员" value="dept_admin" />
          <el-option label="超级管理员" value="super_admin" />
        </el-select>
        <el-select v-model="statusFilter" placeholder="状态" size="default" clearable class="filter-select-sm">
          <el-option label="启用" :value="1" />
          <el-option label="禁用" :value="0" />
        </el-select>
        <el-button size="default" @click="showImport = true">
          <el-icon><Upload /></el-icon>
        </el-button>
        <el-button type="primary" size="default" @click="openForm()">
          <el-icon><Plus /></el-icon> 新建
        </el-button>
      </div>
    </div>

    <!-- 批量操作栏 -->
    <transition name="slide-down">
      <div v-if="selected.length" class="batch-bar">
        <span>已选 {{ selected.length }} 项</span>
        <el-button size="small" @click="batchToggleStatus">{{ batchTargetStatus === 1 ? '批量禁用' : '批量启用' }}</el-button>
        <el-button size="small" type="danger" @click="batchDelete">批量删除</el-button>
        <el-button size="small" link @click="selected = []">取消选择</el-button>
      </div>
    </transition>

    <div class="table-card">
      <el-table
        ref="tableRef"
        :data="pagedUsers"
        row-key="id"
        @selection-change="onSelectionChange"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
        :cell-style="{ fontSize: '14px' }"
      >
        <el-table-column type="selection" width="40" />
        <el-table-column label="用户" min-width="200">
          <template #default="{ row }">
            <div class="user-cell">
              <div class="avatar" :style="{ background: avatarColor(row.username) }">
                {{ row.name?.charAt(0) || '?' }}
              </div>
              <div class="user-info">
                <div class="user-name">
                  {{ row.name || row.username }}
                  <span v-if="row.name && row.name !== row.username" class="user-username">@{{ row.username }}</span>
                </div>
                <div class="user-email">{{ row.email || '—' }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="角色" width="130">
          <template #default="{ row }">
            <el-tag
              :type="roleTagType(row.role)"
              size="small"
              effect="light"
              round
              disable-transitions
            >
              {{ roleMap[row.role] || row.role }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="部门" min-width="120">
          <template #default="{ row }">
            <span class="dept-text">{{ row.department_name || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
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
        <el-table-column label="最后登录" width="160">
          <template #default="{ row }">
            <span class="time-text">{{ formatTime(row.last_login_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right" align="center">
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
                  <el-dropdown-item command="resetPwd">
                    <el-icon><RefreshRight /></el-icon> 重置密码
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

      <!-- 分页 -->
      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="filteredUsers.length"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          background
          small
        />
      </div>
    </div>

    <!-- 用户表单 -->
    <el-dialog v-model="showForm" :title="editing ? '编辑用户' : '新建用户'" width="480" destroy-on-close>
      <el-form :model="form" label-width="80px" label-position="top" class="user-form">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="用户名">
              <el-input v-model="form.username" :disabled="!!editing" placeholder="登录用户名" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="姓名">
              <el-input v-model="form.name" placeholder="显示名称" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="密码" v-if="!editing">
          <el-input v-model="form.password" type="password" show-password placeholder="默认密码" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="邮箱">
              <el-input v-model="form.email" placeholder="email@example.com" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="电话">
              <el-input v-model="form.phone" placeholder="手机号" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="部门">
              <el-tree-select
                v-model="form.department_id"
                :data="deptTree"
                check-strictly
                :props="{ label: 'name', value: 'id' }"
                clearable
                placeholder="选择部门"
                class="full-width"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="角色">
              <el-select v-model="form.role" class="full-width">
                <el-option label="普通成员" value="member" />
                <el-option label="部门管理员" value="dept_admin" />
                <el-option label="超级管理员" value="super_admin" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
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
const roleFilter = ref('')
const statusFilter = ref<number | ''>('')
const selected = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const tableRef = ref()

const filteredUsers = computed(() => {
  let list = users.value
  if (searchKey.value) {
    const k = searchKey.value.toLowerCase()
    list = list.filter((u: any) =>
      u.username?.toLowerCase().includes(k) ||
      u.name?.toLowerCase().includes(k) ||
      u.email?.toLowerCase().includes(k)
    )
  }
  if (roleFilter.value) list = list.filter((u: any) => u.role === roleFilter.value)
  if (statusFilter.value !== '' && statusFilter.value !== null && statusFilter.value !== undefined) {
    list = list.filter((u: any) => u.status === statusFilter.value)
  }
  return list
})

const pagedUsers = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredUsers.value.slice(start, start + pageSize.value)
})

const batchTargetStatus = computed(() => {
  // 如果选中的都是启用状态，则批量操作是禁用；反之亦然
  const enabled = selected.value.filter(u => u.status === 1).length
  return enabled >= selected.value.length / 2 ? 1 : 0
})

function roleTagType(role: string) {
  if (role === 'super_admin') return 'danger'
  if (role === 'dept_admin') return 'warning'
  return 'info'
}

function formatTime(t: string) {
  if (!t || t === '0001-01-01T00:00:00Z') return '—'
  const d = new Date(t)
  if (isNaN(d.getTime())) return '—'
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前'
  if (diff < 604800000) return Math.floor(diff / 86400000) + ' 天前'
  return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

function avatarColor(name: string) {
  const colors = ['#4f6ef7', '#36b37e', '#ff991f', '#ff5630', '#6554c0', '#00b8d9', '#eb5286']
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash)
  return colors[Math.abs(hash) % colors.length]
}

function onSelectionChange(rows: any[]) {
  selected.value = rows
}

async function toggleStatus(row: any) {
  const newStatus = row.status === 1 ? 0 : 1
  await http.put(`/users/${row.id}`, { ...row, status: newStatus })
  row.status = newStatus
  ElMessage.success(newStatus === 1 ? '已启用' : '已禁用')
}

async function loadUsers() {
  const { data } = await http.get('/users', { params: { page: 1, page_size: 1000 } })
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

function handleCmd(cmd: string, row: any) {
  if (cmd === 'edit') openForm(row)
  else if (cmd === 'resetPwd') resetPwd(row)
  else if (cmd === 'delete') del(row)
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
  await ElMessageBox.confirm(`删除用户「${row.name}」？此操作不可恢复。`, '确认', { type: 'warning' })
  await http.delete(`/users/${row.id}`)
  ElMessage.success('已删除')
  loadUsers()
}

async function batchToggleStatus() {
  const newStatus = batchTargetStatus.value === 1 ? 0 : 1
  const label = newStatus === 1 ? '启用' : '禁用'
  await ElMessageBox.confirm(`确定${label}选中的 ${selected.value.length} 个用户？`, '确认')
  for (const u of selected.value) {
    await http.put(`/users/${u.id}`, { status: newStatus })
  }
  ElMessage.success(`已${label} ${selected.value.length} 个用户`)
  selected.value = []
  loadUsers()
}

async function batchDelete() {
  await ElMessageBox.confirm(`确定删除选中的 ${selected.value.length} 个用户？此操作不可恢复。`, '确认', { type: 'warning' })
  for (const u of selected.value) {
    await http.delete(`/users/${u.id}`)
  }
  ElMessage.success(`已删除 ${selected.value.length} 个用户`)
  selected.value = []
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
.admin-page { height: 100%; display: flex; flex-direction: column; padding: 20px; background: #f5f7fa; }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; flex-wrap: wrap; gap: 16px;
  padding-bottom: 16px; border-bottom: 1px solid #e8ecf0;
}
.header-left { display: flex; align-items: baseline; gap: 16px; }
.page-title { font-size: 22px; font-weight: 600; color: #1a1a2e; margin: 0; letter-spacing: -0.02em; }
.user-count { font-size: 14px; color: #909399; font-weight: 400; }

.header-actions { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.search-input { width: 260px; }
.filter-select { width: 140px; }
.filter-select-sm { width: 110px; }

.batch-bar {
  display: flex; align-items: center; gap: 16px;
  padding: 12px 20px; margin-bottom: 16px;
  background: linear-gradient(135deg, #e6f7ff 0%, #f0f5ff 100%);
  border: 1px solid #91d5ff;
  border-radius: 10px; font-size: 14px; color: #1890ff;
  font-weight: 500;
}

.table-card {
  background: #fff; border-radius: 16px;
  border: none; flex: 1; overflow: auto;
  padding: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}

/* 表格行高 */
.table-card :deep(.el-table__row) { height: 56px; }
.table-card :deep(.el-table__body-wrapper tr) { height: 56px; }
.table-card :deep(.el-table__cell) { padding: 12px 0; }
.table-card :deep(.el-table__header-cell) { padding: 14px 0; background: #fafbfc !important; }

/* 用户单元格 */
.user-cell { display: flex; align-items: center; gap: 12px; padding: 4px 0; }
.avatar {
  width: 38px; height: 38px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 15px; font-weight: 600; flex-shrink: 0;
  box-shadow: 0 3px 8px rgba(0,0,0,0.15);
}
.user-info { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.user-name { font-weight: 600; color: #1a1a2e; display: flex; align-items: baseline; gap: 8px; font-size: 15px; }
.user-username { font-size: 13px; color: #909399; font-weight: 400; }
.user-email { font-size: 13px; color: #909399; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.dept-text { font-size: 14px; color: #606266; }
.time-text { font-size: 13px; color: #909399; }

.more-btn {
  width: 32px; height: 32px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  font-size: 18px; color: #909399; transition: all 0.2s ease;
}
.more-btn:hover { color: #409eff; background: #f0f5ff; }

.pagination-wrap {
  display: flex; justify-content: flex-end; padding: 16px 0 0;
  border-top: 1px solid #f0f0f0; margin-top: 12px;
}
.pagination-wrap :deep(.el-pagination) { font-weight: 500; }

.user-form :deep(.el-form-item__label) { font-weight: 500; color: #606266; }
.user-form :deep(.el-input__wrapper) { border-radius: 8px; }
.full-width { width: 100%; }

/* 弹窗 */
:deep(.el-dialog) { border-radius: 16px; }
:deep(.el-dialog__header) { padding: 20px 20px 16px; }
:deep(.el-dialog__body) { padding: 16px 20px; }
:deep(.el-dialog__footer) { padding: 16px 20px 20px; }

/* 动画 */
.slide-down-enter-active, .slide-down-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.slide-down-enter-from, .slide-down-leave-to {
  opacity: 0; transform: translateY(-10px); margin-top: -52px;
}

@media (max-width: 768px) {
  .admin-page { padding: 12px; }
  .page-header { flex-direction: column; align-items: stretch; padding-bottom: 12px; }
  .header-actions { width: 100%; }
  .search-input, .filter-select, .filter-select-sm { width: 100% !important; }
  .table-card { padding: 8px; }
}
</style>
