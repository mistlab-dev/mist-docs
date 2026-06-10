<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">{{ t('admin.dashboard.title') }}</h2>
        <span class="header-sub">{{ t('admin.dashboard.subtitle') }}</span>
      </div>
    </div>

    <!-- 主统计卡片 -->
    <div class="stat-grid">
      <div class="stat-card" v-for="s in mainStats" :key="s.label">
        <div class="stat-icon" :style="{ background: s.gradient }">
          <el-icon :size="22"><component :is="s.icon" /></el-icon>
        </div>
        <div class="stat-body">
          <div class="stat-value">{{ s.value }}</div>
          <div class="stat-label">{{ s.label }}</div>
        </div>
      </div>
    </div>

    <!-- 次要统计 -->
    <div class="stat-grid secondary">
      <div class="stat-card small" v-for="s in subStats" :key="s.label">
        <div class="stat-body">
          <div class="stat-value sm">{{ s.value }}</div>
          <div class="stat-label">{{ s.label }}</div>
        </div>
      </div>
    </div>

    <div class="charts-row">
      <!-- 每日新增图表 -->
      <div class="chart-panel">
        <div class="panel-header">{{ t('admin.dashboard.dailyNewTitle') }}</div>
        <div class="chart-body">
          <div class="chart-container">
            <div v-for="d in stats.daily_new" :key="d.date" class="chart-bar-wrapper">
              <span class="chart-count">{{ d.count }}</span>
              <div class="chart-bar" :style="{ height: barHeight(d.count) + 'px' }" />
              <div class="chart-label">{{ d.date.slice(5) }}</div>
            </div>
          </div>
          <div v-if="!stats.daily_new?.length" class="no-data">{{ t('common.noData') }}</div>
        </div>
      </div>

      <!-- 最近活动 -->
      <div class="chart-panel">
        <div class="panel-header">{{ t('admin.dashboard.recentActivity') }}</div>
        <div class="activity-list">
          <div v-for="a in stats.recent_activities" :key="a.created_at" class="activity-item">
            <div class="activity-avatar" :style="{ background: avatarColor(a.user_name) }">
              {{ a.user_name?.charAt(0) || '?' }}
            </div>
            <div class="activity-content">
              <div class="activity-text">
                <strong>{{ a.user_name }}</strong>
                {{ a.action }}
                <em>{{ a.resource_name }}</em>
              </div>
              <div class="activity-time">{{ formatTime(a.created_at) }}</div>
            </div>
          </div>
          <div v-if="!stats.recent_activities?.length" class="no-data">{{ t('admin.dashboard.noActivity') }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import http from '@/utils/http'
import teamApi from '@/utils/team-api'

const { t } = useI18n()

const stats = ref<any>({})

const mainStats = computed(() => [
  { label: t('admin.dashboard.users'), value: stats.value.users?.total || 0, icon: 'User', gradient: 'linear-gradient(135deg, #667eea, #764ba2)' },
  { label: t('admin.dashboard.documents'), value: stats.value.documents?.total || 0, icon: 'Document', gradient: 'linear-gradient(135deg, #36b37e, #00875a)' },
  { label: t('admin.dashboard.sheets'), value: stats.value.documents?.sheets || 0, icon: 'Grid', gradient: 'linear-gradient(135deg, #ff991f, #ff5630)' },
  { label: t('admin.dashboard.trash'), value: stats.value.trash || 0, icon: 'Delete', gradient: 'linear-gradient(135deg, #8993a4, #505f79)' },
])

const subStats = computed(() => [
  { label: t('admin.dashboard.departments'), value: stats.value.departments || 0 },
  { label: t('admin.dashboard.shareLinks'), value: stats.value.shares || 0 },
  { label: t('admin.dashboard.comments'), value: stats.value.comments?.total || 0 },
  { label: t('admin.dashboard.weekNew'), value: stats.value.week_new || 0 },
])

const maxCount = computed(() => {
  const items = stats.value.daily_new || []
  return Math.max(...items.map((d: any) => d.count), 1)
})

function barHeight(count: number) {
  return Math.max((count / maxCount.value) * 140, 4)
}

function avatarColor(name: string) {
  const colors = ['#4f6ef7', '#36b37e', '#ff991f', '#ff5630', '#6554c0', '#00b8d9', '#eb5286']
  let hash = 0
  for (let i = 0; i < (name || '').length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash)
  return colors[Math.abs(hash) % colors.length]
}

function formatTime(ts: string) {
  if (!ts) return ''
  const d = new Date(ts)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return t('common.justNow')
  if (diff < 3600000) return t('common.minutesAgo', [Math.floor(diff / 60000)])
  if (diff < 86400000) return t('common.hoursAgo', [Math.floor(diff / 3600000)])
  return d.toLocaleDateString()
}

async function loadStats() {
  const { data } = await teamApi.get('/dashboard')
  stats.value = data.data || {}
}

onMounted(loadStats)
</script>

<style scoped>
.admin-page { height: 100%; overflow-y: auto; padding: 20px; background: #f5f7fa; }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 24px; padding-bottom: 16px; border-bottom: 1px solid #e8ecf0;
}
.header-left { display: flex; align-items: baseline; gap: 16px; }
.page-title { font-size: 22px; font-weight: 600; color: #1a1a2e; margin: 0; letter-spacing: -0.02em; }
.header-sub { font-size: 14px; color: #909399; }

/* 统计卡片 */
.stat-grid {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 16px;
}
.stat-grid.secondary { gap: 12px; }

.stat-card {
  background: #fff; border-radius: 16px; padding: 20px;
  display: flex; align-items: center; gap: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
  transition: all 0.2s ease;
}
.stat-card:hover { transform: translateY(-2px); box-shadow: 0 6px 20px rgba(0,0,0,0.08); }

.stat-card.small { padding: 16px 20px; justify-content: center; }

.stat-icon {
  width: 48px; height: 48px; border-radius: 14px;
  display: flex; align-items: center; justify-content: center; color: #fff; flex-shrink: 0;
}

.stat-body { flex: 1; text-align: center; }
.stat-value { font-size: 28px; font-weight: 700; color: #1a1a2e; letter-spacing: -0.02em; }
.stat-value.sm { font-size: 22px; }
.stat-label { font-size: 13px; color: #909399; margin-top: 4px; font-weight: 500; }

/* 图表面板 */
.charts-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.chart-panel {
  background: #fff; border-radius: 16px; overflow: hidden;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.panel-header {
  padding: 16px 20px; font-size: 15px; font-weight: 600; color: #1a1a2e;
  border-bottom: 1px solid #f0f0f0;
}
.chart-body { padding: 20px; }

.chart-container {
  display: flex; align-items: flex-end; gap: 12px; height: 180px; padding-top: 24px;
}
.chart-bar-wrapper {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: flex-end; height: 100%;
}
.chart-bar {
  width: 100%; max-width: 48px;
  background: linear-gradient(to top, #667eea, #764ba2);
  border-radius: 8px 8px 0 0; position: relative; min-height: 4px;
  transition: height 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}
.chart-count { font-size: 13px; color: #606266; margin-bottom: 6px; font-weight: 600; }
.chart-label { font-size: 12px; color: #909399; margin-top: 8px; }

.no-data { color: #c0c4cc; text-align: center; padding: 60px 0; font-size: 14px; }

/* 活动列表 */
.activity-list { max-height: 340px; overflow-y: auto; padding: 8px 12px; }
.activity-item {
  display: flex; align-items: flex-start; gap: 12px; padding: 12px 0;
  border-bottom: 1px solid #f5f5f5;
}
.activity-item:last-child { border-bottom: none; }

.activity-avatar {
  width: 32px; height: 32px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 13px; font-weight: 600; flex-shrink: 0;
}

.activity-content { flex: 1; min-width: 0; }
.activity-text { font-size: 14px; color: #606266; line-height: 1.5; }
.activity-text strong { color: #1a1a2e; font-weight: 600; }
.activity-text em { color: #4f6ef7; font-style: normal; font-weight: 500; }
.activity-time { font-size: 12px; color: #c0c4cc; margin-top: 2px; }

@media (max-width: 768px) {
  .stat-grid { grid-template-columns: repeat(2, 1fr); }
  .charts-row { grid-template-columns: 1fr; }
}
</style>
