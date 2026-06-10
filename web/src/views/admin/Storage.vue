<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">{{ t('admin.storage.title') }}</h2>
        <el-tag :type="health === 'healthy' ? 'success' : health === 'warning' ? 'warning' : 'danger'" effect="light" round size="small">
          {{ health === 'healthy' ? t('admin.storage.healthy') : health === 'warning' ? t('admin.storage.warning') : t('admin.storage.error') }}
        </el-tag>
      </div>
      <el-button size="default" @click="load">
        <el-icon><RefreshRight /></el-icon> {{ t('common.refresh') }}
      </el-button>
    </div>

    <!-- 磁盘 + 加密 -->
    <div class="top-grid">
      <div class="panel disk-panel">
        <div class="panel-title">{{ t('admin.storage.diskUsage') }}</div>
        <div class="disk-stats">
          <div class="disk-item">
            <span class="disk-label">{{ t('admin.storage.used') }}</span>
            <span class="disk-value">{{ disk.used_human }}</span>
          </div>
          <div class="disk-item">
            <span class="disk-label">{{ t('admin.storage.available') }}</span>
            <span class="disk-value">{{ disk.available_human }}</span>
          </div>
          <div class="disk-item">
            <span class="disk-label">{{ t('admin.storage.total') }}</span>
            <span class="disk-value">{{ disk.total_human }}</span>
          </div>
        </div>
        <el-progress
          :percentage="disk.usage_percent"
          :stroke-width="10"
          :color="disk.usage_percent > 90 ? '#f56c6c' : disk.usage_percent > 75 ? '#e6a23c' : '#36b37e'"
          style="margin-top: 16px"
        />
      </div>

      <div class="panel encrypt-panel">
        <div class="panel-title">{{ t('admin.storage.encryptionStatus') }}</div>
        <div class="encrypt-badge">
          <el-tag :type="encryption.enabled ? 'success' : 'danger'" effect="dark" round size="large">
            {{ encryption.enabled ? t('admin.storage.encryptionEnabled') : t('admin.storage.encryptionDisabled') }}
          </el-tag>
        </div>
        <div class="encrypt-detail">
          <span class="disk-label">{{ t('admin.storage.algorithm') }}</span>
          <span class="disk-value">{{ encryption.algorithm || '—' }}</span>
        </div>
        <div class="encrypt-detail">
          <span class="disk-label">{{ t('admin.storage.storagePath') }}</span>
          <code class="path-code">{{ storageRoot }}</code>
        </div>
      </div>
    </div>

    <!-- 文件统计 -->
    <div class="stat-grid">
      <div class="stat-card" v-for="s in fileStats" :key="s.label">
        <div class="stat-dot" :style="{ background: s.color }" />
        <div>
          <div class="stat-val">{{ s.value }}</div>
          <div class="stat-lbl">{{ s.label }}</div>
        </div>
      </div>
    </div>

    <!-- 部门占用 -->
    <div class="panel" v-if="files.departments?.length">
      <div class="panel-title">{{ t('admin.storage.deptDistribution') }}</div>
      <el-table :data="files.departments" size="small"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
      >
        <el-table-column prop="department_id" :label="t('admin.storage.deptId')" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="mono-id">{{ row.department_id }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="file_count" :label="t('admin.storage.fileCount')" width="100" align="center" />
        <el-table-column prop="document_count" :label="t('admin.storage.docCount')" width="100" align="center" />
        <el-table-column prop="total_size_human" :label="t('admin.storage.usage')" width="100" align="center">
          <template #default="{ row }">
            <span class="size-text">{{ row.total_size_human }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 健康检查 -->
    <div class="panel">
      <div class="panel-title">{{ t('admin.storage.healthCheck') }}</div>
      <div class="check-list">
        <div v-for="c in checks" :key="c.name" class="check-item">
          <div class="check-status" :class="c.status" />
          <span class="check-name">{{ checkNames[c.name] || c.name }}</span>
          <span class="check-detail">{{ c.detail }}</span>
        </div>
      </div>
      <div v-if="warnings.length" class="warnings">
        <div v-for="w in warnings" :key="w" class="warning-item"><el-icon color="#e6a23c" style="vertical-align:middle;margin-right:4px"><WarningFilled /></el-icon>{{ w }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { WarningFilled } from '@element-plus/icons-vue'
import http from '@/utils/http'
import teamApi from '@/utils/team-api'

const { t } = useI18n()

const checkNames: any = {
  disk_usage: t('admin.storage.checks.disk_usage'), storage_root: t('admin.storage.checks.storage_root'), write_permission: t('admin.storage.checks.write_permission'),
  encryption: t('admin.storage.checks.encryption'), trash: t('admin.storage.checks.trash'), versions: t('admin.storage.checks.versions'), config: t('admin.storage.checks.config'),
}

const storageRoot = ref('')
const disk = ref<any>({ total_human: '-', used_human: '-', available_human: '-', usage_percent: 0 })
const files = ref<any>({ total_files: 0, total_size_human: '-', departments: [] })
const encryption = ref<any>({ enabled: false })
const checks = ref<any[]>([])
const warnings = ref<string[]>([])
const health = ref('healthy')

const fileStats = computed(() => [
  { label: t('admin.storage.totalFiles'), value: files.value.total_files, color: '#4f6ef7' },
  { label: t('admin.storage.usedSpace'), value: files.value.total_size_human, color: '#6554c0' },
  { label: t('admin.storage.currentVersion'), value: files.value.current_files, color: '#36b37e' },
  { label: t('admin.storage.historyVersion'), value: files.value.version_files, color: '#00b8d9' },
  { label: t('admin.storage.collabState'), value: files.value.yjs_state_files, color: '#ff991f' },
  { label: t('admin.storage.trashFiles'), value: files.value.trash_files, color: '#ff5630' },
])

async function load() {
  const { data } = await teamApi.get('/storage/status')
  storageRoot.value = data.storage_root || ''
  disk.value = data.disk || {}
  files.value = data.files || {}
  encryption.value = data.encryption || {}
  checks.value = data.health?.checks || []
  warnings.value = data.health?.warnings || []
  health.value = data.health?.status || 'unknown'
}

onMounted(load)
</script>

<style scoped>
.admin-page { height: 100%; overflow-y: auto; padding: 20px; background: #f5f7fa; }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; padding-bottom: 16px; border-bottom: 1px solid #e8ecf0;
}
.header-left { display: flex; align-items: center; gap: 16px; }
.page-title { font-size: 22px; font-weight: 600; color: #1a1a2e; margin: 0; letter-spacing: -0.02em; }

/* 顶部面板 */
.top-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }

.panel {
  background: #fff; border-radius: 16px; padding: 20px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04); margin-bottom: 16px;
}
.panel-title {
  font-size: 15px; font-weight: 600; color: #1a1a2e;
  margin-bottom: 16px; padding-bottom: 12px; border-bottom: 1px solid #f0f0f0;
}

.disk-stats { display: flex; gap: 24px; }
.disk-item { display: flex; flex-direction: column; gap: 4px; }
.disk-label { font-size: 13px; color: #909399; }
.disk-value { font-size: 18px; font-weight: 600; color: #1a1a2e; }

.encrypt-badge { text-align: center; margin: 8px 0 16px; }
.encrypt-detail { display: flex; justify-content: space-between; padding: 6px 0; }
.path-code { font-size: 12px; color: #909399; background: #f5f7fa; padding: 2px 8px; border-radius: 4px; font-family: 'SF Mono', Monaco, monospace; }

/* 文件统计 */
.stat-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; margin-bottom: 16px; }
.stat-card {
  background: #fff; border-radius: 14px; padding: 16px;
  display: flex; align-items: center; gap: 12px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.stat-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.stat-val { font-size: 20px; font-weight: 700; color: #1a1a2e; }
.stat-lbl { font-size: 12px; color: #909399; margin-top: 2px; }

.mono-id {
  font-family: 'SF Mono', Monaco, monospace; font-size: 13px;
  color: #606266; background: #f5f7fa; padding: 3px 8px; border-radius: 6px;
}
.size-text { font-weight: 600; color: #1a1a2e; }

/* 健康检查 */
.check-list { display: flex; flex-direction: column; gap: 12px; }
.check-item { display: flex; align-items: center; gap: 12px; padding: 8px 0; }
.check-status {
  width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0;
}
.check-status.ok { background: #36b37e; box-shadow: 0 0 0 3px rgba(54,179,126,0.15); }
.check-status.warn { background: #ff991f; box-shadow: 0 0 0 3px rgba(255,153,31,0.15); }
.check-status.error { background: #ff5630; box-shadow: 0 0 0 3px rgba(255,86,48,0.15); }
.check-name { font-size: 14px; font-weight: 500; color: #1a1a2e; min-width: 100px; }
.check-detail { font-size: 13px; color: #909399; }

.warnings { margin-top: 12px; padding-top: 12px; border-top: 1px solid #fff7e6; }
.warning-item {
  padding: 8px 12px; margin-bottom: 6px; border-radius: 8px;
  background: #fff7e6; font-size: 13px; color: #ad6800;
}

@media (max-width: 768px) {
  .top-grid { grid-template-columns: 1fr; }
  .stat-grid { grid-template-columns: repeat(3, 1fr); }
}
</style>
