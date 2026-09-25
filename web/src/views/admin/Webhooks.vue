<template>
  <div class="admin-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ t('admin.webhooks.title') }}</h2>
        <p class="page-sub">{{ t('admin.webhooks.subtitle') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ t('admin.webhooks.add') }}</el-button>
    </div>

    <div v-if="loading" class="panel">
      <el-skeleton :rows="4" animated />
    </div>
    <div v-else-if="loadError" class="panel empty">
      <p>{{ t('docs.loadFailed') }}</p>
      <el-button @click="load">{{ t('common.refresh') }}</el-button>
    </div>
    <div v-else-if="!hooks.length" class="panel empty">
      <p>{{ t('admin.webhooks.empty') }}</p>
    </div>
    <div v-else class="panel">
      <el-table :data="hooks">
        <el-table-column prop="name" :label="t('admin.webhooks.name')" min-width="140" />
        <el-table-column prop="url" :label="t('admin.webhooks.url')" min-width="240" show-overflow-tooltip />
        <el-table-column prop="events" :label="t('admin.webhooks.events')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="t('common.operation')" width="180">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? t('admin.webhooks.enabled') : t('admin.webhooks.disabled') }}
            </el-tag>
            <el-button link type="primary" @click="toggle(row)">{{ row.enabled ? t('admin.webhooks.disabled') : t('admin.webhooks.enabled') }}</el-button>
            <el-button link type="danger" @click="remove(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showForm" :title="t('admin.webhooks.add')" width="480" destroy-on-close>
      <el-form label-position="top">
        <el-form-item :label="t('admin.webhooks.name')">
          <el-input v-model="form.name" maxlength="80" />
        </el-form-item>
        <el-form-item :label="t('admin.webhooks.url')">
          <el-input v-model="form.url" placeholder="https://example.com/hooks/mistdocs" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForm = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="create">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import teamApi from '@/utils/team-api'

const { t } = useI18n()
const hooks = ref<any[]>([])
const loading = ref(false)
const loadError = ref(false)
const saving = ref(false)
const showForm = ref(false)
const form = ref({ name: '', url: '' })

async function load() {
  loading.value = true
  loadError.value = false
  try {
    const { data } = await teamApi.get('/webhooks')
    hooks.value = data.data || []
  } catch {
    loadError.value = true
    hooks.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { name: '', url: '' }
  showForm.value = true
}

async function create() {
  const name = form.value.name.trim()
  const url = form.value.url.trim()
  if (!name) {
    ElMessage.warning(t('admin.webhooks.nameRequired'))
    return
  }
  if (!/^https?:\/\//i.test(url)) {
    ElMessage.warning(t('admin.webhooks.urlInvalid'))
    return
  }
  saving.value = true
  try {
    await teamApi.post('/webhooks', {
      name,
      url,
      events: '["document.created","document.updated"]',
    })
    ElMessage.success(t('admin.webhooks.created'))
    showForm.value = false
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('common.failed'))
  } finally {
    saving.value = false
  }
}

async function toggle(row: any) {
  try {
    await teamApi.put(`/webhooks/${row.id}/toggle`)
    ElMessage.success(t('admin.webhooks.toggled'))
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('common.failed'))
  }
}

async function remove(row: any) {
  try {
    await ElMessageBox.confirm(t('admin.webhooks.deleteConfirm'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await teamApi.delete(`/webhooks/${row.id}`)
    ElMessage.success(t('admin.webhooks.deleted'))
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('common.failed'))
  }
}

onMounted(load)
</script>

<style scoped>
.admin-page { padding: 8px 4px 24px; background: var(--md-bg); color: var(--md-text); min-height: 100%; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-bottom: 16px; }
.page-title { margin: 0 0 4px; font-size: 20px; color: var(--md-text-title); }
.page-sub { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.panel { background: var(--el-bg-color); border: 1px solid var(--el-border-color-lighter); border-radius: 8px; padding: 16px; }
.empty { text-align: center; color: var(--el-text-color-secondary); padding: 48px 16px; }
</style>
