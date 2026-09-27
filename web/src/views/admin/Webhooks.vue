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
        <el-table-column :label="t('admin.webhooks.events')" min-width="260">
          <template #default="{ row }">
            <div class="event-tags">
              <el-tag v-for="e in (row.event_list || [])" :key="e" size="small" effect="plain" disable-transitions>{{ eventLabel(e) }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.operation')" width="230">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? t('admin.webhooks.enabled') : t('admin.webhooks.disabled') }}
            </el-tag>
            <el-button link type="primary" @click="openEdit(row)">{{ t('common.edit') }}</el-button>
            <el-button link type="primary" @click="toggle(row)">{{ row.enabled ? t('admin.webhooks.disabled') : t('admin.webhooks.enabled') }}</el-button>
            <el-button link type="danger" @click="remove(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showForm" :title="editingId ? t('admin.webhooks.edit') : t('admin.webhooks.add')" width="560" destroy-on-close>
      <el-form label-position="top">
        <el-form-item :label="t('admin.webhooks.name')">
          <el-input v-model="form.name" maxlength="80" />
        </el-form-item>
        <el-form-item :label="t('admin.webhooks.url')">
          <el-input v-model="form.url" placeholder="https://example.com/hooks/mistdocs" />
        </el-form-item>
        <el-form-item :label="t('admin.webhooks.events')">
          <el-checkbox-group v-model="form.events" class="event-grid">
            <el-checkbox v-for="e in availableEvents" :key="e" :value="e">
              {{ eventLabel(e) }} <span class="event-code">{{ e }}</span>
            </el-checkbox>
          </el-checkbox-group>
          <div class="event-hint">{{ t('admin.webhooks.reminderHint') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForm = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" :disabled="!form.events.length" @click="save">{{ editingId ? t('common.save') : t('common.create') }}</el-button>
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
const form = ref<{ name: string; url: string; events: string[] }>({ name: '', url: '', events: [] })
const editingId = ref('')
const DEFAULT_EVENTS = ['document.created', 'document.updated']
// The server sends the catalogue; this list is only a fallback.
const availableEvents = ref<string[]>([
  'document.created', 'document.updated', 'document.deleted', 'document.shared', 'comment.created',
  'document.imported', 'document.locked', 'document.unlocked', 'document.restored', 'deadline.reminder',
])
function eventLabel(e: string) {
  const key = `admin.webhooks.ev.${e.replace('.', '_')}`
  const label = t(key)
  return label === key ? e : label
}

async function load() {
  loading.value = true
  loadError.value = false
  try {
    const { data } = await teamApi.get('/webhooks')
    hooks.value = data.data || []
    if (Array.isArray(data.available_events) && data.available_events.length) availableEvents.value = data.available_events
  } catch {
    loadError.value = true
    hooks.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = ''
  form.value = { name: '', url: '', events: [...DEFAULT_EVENTS] }
  showForm.value = true
}

function openEdit(row: any) {
  editingId.value = row.id
  form.value = { name: row.name, url: row.url, events: [...(row.event_list || [])] }
  showForm.value = true
}

async function save() {
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
    const body = { name, url, events: form.value.events }
    if (editingId.value) {
      await teamApi.put(`/webhooks/${editingId.value}`, body)
      ElMessage.success(t('admin.webhooks.saved'))
    } else {
      await teamApi.post('/webhooks', body)
      ElMessage.success(t('admin.webhooks.created'))
    }
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
.event-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.event-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 2px 16px; width: 100%; }
.event-code { color: var(--el-text-color-secondary); font-size: 11px; margin-left: 4px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.event-hint { color: var(--el-text-color-secondary); font-size: 12px; margin-top: 6px; line-height: 1.5; }
.admin-page { padding: 8px 4px 24px; background: var(--md-bg); color: var(--md-text); min-height: 100%; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-bottom: 16px; }
.page-title { margin: 0 0 4px; font-size: 20px; color: var(--md-text-title); }
.page-sub { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.panel { background: var(--el-bg-color); border: 1px solid var(--el-border-color-lighter); border-radius: 8px; padding: 16px; }
.empty { text-align: center; color: var(--el-text-color-secondary); padding: 48px 16px; }
</style>
