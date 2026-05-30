<template>
  <div class="admin-page">
    <div class="page-header">
      <h2 class="page-title">审计日志</h2>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <div class="filter-left">
        <el-select v-model="filter.action" clearable placeholder="操作类型" size="small" style="width:120px">
          <el-option label="登录" value="login" />
          <el-option label="创建" value="create_doc" />
          <el-option label="编辑" value="edit" />
          <el-option label="删除" value="delete" />
          <el-option label="权限变更" value="set_permission" />
        </el-select>
        <el-input v-model="filter.user_name" clearable placeholder="用户名" size="small" style="width:140px" />
        <el-date-picker v-model="filter.start_date" type="date" value-format="YYYY-MM-DD" placeholder="开始日期" size="small" style="width:140px" />
        <el-date-picker v-model="filter.end_date" type="date" value-format="YYYY-MM-DD" placeholder="结束日期" size="small" style="width:140px" />
      </div>
      <div class="filter-right">
        <el-button type="primary" size="small" @click="load">查询</el-button>
        <el-button size="small" @click="exportCSV">
          <el-icon><Download /></el-icon> 导出
        </el-button>
      </div>
    </div>

    <div class="table-card">
      <el-table :data="audits" stripe>
        <el-table-column prop="created_at" label="时间" width="160">
          <template #default="{ row }">
            <span style="font-size:13px;color:#666">{{ row.created_at }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="user_name" label="用户" width="90" />
        <el-table-column prop="action" label="操作" width="110">
          <template #default="{ row }">
            <el-tag :type="actionColor[row.action] || 'info'" size="small" effect="plain" round>
              {{ actionMap[row.action] || row.action }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="resource_type" label="资源" width="80">
          <template #default="{ row }">
            {{ row.resource_type === 'document' ? '文档' : row.resource_type === 'folder' ? '文件夹' : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="resource_name" label="名称" min-width="180" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="120">
          <template #default="{ row }">
            <span style="font-size:12px;color:#999;font-family:monospace">{{ row.ip }}</span>
          </template>
        </el-table-column>
        <el-table-column label="详情" width="70" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="showDetail(row)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="pagination-bar">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        size="small"
        @current-change="load"
      />
    </div>

    <el-dialog v-model="showDetailDialog" title="操作详情" width="500">
      <pre class="detail-pre">{{ detail }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import http from '@/utils/http'

const actionMap: any = { login: '登录', logout: '登出', create_doc: '创建文档', edit: '编辑', delete: '删除', view: '查看', set_permission: '设置权限' }
const actionColor: any = { login: 'success', logout: 'info', create_doc: 'primary', edit: 'warning', delete: 'danger', set_permission: 'warning' }
const audits = ref<any[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const filter = ref({ action: '', user_name: '', start_date: '', end_date: '' })
const showDetailDialog = ref(false)
const detail = ref('')

async function load() {
  const params = { page: page.value, page_size: pageSize, ...filter.value }
  const { data } = await http.get('/audits', { params })
  audits.value = data.data || []
  total.value = audits.value.length < pageSize ? (page.value - 1) * pageSize + audits.value.length : page.value * pageSize + 1
}

async function exportCSV() {
  const params = { ...filter.value }
  const { data } = await http.get('/audits/export', { params, responseType: 'blob' })
  const url = URL.createObjectURL(data)
  const a = document.createElement('a')
  a.href = url
  a.download = 'audits.csv'
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('已导出')
}

function showDetail(row: any) {
  detail.value = row.detail || '{}'
  showDetailDialog.value = true
}

onMounted(load)
</script>

<style scoped>
.admin-page { height: 100%; display: flex; flex-direction: column; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.page-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }

.filter-bar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 12px; flex-wrap: wrap; gap: 8px;
}
.filter-left { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.filter-right { display: flex; align-items: center; gap: 8px; }

.table-card { background: #fff; border-radius: 10px; border: 1px solid #e8ecf0; flex: 1; overflow: auto; padding: 4px; }
.pagination-bar { display: flex; justify-content: flex-end; margin-top: 12px; }

.detail-pre { white-space: pre-wrap; word-break: break-all; background: #f5f7fa; padding: 16px; border-radius: 8px; font-size: 13px; margin: 0; }

@media (max-width: 768px) {
  .filter-bar { flex-direction: column; align-items: stretch; }
  .filter-left { width: 100%; }
  .filter-left .el-select, .filter-left .el-input { width: 100% !important; }
}
</style>
