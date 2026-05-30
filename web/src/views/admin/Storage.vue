<template>
  <div class="admin-page">
    <div class="page-header">
      <h2 class="page-title">存储监控</h2>
    </div>

    <!-- 健康状态 -->
    <div class="health-card" :class="health">
      <div class="health-icon">
        <svg v-if="health === 'healthy'" viewBox="0 0 24 24" fill="none" stroke="#67c23a" stroke-width="2"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
        <svg v-else-if="health === 'warning'" viewBox="0 0 24 24" fill="none" stroke="#e6a23c" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="#f56c6c" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
      </div>
      <div class="health-info">
        <div class="health-title">{{ health === 'healthy' ? '系统健康' : health === 'warning' ? '需要注意' : '存在问题' }}</div>
        <div class="health-path">{{ storageRoot }}</div>
      </div>
    </div>

    <!-- 磁盘 + 加密 -->
    <el-row :gutter="16" style="margin-bottom:16px">
      <el-col :span="12" :xs="24" style="margin-bottom:8px">
        <div class="info-card">
          <div class="card-label">磁盘使用</div>
          <div class="disk-row"><span class="disk-key">总容量</span><span class="disk-val">{{ disk.total_human }}</span></div>
          <div class="disk-row"><span class="disk-key">已使用</span><span class="disk-val">{{ disk.used_human }}</span></div>
          <div class="disk-row"><span class="disk-key">可用</span><span class="disk-val">{{ disk.available_human }}</span></div>
          <el-progress :percentage="disk.usage_percent" :stroke-width="8" :color="disk.usage_percent > 90 ? '#f56c6c' : disk.usage_percent > 75 ? '#e6a23c' : '#67c23a'" style="margin-top:12px" />
        </div>
      </el-col>
      <el-col :span="12" :xs="24" style="margin-bottom:8px">
        <div class="info-card">
          <div class="card-label">加密状态</div>
          <div style="margin-top:16px;text-align:center">
            <el-tag :type="encryption.enabled ? 'success' : 'danger'" effect="dark" round size="large">
              {{ encryption.enabled ? 'AES-256-GCM 已启用' : '未启用' }}
            </el-tag>
          </div>
          <div class="disk-row" style="margin-top:16px"><span class="disk-key">算法</span><span class="disk-val">{{ encryption.algorithm || '-' }}</span></div>
        </div>
      </el-col>
    </el-row>

    <!-- 文件统计 -->
    <div class="stats-grid">
      <div class="stat-item"><div class="stat-val" style="color:#409eff">{{ files.total_files }}</div><div class="stat-lbl">文件总数</div></div>
      <div class="stat-item"><div class="stat-val">{{ files.total_size_human }}</div><div class="stat-lbl">占用空间</div></div>
      <div class="stat-item"><div class="stat-val" style="color:#67c23a">{{ files.current_files }}</div><div class="stat-lbl">当前版本</div></div>
      <div class="stat-item"><div class="stat-val">{{ files.version_files }}</div><div class="stat-lbl">历史版本</div></div>
      <div class="stat-item"><div class="stat-val" style="color:#e6a23c">{{ files.yjs_state_files }}</div><div class="stat-lbl">协同状态</div></div>
      <div class="stat-item"><div class="stat-val" style="color:#f56c6c">{{ files.trash_files }}</div><div class="stat-lbl">回收站</div></div>
    </div>

    <!-- 部门占用 -->
    <div class="table-card" style="margin-top:16px">
      <div class="card-label" style="padding:12px 16px 0">部门存储分布</div>
      <el-table :data="files.departments" stripe size="small">
        <el-table-column prop="department_id" label="部门 ID" min-width="240" show-overflow-tooltip />
        <el-table-column prop="file_count" label="文件数" width="100" />
        <el-table-column prop="document_count" label="文档数" width="100" />
        <el-table-column prop="total_size_human" label="占用" width="100" />
      </el-table>
    </div>

    <!-- 健康检查 -->
    <div class="table-card" style="margin-top:16px">
      <div class="card-label" style="padding:12px 16px 0">健康检查</div>
      <el-table :data="checks" stripe size="small">
        <el-table-column prop="name" label="检查项" width="120">
          <template #default="{ row }">{{ checkNames[row.name] || row.name }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }">
            <span class="status-dot" :class="row.status === 'ok' ? 'active' : row.status === 'warn' ? 'warn' : 'error'" />
          </template>
        </el-table-column>
        <el-table-column prop="detail" label="详情" />
      </el-table>
      <div v-if="warnings.length" style="padding:8px 16px">
        <el-alert v-for="w in warnings" :key="w" type="warning" :closable="false" style="margin-bottom:4px">{{ w }}</el-alert>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
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
.admin-page { height: 100%; overflow-y: auto; padding: 0 4px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.page-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }

.health-card {
  display: flex; align-items: center; gap: 16px;
  padding: 20px 24px; border-radius: 12px; margin-bottom: 16px;
  border: 1px solid #e8ecf0;
}
.health-card.healthy { background: linear-gradient(135deg, #f0f9eb, #fff); }
.health-card.warning { background: linear-gradient(135deg, #fdf6ec, #fff); }
.health-card:not(.healthy):not(.warning) { background: linear-gradient(135deg, #fef0f0, #fff); }
.health-icon svg { width: 40px; height: 40px; }
.health-title { font-size: 18px; font-weight: 600; color: #303133; }
.health-path { font-size: 13px; color: #909399; margin-top: 4px; }

.info-card {
  background: #fff; border-radius: 10px; border: 1px solid #e8ecf0;
  padding: 16px 20px; height: 100%;
}
.card-label { font-size: 14px; font-weight: 600; color: #303133; margin-bottom: 12px; }

.disk-row { display: flex; justify-content: space-between; padding: 4px 0; }
.disk-key { color: #909399; font-size: 13px; }
.disk-val { color: #303133; font-size: 13px; font-weight: 500; }

.stats-grid {
  display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px;
}
.stat-item {
  background: #fff; border: 1px solid #e8ecf0; border-radius: 10px;
  padding: 16px; text-align: center;
}
.stat-val { font-size: 22px; font-weight: 700; color: #303133; }
.stat-lbl { font-size: 12px; color: #909399; margin-top: 4px; }

.table-card { background: #fff; border-radius: 10px; border: 1px solid #e8ecf0; overflow: hidden; }

.status-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; }
.status-dot.active { background: #67c23a; box-shadow: 0 0 0 2px rgba(103,194,58,0.2); }
.status-dot.warn { background: #e6a23c; box-shadow: 0 0 0 2px rgba(230,162,60,0.2); }
.status-dot.error { background: #f56c6c; box-shadow: 0 0 0 2px rgba(245,108,108,0.2); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(3, 1fr); }
}
</style>
