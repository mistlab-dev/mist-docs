<template>
  <div class="trash-page">
    <div class="trash-header">
      <div class="header-left">
        <h2 class="page-title">回收站</h2>
        <span class="doc-count" v-if="trash.length">{{ trash.length }} 个文档</span>
      </div>
      <div class="header-right">
        <el-button v-if="trash.length" type="danger" plain size="small" @click="emptyTrash">
          <el-icon><Delete /></el-icon> 清空回收站
        </el-button>
      </div>
    </div>

    <div v-if="!trash.length" class="empty-state">
      <svg class="empty-svg" viewBox="0 0 200 160" fill="none">
        <path d="M60 50 L80 30 L120 30 L140 50 L160 50 L160 140 L40 140 L40 50 Z" fill="#f0f2f5" stroke="#dcdfe6" stroke-width="1.5"/>
        <path d="M80 30 L80 50 L120 50 L120 30" fill="none" stroke="#dcdfe6" stroke-width="1.5"/>
        <line x1="85" y1="75" x2="115" y2="75" stroke="#dcdfe6" stroke-width="2" stroke-linecap="round"/>
        <line x1="85" y1="90" x2="115" y2="90" stroke="#dcdfe6" stroke-width="2" stroke-linecap="round"/>
        <line x1="85" y1="105" x2="105" y2="105" stroke="#dcdfe6" stroke-width="2" stroke-linecap="round"/>
      </svg>
      <p class="empty-title">回收站为空</p>
      <p class="empty-desc">删除的文档会在这里保留 30 天</p>
    </div>

    <div v-else class="trash-table">
      <div class="table-row table-header-row">
        <div class="table-col col-title">名称</div>
        <div class="table-col col-type">类型</div>
        <div class="table-col col-time">删除时间</div>
        <div class="table-col col-actions">操作</div>
      </div>
      <div v-for="item in trash" :key="item.id" class="table-row">
        <div class="table-col col-title">
          <div class="title-icon" :class="item.type">
            <svg v-if="item.type === 'doc'" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L2.414 12.586A2 2 0 014 12h4.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg>
            <svg v-else viewBox="0 0 20 20" fill="currentColor"><path d="M3 3h14a1 1 0 011 1v12a1 1 0 01-1 1H3a1 1 0 01-1-1V4a1 1 0 011-1zm1 2v2h4V5H4zm6 0v2h4V5h-4zm-6 4v2h4V9H4zm6 0v2h4V9h-4z"/></svg>
          </div>
          <span class="title-text">{{ item.title }}</span>
        </div>
        <div class="table-col col-type">
          <el-tag :type="item.type === 'doc' ? 'info' : 'success'" size="small" effect="plain" round>
            {{ item.type === 'doc' ? '文档' : '表格' }}
          </el-tag>
        </div>
        <div class="table-col col-time">{{ formatTime(item.updated_at) }}</div>
        <div class="table-col col-actions">
          <el-button link type="primary" size="small" @click="restore(item)">
            <svg class="act-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M.5 1.163a1.5 1.5 0 011.5-1.163h5a1.5 1.5 0 010 3h-1.5l3.5 3.5a1.5 1.5 0 01-2.122 2.122L3.378 5.122v1.5a1.5 1.5 0 01-3 0V1.163z" transform="translate(4, 4) scale(0.8)"/></svg>
            恢复
          </el-button>
          <el-button link type="danger" size="small" @click="purge(item)">永久删除</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import http from '@/utils/http'

const trash = ref<any[]>([])

async function load() {
  const { data } = await http.get('/docs/trash')
  trash.value = data.data || []
}

async function restore(row: any) {
  await http.post(`/docs/trash/restore/${row.id}`)
  ElMessage.success('已恢复')
  load()
}

async function purge(row: any) {
  await ElMessageBox.confirm(`永久删除「${row.title}」？不可恢复！`, '危险操作', { type: 'warning' })
  await http.delete(`/docs/trash/purge/${row.id}`)
  ElMessage.success('已永久删除')
  load()
}

async function emptyTrash() {
  await ElMessageBox.confirm('清空回收站？所有文档将永久删除！', '危险操作', { type: 'error' })
  let ok = 0
  for (const item of trash.value) {
    try { await http.delete(`/docs/trash/purge/${item.id}`); ok++ } catch {}
  }
  ElMessage.success(`已永久删除 ${ok} 个文档`)
  load()
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前'
  if (diff < 604800000) return Math.floor(diff / 86400000) + ' 天前'
  return d.toLocaleDateString('zh-CN')
}

onMounted(load)
</script>

<style scoped>
.trash-page { height: 100%; display: flex; flex-direction: column; }
.trash-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px;
}
.header-left { display: flex; align-items: baseline; gap: 8px; }
.page-title { font-size: 20px; font-weight: 600; color: #1a1a2e; margin: 0; }
.doc-count { font-size: 13px; color: #999; }

.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; flex: 1; padding: 60px 20px;
}
.empty-svg { width: 200px; height: 160px; margin-bottom: 20px; }
.empty-title { font-size: 16px; color: #555; margin: 0 0 4px; }
.empty-desc { font-size: 13px; color: #999; margin: 0; }

.trash-table {
  background: #fff; border-radius: 10px; border: 1px solid #e8ecf0; flex: 1; overflow-y: auto;
}
.table-row {
  display: flex; align-items: center; padding: 0 16px; height: 48px;
  border-bottom: 1px solid #f0f2f5; transition: background 0.15s;
}
.table-row:last-child { border-bottom: none; }
.table-row:hover { background: #f8f9fb; }
.table-header-row {
  background: #fafbfc; font-size: 12px; font-weight: 600; color: #909399;
  border-bottom: 1px solid #e8ecf0;
}
.table-header-row:hover { background: #fafbfc; }
.table-col { flex-shrink: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.col-title { flex: 1; min-width: 0; display: flex; align-items: center; gap: 8px; font-size: 14px; color: #333; }
.col-type { width: 72px; text-align: center; }
.col-time { width: 120px; color: #999; font-size: 13px; }
.col-actions { width: 140px; text-align: right; display: flex; gap: 4px; justify-content: flex-end; }

.title-icon {
  width: 28px; height: 28px; border-radius: 6px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.title-icon svg { width: 16px; height: 16px; }
.title-icon.doc { background: #ecf5ff; color: #409eff; }
.title-icon.sheet { background: #f0f9eb; color: #67c23a; }
.title-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 768px) {
  .col-type { display: none; }
  .col-time { width: auto; font-size: 12px; }
  .table-row { height: 52px; padding: 0 10px; }
}
</style>
