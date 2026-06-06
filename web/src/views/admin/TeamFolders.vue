<template>
  <div class="team-folders">
    <div class="header">
      <h2>文件夹管理</h2>
      <el-button type="primary" @click="showCreate = true">新建文件夹</el-button>
    </div>

    <el-tree
      :data="tree"
      node-key="id"
      default-expand-all
      :props="{ label: 'name', children: 'children' }"
    >
      <template #default="{ node, data }">
        <div class="tree-node">
          <span>{{ data.name }}</span>
          <span class="actions">
            <el-button size="small" link @click="editFolder(data)">编辑</el-button>
            <el-button size="small" link type="danger" @click="deleteFolder(data)">删除</el-button>
          </span>
        </div>
      </template>
    </el-tree>

    <el-dialog v-model="showCreate" title="新建文件夹" width="400px">
      <el-form>
        <el-form-item label="名称">
          <el-input v-model="newFolder.name" />
        </el-form-item>
        <el-form-item label="父文件夹">
          <el-select v-model="newFolder.parent_id" clearable placeholder="根目录">
            <el-option v-for="f in flatFolders" :key="f.id" :label="f.name" :value="f.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" @click="createFolder">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import http from '@/utils/http'
import { ElMessage, ElMessageBox } from 'element-plus'

const auth = useAuthStore()
const tree = ref<any[]>([])
const showCreate = ref(false)
const newFolder = ref({ name: '', parent_id: '' })

const flatFolders = computed(() => {
  const result: any[] = []
  function walk(nodes: any[]) {
    for (const n of nodes) {
      result.push(n)
      if (n.children) walk(n.children)
    }
  }
  walk(tree.value)
  return result
})

async function loadTree() {
  const teamId = auth.currentTeamId
  if (!teamId) return
  const { data } = await http.get(`/teams/${teamId}/folders/tree`)
  tree.value = data.data || []
}

async function createFolder() {
  if (!newFolder.value.name) {
    ElMessage.warning('请输入名称')
    return
  }
  const teamId = auth.currentTeamId
  await http.post(`/teams/${teamId}/folders`, newFolder.value)
  ElMessage.success('已创建')
  showCreate.value = false
  newFolder.value = { name: '', parent_id: '' }
  loadTree()
}

async function editFolder(data: any) {
  const { value } = await ElMessageBox.prompt('新名称', '编辑文件夹', {
    inputValue: data.name,
  })
  const teamId = auth.currentTeamId
  await http.put(`/teams/${teamId}/folders/${data.id}`, { name: value })
  ElMessage.success('已更新')
  loadTree()
}

async function deleteFolder(data: any) {
  await ElMessageBox.confirm(`确定删除「${data.name}」？子文件夹将被移到根目录。`, '确认')
  const teamId = auth.currentTeamId
  await http.delete(`/teams/${teamId}/folders/${data.id}`)
  ElMessage.success('已删除')
  loadTree()
}

onMounted(loadTree)
</script>

<style scoped>
.team-folders { padding: 20px; }
.header { display: flex; justify-content: space-between; margin-bottom: 20px; }
.tree-node { display: flex; justify-content: space-between; width: 100%; }
.actions { display: none; }
.tree-node:hover .actions { display: inline; }
</style>