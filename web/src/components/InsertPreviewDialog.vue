<template>
  <el-dialog :model-value="modelValue" :title="t('insertPreview.title')" width="760px" class="insert-preview"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)" @open="onOpen">
    <p class="ip-intro">{{ t('insertPreview.intro') }}</p>

    <el-form :model="form" label-width="96px" class="ip-form" @submit.prevent>
      <el-form-item>
        <el-radio-group v-model="form.mode" @change="result = null">
          <el-radio-button value="new">{{ t('insertPreview.modeNew') }}</el-radio-button>
          <el-radio-button value="existing">{{ t('insertPreview.modeExisting') }}</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <template v-if="form.mode === 'existing'">
        <el-form-item :label="t('insertPreview.pickOrder')">
          <el-select v-model="form.deadline_id" filterable :placeholder="t('insertPreview.pickOrderHint')" class="ip-wide" @change="onPick">
            <el-option v-for="o in openOrders" :key="o.id" :value="o.id" :label="`${o.order_no || ''} ${o.title}`.trim()">
              <span class="ip-opt-no">{{ o.order_no }}</span> {{ o.title }}
              <span class="ip-opt-due">{{ t('insertPreview.due', { date: o.due_date }) }}</span>
            </el-option>
          </el-select>
        </el-form-item>
      </template>
      <template v-else>
        <div class="ip-row">
          <el-form-item :label="t('deadlines.orderNo')">
            <el-input v-model="form.order_no" placeholder="SO-2026-0001" />
          </el-form-item>
          <el-form-item :label="t('deadlines.orderTitle')">
            <el-input v-model="form.title" />
          </el-form-item>
        </div>
        <div class="ip-row">
          <el-form-item :label="t('deadlines.customer')">
            <el-input v-model="form.customer" />
          </el-form-item>
          <el-form-item :label="t('insertPreview.startDate')">
            <el-date-picker v-model="form.start_date" type="date" value-format="YYYY-MM-DD" />
          </el-form-item>
        </div>
      </template>
      <div class="ip-row">
        <el-form-item :label="t('insertPreview.wantDate')" required>
          <el-date-picker v-model="form.due_date" type="date" value-format="YYYY-MM-DD" />
        </el-form-item>
        <el-form-item :label="t('deadlines.priority')">
          <el-select v-model="form.priority">
            <el-option :label="t('deadlines.priorityInserted')" value="inserted" />
            <el-option :label="t('deadlines.priorityUrgent')" value="urgent" />
            <el-option :label="t('deadlines.priorityNormal')" value="normal" />
          </el-select>
        </el-form-item>
      </div>
    </el-form>

    <!-- ==================== 结果卡片 ==================== -->
    <div v-if="result" class="ip-card" data-test="preview-card">
      <div class="ip-head">
        <el-tag :type="conclusionType(result.conclusion)" effect="dark" size="large" class="ip-verdict">
          {{ t(summary.key, summary.args) }}
        </el-tag>
        <span class="ip-basis">{{ t('insertPreview.basis', { n: result.per_day }) }}</span>
      </div>

      <div class="ip-insert">
        <span class="ip-no">{{ result.insert.order_no || result.insert.title }}</span>
        <el-tag size="small" effect="plain">{{ priorityLabel(result.insert.priority) }}</el-tag>
        <span class="ip-dates">
          <template v-if="result.insert.old_finish">{{ shortDate(result.insert.old_finish) }} → </template>{{ t('insertPreview.insertFinish', { date: shortDate(result.insert.new_finish) }) }}
        </span>
        <span :class="result.insert.late_days ? 'ip-bad' : 'ip-good'">
          {{ result.insert.late_days ? t('insertPreview.missesDate', { n: result.insert.late_days }) : t('insertPreview.meetsDate') }}
        </span>
      </div>

      <template v-for="sec in sections" :key="sec.key">
        <div v-if="sec.items.length" class="ip-section" :class="sec.cls">
          <div class="ip-sec-title">{{ sec.title }} · {{ sec.items.length }}</div>
          <div v-for="it in sec.items" :key="it.id" class="ip-item">
            <span class="ip-no">{{ it.order_no }}</span>
            <span class="ip-title">{{ it.title }}</span>
            <span class="ip-dates">{{ shortDate(it.old_finish) }} → <b>{{ shortDate(it.new_finish) }}</b></span>
            <span class="ip-delta">{{ t('insertPreview.plusDays', { n: it.delay_days }) }}</span>
            <span class="ip-due">{{ t('insertPreview.due', { date: shortDate(it.due_date) }) }}<template v-if="it.late_days"> · {{ t('insertPreview.lateDays', { n: it.late_days }) }}</template></span>
            <el-tag v-for="f in flagLabels(it.flags)" :key="f" size="small" type="danger" effect="plain" class="ip-flag">⚠ {{ f }}</el-tag>
          </div>
        </div>
      </template>

      <el-collapse v-if="result.unaffected.length || result.started.length" class="ip-more">
        <el-collapse-item v-if="result.unaffected.length" :title="t('insertPreview.secUnaffected', { n: result.unaffected.length })" name="u">
          <div v-for="it in result.unaffected" :key="it.id" class="ip-item ip-quiet">
            <span class="ip-no">{{ it.order_no }}</span><span class="ip-title">{{ it.title }}</span>
            <span class="ip-dates">{{ shortDate(it.new_finish) }}</span>
            <span class="ip-due">{{ t('insertPreview.due', { date: shortDate(it.due_date) }) }}</span>
          </div>
        </el-collapse-item>
        <el-collapse-item v-if="result.started.length" :title="t('insertPreview.secStarted', { n: result.started.length })" name="s">
          <div v-for="it in result.started" :key="it.id" class="ip-item ip-quiet">
            <span class="ip-no">{{ it.order_no }}</span><span class="ip-title">{{ it.title }}</span>
            <span class="ip-due">{{ t('insertPreview.due', { date: shortDate(it.due_date) }) }}</span>
          </div>
        </el-collapse-item>
      </el-collapse>
      <slot name="actions" :result="result" :request="lastRequest" />
    </div>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">{{ t('common.close') }}</el-button>
      <el-button type="primary" :loading="running" @click="run">{{ result ? t('insertPreview.rerun') : t('insertPreview.run') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import teamApi from '@/utils/team-api'
import { conclusionText, conclusionType, shortDate, flagKey, type PreviewResult } from '@/utils/insertPreview'

defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const { t } = useI18n()

interface OpenOrder { id: string; order_no: string; title: string; due_date: string; priority: string; status: string }

const form = reactive({
  mode: 'new' as 'new' | 'existing',
  deadline_id: '',
  order_no: '',
  title: '',
  customer: '',
  start_date: '',
  due_date: '',
  priority: 'inserted',
})
const openOrders = ref<OpenOrder[]>([])
const result = ref<PreviewResult | null>(null)
const lastRequest = ref<Record<string, string>>({})
const running = ref(false)

const summary = computed(() => (result.value ? conclusionText(result.value) : { key: '', args: {} }))
const sections = computed(() => result.value ? [
  { key: 'b', cls: 'ip-breach', title: t('insertPreview.secBreach'), items: result.value.new_breaches },
  { key: 'd', cls: 'ip-delay', title: t('insertPreview.secDelay'), items: result.value.delayed },
] : [])

function priorityLabel(p: string) {
  return p === 'urgent' ? t('deadlines.priorityUrgent') : p === 'inserted' ? t('deadlines.priorityInserted') : t('deadlines.priorityNormal')
}
function flagLabels(flags?: string[]) {
  return (flags || []).map(flagKey).filter((k): k is string => !!k).map(k => t(k))
}

async function onOpen() {
  try {
    const { data } = await teamApi.get('/deadlines', { params: { limit: 500, sort: 'due' } })
    openOrders.value = ((data.data || []) as OpenOrder[]).filter(o => o.status !== 'done')
  } catch {
    openOrders.value = []
  }
}
function onPick(id: string) {
  const o = openOrders.value.find(x => x.id === id)
  if (o && !form.due_date) form.due_date = o.due_date
}

async function run() {
  if (!form.due_date) return ElMessage.warning(t('insertPreview.needDate'))
  const body: Record<string, string> = { due_date: form.due_date, priority: form.priority }
  if (form.mode === 'existing') {
    if (!form.deadline_id) return ElMessage.warning(t('insertPreview.needOrder'))
    body.deadline_id = form.deadline_id
  } else {
    if (!form.order_no.trim() && !form.title.trim()) return ElMessage.warning(t('insertPreview.needOrder'))
    Object.assign(body, { order_no: form.order_no, title: form.title, customer: form.customer })
    if (form.start_date) body.start_date = form.start_date
  }
  running.value = true
  try {
    const { data } = await teamApi.post('/deadlines/preview-insert', body)
    result.value = data.data
    lastRequest.value = body
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('insertPreview.failed'))
  } finally {
    running.value = false
  }
}

defineExpose({ run, result })
</script>

<style scoped>
.ip-intro { margin: -6px 0 14px; color: var(--md-text-slate); font-size: 13px; line-height: 1.6; }
.ip-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0 16px; }
.ip-wide { width: 100%; }
.ip-opt-no { font-weight: 600; margin-right: 6px; }
.ip-opt-due { float: right; color: var(--md-text-slate-dim); font-size: 12px; }
.ip-card {
  margin-top: 6px; padding: 14px 16px; border-radius: 10px;
  background: var(--md-dl-surface); border: 1px solid var(--md-dl-border);
}
.ip-head { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
.ip-verdict { font-size: 14px; font-weight: 600; }
.ip-basis { color: var(--md-text-slate-dim); font-size: 12px; }
.ip-insert { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; padding: 8px 0 12px; border-bottom: 1px dashed var(--md-dl-border); }
.ip-good { color: var(--el-color-success); font-size: 13px; }
.ip-bad { color: var(--el-color-danger); font-size: 13px; }
.ip-section { margin-top: 12px; }
.ip-sec-title { font-size: 13px; font-weight: 600; margin-bottom: 6px; }
.ip-breach .ip-sec-title { color: var(--el-color-danger); }
.ip-delay .ip-sec-title { color: var(--el-color-warning); }
.ip-item {
  display: flex; align-items: center; gap: 10px; flex-wrap: wrap;
  padding: 6px 8px; border-radius: 6px; font-size: 13px;
}
.ip-breach .ip-item { background: var(--md-dl-overdue-bg); }
.ip-delay .ip-item { background: var(--md-dl-next3-bg); }
.ip-item + .ip-item { margin-top: 4px; }
.ip-no { font-weight: 600; color: var(--md-text); }
.ip-title { color: var(--md-text-slate); }
.ip-dates { font-variant-numeric: tabular-nums; }
.ip-delta { color: var(--el-color-warning); font-weight: 600; }
.ip-breach .ip-delta { color: var(--el-color-danger); }
.ip-due { color: var(--md-text-slate-dim); font-size: 12px; }
.ip-flag { margin-left: 2px; }
.ip-quiet { padding: 3px 0; }
.ip-more { margin-top: 12px; }
</style>
