<template>
  <div class="team-folders">
    <div class="header">
      <h2>{{ t('admin.teamFolders.title') }}</h2>
      <el-button type="primary" @click="showCreate = true">{{ t('admin.teamFolders.newFolder') }}</el-button>
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
            <el-button size="small" link @click="editFolder(data)">{{ t('common.edit') }}</el-button>
            <el-button size="small" link type="danger" @click="deleteFolder(data)">{{ t('common.delete') }}</el-button>
          </span>
        </div>
      </template>
    </el-tree>

    <el-dialog v-model="showCreate" :title="t('admin.teamFolders.newFolder')" width="400px">
      <el-form>
        <el-form-item :label="t('common.name')">
          <el-input v-model="newFolder.name" />
        </el-form-item>
        <el-form-item :label="t('admin.teamFolders.parentFolder')">
          <el-select v-model="newFolder.parent_id" clearable :placeholder="t('admin.teamFolders.rootFolder')">
            <el-option v-for="f in flatFolders" :key="f.id" :label="f.name" :value="f.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createFolder">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import teamApi from '@/utils/team-api'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = useI18n()

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
  const { data } = await teamApi.get('/folders/tree')
  tree.value = data.data || []
}

async function createFolder() {
  if (!newFolder.value.name) {
    ElMessage.warning(t('admin.teamFolders.nameRequired'))
    return
  }
  await teamApi.post('/folders', newFolder.value)
  ElMessage.success(t('admin.teamFolders.createSuccess'))
  showCreate.value = false
  newFolder.value = { name: '', parent_id: '' }
  loadTree()
}

async function editFolder(data: any) {
  const { value } = await ElMessageBox.prompt(t('admin.teamFolders.newName'), t('admin.teamFolders.editFolder'), {
    inputValue: data.name,
  })
  await teamApi.put(`/folders/${data.id}`, { name: value })
  ElMessage.success(t('admin.teamFolders.updateSuccess'))
  loadTree()
}

async function deleteFolder(data: any) {
  await ElMessageBox.confirm(t('admin.teamFolders.deleteConfirm', [data.name]), t('admin.teamFolders.deleteConfirmTitle'))
  await teamApi.delete(`/folders/${data.id}`)
  ElMessage.success(t('admin.teamFolders.deleteSuccess'))
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