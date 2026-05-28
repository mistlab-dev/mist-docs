<template>
  <div>
    <h3 style="margin-bottom:16px">存储监控</h3>

    <!-- 健康状态卡片 -->
    <el-card style="margin-bottom:16px">
      <div style="display:flex;align-items:center;gap:12px">
        <el-icon :size="32" :color="healthColor">
          <SuccessFilled v-if="health === 'healthy'" />
          <WarningFilled v-else-if="health === 'warning'" />
          <CircleCloseFilled v-else />
        </el-icon>
        <div>
          <div style="font-size:20px;font-weight:bold">
            {{ health === 'healthy' ? '系统健康' : health === 'warning' ? '需要注意' : '存在问题' }}
          </div>
          <div style="color:#999">
            {{ storageRoot }}
          </div>
        </div>
      </div>
    </el-card>

    <!-- 磁盘 + 加密 -->
    <el-row :gutter="16" style="margin-bottom:16px">
      <el-col :span="12">
        <el-card>
          <template #header>磁盘使用</template>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="总容量">{{ disk.total_human }}</el-descriptions-item>
            <el-descriptions-item label="已使用">{{ disk.used_human }}</el-descriptions-item>
            <el-descriptions-item label="可用">{{ disk.available_human }}</el-descriptions-item>
            <el-descriptions-item label="使用率">
              <el-progress :percentage="disk.usage_percent" :color="disk.usage_percent > 90 ? '#f56c6c' : disk.usage_percent > 75 ? '#e6a23c' : '#67c23a'" />
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>加密状态</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="加密启用">
              <el-tag :type="encryption.enabled ? 'success' : 'danger'" size="small">
                {{ encryption.enabled ? 'AES-256-GCM 已启用' : '未启用' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="算法">{{ encryption.algorithm || '-' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>

    <!-- 文件统计 -->
    <el-card style="margin-bottom:16px">
      <template #header>文件统计</template>
      <el-row :gutter="16">
        <el-col :span="4">
          <div style="text-align:center">
            <div style="font-size:28px;font-weight:bold;color:#409eff">{{ files.total_files }}</div>
            <div style="color:#999">文件总数</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div style="text-align:center">
            <div style="font-size:28px;font-weight:bold">{{ files.total_size_human }}</div>
            <div style="color:#999">占用空间</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div style="text-align:center">
            <div style="font-size:28px;font-weight:bold;color:#67c23a">{{ files.current_files }}</div>
            <div style="color:#999">当前版本</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div style="text-align:center">
            <div style="font-size:28px;font-weight:bold">{{ files.version_files }}</div>
            <div style="color:#999">历史版本</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div style="text-align:center">
            <div style="font-size:28px;font-weight:bold;color:#e6a23c">{{ files.yjs_state_files }}</div>
            <div style="color:#999">协同状态</div>
          </div>
        </el-col>
        <el-col :span="4">
          <div style="text-align:center">
            <div style="font-size:28px;font-weight:bold;color:#f56c6c">{{ files.trash_files }}</div>
            <div style="color:#999">回收站</div>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- 部门占用 -->
    <el-card style="margin-bottom:16px">
      <template #header>部门存储分布</template>
      <el-table :data="files.departments" stripe size="small">
        <el-table-column prop="department_id" label="部门ID" width="280" />
        <el-table-column prop="file_count" label="文件数" width="100" />
        <el-table-column prop="document_count" label="文档数" width="100" />
        <el-table-column prop="total_size_human" label="占用" width="100" />
      </el-table>
    </el-card>

    <!-- 健康检查 -->
    <el-card>
      <template #header>健康检查</template>
      <el-table :data="checks" stripe size="small">
        <el-table-column prop="name" label="检查项" width="120">
          <template #default="{ row }">
            {{ checkNames[row.name] || row.name }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="statusColor[row.status]" size="small">{{ statusText[row.status] }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="detail" label="详情" />
      </el-table>

      <div v-if="warnings.length" style="margin-top:12px">
        <el-alert v-for="w in warnings" :key="w" type="warning" :closable="false" style="margin-bottom:4px">{{ w }}</el-alert>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import http from '@/utils/http'

const checkNames: any = {
  disk_usage: '磁盘用量',
  storage_root: '存储目录',
  write_permission: '写入权限',
  encryption: '加密状态',
  trash: '回收站',
  versions: '版本数量',
  config: '配置完整性',
}
const statusText: any = { ok: '正常', warn: '注意', error: '异常' }
const statusColor: any = { ok: 'success', warn: 'warning', error: 'danger' }

const storageRoot = ref('')
const disk = ref<any>({ total_human: '-', used_human: '-', available_human: '-', usage_percent: 0 })
const files = ref<any>({ total_files: 0, total_size_human: '-', departments: [] })
const encryption = ref<any>({ enabled: false })
const checks = ref<any[]>([])
const warnings = ref<string[]>([])
const health = ref('healthy')

const healthColor = computed(() => health.value === 'healthy' ? '#67c23a' : health.value === 'warning' ? '#e6a23c' : '#f56c6c')

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