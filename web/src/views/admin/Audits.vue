<template>
  <div>
    <h3 style="margin-bottom:16px">审计日志</h3>
    <el-form inline style="margin-bottom:16px">
      <el-form-item label="操作类型">
        <el-select v-model="filter.action" clearable>
          <el-option label="全部" value="" />
          <el-option label="登录" value="login" />
          <el-option label="创建" value="create_doc" />
          <el-option label="编辑" value="edit" />
          <el-option label="删除" value="delete" />
          <el-option label="权限变更" value="set_permission" />
        </el-select>
      </el-form-item>
      <el-form-item label="用户">
        <el-input v-model="filter.user_name" clearable placeholder="用户名" />
      </el-form-item>
      <el-form-item label="开始日期">
        <el-date-picker v-model="filter.start_date" type="date" value-format="YYYY-MM-DD" />
      </el-form-item>
      <el-form-item label="结束日期">
        <el-date-picker v-model="filter.end_date" type="date" value-format="YYYY-MM-DD" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="load">查询</el-button>
        <el-button @click="exportCSV">导出 CSV</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="audits" stripe>
      <el-table-column prop="created_at" label="时间" width="170" />
      <el-table-column prop="user_name" label="用户" width="100" />
      <el-table-column prop="action" label="操作" width="130">
        <template #default="{ row }">
          <el-tag :type="actionColor[row.action] || 'info'" size="small">
            {{ actionMap[row.action] || row.action }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="resource_type" label="资源类型" width="90">
        <template #default="{ row }">
          {{ row.resource_type === 'document' ? '文档' : row.resource_type === 'folder' ? '文件夹' : '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="resource_name" label="资源名称" min-width="200" />
      <el-table-column prop="ip" label="IP" width="130" />
      <el-table-column label="详情" width="100">
        <template #default="{ row }">
          <el-button link size="small" @click="showDetail(row)">查看</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="total, prev, pager, next"
      style="margin-top:12px"
      @current-change="load"
    />

    <el-dialog v-model="showDetailDialog" title="操作详情" width="400">
      <pre style="white-space:pre-wrap;word-break:break-all">{{ detail }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import http from '@/utils/http'

const actionMap: any = { login: '登录', logout: '登出', create_doc: '创建文档', edit: '编辑', delete: '删除', view: '查看', set_permission: '设置权限' }
const actionColor: any = { login: 'success', logout: 'info', create_doc: 'primary', edit: 'warning', delete: 'danger', set_permission: 'warning' }
const audits = ref<any[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const filter = ref({ action: '', user_name: '', start_date: '', end_date: '' })
const showDetailDialog = ref(false)
const detail = ref('')

async function load() {
  const params = { page: page.value, page_size: pageSize, ...filter.value }
  const { data } = await http.get('/audits', { params })
  audits.value = data.data || []
  // Note: backend may not return total; approximate
  total.value = audits.value.length < pageSize ? (page.value - 1) * pageSize + audits.value.length : page.value * pageSize + 1
}

async function exportCSV() {
  const params = { ...filter.value }
  const { data } = await http.get('/audits/export', { params, responseType: 'blob' })
  const url = URL.createObjectURL(data)
  const a = document.createElement('a')
  a.href = url
  a.download = 'audits.csv'
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('已导出')
}

function showDetail(row: any) {
  detail.value = row.detail || '{}'
  showDetailDialog.value = true
}

onMounted(load)
</script>