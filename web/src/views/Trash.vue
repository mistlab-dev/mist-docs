<template>
  <div>
    <h3>回收站</h3>
    <el-table :data="trash" stripe>
      <el-table-column prop="title" label="名称" min-width="300" />
      <el-table-column prop="type" label="类型" width="80">
        <template #default="{ row }">
          <el-tag size="small">{{ row.type === 'doc' ? '文档' : '表格' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" label="删除时间" width="170" />
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="restore(row)">恢复</el-button>
          <el-button link type="danger" size="small" @click="purge(row)">永久删除</el-button>
        </template>
      </el-table-column>
    </el-table>
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

onMounted(load)
</script>
