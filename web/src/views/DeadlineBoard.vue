<template>
  <div class="deadline-page">
    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <h2 class="page-title">{{ t('deadlines.title') }}</h2>
        <span class="page-sub">{{ t('deadlines.subtitle') }}</span>
      </div>
      <div class="toolbar-right">
        <el-button @click="loadAll" :loading="loading">
          <el-icon><Refresh /></el-icon>
        </el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon> {{ t('deadlines.newDeadline') }}
        </el-button>
      </div>
    </div>

    <!-- 统计卡 -->
    <div class="stats">
      <div class="stat-card overdue" @click="jumpTo('overdue')">
        <div class="stat-num">{{ counts.overdue }}</div>
        <div class="stat-label">{{ t('deadlines.overdue') }}</div>
      </div>
      <div class="stat-card today" @click="jumpTo('today')">
        <div class="stat-num">{{ counts.today }}</div>
        <div class="stat-label">{{ t('deadlines.today') }}</div>
      </div>
      <div class="stat-card next3" @click="jumpTo('next_3_days')">
        <div class="stat-num">{{ counts.next_3_days }}</div>
        <div class="stat-label">{{ t('deadlines.next3') }}</div>
      </div>
      <div class="stat-card next7" @click="jumpTo('next_7_days')">
        <div class="stat-num">{{ counts.next_7_days }}</div>
        <div class="stat-label">{{ t('deadlines.next7') }}</div>
      </div>
      <div class="stat-card plain">
        <div class="stat-num">{{ counts.open_total }}</div>
        <div class="stat-label">{{ t('deadlines.openTotal') }}</div>
      </div>
      <div class="stat-card plain">
        <div class="stat-num">{{ counts.on_time_30d }}%</div>
        <div class="stat-label">{{ t('deadlines.onTime30d') }}</div>
      </div>
    </div>

    <el-tabs v-model="tab" class="board-tabs">
      <!-- ==================== 看板 ==================== -->
      <el-tab-pane :label="t('deadlines.board')" name="board">
        <div class="columns">
          <div v-for="col in columns" :key="col.key" class="column">
            <div class="column-head" :class="col.key">
              <span>{{ col.label }}</span>
              <span class="column-count">{{ col.items.length }}</span>
            </div>
            <div class="column-body">
              <div v-if="!col.items.length" class="column-empty">
                {{ col.emptyText }}
              </div>
              <div
                v-for="d in col.items"
                :key="d.id"
                class="deadline-card"
                :class="d.risk_level"
                @click="openDetail(d)"
              >
                <div class="card-top">
                  <span class="card-order">{{ d.order_no || '—' }}</span>
                  <el-tag v-if="d.priority !== 'normal'" size="small" :type="d.priority === 'urgent' ? 'danger' : 'warning'">
                    {{ d.priority === 'urgent' ? t('deadlines.priorityUrgent') : t('deadlines.priorityInserted') }}
                  </el-tag>
                </div>
                <div class="card-title">{{ d.title }}</div>
                <div class="card-meta">
                  <span>{{ t('deadlines.dueDate') }} {{ d.due_date }}</span>
                  <span class="days" :class="d.risk_level">{{ daysText(d) }}</span>
                </div>
                <div class="card-foot">
                  <span class="owner" :class="{ unassigned: !d.owner_name }">{{ d.owner_name || t('deadlines.unassigned') }}</span>
                  <span class="status">{{ statusText(d.status) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <!-- ==================== 全部订单 ==================== -->
      <el-tab-pane :label="t('deadlines.list')" name="list">
        <div class="filters">
          <el-input
            v-model="filters.q"
            :placeholder="t('deadlines.searchPlaceholder')"
            clearable
            style="width: 260px"
            @keyup.enter="applyFilters"
            @clear="applyFilters"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="filters.status" :placeholder="t('deadlines.allStatus')" clearable style="width: 130px" @change="applyFilters">
            <el-option :label="t('deadlines.statusPending')" value="pending" />
            <el-option :label="t('deadlines.statusRunning')" value="running" />
            <el-option :label="t('deadlines.statusDone')" value="done" />
            <el-option :label="t('deadlines.statusOverdue')" value="overdue" />
          </el-select>
          <el-select v-model="filters.risk" :placeholder="t('deadlines.allRisk')" clearable style="width: 130px" @change="applyFilters">
            <el-option :label="t('deadlines.riskOverdue')" value="overdue" />
            <el-option :label="t('deadlines.riskCritical')" value="critical" />
            <el-option :label="t('deadlines.riskWarning')" value="warning" />
            <el-option :label="t('deadlines.openTotal')" value="open" />
          </el-select>
          <el-select v-model="filters.priority" :placeholder="t('deadlines.allPriority')" clearable style="width: 130px" @change="applyFilters">
            <el-option :label="t('deadlines.priorityNormal')" value="normal" />
            <el-option :label="t('deadlines.priorityUrgent')" value="urgent" />
            <el-option :label="t('deadlines.priorityInserted')" value="inserted" />
          </el-select>
          <el-button @click="applyFilters">{{ t('common.search') }}</el-button>
        </div>

        <el-table :data="list" v-loading="loading" class="deadline-table" empty-text="—">
          <el-table-column prop="order_no" :label="t('deadlines.orderNo')" width="130" />
          <el-table-column prop="title" :label="t('deadlines.orderTitle')" min-width="160" show-overflow-tooltip />
          <el-table-column prop="customer" :label="t('deadlines.customer')" width="130" show-overflow-tooltip />
          <el-table-column :label="t('deadlines.dueDate')" width="130">
            <template #default="{ row }">
              <span :class="['due-cell', row.risk_level]">{{ row.due_date }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('deadlines.daysLeft')" width="110">
            <template #default="{ row }">
              <span :class="['days', row.risk_level]">{{ daysText(row) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('deadlines.status')" width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('deadlines.progress')" width="140">
            <template #default="{ row }">
              <el-progress :percentage="row.progress" :stroke-width="10" />
            </template>
          </el-table-column>
          <el-table-column :label="t('deadlines.owner')" width="110">
            <template #default="{ row }">
              <span :class="{ 'cell-unassigned': !row.owner_name }">
                {{ row.owner_name || t('deadlines.unassigned') }}
              </span>
            </template>
          </el-table-column>
          <!-- 操作收敛成文字按钮：以前每行一个实心蓝块，整表都是高饱和色块 -->
          <el-table-column :label="t('common.operation')" width="120" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openDetail(row)">{{ t('common.detail') }}</el-button>
              <el-button link @click="openEdit(row)">{{ t('common.edit') }}</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="list-foot">
          <span class="total">{{ t('deadlines.totalCount', { num: listTotal }) }}</span>
          <el-pagination
            v-model:current-page="page"
            :page-size="pageSize"
            :total="listTotal"
            layout="prev, pager, next"
            :pager-count="5"
            background
            hide-on-single-page
            @current-change="loadList"
          />
        </div>
      </el-tab-pane>

      <!-- ==================== 提醒规则 ==================== -->
      <el-tab-pane :label="t('deadlines.rules')" name="rules">
        <div class="rules-head">
          <el-button @click="seedRules">{{ t('deadlines.seedRules') }}</el-button>
          <el-button type="primary" @click="openRuleCreate">
            <el-icon><Plus /></el-icon> {{ t('deadlines.newRule') }}
          </el-button>
        </div>

        <el-table :data="rules" v-loading="loading" empty-text="—">
          <el-table-column prop="name" :label="t('deadlines.ruleName')" min-width="160" />
          <el-table-column :label="t('deadlines.offsetDays')" width="140">
            <template #default="{ row }">{{ offsetText(row.offset_days) }}</template>
          </el-table-column>
          <el-table-column :label="t('deadlines.channel')" width="140">
            <template #default="{ row }">
              {{ row.channel === 'webhook' ? t('deadlines.channelWebhook') : t('deadlines.channelInapp') }}
            </template>
          </el-table-column>
          <el-table-column :label="t('deadlines.target')" width="120">
            <template #default="{ row }">
              {{ row.target === 'creator' ? t('deadlines.targetCreator') : t('deadlines.targetOwner') }}
            </template>
          </el-table-column>
          <el-table-column :label="t('deadlines.enabled')" width="100">
            <template #default="{ row }">
              <el-switch :model-value="row.enabled" @change="(v: boolean) => toggleRule(row, v)" />
            </template>
          </el-table-column>
          <el-table-column :label="t('common.edit')" width="140">
            <template #default="{ row }">
              <el-button size="small" @click="openRuleEdit(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="removeRule(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ==================== 提醒记录 ==================== -->
      <el-tab-pane :label="t('deadlines.log')" name="log">
        <el-table :data="logs" v-loading="loading" empty-text="—">
          <el-table-column :label="t('deadlines.sentAt')" width="180">
            <template #default="{ row }">{{ fmtTime(row.sent_at) }}</template>
          </el-table-column>
          <el-table-column prop="order_no" :label="t('deadlines.orderNo')" width="130" />
          <el-table-column prop="title" :label="t('deadlines.orderTitle')" min-width="180" show-overflow-tooltip />
          <el-table-column :label="t('deadlines.result')" width="110">
            <template #default="{ row }">
              <el-tag size="small" :type="row.result === 'ok' ? 'success' : 'danger'">
                {{ row.result === 'ok' ? t('deadlines.resultOk') : t('deadlines.resultFailed') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="detail" :label="t('common.detail')" min-width="200" show-overflow-tooltip />
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- ==================== 新建 / 编辑 ==================== -->
    <el-dialog v-model="formVisible" :title="form.id ? t('deadlines.editDeadline') : t('deadlines.newDeadline')" width="620px">
      <el-form :model="form" label-width="110px">
        <el-form-item :label="t('deadlines.orderNo')">
          <el-input v-model="form.order_no" placeholder="PO-2026-0001" />
        </el-form-item>
        <el-form-item :label="t('deadlines.orderTitle')" required>
          <el-input v-model="form.title" />
        </el-form-item>
        <el-form-item :label="t('deadlines.customer')">
          <el-input v-model="form.customer" />
        </el-form-item>
        <el-form-item :label="t('deadlines.quantity')">
          <el-input-number v-model="form.quantity" :min="0" />
        </el-form-item>
        <el-form-item :label="t('deadlines.dueDate')" required>
          <el-date-picker v-model="form.due_date" type="date" value-format="YYYY-MM-DD" />
        </el-form-item>
        <el-form-item :label="t('deadlines.startDate')">
          <el-date-picker v-model="form.start_date" type="date" value-format="YYYY-MM-DD" />
        </el-form-item>
        <el-form-item :label="t('deadlines.status')">
          <el-select v-model="form.status">
            <el-option :label="t('deadlines.statusPending')" value="pending" />
            <el-option :label="t('deadlines.statusRunning')" value="running" />
            <el-option :label="t('deadlines.statusDone')" value="done" />
            <el-option :label="t('deadlines.statusOverdue')" value="overdue" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('deadlines.priority')">
          <el-select v-model="form.priority">
            <el-option :label="t('deadlines.priorityNormal')" value="normal" />
            <el-option :label="t('deadlines.priorityUrgent')" value="urgent" />
            <el-option :label="t('deadlines.priorityInserted')" value="inserted" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('deadlines.progress')">
          <el-slider v-model="form.progress" :max="100" />
        </el-form-item>
        <el-form-item :label="t('deadlines.remark')">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('deadlines.reason')">
          <el-input v-model="form.reason" type="textarea" :rows="2" :placeholder="t('deadlines.reasonHint')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- ==================== 详情 ==================== -->
    <el-dialog v-model="detailVisible" :title="detail?.order_no || detail?.title || ''" width="720px">
      <div v-if="detail" class="detail">
        <div class="detail-grid">
          <div><label>{{ t('deadlines.orderTitle') }}</label><span>{{ detail.title }}</span></div>
          <div><label>{{ t('deadlines.customer') }}</label><span>{{ detail.customer || '—' }}</span></div>
          <div><label>{{ t('deadlines.quantity') }}</label><span>{{ detail.quantity }}</span></div>
          <div><label>{{ t('deadlines.dueDate') }}</label><span :class="detail.risk_level">{{ detail.due_date }}</span></div>
          <div><label>{{ t('deadlines.daysLeft') }}</label><span :class="detail.risk_level">{{ daysText(detail) }}</span></div>
          <div><label>{{ t('deadlines.status') }}</label><span>{{ statusText(detail.status) }}</span></div>
          <div><label>{{ t('deadlines.progress') }}</label><span>{{ detail.progress }}%</span></div>
          <div><label>{{ t('deadlines.owner') }}</label><span>{{ detail.owner_name || t('deadlines.unassigned') }}</span></div>
          <div v-if="detail.remark"><label>{{ t('deadlines.remark') }}</label><span>{{ detail.remark }}</span></div>
        </div>

        <el-divider>{{ t('deadlines.eventHistory') }}</el-divider>
        <div v-if="!events.length" class="muted">{{ t('deadlines.noEvents') }}</div>
        <div v-else class="timeline">
          <div v-for="e in events" :key="e.id" class="event">
            <span class="event-time">{{ fmtTime(e.created_at) }}</span>
            <span class="event-type">{{ eventText(e.event_type) }}</span>
            <span class="event-change">
              <template v-if="e.field">
                {{ e.field }}: {{ e.old_value || '—' }} → {{ e.new_value || '—' }}
              </template>
              <template v-else>{{ e.new_value }}</template>
            </span>
            <span v-if="e.reason" class="event-reason">{{ e.reason }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="detail-footer">
          <el-button type="danger" plain @click="removeDeadline">{{ t('common.delete') }}</el-button>
          <span class="footer-gap" />
          <el-button @click="detailVisible = false">{{ t('common.close') }}</el-button>
          <el-button type="primary" @click="openEdit(detail)">{{ t('common.edit') }}</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- ==================== 规则编辑 ==================== -->
    <el-dialog v-model="ruleVisible" :title="ruleForm.id ? t('deadlines.editRule') : t('deadlines.newRule')" width="520px">
      <el-form :model="ruleForm" label-width="110px">
        <el-form-item :label="t('deadlines.ruleName')" required>
          <el-input v-model="ruleForm.name" />
        </el-form-item>
        <el-form-item :label="t('deadlines.offsetDays')">
          <el-input-number v-model="ruleForm.offset_days" :min="-90" :max="180" />
          <div class="hint">{{ t('deadlines.offsetHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('deadlines.channel')">
          <el-select v-model="ruleForm.channel">
            <el-option :label="t('deadlines.channelInapp')" value="inapp" />
            <el-option :label="t('deadlines.channelWebhook')" value="webhook" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('deadlines.target')">
          <el-select v-model="ruleForm.target">
            <el-option :label="t('deadlines.targetOwner')" value="owner" />
            <el-option :label="t('deadlines.targetCreator')" value="creator" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('deadlines.enabled')">
          <el-switch v-model="ruleForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="saveRule">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search } from '@element-plus/icons-vue'
import teamApi from '@/utils/team-api'

const { t } = useI18n()

interface Deadline {
  id: string
  order_no: string
  title: string
  customer?: string
  quantity: number
  start_date?: string
  due_date: string
  status: string
  progress: number
  priority: string
  owner_id?: string
  owner_name?: string
  remark?: string
  days_left: number
  risk_level: string
}

interface Rule {
  id: string
  name: string
  offset_days: number
  channel: string
  target: string
  enabled: boolean
}

const tab = ref('board')
const loading = ref(false)
const saving = ref(false)

const counts = reactive({
  overdue: 0,
  today: 0,
  next_3_days: 0,
  next_7_days: 0,
  open_total: 0,
  on_time_30d: 0,
})

const overdue = ref<Deadline[]>([])
const today = ref<Deadline[]>([])
const next3 = ref<Deadline[]>([])
const next7 = ref<Deadline[]>([])
const list = ref<Deadline[]>([])
const listTotal = ref(0)
const page = ref(1)
const pageSize = 50
const rules = ref<Rule[]>([])
const logs = ref<any[]>([])
const events = ref<any[]>([])

const filters = reactive({ q: '', status: '', risk: '', priority: '' })

const formVisible = ref(false)
const detailVisible = ref(false)
const ruleVisible = ref(false)
const detail = ref<Deadline | null>(null)

const emptyForm = () => ({
  id: '',
  order_no: '',
  title: '',
  customer: '',
  quantity: 0,
  start_date: '',
  due_date: '',
  status: 'pending',
  progress: 0,
  priority: 'normal',
  owner_id: '',
  remark: '',
  reason: '',
})
const form = reactive(emptyForm())

const ruleForm = reactive({
  id: '',
  name: '',
  offset_days: 3,
  channel: 'inapp',
  target: 'owner',
  enabled: true,
})

const columns = computed(() => [
  {
    key: 'overdue',
    label: t('deadlines.overdue'),
    items: overdue.value,
    emptyText: t('deadlines.emptyOverdue'),
  },
  { key: 'today', label: t('deadlines.today'), items: today.value, emptyText: t('deadlines.emptyToday') },
  { key: 'next3', label: t('deadlines.next3'), items: next3.value, emptyText: t('deadlines.emptyNext3') },
  { key: 'next7', label: t('deadlines.next7'), items: next7.value, emptyText: t('deadlines.emptyNext7') },
])

// ---- 展示辅助 ----

// 统一剩余天数文案：以前混用 "7 已延期" / "2 天" / "今天到期"，
// 同一个视觉位置有三种句式，扫一眼读不出规律。
function daysText(d: Deadline) {
  if (d.status === 'done') return t('deadlines.statusDone')
  if (d.days_left < 0) return t('deadlines.daysOverdue', { days: -d.days_left })
  if (d.days_left === 0) return t('deadlines.today')
  return t('deadlines.daysRemaining', { days: d.days_left })
}

function statusText(s: string) {
  const map: Record<string, string> = {
    pending: t('deadlines.statusPending'),
    running: t('deadlines.statusRunning'),
    done: t('deadlines.statusDone'),
    overdue: t('deadlines.statusOverdue'),
  }
  return map[s] || s
}

function statusTagType(s: string) {
  if (s === 'done') return 'success'
  if (s === 'overdue') return 'danger'
  if (s === 'running') return 'primary'
  return 'info'
}

function offsetText(n: number) {
  if (n > 0) return `${n} 天前提醒`
  if (n === 0) return '当天提醒'
  return `逾期 ${-n} 天提醒`
}

function eventText(tp: string) {
  const map: Record<string, string> = {
    created: t('deadlines.eventCreated'),
    updated: t('deadlines.eventUpdated'),
    status_changed: t('deadlines.eventStatusChanged'),
    date_changed: t('deadlines.eventDateChanged'),
    deleted: t('deadlines.eventDeleted'),
  }
  return map[tp] || tp
}

function fmtTime(v: string) {
  if (!v) return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

// ---- 数据加载 ----

async function loadBoard() {
  try {
    const { data } = await teamApi.get('/deadlines/board')
    overdue.value = data.overdue || []
    today.value = data.today || []
    next3.value = data.next_3_days || []
    next7.value = data.next_7_days || []
    Object.assign(counts, data.counts || {})
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('deadlines.loadFailed'))
  }
}

// 筛选项一变就回到第一页，否则停在第 3 页换筛选条件会直接显示空表。
function applyFilters() {
  page.value = 1
  loadList()
}

async function loadList() {
  try {
    const params: Record<string, string | number> = { limit: pageSize, offset: (page.value - 1) * pageSize }
    if (filters.q) params.q = filters.q
    if (filters.status) params.status = filters.status
    if (filters.risk) params.risk = filters.risk
    if (filters.priority) params.priority = filters.priority
    const { data } = await teamApi.get('/deadlines', { params })
    list.value = data.data || []
    listTotal.value = data.total ?? list.value.length
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('deadlines.loadFailed'))
  }
}

async function loadRules() {
  try {
    const { data } = await teamApi.get('/reminder-rules')
    rules.value = data.data || []
  } catch {
    rules.value = []
  }
}

async function loadLog() {
  try {
    const { data } = await teamApi.get('/reminder-log')
    logs.value = data.data || []
  } catch {
    logs.value = []
  }
}

async function loadAll() {
  loading.value = true
  try {
    await Promise.all([loadBoard(), loadList(), loadRules(), loadLog()])
  } finally {
    loading.value = false
  }
}

// ---- 操作 ----

function jumpTo(key: string) {
  tab.value = 'list'
  page.value = 1
  // Map a board bucket to the equivalent list filter.
  if (key === 'overdue') {
    filters.risk = 'overdue'
  } else if (key === 'today') {
    filters.risk = 'critical'
  } else {
    filters.risk = 'open'
  }
  loadList()
}

function openCreate() {
  Object.assign(form, emptyForm())
  formVisible.value = true
}

function openEdit(d: Deadline | null) {
  if (!d) return
  Object.assign(form, {
    id: d.id,
    order_no: d.order_no,
    title: d.title,
    customer: d.customer || '',
    quantity: d.quantity,
    start_date: d.start_date || '',
    due_date: d.due_date,
    status: d.status,
    progress: d.progress,
    priority: d.priority,
    owner_id: d.owner_id || '',
    remark: d.remark || '',
    reason: '',
  })
  detailVisible.value = false
  formVisible.value = true
}

async function save() {
  if (!form.title && !form.order_no) {
    ElMessage.warning(t('deadlines.orderTitle'))
    return
  }
  if (!form.due_date) {
    ElMessage.warning(t('deadlines.dueDate'))
    return
  }
  saving.value = true
  try {
    const payload = { ...form }
    if (form.id) {
      await teamApi.put(`/deadlines/${form.id}`, payload)
    } else {
      await teamApi.post('/deadlines', payload)
    }
    ElMessage.success(t('deadlines.saveOk'))
    formVisible.value = false
    await loadAll()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('deadlines.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function openDetail(d: Deadline) {
  detail.value = d
  detailVisible.value = true
  events.value = []
  try {
    const { data } = await teamApi.get(`/deadlines/${d.id}/events`)
    events.value = data.data || []
  } catch {
    // history is best-effort; the detail view still renders
  }
}

async function removeDeadline() {
  if (!detail.value) return
  try {
    await ElMessageBox.confirm(t('deadlines.deleteConfirm'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await teamApi.delete(`/deadlines/${detail.value.id}`)
    ElMessage.success(t('deadlines.deleteOk'))
    detailVisible.value = false
    await loadAll()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('deadlines.saveFailed'))
  }
}

// ---- 规则 ----

function openRuleCreate() {
  Object.assign(ruleForm, { id: '', name: '', offset_days: 3, channel: 'inapp', target: 'owner', enabled: true })
  ruleVisible.value = true
}

function openRuleEdit(r: Rule) {
  Object.assign(ruleForm, { ...r })
  ruleVisible.value = true
}

async function saveRule() {
  if (!ruleForm.name) {
    ElMessage.warning(t('deadlines.ruleName'))
    return
  }
  saving.value = true
  try {
    if (ruleForm.id) {
      await teamApi.put(`/reminder-rules/${ruleForm.id}`, ruleForm)
    } else {
      await teamApi.post('/reminder-rules', ruleForm)
    }
    ElMessage.success(t('deadlines.saveOk'))
    ruleVisible.value = false
    await loadRules()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('deadlines.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function toggleRule(r: Rule, enabled: boolean) {
  try {
    await teamApi.put(`/reminder-rules/${r.id}`, { enabled })
    r.enabled = enabled
  } catch {
    ElMessage.error(t('deadlines.saveFailed'))
  }
}

async function removeRule(r: Rule) {
  try {
    await ElMessageBox.confirm(t('deadlines.deleteRuleConfirm'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await teamApi.delete(`/reminder-rules/${r.id}`)
    ElMessage.success(t('deadlines.ruleDeleted'))
    await loadRules()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('deadlines.saveFailed'))
  }
}

async function seedRules() {
  try {
    const { data } = await teamApi.post('/reminder-rules/seed')
    if (data.created > 0) {
      ElMessage.success(t('deadlines.seedOk', [data.created]))
    } else {
      ElMessage.info(data.message || t('deadlines.seedExists'))
    }
    await loadRules()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('deadlines.saveFailed'))
  }
}

onMounted(loadAll)
</script>

<style scoped>
.deadline-page {
  padding: 16px 20px;
  /* 以前是 height:100% + overflow-y:auto —— el-main 本身已经是滚动容器，
     双层滚动在小屏上会出现两条滚动条、且滚到底谁先到底取决于指针在哪。
     改为 min-height 让 el-main 做唯一的滚动容器。 */
  min-height: 100%;
  /* 与 docs-page / trash-page / admin-page 同为 #f5f7fa，
     之前是透明的，所以看板页是刺眼的纯白，与其它页并排切换时明显脱层。 */
  background: #f5f7fa;
  box-sizing: border-box;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  gap: 12px;
  flex-wrap: wrap;
}
.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}
.page-sub {
  color: #94a3b8;
  font-size: 13px;
  margin-left: 10px;
}
.toolbar-right {
  display: flex;
  gap: 8px;
}

/* 统计卡 */
.stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.stat-card {
  border-radius: 10px;
  padding: 14px 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.stat-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08);
}
.stat-card.plain {
  cursor: default;
}
.stat-card.overdue {
  background: #fef2f2;
  border-color: #fecaca;
}
.stat-card.today {
  background: #fff7ed;
  border-color: #fed7aa;
}
.stat-card.next3 {
  background: #fefce8;
  border-color: #fef08a;
}
.stat-card.next7 {
  background: #f0f9ff;
  border-color: #bae6fd;
}
.stat-num {
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
}
.stat-label {
  font-size: 12px;
  color: #64748b;
  margin-top: 2px;
}

/* tab 下间距和标签内边距：Element Plus 默认首个 tab 只有右内边距、
   末个只有左内边距，四个标签间距看起来不匀。改成均匀 16px。 */
.board-tabs :deep(.el-tabs__item) {
  padding: 0 16px;
}

/* 看板列 */
.columns {
  display: grid;
  grid-template-columns: repeat(4, minmax(240px, 1fr));
  gap: 14px;
  /* 以前是 align-items: start → 四列各自按内容高度收缩，底边参差不齐。
     改成 stretch 让四列等高，看板下沿是一条齐线。 */
  align-items: stretch;
}
@media (max-width: 1100px) {
  .columns {
    grid-template-columns: repeat(2, minmax(240px, 1fr));
  }
}
@media (max-width: 640px) {
  .columns {
    grid-template-columns: 1fr;
  }
}
.column {
  background: #f8fafc;
  border-radius: 10px;
  border: 1px solid #e2e8f0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 180px;
}
.column-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid #e2e8f0;
}
.column-head.overdue {
  background: #fee2e2;
  color: #b91c1c;
}
.column-head.today {
  background: #ffedd5;
  color: #c2410c;
}
.column-head.next3 {
  background: #fef9c3;
  color: #a16207;
}
.column-head.next7 {
  background: #e0f2fe;
  color: #0369a1;
}
.column-count {
  font-weight: 700;
}
.column-body {
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
  max-height: 60vh;
  overflow-y: auto;
}
.column-empty {
  color: #94a3b8;
  font-size: 12px;
  text-align: center;
  padding: 18px 0;
  /* 空列不再是一块弱到像坏掉的空白：给个浅浅的占位框 */
  border: 1px dashed #e2e8f0;
  border-radius: 8px;
  margin: 2px 0;
}

/* 订单卡 */
.deadline-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-left: 3px solid #cbd5e1;
  border-radius: 8px;
  padding: 10px 12px;
  cursor: pointer;
  transition: box-shadow 0.15s ease;
}
.deadline-card:hover {
  box-shadow: 0 2px 10px rgba(15, 23, 42, 0.1);
}
.deadline-card.overdue {
  border-left-color: #dc2626;
}
.deadline-card.critical {
  border-left-color: #ea580c;
}
.deadline-card.warning {
  border-left-color: #eab308;
}
.deadline-card.ok {
  border-left-color: #22c55e;
}
.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  /* 有优先级标签的卡头部 20px、无标签只有 14px，导致同列卡片高度
     在 116/110 之间跳。固定最小高度让所有卡等齐。 */
  min-height: 20px;
}
.card-order {
  font-size: 12px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.card-title {
  font-size: 13px;
  font-weight: 500;
  margin: 4px 0 6px;
  word-break: break-all;
}
.card-meta,
.card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: #64748b;
}
.card-foot {
  margin-top: 4px;
  padding-top: 6px;
  border-top: 1px dashed #e2e8f0;
}
.cell-unassigned,
.owner.unassigned {
  color: #cbd5e1;
}

/* 列表底部：总数 + 分页 */
.list-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
  gap: 12px;
  flex-wrap: wrap;
}
.list-foot .total {
  font-size: 12px;
  color: #64748b;
}

/* 风险着色 */
.days.overdue,
.due-cell.overdue {
  color: #dc2626;
  font-weight: 600;
}
.days.critical,
.due-cell.critical {
  color: #ea580c;
  font-weight: 600;
}
.days.warning,
.due-cell.warning {
  color: #a16207;
}
.days.ok,
.due-cell.ok {
  color: #16a34a;
}

/* 筛选 */
.filters {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

/* 规则 */
.rules-head {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
}
.hint {
  font-size: 12px;
  color: #94a3b8;
  line-height: 1.5;
}

/* 详情弹窗页脚：删除是破坏性操作，不该紧贴「关闭/编辑」，
   推到最左侧隔开，减少误点。 */
.detail-footer {
  display: flex;
  align-items: center;
  width: 100%;
}
.detail-footer .footer-gap {
  flex: 1;
}

/* 详情 */
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px 18px;
}
.detail-grid label {
  display: inline-block;
  width: 78px;
  color: #94a3b8;
  font-size: 13px;
}
.detail-grid span {
  font-size: 13px;
}
.muted {
  color: #94a3b8;
  font-size: 13px;
}
.timeline {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 260px;
  overflow-y: auto;
}
.event {
  display: flex;
  gap: 10px;
  font-size: 12px;
  align-items: baseline;
  flex-wrap: wrap;
}
.event-time {
  color: #94a3b8;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.event-type {
  background: #eef2ff;
  color: #4f46e5;
  border-radius: 4px;
  padding: 1px 6px;
}
.event-change {
  color: #334155;
}
.event-reason {
  color: #0f766e;
}
</style>
