<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">权限管理</h2>
        <span class="header-count">共 {{ perms.length }} 条规则</span>
      </div>
      <el-button type="primary" size="default" @click="showForm = true">
        <el-icon><Plus /></el-icon> 设置权限
      </el-button>
    </div>

    <!-- 筛选 -->
    <div class="filter-bar">
      <el-radio-group v-model="filter.resource_type" size="default" @change="load">
        <el-radio-button value="document">文档</el-radio-button>
        <el-radio-button value="folder">文件夹</el-radio-button>
      </el-radio-group>
    </div>

    <div class="table-card">
      <el-table
        :data="perms"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
        :cell-style="{ fontSize: '14px' }"
      >
        <el-table-column label="资源" min-width="240">
          <template #default="{ row }">
            <div class="type-cell">
              <div class="type-icon" :style="{ background: row.resource_type === 'document' ? '#e6f7ff' : '#fff7e6' }">
                <svg v-if="row.resource_type === 'document'" viewBox="0 0 20 20" fill="#409eff"><path d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg>
                <svg v-else viewBox="0 0 20 20" fill="#fa8c16"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
              </div>
              <code class="mono-id">{{ row.resource_id }}</code>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="对象" width="160">
          <template #default="{ row }">
            <div class="target-cell">
              <el-tag size="small" :type="row.target_type === 'user' ? '' : 'warning'" effect="light" round>
                {{ row.target_type === 'user' ? '用户' : '部门' }}
              </el-tag>
              <code class="mono-id sm">{{ row.target_id?.slice(0, 8) }}...</code>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="权限" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="permColor[row.permission]" size="small" effect="light" round disable-transitions>
              {{ permMap[row.permission] }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="继承" width="70" align="center">
          <template #default="{ row }">
            <span v-if="row.inherit" class="inherit-badge">✓</span>
            <span v-else class="no-inherit">—</span>
          </template>
        </el-table-column>
        <el-table-column label="" width="60" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="del(row)" class="del-btn">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showForm" title="设置权限" width="480" destroy-on-close>
      <el-form :model="form" label-position="top" class="perm-form">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="资源类型">
              <el-select v-model="form.resource_type" class="full-width">
                <el-option label="文档" value="document" />
                <el-option label="文件夹" value="folder" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="对象类型">
              <el-select v-model="form.target_type" class="full-width">
                <el-option label="用户" value="user" />
                <el-option label="部门" value="department" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="资源 ID">
          <el-input v-model="form.resource_id" placeholder="输入资源 ID" />
        </el-form-item>
        <el-form-item label="对象 ID">
          <el-input v-model="form.target_id" placeholder="输入用户或部门 ID" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="权限级别">
              <el-select v-model="form.permission" class="full-width">
                <el-option label="只读" value="read" />
                <el-option label="读写" value="write" />
                <el-option label="管理" value="admin" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="子资源继承">
              <el-switch v-model="form.inherit" active-text="继承" inactive-text="不继承" />
            </el-form-item>
          </el-col>
        </el-row>
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
  resource_type: 'document', resource_id: '',
  target_type: 'user', target_id: '',
  permission: 'read', inherit: true,
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
.admin-page { height: 100%; display: flex; flex-direction: column; padding: 20px; background: #f5f7fa; }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; padding-bottom: 16px; border-bottom: 1px solid #e8ecf0;
}
.header-left { display: flex; align-items: baseline; gap: 16px; }
.page-title { font-size: 22px; font-weight: 600; color: #1a1a2e; margin: 0; letter-spacing: -0.02em; }
.header-count { font-size: 14px; color: #909399; }

.filter-bar { margin-bottom: 16px; }

.table-card {
  background: #fff; border-radius: 16px;
  border: none; flex: 1; overflow: auto;
  padding: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.table-card :deep(.el-table__row) { height: 56px; }
.table-card :deep(.el-table__cell) { padding: 12px 0; }
.table-card :deep(.el-table__header-cell) { padding: 14px 0; background: #fafbfc !important; }

.type-cell { display: flex; align-items: center; gap: 10px; }
.type-icon {
  width: 30px; height: 30px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.type-icon svg { width: 16px; height: 16px; }

.target-cell { display: flex; align-items: center; gap: 8px; }

.mono-id {
  font-family: 'SF Mono', Monaco, monospace; font-size: 13px;
  color: #606266; background: #f5f7fa; padding: 3px 8px; border-radius: 6px;
}
.mono-id.sm { font-size: 11px; color: #909399; }

.inherit-badge { color: #36b37e; font-weight: 700; }
.no-inherit { color: #c0c4cc; }

.del-btn { font-size: 13px; }

.perm-form :deep(.el-form-item__label) { font-weight: 500; color: #606266; }
.perm-form :deep(.el-input__wrapper) { border-radius: 8px; }
.full-width { width: 100%; }

:deep(.el-dialog) { border-radius: 16px; }

@media (max-width: 768px) {
  .admin-page { padding: 12px; }
  .filter-bar { overflow-x: auto; }
}
</style>
