<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">{{ t('admin.storage.title') }}</h2>
        <el-tag v-if="quota.unlimited" type="info" effect="light" round size="small">
          {{ t('admin.storage.quotaUnlimited') }}
        </el-tag>
        <el-tag v-else :type="percentColor" effect="light" round size="small">
          {{ Math.round(quota.usage_percent || 0) }}%
        </el-tag>
      </div>
      <el-button size="default" @click="load">
        <el-icon><RefreshRight /></el-icon> {{ t('common.refresh') }}
      </el-button>
    </div>

    <!-- 用量概览 -->
    <div class="panel">
      <div class="panel-title">{{ t('admin.storage.teamUsage') }}</div>
      <div class="quota-stats">
        <div class="quota-item">
          <span class="disk-label">{{ t('admin.storage.used') }}</span>
          <span class="quota-value">{{ usage.total_human }}</span>
        </div>
        <div class="quota-item" v-if="!quota.unlimited">
          <span class="disk-label">{{ t('admin.storage.usedOf') }}</span>
          <span class="quota-value">{{ usage.total_human }} / {{ quota.max_human }}</span>
        </div>
        <div class="quota-item" v-if="!quota.unlimited">
          <span class="disk-label">{{ t('admin.storage.remaining') }}</span>
          <span class="quota-value">{{ quota.remaining_human }}</span>
        </div>
        <div class="quota-item">
          <span class="disk-label">{{ t('admin.storage.yourPlan') }}</span>
          <span class="quota-value">{{ quota.max_mb === 0 ? t('admin.storage.infinite') : quota.max_mb + ' MB' }}</span>
        </div>
      </div>
      <el-progress
        v-if="!quota.unlimited"
        :percentage="quota.usage_percent > 100 ? 100 : quota.usage_percent"
        :stroke-width="12"
        :color="quota.usage_percent > 90 ? '#f56c6c' : quota.usage_percent > 75 ? '#e6a23c' : '#36b37e'"
        style="margin-top: 16px"
      />
      <div class="admin-hint" v-if="!isAdmin">{{ t('admin.storage.adminOnly') }}</div>
    </div>

    <!-- 存储构成 -->
    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-dot" style="background:#4f6ef7" />
        <div>
          <div class="stat-val">{{ usage.documents.human }}</div>
          <div class="stat-lbl">{{ t('admin.storage.docStorage') }} · {{ usage.documents.count }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-dot" style="background:#00b8d9" />
        <div>
          <div class="stat-val">{{ usage.versions.human }}</div>
          <div class="stat-lbl">{{ t('admin.storage.versionStorage') }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-dot" style="background:#ff991f" />
        <div>
          <div class="stat-val">{{ usage.media.human }}</div>
          <div class="stat-lbl">{{ t('admin.storage.mediaStorage') }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-dot" style="background:#ff5630" />
        <div>
          <div class="stat-val">{{ usage.trash.human }}</div>
          <div class="stat-lbl">{{ t('admin.storage.trashStorage') }} · {{ usage.trash.count }}</div>
        </div>
      </div>
    </div>

    <!-- 类型分布 -->
    <div class="panel" v-if="Object.keys(usage.by_type || {}).length">
      <div class="panel-title">{{ t('admin.storage.storageDetailed') }}</div>
      <el-table :data="typeRows" size="small"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
      >
        <el-table-column prop="type" label="Type" min-width="180" />
        <el-table-column prop="human" :label="t('admin.storage.usage')" width="160" align="center">
          <template #default="{ row }"><span class="size-text">{{ row.human }}</span></template>
        </el-table-column>
        <el-table-column prop="bytes" label="Bytes" width="140" align="center">
          <template #default="{ row }"><code class="mono-id">{{ row.bytes }}</code></template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import teamApi from '@/utils/team-api'

const { t } = useI18n()

const usage = ref<any>({
  total_human: '-',
  documents: { human: '-', count: 0 },
  versions: { human: '-' },
  media: { human: '-' },
  trash: { human: '-', count: 0 },
  by_type: {},
})
const quota = ref<any>({ max_mb: 0, unlimited: true, usage_percent: 0 })
const isAdmin = ref(false)

const percentColor = computed(() => {
  const p = quota.value.usage_percent || 0
  return p > 90 ? 'danger' : p > 75 ? 'warning' : 'success'
})

const typeRows = computed(() =>
  Object.entries(usage.value.by_type || {}).map(([type, bytes]) => ({
    type, bytes, human: humanSize(bytes as number),
  }))
)

function humanSize(b: number): string {
  if (b == null || Number.isNaN(b)) return '-'
  const unit = 1024
  if (b < unit) return `${b} B`
  const units = ['K', 'M', 'G', 'T', 'P']
  let i = -1
  let n = b
  do { n /= unit; i++ } while (n >= unit && i < units.length - 1)
  return `${n.toFixed(1)} ${units[i]}B`
}

async function load() {
  const { data } = await teamApi.get('/storage/status')
  usage.value = { ...usage.value, ...(data.usage || {}) }
  quota.value = data.quota || quota.value
  isAdmin.value = !!data.is_admin
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

.panel {
  background: #fff; border-radius: 16px; padding: 20px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04); margin-bottom: 16px;
}
.panel-title {
  font-size: 15px; font-weight: 600; color: #1a1a2e;
  margin-bottom: 16px; padding-bottom: 12px; border-bottom: 1px solid #f0f0f0;
}

.quota-stats { display: flex; gap: 40px; flex-wrap: wrap; }
.quota-item { display: flex; flex-direction: column; gap: 4px; }
.disk-label { font-size: 13px; color: #909399; }
.quota-value { font-size: 20px; font-weight: 700; color: #1a1a2e; }

.admin-hint { margin-top: 12px; font-size: 12px; color: #e6a23c; }

.stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
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

@media (max-width: 768px) {
  .stat-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
