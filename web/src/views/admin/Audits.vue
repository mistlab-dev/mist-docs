<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">{{ t('admin.audits.title') }}</h2>
        <span class="header-count">{{ t('admin.audits.recordsCount', [total]) }}</span>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <div class="filter-left">
        <el-select v-model="filter.action" clearable :placeholder="t('admin.audits.actionType')" size="default" class="filter-select">
          <el-option :label="t('admin.audits.login')" value="login" />
          <el-option :label="t('admin.audits.create')" value="create_doc" />
          <el-option :label="t('admin.audits.editAction')" value="edit" />
          <el-option :label="t('admin.audits.delete')" value="delete" />
          <el-option :label="t('admin.audits.permissionChange')" value="set_permission" />
        </el-select>
        <el-input v-model="filter.user_name" clearable :placeholder="t('admin.audits.userName')" size="default" class="filter-user" />
        <el-date-picker v-model="filter.start_date" type="date" value-format="YYYY-MM-DD" :placeholder="t('admin.audits.startDate')" size="default" class="filter-date" />
        <span class="filter-sep">—</span>
        <el-date-picker v-model="filter.end_date" type="date" value-format="YYYY-MM-DD" :placeholder="t('admin.audits.endDate')" size="default" class="filter-date" />
      </div>
      <div class="filter-right">
        <el-button type="primary" size="default" @click="load">{{ t('admin.audits.query') }}</el-button>
        <el-button size="default" @click="exportCSV">
          <el-icon><Download /></el-icon> {{ t('common.export') }}
        </el-button>
      </div>
    </div>

    <div class="table-card">
      <el-table
        :data="audits"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
        :cell-style="{ fontSize: '14px' }"
      >
        <el-table-column :label="t('admin.audits.time')" width="170">
          <template #default="{ row }">
            <div class="time-cell">
              <el-icon :size="14" color="#909399"><Clock /></el-icon>
              <span>{{ row.created_at }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('admin.audits.user')" width="120">
          <template #default="{ row }">
            <div class="user-cell">
              <div class="mini-avatar" :style="{ background: avatarColor(row.user_name) }">
                {{ row.user_name?.charAt(0) || '?' }}
              </div>
              <span>{{ row.user_name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('admin.audits.action')" width="120">
          <template #default="{ row }">
            <el-tag :type="actionColor[row.action] || 'info'" size="small" effect="light" round disable-transitions>
              {{ actionMap[row.action] || row.action }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('admin.audits.resource')" width="80">
          <template #default="{ row }">
            <span class="resource-type">{{ resourceMap[row.resource_type] || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="resource_name" :label="t('admin.audits.resourceName')" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="res-name">{{ row.resource_name || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="IP" width="130">
          <template #default="{ row }">
            <code class="ip-code">{{ row.ip || '—' }}</code>
          </template>
        </el-table-column>
        <el-table-column label="" width="60" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link size="small" class="detail-btn" @click="showDetail(row)">{{ t('admin.audits.detail') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="pagination-wrap">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, sizes, prev, pager, next"
        :page-sizes="[20, 50, 100]"
        background
        small
        @current-change="load"
      />
    </div>

    <el-dialog v-model="showDetailDialog" :title="t('admin.audits.detailTitle')" width="520">
      <pre class="detail-pre">{{ formatDetail(detail) }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import http from '@/utils/http'
import teamApi from '@/utils/team-api'

const { t } = useI18n()

const actionMap: any = { login: t('admin.audits.login'), logout: t('admin.audits.logout'), create_doc: t('admin.audits.create'), edit: t('admin.audits.editAction'), delete: t('admin.audits.delete'), view: t('admin.audits.view'), set_permission: t('admin.audits.permissionChange') }
const actionColor: any = { login: 'success', logout: 'info', create_doc: 'primary', edit: 'warning', delete: 'danger', set_permission: 'warning' }
const resourceMap: any = { document: t('common.doc'), folder: t('common.folder'), user: t('common.user'), department: t('common.department') }

const audits = ref<any[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const filter = ref({ action: '', user_name: '', start_date: '', end_date: '' })
const showDetailDialog = ref(false)
const detail = ref('')

function avatarColor(name: string) {
  const colors = ['#4f6ef7', '#36b37e', '#ff991f', '#ff5630', '#6554c0', '#00b8d9', '#eb5286']
  let hash = 0
  for (let i = 0; i < (name || '').length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash)
  return colors[Math.abs(hash) % colors.length]
}

function formatDetail(raw: string) {
  try { return JSON.stringify(JSON.parse(raw || '{}'), null, 2) } catch { return raw || '{}' }
}

async function load() {
  const params = { page: page.value, page_size: pageSize, ...filter.value }
  const { data } = await teamApi.get('/audits', { params })
  audits.value = data.data || []
  total.value = audits.value.length < pageSize ? (page.value - 1) * pageSize + audits.value.length : page.value * pageSize + 1
}

async function exportCSV() {
  const params = { ...filter.value }
  const { data } = await teamApi.get('/audits/export', { params, responseType: 'blob' })
  const url = URL.createObjectURL(data)
  const a = document.createElement('a')
  a.href = url; a.download = 'audits.csv'; a.click()
  URL.revokeObjectURL(url)
  ElMessage.success(t('admin.audits.exportSuccess'))
}

function showDetail(row: any) {
  detail.value = row.detail || '{}'
  showDetailDialog.value = true
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

.filter-bar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 16px; flex-wrap: wrap; gap: 12px;
}
.filter-left { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.filter-select { width: 140px; }
.filter-user { width: 150px; }
.filter-date { width: 150px; }
.filter-sep { color: #c0c4cc; }

.filter-right { display: flex; align-items: center; gap: 8px; }

.table-card {
  background: #fff; border-radius: 16px;
  border: none; flex: 1; overflow: auto;
  padding: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.table-card :deep(.el-table__row) { height: 56px; }
.table-card :deep(.el-table__cell) { padding: 12px 0; }
.table-card :deep(.el-table__header-cell) { padding: 14px 0; background: #fafbfc !important; }

.time-cell { display: flex; align-items: center; gap: 6px; font-size: 13px; color: #606266; }

.user-cell { display: flex; align-items: center; gap: 8px; }
.mini-avatar {
  width: 26px; height: 26px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 12px; font-weight: 600; flex-shrink: 0;
}

.resource-type { font-size: 13px; color: #909399; }
.res-name { color: #303133; }
.ip-code {
  font-size: 12px; color: #909399; background: #f5f7fa;
  padding: 2px 8px; border-radius: 4px; font-family: 'SF Mono', Monaco, monospace;
}

.detail-btn { font-size: 13px; color: #409eff; }

.pagination-wrap {
  display: flex; justify-content: flex-end; padding: 16px 0 0;
}

.detail-pre {
  white-space: pre-wrap; word-break: break-all;
  background: #f5f7fa; padding: 16px; border-radius: 12px;
  font-size: 13px; margin: 0; font-family: 'SF Mono', Monaco, monospace;
  line-height: 1.6; color: #303133;
}

:deep(.el-dialog) { border-radius: 16px; }

@media (max-width: 768px) {
  .admin-page { padding: 12px; }
  .filter-bar { flex-direction: column; align-items: stretch; }
  .filter-left { width: 100%; flex-direction: column; }
  .filter-select, .filter-user, .filter-date { width: 100% !important; }
}
</style>
