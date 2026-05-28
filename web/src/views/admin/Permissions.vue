<template>
  <div>
    <h3 style="margin-bottom:16px">权限管理</h3>
    <el-form inline style="margin-bottom:16px">
      <el-form-item label="资源类型">
        <el-select v-model="filter.resource_type">
          <el-option label="文档" value="document" />
          <el-option label="文件夹" value="folder" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="load">查询</el-button>
      </el-form-item>
    </el-form>

    <el-button type="primary" @click="showForm = true" style="margin-bottom:12px">
      <el-icon><Plus /></el-icon> 设置权限
    </el-button>

    <el-table :data="perms" stripe>
      <el-table-column prop="resource_type" label="类型" width="100">
        <template #default="{ row }">
          <el-tag size="small">{{ row.resource_type === 'document' ? '文档' : '文件夹' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="resource_id" label="资源ID" width="280" />
      <el-table-column prop="target_type" label="对象" width="80">
        <template #default="{ row }">
          <el-tag size="small">{{ row.target_type === 'user' ? '用户' : '部门' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="target_id" label="对象ID" width="280" />
      <el-table-column prop="permission" label="权限" width="100">
        <template #default="{ row }">
          <el-tag :type="permColor[row.permission]" size="small">{{ permMap[row.permission] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button link type="danger" size="small" @click="del(row)">移除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showForm" title="设置权限" width="450">
      <el-form :model="form" label-width="80px">
        <el-form-item label="资源类型">
          <el-select v-model="form.resource_type">
            <el-option label="文档" value="document" />
            <el-option label="文件夹" value="folder" />
          </el-select>
        </el-form-item>
        <el-form-item label="资源ID">
          <el-input v-model="form.resource_id" />
        </el-form-item>
        <el-form-item label="对象类型">
          <el-select v-model="form.target_type">
            <el-option label="用户" value="user" />
            <el-option label="部门" value="department" />
          </el-select>
        </el-form-item>
        <el-form-item label="对象ID">
          <el-input v-model="form.target_id" />
        </el-form-item>
        <el-form-item label="权限级别">
          <el-select v-model="form.permission">
            <el-option label="只读" value="read" />
            <el-option label="读写" value="write" />
            <el-option label="管理" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="继承">
          <el-switch v-model="form.inherit" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForm = false">取消</el-button>
        <el-button type="primary" @click="submit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import http from '@/utils/http'

const permMap: any = { read: '只读', write: '读写', admin: '管理' }
const permColor: any = { read: 'info', write: 'warning', admin: 'danger' }
const perms = ref<any[]>([])
const filter = ref({ resource_type: 'document' })
const showForm = ref(false)
const form = ref({
  resource_type: 'document',
  resource_id: '',
  target_type: 'user',
  target_id: '',
  permission: 'read',
  inherit: true,
})

async function load() {
  const { data } = await http.get('/permissions', { params: filter.value })
  perms.value = data.data || []
}

async function submit() {
  await http.post('/permissions', form.value)
  ElMessage.success('已设置')
  showForm.value = false
  load()
}

async function del(row: any) {
  await ElMessageBox.confirm('移除此权限？', '确认')
  await http.delete(`/permissions/${row.id}`)
  ElMessage.success('已移除')
  load()
}

onMounted(load)
</script>