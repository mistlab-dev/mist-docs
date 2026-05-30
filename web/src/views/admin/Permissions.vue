<template>
  <div class="admin-page">
    <div class="page-header">
      <h2 class="page-title">权限管理</h2>
      <el-button type="primary" size="small" @click="showForm = true">
        <el-icon><Plus /></el-icon> 设置权限
      </el-button>
    </div>

    <!-- 筛选 -->
    <div class="filter-bar">
      <el-select v-model="filter.resource_type" size="small" style="width:120px" @change="load">
        <el-option label="文档" value="document" />
        <el-option label="文件夹" value="folder" />
      </el-select>
    </div>

    <div class="table-card">
      <el-table :data="perms" stripe>
        <el-table-column prop="resource_type" label="资源类型" width="100">
          <template #default="{ row }">
            <div class="type-cell">
              <svg v-if="row.resource_type === 'document'" class="type-icon doc" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg>
              <svg v-else class="type-icon folder" viewBox="0 0 20 20" fill="currentColor"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
              {{ row.resource_type === 'document' ? '文档' : '文件夹' }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="resource_id" label="资源 ID" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="mono-id">{{ row.resource_id }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="target_type" label="对象" width="80">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.target_type === 'user' ? '用户' : '部门' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_id" label="对象 ID" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="mono-id">{{ row.target_id }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="permission" label="权限" width="90">
          <template #default="{ row }">
            <el-tag :type="permColor[row.permission]" size="small" effect="plain" round>
              {{ permMap[row.permission] }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="del(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showForm" title="设置权限" width="480">
      <el-form :model="form" label-width="80px">
        <el-form-item label="资源类型">
          <el-select v-model="form.resource_type">
            <el-option label="文档" value="document" />
            <el-option label="文件夹" value="folder" />
          </el-select>
        </el-form-item>
        <el-form-item label="资源ID">
          <el-input v-model="form.resource_id" />
        </el-form-item>
        <el-form-item label="对象类型">
          <el-select v-model="form.target_type">
            <el-option label="用户" value="user" />
            <el-option label="部门" value="department" />
          </el-select>
        </el-form-item>
        <el-form-item label="对象ID">
          <el-input v-model="form.target_id" />
        </el-form-item>
        <el-form-item label="权限级别">
          <el-select v-model="form.permission">
            <el-option label="只读" value="read" />
            <el-option label="读写" value="write" />
            <el-option label="管理" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="继承">
          <el-switch v-model="form.inherit" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForm = false">取消</el-button>
        <el-button type="primary" @click="submit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import http from '@/utils/http'

const permMap: any = { read: '只读', write: '读写', admin: '管理' }
const permColor: any = { read: 'info', write: 'warning', admin: 'danger' }
const perms = ref<any[]>([])
const filter = ref({ resource_type: 'document' })
const showForm = ref(false)
const form = ref({
  resource_type: 'document',
  resource_id: '',
  target_type: 'user',
  target_id: '',
  permission: 'read',
  inherit: true,
})

async function load() {
  const { data } = await http.get('/permissions', { params: filter.value })
  perms.value = data.data || []
}

async function submit() {
  await http.post('/permissions', form.value)
  ElMessage.success('已设置')
  showForm.value = false
  load()
}

async function del(row: any) {
  await ElMessageBox.confirm('移除此权限？', '确认')
  await http.delete(`/permissions/${row.id}`)
  ElMessage.success('已移除')
  load()
}

onMounted(load)
</script>

<style scoped>
.admin-page { height: 100%; display: flex; flex-direction: column; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.page-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }

.filter-bar { margin-bottom: 12px; }

.table-card { background: #fff; border-radius: 10px; border: 1px solid #e8ecf0; flex: 1; overflow: auto; padding: 4px; }

.type-cell { display: flex; align-items: center; gap: 6px; }
.type-icon { width: 16px; height: 16px; flex-shrink: 0; }
.type-icon.doc { color: #409eff; }
.type-icon.folder { color: #e6a23c; }
.mono-id { font-family: 'SF Mono', Monaco, monospace; font-size: 12px; color: #666; background: #f5f7fa; padding: 2px 6px; border-radius: 4px; }

@media (max-width: 768px) {
  .table-card { border-radius: 0; border-left: none; border-right: none; }
}
</style>
