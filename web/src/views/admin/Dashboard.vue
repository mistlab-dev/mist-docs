<template>
  <div class="admin-page">
    <div class="page-header">
      <h2 class="page-title">系统概览</h2>
    </div>

    <!-- 统计卡片 -->
    <el-row :gutter="16" class="stat-cards">
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon" style="background:#409eff"><el-icon :size="24"><User /></el-icon></div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.users?.total || 0 }}</div>
            <div class="stat-label">用户</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon" style="background:#67c23a"><el-icon :size="24"><Document /></el-icon></div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.documents?.total || 0 }}</div>
            <div class="stat-label">文档</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon" style="background:#e6a23c"><el-icon :size="24"><Grid /></el-icon></div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.documents?.sheets || 0 }}</div>
            <div class="stat-label">表格</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-icon" style="background:#909399"><el-icon :size="24"><Delete /></el-icon></div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.trash || 0 }}</div>
            <div class="stat-label">回收站</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top:16px">
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card small">
          <div class="stat-info">
            <div class="stat-value">{{ stats.departments || 0 }}</div>
            <div class="stat-label">部门</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card small">
          <div class="stat-info">
            <div class="stat-value">{{ stats.shares || 0 }}</div>
            <div class="stat-label">分享链接</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card small">
          <div class="stat-info">
            <div class="stat-value">{{ stats.comments?.total || 0 }}</div>
            <div class="stat-label">评论</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12">
        <el-card shadow="hover" class="stat-card small">
          <div class="stat-info">
            <div class="stat-value">{{ stats.week_new || 0 }}</div>
            <div class="stat-label">本周新增</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top:16px">
      <!-- 每日新增图表 -->
      <el-col :span="12" :xs="24">
        <el-card>
          <template #header>近 7 天新增文档</template>
          <div class="chart-container">
            <div v-for="d in stats.daily_new" :key="d.date" class="chart-bar-wrapper">
              <div class="chart-bar" :style="{ height: barHeight(d.count) + 'px' }">
                <span class="chart-count">{{ d.count }}</span>
              </div>
              <div class="chart-label">{{ d.date.slice(5) }}</div>
            </div>
            <div v-if="!stats.daily_new?.length" class="no-data">暂无数据</div>
          </div>
        </el-card>
      </el-col>

      <!-- 最近活动 -->
      <el-col :span="12" :xs="24">
        <el-card>
          <template #header>最近活动</template>
          <div class="activity-list">
            <div v-for="a in stats.recent_activities" :key="a.created_at" class="activity-item">
              <el-tag size="small" :type="activityType(a.action)">{{ a.action }}</el-tag>
              <span class="activity-text">
                <strong>{{ a.user_name }}</strong>
                {{ a.action }}
                <em>{{ a.resource_name }}</em>
              </span>
              <span class="activity-time">{{ formatTime(a.created_at) }}</span>
            </div>
            <div v-if="!stats.recent_activities?.length" class="no-data">暂无活动</div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import http from '@/utils/http'

const stats = ref<any>({})

async function loadStats() {
  const { data } = await http.get('/admin/dashboard')
  stats.value = data.data || {}
}

const maxCount = computed(() => {
  const items = stats.value.daily_new || []
  return Math.max(...items.map((d: any) => d.count), 1)
})

function barHeight(count: number) {
  return Math.max((count / maxCount.value) * 120, 4)
}

function activityType(action: string) {
  if (action.includes('delete') || action.includes('remove')) return 'danger'
  if (action.includes('create') || action.includes('add')) return 'success'
  if (action.includes('update') || action.includes('edit')) return 'warning'
  return 'info'
}

function formatTime(t: string) {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前'
  return d.toLocaleDateString()
}

onMounted(loadStats)
</script>

<style scoped>
.admin-page { height: 100%; overflow-y: auto; }
.page-header { margin-bottom: 20px; }
.page-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }

.stat-cards .el-col { margin-bottom: 8px; }
.stat-card { display: flex; align-items: center; gap: 16px; padding: 8px; }
.stat-card.small { justify-content: center; }
.stat-icon { width: 48px; height: 48px; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #fff; }
.stat-info { flex: 1; text-align: center; }
.stat-value { font-size: 24px; font-weight: 700; color: #303133; }
.stat-label { font-size: 13px; color: #909399; margin-top: 4px; }

.stat-card :deep(.el-card__body) { display: flex; align-items: center; gap: 16px; width: 100%; }

.chart-container { display: flex; align-items: flex-end; gap: 8px; height: 160px; padding-top: 20px; }
.chart-bar-wrapper { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: flex-end; height: 100%; }
.chart-bar { width: 100%; background: linear-gradient(to top, #409eff, #79bbff); border-radius: 4px 4px 0 0; position: relative; min-height: 4px; transition: height 0.3s ease; }
.chart-count { position: absolute; top: -20px; left: 50%; transform: translateX(-50%); font-size: 12px; color: #606266; }
.chart-label { font-size: 11px; color: #909399; margin-top: 4px; }
.no-data { color: #c0c4cc; text-align: center; padding: 40px 0; }

.activity-list { max-height: 300px; overflow-y: auto; }
.activity-item { display: flex; align-items: center; gap: 8px; padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
.activity-item:last-child { border-bottom: none; }
.activity-text { flex: 1; font-size: 13px; color: #606266; }
.activity-text em { color: #409eff; font-style: normal; }
.activity-time { font-size: 12px; color: #c0c4cc; white-space: nowrap; }
</style>
