<template>
  <div class="trash-page">
    <!-- 页头 -->
    <div class="page-header">
      <div class="header-info">
        <h2 class="page-title">回收站</h2>
        <span v-if="trash.length" class="page-count">{{ trash.length }} 个文档</span>
      </div>
      <div class="header-actions">
        <el-input v-model="searchKey" placeholder="搜索..." style="width:180px" clearable size="default">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button v-if="trash.length" type="danger" plain @click="emptyTrash">
          <el-icon><Delete /></el-icon> 清空回收站
        </el-button>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-if="!filteredTrash.length && !loading" class="empty-state">
      <div class="empty-icon">🗑️</div>
      <p v-if="searchKey" class="empty-title">没有匹配的文档</p>
      <template v-else>
        <p class="empty-title">回收站为空</p>
        <p class="empty-desc">删除的文档会在这里保留 30 天</p>
      </template>
    </div>

    <!-- 加载 -->
    <div v-if="loading" class="loading-state">
      <el-skeleton :rows="6" animated />
    </div>

    <!-- 列表 -->
    <div v-else-if="filteredTrash.length" class="trash-card">
      <el-table
        :data="filteredTrash"
        :header-cell-style="{ background: '#fafbfc', color: '#5a5f6b', fontWeight: 500, fontSize: '13px' }"
        :cell-style="{ fontSize: '14px' }"
        :row-style="{ height: '56px' }"
      >
        <el-table-column label="名称" min-width="280">
          <template #default="{ row }">
            <div class="doc-title">
              <div class="type-icon" :class="row.type">
                <el-icon :size="14"><Document v-if="row.type === 'doc'" /><Grid v-else /></el-icon>
              </div>
              <span class="title-text">{{ row.title }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.type === 'doc' ? '' : 'success'" size="small" effect="light" round>
              {{ row.type === 'doc' ? '文档' : '表格' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="删除时间" width="160">
          <template #default="{ row }">
            <span class="time-text">{{ formatTime(row.updated_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="" width="180" fixed="right" align="center">
          <template #default="{ row }">
            <div class="row-actions">
              <el-button link type="primary" @click="restore(row)">
                <el-icon><RefreshRight /></el-icon> 恢复
              </el-button>
              <el-button link type="danger" @click="purge(row)">永久删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import http from '@/utils/http'

const trash = ref<any[]>([])
const searchKey = ref('')
const loading = ref(false)

const filteredTrash = computed(() => {
  if (!searchKey.value) return trash.value
  const q = searchKey.value.toLowerCase()
  return trash.value.filter(t => t.title.toLowerCase().includes(q))
})

async function load() {
  loading.value = true
  try {
    const { data } = await http.get('/docs/trash')
    trash.value = data.data || []
  } finally { loading.value = false }
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
.trash-page {
  height: 100%; display: flex; flex-direction: column;
  padding: 20px; background: #f5f7fa;
}

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 16px; flex-wrap: wrap; gap: 12px;
}
.header-info { display: flex; align-items: baseline; gap: 10px; }
.page-title { font-size: 22px; font-weight: 700; color: #1a1a2e; margin: 0; }
.page-count { font-size: 13px; color: #909399; }
.header-actions { display: flex; gap: 8px; align-items: center; }

.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; flex: 1; padding: 80px 20px;
}
.empty-icon { font-size: 56px; margin-bottom: 16px; opacity: 0.5; }
.empty-title { font-size: 16px; color: #606266; margin: 0 0 4px; }
.empty-desc { font-size: 13px; color: #909399; margin: 0; }

.loading-state { padding: 40px; }

.trash-card {
  background: #fff; border-radius: 16px; overflow: hidden;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04); flex: 1;
}
.trash-card :deep(.el-table__row:hover) { background: #f9fbff !important; }
.trash-card :deep(.el-table__cell) { padding: 12px 0; }

.doc-title { display: flex; align-items: center; gap: 10px; }
.type-icon {
  width: 28px; height: 28px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.type-icon.doc { background: #e8f0fe; color: #4f6ef7; }
.type-icon:not(.doc) { background: #e6f7f0; color: #36b37e; }
.title-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 500; }
.time-text { font-size: 13px; color: #909399; }
.row-actions { display: flex; align-items: center; gap: 4px; }

@media (max-width: 768px) {
  .trash-page { padding: 12px; }
  .header-actions { width: 100%; }
  .header-actions .el-input { flex: 1; }
}
</style>
