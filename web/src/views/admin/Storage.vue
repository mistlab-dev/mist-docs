<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">存储监控</h2>
        <el-tag :type="health === 'healthy' ? 'success' : health === 'warning' ? 'warning' : 'danger'" effect="light" round size="small">
          {{ health === 'healthy' ? '健康' : health === 'warning' ? '注意' : '异常' }}
        </el-tag>
      </div>
      <el-button size="default" @click="load">
        <el-icon><RefreshRight /></el-icon> 刷新
      </el-button>
    </div>

    <!-- 磁盘 + 加密 -->
    <div class="top-grid">
      <div class="panel disk-panel">
        <div class="panel-title">磁盘使用</div>
        <div class="disk-stats">
          <div class="disk-item">
            <span class="disk-label">已用</span>
            <span class="disk-value">{{ disk.used_human }}</span>
          </div>
          <div class="disk-item">
            <span class="disk-label">可用</span>
            <span class="disk-value">{{ disk.available_human }}</span>
          </div>
          <div class="disk-item">
            <span class="disk-label">总计</span>
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
        <div class="panel-title">加密状态</div>
        <div class="encrypt-badge">
          <el-tag :type="encryption.enabled ? 'success' : 'danger'" effect="dark" round size="large">
            {{ encryption.enabled ? '🔒 AES-256-GCM 已启用' : '🔓 未启用加密' }}
          </el-tag>
        </div>
        <div class="encrypt-detail">
          <span class="disk-label">算法</span>
          <span class="disk-value">{{ encryption.algorithm || '—' }}</span>
        </div>
        <div class="encrypt-detail">
          <span class="disk-label">存储路径</span>
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
      <div class="panel-title">部门存储分布</div>
      <el-table :data="files.departments" size="small"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
      >
        <el-table-column prop="department_id" label="部门 ID" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="mono-id">{{ row.department_id }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="file_count" label="文件数" width="100" align="center" />
        <el-table-column prop="document_count" label="文档数" width="100" align="center" />
        <el-table-column prop="total_size_human" label="占用" width="100" align="center">
          <template #default="{ row }">
            <span class="size-text">{{ row.total_size_human }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 健康检查 -->
    <div class="panel">
      <div class="panel-title">健康检查</div>
      <div class="check-list">
        <div v-for="c in checks" :key="c.name" class="check-item">
          <div class="check-status" :class="c.status" />
          <span class="check-name">{{ checkNames[c.name] || c.name }}</span>
          <span class="check-detail">{{ c.detail }}</span>
        </div>
      </div>
      <div v-if="warnings.length" class="warnings">
        <div v-for="w in warnings" :key="w" class="warning-item">⚠️ {{ w }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import http from '@/utils/http'

const checkNames: any = {
  disk_usage: '磁盘用量', storage_root: '存储目录', write_permission: '写入权限',
  encryption: '加密状态', trash: '回收站', versions: '版本数量', config: '配置完整性',
}

const storageRoot = ref('')
const disk = ref<any>({ total_human: '-', used_human: '-', available_human: '-', usage_percent: 0 })
const files = ref<any>({ total_files: 0, total_size_human: '-', departments: [] })
const encryption = ref<any>({ enabled: false })
const checks = ref<any[]>([])
const warnings = ref<string[]>([])
const health = ref('healthy')

const fileStats = computed(() => [
  { label: '文件总数', value: files.value.total_files, color: '#4f6ef7' },
  { label: '占用空间', value: files.value.total_size_human, color: '#6554c0' },
  { label: '当前版本', value: files.value.current_files, color: '#36b37e' },
  { label: '历史版本', value: files.value.version_files, color: '#00b8d9' },
  { label: '协同状态', value: files.value.yjs_state_files, color: '#ff991f' },
  { label: '回收站', value: files.value.trash_files, color: '#ff5630' },
])

async function load() {
  const { data } = await http.get('/storage/status')
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
