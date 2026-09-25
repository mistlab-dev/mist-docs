<template>
  <div class="admin-page">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">{{ t('admin.permissions.title') }}</h2>
        <span class="header-count">{{ t('admin.permissions.rulesCount', [perms.length]) }}</span>
      </div>
      <el-button type="primary" size="default" @click="showForm = true">
        <el-icon><Plus /></el-icon> {{ t('admin.permissions.setPermission') }}
      </el-button>
    </div>

    <!-- 筛选 -->
    <div class="filter-bar">
      <el-radio-group v-model="filter.resource_type" size="default" @change="load">
        <el-radio-button value="document">{{ t('common.doc') }}</el-radio-button>
        <el-radio-button value="folder">{{ t('common.folder') }}</el-radio-button>
      </el-radio-group>
    </div>

    <div class="table-card">
      <el-table
        :data="perms"
        :header-cell-style="{ background: 'var(--md-surface-2)', color: 'var(--md-text-secondary)', fontWeight: 500, fontSize: '13px' }"
        :cell-style="{ fontSize: '14px' }"
      >
        <el-table-column :label="t('admin.permissions.resource')" min-width="240">
          <template #default="{ row }">
            <div class="type-cell">
              <div class="type-icon" :style="{ background: row.resource_type === 'document' ? 'var(--md-tint-doc)' : 'var(--md-tint-folder)' }">
                <svg v-if="row.resource_type === 'document'" viewBox="0 0 20 20" fill="#409eff"><path d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg>
                <svg v-else viewBox="0 0 20 20" fill="#fa8c16"><path d="M3 4a1 1 0 011-1h4a1 1 0 01.8.4L10.5 6H17a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1V4z"/></svg>
              </div>
              <span class="res-name">{{ row.resource_name || row.resource_id }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('admin.permissions.target')" width="160">
          <template #default="{ row }">
            <div class="target-cell">
              <span>{{ row.user_name || row.target_id }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('admin.permissions.permission')" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="permColor[row.permission]" size="small" effect="light" round disable-transitions>
              {{ permMap[row.permission] }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('admin.permissions.inherit')" width="70" align="center">
          <template #default="{ row }">
            <span v-if="row.inherit" class="inherit-badge">✓</span>
            <span v-else class="no-inherit">—</span>
          </template>
        </el-table-column>
        <el-table-column label="" width="60" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="del(row)" class="del-btn">{{ t('common.remove') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showForm" :title="t('admin.permissions.setPermission')" width="480" destroy-on-close @open="loadChoices">
      <el-form :model="form" label-position="top" class="perm-form">
        <el-form-item :label="t('admin.permissions.resourceType')">
          <el-select v-model="form.resource_type" class="full-width" @change="form.resource_id = ''">
            <el-option :label="t('common.doc')" value="document" />
            <el-option :label="t('common.folder')" value="folder" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('admin.permissions.resource')">
          <el-select v-model="form.resource_id" class="full-width" filterable :placeholder="t('admin.permissions.pickResource')">
            <el-option v-for="item in resourceChoices" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('admin.permissions.target')">
          <el-select v-model="form.target_id" class="full-width" filterable :placeholder="t('admin.permissions.pickMember')">
            <el-option v-for="m in members" :key="m.id" :label="m.display_name || m.username" :value="m.id" />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('admin.permissions.permissionLevel')">
              <el-select v-model="form.permission" class="full-width">
                <el-option :label="t('admin.permissions.read')" value="read" />
                <el-option :label="t('admin.permissions.comment')" value="comment" />
                <el-option :label="t('admin.permissions.write')" value="write" />
                <el-option :label="t('admin.permissions.adminPerm')" value="admin" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item v-if="form.resource_type === 'folder'" :label="t('admin.permissions.childInherit')">
              <el-switch v-model="form.inherit" :active-text="t('admin.permissions.inheritYes')" :inactive-text="t('admin.permissions.inheritNo')" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="showForm = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="submit">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import teamApi from '@/utils/team-api'

const { t } = useI18n()

const permMap: any = { read: t('admin.permissions.read'), comment: t('admin.permissions.comment'), write: t('admin.permissions.write'), admin: t('admin.permissions.adminPerm') }
const permColor: any = { read: 'info', comment: '', write: 'warning', admin: 'danger' }
const perms = ref<any[]>([])
const filter = ref({ resource_type: 'document' })
const showForm = ref(false)
const form = ref({
  resource_type: 'document', resource_id: '',
  target_type: 'user', target_id: '',
  permission: 'read', inherit: true,
})
const documents = ref<{ id: string; name: string }[]>([])
const folders = ref<{ id: string; name: string }[]>([])
const members = ref<any[]>([])
const resourceChoices = computed(() => form.value.resource_type === 'folder' ? folders.value : documents.value)

function flattenFolders(nodes: any[], prefix = ''): { id: string; name: string }[] {
  const out: { id: string; name: string }[] = []
  for (const n of nodes || []) {
    const name = prefix ? `${prefix} / ${n.name}` : n.name
    out.push({ id: n.id, name })
    out.push(...flattenFolders(n.children, name))
  }
  return out
}

async function loadChoices() {
  try {
    const [docs, tree, people] = await Promise.all([
      teamApi.get('/documents', { params: { page: 1, page_size: 100 } }),
      teamApi.get('/folders/tree'),
      teamApi.get('/members'),
    ])
    documents.value = (docs.data.data || []).map((d: any) => ({ id: d.id, name: d.title }))
    const roots = tree.data.data || tree.data || []
    folders.value = flattenFolders(Array.isArray(roots) ? roots : [])
    members.value = people.data.data || []
  } catch {
    ElMessage.error(t('common.failed'))
  }
}

async function load() {
  const { data } = await teamApi.get('/permissions', { params: filter.value })
  perms.value = data.data || []
}

async function submit() {
  if (!form.value.resource_id || !form.value.target_id) {
    ElMessage.error(t('admin.permissions.pickResource'))
    return
  }
  await teamApi.post('/permissions', { ...form.value, target_type: 'user' })
  ElMessage.success(t('admin.permissions.setSuccess'))
  showForm.value = false
  load()
}

async function del(row: any) {
  await ElMessageBox.confirm(t('admin.permissions.removeConfirmMsg'), t('admin.permissions.removeConfirmTitle'))
  await teamApi.delete(`/permissions/${row.id}`)
  ElMessage.success(t('admin.permissions.removeSuccess'))
  load()
}

onMounted(load)
</script>

<style scoped>
.admin-page { height: 100%; display: flex; flex-direction: column; padding: 20px; background: var(--md-bg); }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; padding-bottom: 16px; border-bottom: 1px solid var(--md-border);
}
.header-left { display: flex; align-items: baseline; gap: 16px; }
.page-title { font-size: 22px; font-weight: 600; color: var(--md-text-title); margin: 0; letter-spacing: -0.02em; }
.header-count { font-size: 14px; color: var(--md-text-dim); }

.filter-bar { margin-bottom: 16px; }

.table-card {
  background: var(--md-surface); border-radius: 16px;
  border: none; flex: 1; overflow: auto;
  padding: 16px;
  box-shadow: var(--md-shadow);
}
.table-card :deep(.el-table__row) { height: 56px; }
.table-card :deep(.el-table__cell) { padding: 12px 0; }
.table-card :deep(.el-table__header-cell) { padding: 14px 0; background: var(--md-surface-2) !important; }

.type-cell { display: flex; align-items: center; gap: 10px; }
.type-icon {
  width: 30px; height: 30px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.type-icon svg { width: 16px; height: 16px; }

.target-cell { display: flex; align-items: center; gap: 8px; }

.mono-id {
  font-family: 'SF Mono', Monaco, monospace; font-size: 13px;
  color: var(--md-text-muted); background: var(--md-bg); padding: 3px 8px; border-radius: 6px;
}
.mono-id.sm { font-size: 11px; color: var(--md-text-dim); }

.inherit-badge { color: var(--md-tint-sheet-text); font-weight: 700; }
.no-inherit { color: var(--md-text-faint); }

.del-btn { font-size: 13px; }

.perm-form :deep(.el-form-item__label) { font-weight: 500; color: var(--md-text-muted); }
.perm-form :deep(.el-input__wrapper) { border-radius: 8px; }
.full-width { width: 100%; }

:deep(.el-dialog) { border-radius: 16px; }

@media (max-width: 768px) {
  .admin-page { padding: 12px; }
  .filter-bar { overflow-x: auto; }
}
</style>
