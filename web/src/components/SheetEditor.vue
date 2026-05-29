<template>
  <div class="sheet-container" @keydown="onGlobalKeydown">
    <!-- 公式栏 -->
    <div class="formula-bar">
      <div class="cell-ref">{{ currentCellRef }}</div>
      <div class="formula-divider">fx</div>
      <input class="formula-input" v-model="formulaValue" @keydown.enter="applyFormula"
        @keydown.escape="cancelFormula" placeholder="输入内容或公式..." />
    </div>

    <!-- 工具栏 -->
    <div class="sheet-toolbar">
      <!-- 撤销重做 -->
      <el-button-group>
        <el-button size="small" :disabled="!canUndo" @click="undo" title="撤销 Ctrl+Z">
          <el-icon><RefreshLeft /></el-icon>
        </el-button>
        <el-button size="small" :disabled="!canRedo" @click="redo" title="重做 Ctrl+Y">
          <el-icon><RefreshRight /></el-icon>
        </el-button>
      </el-button-group>

      <el-divider direction="vertical" />

      <!-- 格式 -->
      <el-button-group>
        <el-button size="small" :type="getCellMeta(selection?.startRow, selection?.startCol)?.bold ? 'primary' : ''"
          @click="toggleFormat('bold')" title="加粗"><b>B</b></el-button>
        <el-button size="small" :type="getCellMeta(selection?.startRow, selection?.startCol)?.italic ? 'primary' : ''"
          @click="toggleFormat('italic')" title="斜体"><i>I</i></el-button>
      </el-button-group>

      <el-divider direction="vertical" />

      <!-- 颜色 -->
      <el-color-picker size="small" v-model="cellTextColor" @change="applyTextColor" title="文字颜色" />
      <el-color-picker size="small" v-model="cellBgColor" @change="applyBgColor" title="背景颜色" />

      <el-divider direction="vertical" />

      <!-- 对齐 -->
      <el-button-group>
        <el-button size="small" @click="setAlign('left')" title="左对齐">≡</el-button>
        <el-button size="small" @click="setAlign('center')" title="居中">≡</el-button>
        <el-button size="small" @click="setAlign('right')" title="右对齐">≡</el-button>
      </el-button-group>

      <el-divider direction="vertical" />

      <!-- 列类型 -->
      <el-select size="small" v-model="currentColType" @change="setColType" style="width:100px" title="列类型">
        <el-option label="自动" value="auto" />
        <el-option label="文本" value="text" />
        <el-option label="数字" value="number" />
        <el-option label="百分比" value="percent" />
        <el-option label="货币 ¥" value="currency" />
        <el-option label="日期" value="date" />
      </el-select>

      <el-divider direction="vertical" />

      <!-- 行列操作 -->
      <el-button-group>
        <el-button size="small" @click="addRowAbove" title="上方插入行"><el-icon><Top /></el-icon></el-button>
        <el-button size="small" @click="addRowBelow" title="下方插入行"><el-icon><Bottom /></el-icon></el-button>
        <el-button size="small" @click="deleteRow" title="删除行"><el-icon><Delete /></el-icon></el-button>
      </el-button-group>
      <el-button-group style="margin-left:4px">
        <el-button size="small" @click="addColLeft" title="左侧插入列"><el-icon><Back /></el-icon></el-button>
        <el-button size="small" @click="addColRight" title="右侧插入列"><el-icon><Right /></el-icon></el-button>
        <el-button size="small" @click="deleteCol" title="删除列"><el-icon><Delete /></el-icon></el-button>
      </el-button-group>

      <el-divider direction="vertical" />

      <!-- 冻结 -->
      <el-dropdown trigger="click" @command="toggleFreeze">
        <el-button size="small" title="冻结">
          <el-icon><Lock /></el-icon>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="row">{{ freezeRows > 0 ? '✓ ' : '' }}冻结首行</el-dropdown-item>
            <el-dropdown-item command="col">{{ freezeCols > 0 ? '✓ ' : '' }}冻结首列</el-dropdown-item>
            <el-dropdown-item command="none" divided>取消冻结</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <!-- 条件格式 -->
      <el-button size="small" @click="showCondDialog = true" title="条件格式">
        <el-icon><MagicStick /></el-icon>
      </el-button>

      <!-- 图表 -->
      <el-button size="small" @click="showChart = !showChart" :type="showChart ? 'primary' : ''">
        <el-icon><TrendCharts /></el-icon>
      </el-button>

      <span class="sheet-info">{{ rows.length }} 行 × {{ colCount }} 列</span>
    </div>

    <!-- 表格区域 -->
    <div class="sheet-scroll" ref="scrollRef" @contextmenu.prevent="showContextMenu" @click="hideContextMenu">
      <table class="sheet-table" ref="tableRef">
        <colgroup>
          <col style="width:40px" />
          <col v-for="(w, c) in colWidths" :key="c" :style="{ width: w + 'px' }" />
        </colgroup>
        <thead>
          <tr>
            <th class="corner" :class="{ 'frozen-corner': freezeRows > 0 && freezeCols > 0 }"></th>
            <th v-for="c in colCount" :key="c"
              class="col-header"
              :class="{
                selected: isColSelected(c - 1),
                'frozen-col-header': freezeRows > 0,
                sorted: sortCol === c - 1
              }"
              @click="selectCol(c - 1)"
            >
              <span>{{ colName(c - 1) }}</span>
              <span v-if="sortCol === c - 1" class="sort-icon">{{ sortDir === 'asc' ? '▲' : '▼' }}</span>
              <span v-if="filterActiveCols.has(c - 1)" class="filter-icon">⦿</span>
              <el-dropdown trigger="click" @command="(cmd: string) => handleColMenu(cmd, c - 1)" :hide-on-click="false" size="small">
                <span class="col-menu-trigger" @click.stop>▾</span>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="sort-asc">↑ 升序排列</el-dropdown-item>
                    <el-dropdown-item command="sort-desc">↓ 降序排列</el-dropdown-item>
                    <el-dropdown-item command="sort-clear" divided>取消排序</el-dropdown-item>
                    <el-dropdown-item command="filter" divided>
                      {{ filterActiveCols.has(c - 1) ? '✓ ' : '' }}筛选此列
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <div class="col-resize-handle" @mousedown.stop="startColResize(c - 1, $event)"></div>
            </th>
          </tr>
        </thead>
        <tbody>
          <template v-for="(row, ri) in rows" :key="ri">
            <tr v-show="!isRowFiltered(ri)" :class="{ 'frozen-row': freezeRows > 0 && ri < freezeRows }">
              <td class="row-header"
                :class="{
                  selected: isRowSelected(ri),
                  'frozen-row-header': freezeCols > 0,
                  'frozen-corner-cell': freezeRows > 0 && ri < freezeRows && freezeCols > 0
                }"
                @click="selectRow(ri)"
              >{{ ri + 1 }}</td>
              <td v-for="c in colCount" :key="c"
                class="cell"
                :class="{
                  selected: isSelected(ri, c - 1),
                  'selected-head': isSelectionHead(ri, c - 1),
                  editing: editingCell?.row === ri && editingCell?.col === c - 1,
                  'frozen-col-cell': freezeCols > 0 && c - 1 < freezeCols,
                  'frozen-row-cell': freezeRows > 0 && ri < freezeRows,
                  'frozen-corner-cell': freezeRows > 0 && ri < freezeRows && freezeCols > 0 && c - 1 < freezeCols,
                }"
                :style="getCellStyle(ri, c - 1)"
                :colspan="getColspan(ri, c - 1)"
                :rowspan="getRowspan(ri, c - 1)"
                v-show="!isCellHidden(ri, c - 1)"
                @click="selectCell(ri, c - 1, $event)"
                @dblclick="startEdit(ri, c - 1)"
              >
                <template v-if="editingCell?.row === ri && editingCell?.col === c - 1">
                  <input ref="editInput" class="cell-edit-input" v-model="editingValue"
                    @keydown.enter.prevent="finishEdit"
                    @keydown.tab.prevent="finishEdit(); moveNext()"
                    @keydown.escape="cancelEdit"
                    @keydown="handleEditKey" />
                </template>
                <template v-else>
                  <span class="cell-display" :style="getCellTextStyle(ri, c - 1)">{{ getCellDisplay(ri, c - 1) }}</span>
                </template>
                <!-- 拖拽填充柄 -->
                <div v-if="isSelectionHead(ri, c - 1) && !editingCell" class="fill-handle"
                  @mousedown.stop="startFill($event)"></div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- 右键菜单 -->
    <div v-if="contextMenu.show" class="context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }">
      <div class="menu-item" @click="ctxInsertRowAbove">上方插入行</div>
      <div class="menu-item" @click="ctxInsertRowBelow">下方插入行</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxInsertColLeft">左侧插入列</div>
      <div class="menu-item" @click="ctxInsertColRight">右侧插入列</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxCopy">复制 Ctrl+C</div>
      <div class="menu-item" @click="ctxCut">剪切 Ctrl+X</div>
      <div class="menu-item" @click="ctxPaste">粘贴 Ctrl+V</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxDeleteRow">删除行</div>
      <div class="menu-item" @click="ctxDeleteCol">删除列</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxClearCells">清空单元格</div>
      <div class="menu-item" @click="ctxMergeToggle">{{ hasMerge ? '取消合并' : '合并单元格' }}</div>
    </div>

    <!-- 图表面板 -->
    <div v-if="showChart" class="chart-panel">
      <div class="chart-header">
        <el-select v-model="chartType" size="small" style="width:100px">
          <el-option label="柱状图" value="bar" />
          <el-option label="折线图" value="line" />
          <el-option label="饼图" value="pie" />
        </el-select>
        <el-select v-model="chartDataRange" size="small" style="width:140px" placeholder="数据范围">
          <el-option label="当前列" value="col" />
          <el-option label="选中区域" value="selection" />
          <el-option label="全部数据" value="all" />
        </el-select>
        <input v-model="chartTitle" placeholder="图表标题" class="chart-title-input" />
        <el-button size="small" @click="exportChart" title="导出PNG">导出</el-button>
        <el-button size="small" text @click="showChart = false">✕</el-button>
      </div>
      <div class="chart-body">
        <canvas ref="chartCanvas" width="600" height="350"></canvas>
      </div>
    </div>

    <!-- 条件格式对话框 -->
    <el-dialog v-model="showCondDialog" title="条件格式" width="420px">
      <div v-for="(rule, i) in condRules" :key="i" class="cond-rule-item">
        <span>当单元格 {{ rule.condition }} {{ rule.value }} 时</span>
        <el-color-picker v-model="rule.bgColor" size="small" />
        <el-button size="small" text @click="condRules.splice(i, 1)">删除</el-button>
      </div>
      <el-divider />
      <div class="cond-new-rule">
        <el-select v-model="newCond.condition" size="small" style="width:120px">
          <el-option label="大于" value=">" />
          <el-option label="小于" value="<" />
          <el-option label="等于" value="=" />
          <el-option label="不等于" value="!=" />
          <el-option label="包含" value="contains" />
        </el-select>
        <el-input v-model="newCond.value" size="small" style="width:100px" placeholder="值" />
        <el-color-picker v-model="newCond.bgColor" size="small" />
        <el-button size="small" type="primary" @click="addCondRule">添加</el-button>
      </div>
    </el-dialog>

    <!-- 筛选面板 -->
    <div v-if="showFilterPanel" class="filter-panel"
      :style="{ left: filterPanelPos.x + 'px', top: filterPanelPos.y + 'px' }">
      <div class="filter-panel-header">
        <el-checkbox v-model="filterSelectAll" @change="toggleFilterAll">全选</el-checkbox>
      </div>
      <div class="filter-panel-list">
        <div v-for="val in filterUniqueValues" :key="val" class="filter-panel-item">
          <el-checkbox :model-value="filterSelectedValues.has(val)"
            @change="(v: boolean) => toggleFilterValue(val, v)">{{ val || '(空)' }}</el-checkbox>
        </div>
      </div>
      <div class="filter-panel-footer">
        <el-button size="small" @click="applyFilter(true)">确定</el-button>
        <el-button size="small" @click="applyFilter(false)">取消</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Top, Bottom, Back, Right, Delete, TrendCharts, RefreshLeft, RefreshRight, Lock, MagicStick } from '@element-plus/icons-vue'

// ─── Props & Emit ───
const props = defineProps<{ initialData?: string }>()
const emit = defineEmits<{ (e: 'change', data: string): void }>()

// ─── Core State ───
const rows = ref<string[][]>([])
const colCount = ref(10)
const colWidths = ref<number[]>([])
const scrollRef = ref<HTMLElement | null>(null)
const tableRef = ref<HTMLElement | null>(null)
const editInput = ref<any[]>([])

// ─── Selection ───
const selection = ref<{ startRow: number; startCol: number; endRow: number; endCol: number } | null>(null)
const editingCell = ref<{ row: number; col: number } | null>(null)
const editingValue = ref('')

// ─── Formula Bar ───
const formulaValue = ref('')
const currentCellRef = computed(() => {
  if (!selection.value) return ''
  return `${colName(selection.value.startCol)}${selection.value.startRow + 1}`
})

// ─── Cell Metadata ───
interface CellMeta {
  bold?: boolean; italic?: boolean; color?: string; bgColor?: string
  align?: 'left' | 'center' | 'right'; wrap?: boolean
  colspan?: number; rowspan?: number; merged?: boolean
}
const cellMeta = ref<Map<string, CellMeta>>(new Map())

function metaKey(r: number, c: number) { return `${r}:${c}` }
function getCellMeta(r?: number, c?: number): CellMeta | undefined {
  if (r == null || c == null) return undefined
  return cellMeta.value.get(metaKey(r, c))
}
function setCellMeta(r: number, c: number, m: Partial<CellMeta>) {
  const key = metaKey(r, c)
  const cur = cellMeta.value.get(key) || {}
  cellMeta.value.set(key, { ...cur, ...m })
}
function delCellMeta(r: number, c: number) { cellMeta.value.delete(metaKey(r, c)) }

// ─── Column Types ───
const colTypes = ref<string[]>([])
const currentColType = computed(() => {
  const c = selection.value?.startCol
  if (c == null) return 'auto'
  return colTypes.value[c] || 'auto'
})

function setColType(type: string) {
  if (!selection.value) return
  const c = selection.value.startCol
  colTypes.value[c] = type
}

// ─── Format Toolbar State ───
const cellTextColor = ref('')
const cellBgColor = ref('')

// ─── Undo/Redo ───
interface Snapshot {
  rows: string[][]; meta: [string, CellMeta][]; colTypes: string[]
}
const undoStack = ref<Snapshot[]>([])
const redoStack = ref<Snapshot[]>([])
const canUndo = computed(() => undoStack.value.length > 0)
const canRedo = computed(() => redoStack.value.length > 0)

function takeSnapshot(): Snapshot {
  return {
    rows: rows.value.map(r => [...r]),
    meta: Array.from(cellMeta.value.entries()),
    colTypes: [...colTypes.value]
  }
}
function pushUndo() {
  undoStack.value.push(takeSnapshot())
  if (undoStack.value.length > 50) undoStack.value.shift()
  redoStack.value = []
}
function undo() {
  if (!canUndo.value) return
  redoStack.value.push(takeSnapshot())
  const snap = undoStack.value.pop()!
  restoreSnapshot(snap)
}
function redo() {
  if (!canRedo.value) return
  undoStack.value.push(takeSnapshot())
  const snap = redoStack.value.pop()!
  restoreSnapshot(snap)
}
function restoreSnapshot(snap: Snapshot) {
  rows.value = snap.rows
  colCount.value = snap.rows[0]?.length || colCount.value
  cellMeta.value = new Map(snap.meta)
  colTypes.value = snap.colTypes
  emitChange()
}

// ─── Freeze ───
const freezeRows = ref(0)
const freezeCols = ref(0)

function toggleFreeze(cmd: string) {
  if (cmd === 'row') freezeRows.value = freezeRows.value > 0 ? 0 : 1
  else if (cmd === 'col') freezeCols.value = freezeCols.value > 0 ? 0 : 1
  else { freezeRows.value = 0; freezeCols.value = 0 }
}

// ─── Sort & Filter ───
const sortCol = ref(-1)
const sortDir = ref<'asc' | 'desc' | 'none'>('none')
const sortIndex = ref<number[]>([]) // maps display row -> data row

const filterActiveCols = ref<Set<number>>(new Set())
const filterHiddenRows = ref<Set<number>>(new Set())
const showFilterPanel = ref(false)
const filterPanelPos = ref({ x: 0, y: 0 })
const filterUniqueValues = ref<string[]>([])
const filterSelectedValues = ref<Set<string>>(new Set())
const filterSelectAll = ref(true)
let filterTargetCol = -1

function handleColMenu(cmd: string, col: number) {
  if (cmd === 'sort-asc') { sortCol.value = col; sortDir.value = 'asc'; rebuildSortIndex() }
  else if (cmd === 'sort-desc') { sortCol.value = col; sortDir.value = 'desc'; rebuildSortIndex() }
  else if (cmd === 'sort-clear') { sortCol.value = -1; sortDir.value = 'none'; sortIndex.value = [] }
  else if (cmd === 'filter') { toggleFilter(col) }
}

function rebuildSortIndex() {
  const c = sortCol.value
  if (c < 0) { sortIndex.value = []; return }
  const indices = rows.value.map((_, i) => i)
  const dir = sortDir.value === 'asc' ? 1 : -1
  indices.sort((a, b) => {
    const va = parseFloat(rows.value[a]?.[c]) || 0
    const vb = parseFloat(rows.value[b]?.[c]) || 0
    if (!isNaN(va) && !isNaN(vb)) return (va - vb) * dir
    return String(rows.value[a]?.[c] || '').localeCompare(String(rows.value[b]?.[c] || '')) * dir
  })
  sortIndex.value = indices
}

function toggleFilter(col: number) {
  if (filterActiveCols.value.has(col)) {
    filterActiveCols.value.delete(col)
    filterHiddenRows.value.clear()
    return
  }
  // Collect unique values
  const vals = new Set<string>()
  rows.value.forEach(r => vals.add(r?.[col] || ''))
  filterUniqueValues.value = Array.from(vals)
  filterSelectedValues.value = new Set(vals)
  filterSelectAll.value = true
  filterTargetCol = col
  showFilterPanel.value = true
}

function toggleFilterAll(checked: boolean) {
  if (checked) filterSelectedValues.value = new Set(filterUniqueValues.value)
  else filterSelectedValues.value = new Set()
}
function toggleFilterValue(val: string, checked: boolean) {
  if (checked) filterSelectedValues.value.add(val)
  else filterSelectedValues.value.delete(val)
}
function applyFilter(ok: boolean) {
  if (ok && filterTargetCol >= 0) {
    filterHiddenRows.value.clear()
    filterActiveCols.value.add(filterTargetCol)
    rows.value.forEach((r, i) => {
      if (!filterSelectedValues.value.has(r?.[filterTargetCol] || '')) filterHiddenRows.value.add(i)
    })
  }
  showFilterPanel.value = false
}
function isRowFiltered(ri: number): boolean { return filterHiddenRows.value.has(ri) }

// ─── Conditional Formatting ───
interface CondRule { condition: string; value: string; bgColor: string }
const condRules = ref<CondRule[]>([])
const showCondDialog = ref(false)
const newCond = ref<CondRule>({ condition: '>', value: '', bgColor: '#ffeb3b' })

function addCondRule() {
  if (newCond.value.value) condRules.value.push({ ...newCond.value })
}
function evalCond(cellVal: string, rule: CondRule): boolean {
  const n = parseFloat(cellVal), v = parseFloat(rule.value)
  switch (rule.condition) {
    case '>': return !isNaN(n) && !isNaN(v) && n > v
    case '<': return !isNaN(n) && !isNaN(v) && n < v
    case '=': return cellVal === rule.value
    case '!=': return cellVal !== rule.value
    case 'contains': return cellVal.includes(rule.value)
  }
  return false
}
function getCondBgColor(row: number, col: number): string {
  const val = rows.value[row]?.[col] || ''
  for (const rule of condRules.value) {
    if (evalCond(val, rule)) return rule.bgColor
  }
  return ''
}

// ─── Context Menu & Chart State ───
const contextMenu = ref<{ show: boolean; x: number; y: number }>({ show: false, x: 0, y: 0 })
const showChart = ref(false)
const chartType = ref('bar')
const chartDataRange = ref('col')
const chartCanvas = ref<HTMLCanvasElement | null>(null)
const chartTitle = ref('')

// ─── Clipboard ───
let clipData: string[][] | null = null
let clipCutFlag = false

function clipCopy() {
  if (!selection.value) return
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  clipData = []
  for (let r = r1; r <= r2; r++) {
    const row: string[] = []
    for (let c = c1; c <= c2; c++) row.push(rows.value[r]?.[c] || '')
    clipData.push(row)
  }
  clipCutFlag = false
  const tsv = clipData.map(r => r.join('\t')).join('\n')
  navigator.clipboard?.writeText(tsv).catch(() => {})
}

function clipCut() {
  clipCopy()
  clipCutFlag = true
}

function clipPaste() {
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol } = selection.value
  const doPaste = (data: string[][]) => {
    data.forEach((srcRow, dr) => {
      const tr = startRow + dr
      if (tr >= rows.value.length) return
      srcRow.forEach((val, dc) => {
        const tc = startCol + dc
        if (tc >= colCount.value) return
        rows.value[tr][tc] = val
      })
    })
    emitChange()
  }
  if (clipData) {
    doPaste(clipData)
    clipCutFlag = false
  } else {
    navigator.clipboard?.readText().then(tsv => {
      const data = tsv.split('\n').map(line => line.split('\t'))
      if (data.length && data[0].length) doPaste(data)
    }).catch(() => {})
  }
}

// ─── Merge Cells ───
const hasMerge = computed(() => {
  if (!selection.value) return false
  const m = getCellMeta(selection.value.startRow, selection.value.startCol)
  return (m?.colspan && m.colspan > 1) || (m?.rowspan && m.rowspan > 1)
})

function toggleMerge() {
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  const cspan = c2 - c1 + 1, rspan = r2 - r1 + 1
  const m = getCellMeta(r1, c1)
  if (m?.colspan && m.colspan > 1) {
    for (let r = r1; r <= r2; r++)
      for (let c = c1; c <= c2; c++) delCellMeta(r, c)
  } else {
    setCellMeta(r1, c1, { colspan: cspan, rowspan: rspan })
    for (let r = r1; r <= r2; r++)
      for (let c = c1; c <= c2; c++) {
        if (r === r1 && c === c1) continue
        setCellMeta(r, c, { merged: true })
      }
  }
  emitChange()
}

function getColspan(r: number, c: number): number | undefined {
  const m = getCellMeta(r, c)
  return m?.colspan && m.colspan > 1 ? m.colspan : undefined
}
function getRowspan(r: number, c: number): number | undefined {
  const m = getCellMeta(r, c)
  return m?.rowspan && m.rowspan > 1 ? m.rowspan : undefined
}
function isCellHidden(r: number, c: number): boolean {
  return !!getCellMeta(r, c)?.merged
}

// ─── Cell Style ───
function getCellStyle(r: number, c: number) {
  const m = getCellMeta(r, c)
  const bg = m?.bgColor || getCondBgColor(r, c) || ''
  return {
    backgroundColor: bg || undefined,
    textAlign: m?.align || (isNumberCol(c) ? 'right' : 'left'),
    whiteSpace: m?.wrap ? 'pre-wrap' : undefined,
    fontWeight: m?.bold ? 'bold' : undefined,
    fontStyle: m?.italic ? 'italic' : undefined,
  }
}
function getCellTextStyle(r: number, c: number) {
  const m = getCellMeta(r, c)
  return { color: m?.color || undefined }
}
function isNumberCol(c: number): boolean {
  const t = colTypes.value[c]
  return t === 'number' || t === 'currency' || t === 'percent'
}

// ─── Format Toolbar ───
function toggleFormat(key: 'bold' | 'italic') {
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  const cur = !!getCellMeta(r1, c1)?.[key]
  for (let r = r1; r <= r2; r++)
    for (let c = c1; c <= c2; c++) setCellMeta(r, c, { [key]: !cur })
  emitChange()
}
function applyTextColor(color: string) {
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++)
      setCellMeta(r, c, { color: color || undefined })
  emitChange()
}
function applyBgColor(color: string) {
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++)
      setCellMeta(r, c, { bgColor: color || undefined })
  emitChange()
}
function setAlign(align: 'left' | 'center' | 'right') {
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++)
      setCellMeta(r, c, { align })
  emitChange()
}

// ─── Drag Fill ───
const isDragging = ref(false)
const fillTarget = ref<{ row: number; col: number } | null>(null)

function startFill(e: MouseEvent) {
  if (!selection.value) return
  e.preventDefault()
  isDragging.value = true
  const startR = Math.min(selection.value.startRow, selection.value.endRow)
  const startC = Math.min(selection.value.startCol, selection.value.endCol)
  const onMove = (ev: MouseEvent) => {
    const el = document.elementFromPoint(ev.clientX, ev.clientY)
    const td = el?.closest('td.cell')
    if (td) {
      const ri = Array.from(td.parentElement!.children).indexOf(td) - 1
      fillTarget.value = { row: ri, col: startC }
    }
  }
  const onUp = () => {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
    isDragging.value = false
    if (!fillTarget.value || !selection.value) return
    pushUndo()
    const endR = fillTarget.value.row
    const c = startC
    const srcVal = rows.value[startR]?.[c] || ''
    const srcNum = parseFloat(srcVal)
    for (let r = startR + 1; r <= endR; r++) {
      if (!isNaN(srcNum)) rows.value[r][c] = String(srcNum + (r - startR))
      else rows.value[r][c] = srcVal
    }
    fillTarget.value = null
    emitChange()
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

// ─── Chart Drawing ───
function drawChart() {
  const canvas = chartCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const W = canvas.width, H = canvas.height
  ctx.clearRect(0, 0, W, H)
  ctx.fillStyle = '#fff'; ctx.fillRect(0, 0, W, H)
  let labels: string[] = [], values: number[] = []
  if (chartDataRange.value === 'col' && selection.value) {
    const c = selection.value.startCol
    for (let r = 0; r < rows.value.length; r++) {
      if (isRowFiltered(r)) continue
      labels.push(rows.value[r]?.[0] || String(r + 1))
      values.push(parseFloat(rows.value[r]?.[c]) || 0)
    }
  } else {
    for (let r = 0; r < rows.value.length; r++) {
      if (isRowFiltered(r)) continue
      labels.push(rows.value[r]?.[0] || String(r + 1))
      values.push(parseFloat(rows.value[r]?.[1]) || 0)
    }
  }
  if (!values.length) return
  const colors = ['#409EFF', '#67C23A', '#E6A23C', '#F56C6C', '#909399', '#00D1B2', '#FF6B6B', '#48C774']
  if (chartTitle.value) {
    ctx.fillStyle = '#333'; ctx.font = 'bold 14px sans-serif'; ctx.textAlign = 'center'
    ctx.fillText(chartTitle.value, W / 2, 18)
  }
  const titleH = chartTitle.value ? 24 : 0
  if (chartType.value === 'pie') {
    const total = values.reduce((a, b) => a + b, 0)
    if (total === 0) return
    const cx = W / 2, cy = titleH + (H - titleH) / 2, radius = Math.min(W, H - titleH) / 2 - 40
    let startAngle = -Math.PI / 2
    values.forEach((v, i) => {
      const slice = (v / total) * Math.PI * 2
      ctx.beginPath(); ctx.moveTo(cx, cy); ctx.arc(cx, cy, radius, startAngle, startAngle + slice)
      ctx.fillStyle = colors[i % colors.length]; ctx.fill()
      ctx.strokeStyle = '#fff'; ctx.lineWidth = 2; ctx.stroke()
      const mid = startAngle + slice / 2
      const lx = cx + (radius * 0.65) * Math.cos(mid), ly = cy + (radius * 0.65) * Math.sin(mid)
      ctx.fillStyle = '#fff'; ctx.font = '11px sans-serif'; ctx.textAlign = 'center'
      if (labels[i]) ctx.fillText(labels[i], lx, ly)
      startAngle += slice
    })
  } else {
    const max = Math.max(...values) * 1.2 || 1
    const padL = 50, padR = 20, padT = 20 + titleH, padB = 40
    const chartW = W - padL - padR, chartH = H - padT - padB
    ctx.strokeStyle = '#ddd'
    ctx.beginPath(); ctx.moveTo(padL, padT); ctx.lineTo(padL, padT + chartH); ctx.stroke()
    ctx.beginPath(); ctx.moveTo(padL, padT + chartH); ctx.lineTo(padL + chartW, padT + chartH); ctx.stroke()
    ctx.fillStyle = '#999'; ctx.font = '10px sans-serif'; ctx.textAlign = 'right'
    for (let i = 0; i <= 5; i++) {
      const y = padT + (chartH / 5) * i
      ctx.fillText((max * (5 - i) / 5).toFixed(0), padL - 6, y + 4)
      ctx.strokeStyle = '#f0f0f0'; ctx.beginPath(); ctx.moveTo(padL, y); ctx.lineTo(padL + chartW, y); ctx.stroke()
    }
    const step = chartW / (values.length || 1)
    ctx.textAlign = 'center'
    labels.forEach((l, i) => ctx.fillText(l, padL + step * i + step / 2, padT + chartH + 16))
    if (chartType.value === 'bar') {
      const bw = Math.min(step * 0.6, 40)
      values.forEach((v, i) => {
        const h = (v / max) * chartH
        ctx.fillStyle = colors[i % colors.length]
        ctx.fillRect(padL + step * i + (step - bw) / 2, padT + chartH - h, bw, h)
      })
    } else {
      ctx.strokeStyle = '#409EFF'; ctx.lineWidth = 2; ctx.beginPath()
      values.forEach((v, i) => {
        const x = padL + step * i + step / 2, y = padT + chartH - (v / max) * chartH
        i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)
      }); ctx.stroke()
      values.forEach((v, i) => {
        const x = padL + step * i + step / 2, y = padT + chartH - (v / max) * chartH
        ctx.fillStyle = '#409EFF'; ctx.beginPath(); ctx.arc(x, y, 4, 0, Math.PI * 2); ctx.fill()
      })
    }
  }
}

function exportChart() {
  drawChart()
  const canvas = chartCanvas.value
  if (!canvas) return
  const link = document.createElement('a')
  link.download = 'chart.png'
  link.href = canvas.toDataURL('image/png')
  link.click()
}

watch([showChart, chartType, chartDataRange, chartTitle], () => { nextTick(drawChart) })

// ─── Selection Helpers ───
function colName(i: number): string {
  let name = '', n = i + 1
  while (n > 0) { n--; name = String.fromCharCode(65 + (n % 26)) + name; n = Math.floor(n / 26) }
  return name
}
function makeRow(cols: number) { return new Array(cols).fill('') }
function initEmpty(numRows = 50, numCols = 10) {
  colCount.value = numCols
  colWidths.value = Array(numCols).fill(120)
  colTypes.value = Array(numCols).fill('auto')
  rows.value = Array.from({ length: numRows }, () => makeRow(numCols))
}

// ─── Load Data ───
function loadData() {
  if (props.initialData && props.initialData !== '{}') {
    try {
      const p = JSON.parse(props.initialData)
      if (p.rows?.length) {
        rows.value = p.rows
        colCount.value = p.cols || p.rows[0]?.length || 10
        colWidths.value = p.colWidths || Array(colCount.value).fill(120)
        colTypes.value = p.colTypes || Array(colCount.value).fill('auto')
        if (p.cellMeta) cellMeta.value = new Map(p.cellMeta)
        if (p.freezeRows) freezeRows.value = p.freezeRows
        if (p.freezeCols) freezeCols.value = p.freezeCols
        if (p.condRules) condRules.value = p.condRules
        return
      }
    } catch { /* ignore */ }
  }
  initEmpty()
}

// ─── Cell Selection ───
function selectCell(row: number, col: number, e?: MouseEvent) {
  if (e?.shiftKey && selection.value) {
    selection.value = { ...selection.value, endRow: row, endCol: col }
  } else {
    selection.value = { startRow: row, startCol: col, endRow: row, endCol: col }
  }
  editingCell.value = null
  updateFormula()
  const m = getCellMeta(row, col)
  cellTextColor.value = m?.color || ''
  cellBgColor.value = m?.bgColor || ''
}
function selectRow(row: number) {
  selection.value = { startRow: row, startCol: 0, endRow: row, endCol: colCount.value - 1 }
  editingCell.value = null
}
function selectCol(col: number) {
  selection.value = { startRow: 0, startCol: col, endRow: rows.value.length - 1, endCol: col }
  editingCell.value = null
}
function isSelected(row: number, col: number): boolean {
  if (!selection.value) return false
  const r1 = Math.min(selection.value.startRow, selection.value.endRow), r2 = Math.max(selection.value.startRow, selection.value.endRow)
  const c1 = Math.min(selection.value.startCol, selection.value.endCol), c2 = Math.max(selection.value.startCol, selection.value.endCol)
  return row >= r1 && row <= r2 && col >= c1 && col <= c2
}
function isSelectionHead(row: number, col: number): boolean {
  return selection.value?.startRow === row && selection.value?.startCol === col
}
function isRowSelected(row: number): boolean {
  if (!selection.value) return false
  return row >= Math.min(selection.value.startRow, selection.value.endRow) && row <= Math.max(selection.value.startRow, selection.value.endRow)
}
function isColSelected(col: number): boolean {
  if (!selection.value) return false
  return col >= Math.min(selection.value.startCol, selection.value.endCol) && col <= Math.max(selection.value.startCol, selection.value.endCol)
}

// ─── Editing ───
function startEdit(row: number, col: number) {
  editingCell.value = { row, col }
  editingValue.value = rows.value[row]?.[col] || ''
  nextTick(() => {
    if (editInput.value?.length) {
      const el = editInput.value[0]?.$el || editInput.value[0]
      el?.focus()
    }
  })
}
function finishEdit() {
  if (!editingCell.value) return
  pushUndo()
  rows.value[editingCell.value.row][editingCell.value.col] = editingValue.value
  editingCell.value = null
  emitChange()
}
function cancelEdit() { editingCell.value = null; updateFormula() }
function moveNext() {
  if (!selection.value) return
  const { startRow, startCol } = selection.value
  if (startCol + 1 < colCount.value) selectCell(startRow, startCol + 1)
  else if (startRow + 1 < rows.value.length) selectCell(startRow + 1, 0)
}
function updateFormula() {
  if (!selection.value) { formulaValue.value = ''; return }
  formulaValue.value = rows.value[selection.value.startRow]?.[selection.value.startCol] || ''
}
function applyFormula() {
  if (!selection.value) return
  pushUndo()
  rows.value[selection.value.startRow][selection.value.startCol] = formulaValue.value
  editingCell.value = null
  emitChange()
}
function cancelFormula() { updateFormula() }

// ─── Cell Display & Formatting ───
function getCellDisplay(row: number, col: number): string {
  const val = rows.value[row]?.[col] || ''
  if (val.startsWith('=')) return evaluateFormula(val)
  const ct = colTypes.value[col] || 'auto'
  const n = parseFloat(val)
  switch (ct) {
    case 'number': if (!isNaN(n) && val) return formatNumber(n)
    case 'currency': if (!isNaN(n) && val) return '¥' + formatNumber(n)
    case 'percent': if (!isNaN(n) && val) return (n * 100).toFixed(2) + '%'
    case 'date': if (val) return formatDate(val)
  }
  return val
}
function formatNumber(n: number): string {
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}
function formatDate(val: string): string {
  const d = new Date(val)
  if (isNaN(d.getTime())) return val
  return d.toLocaleDateString('zh-CN')
}

// ─── Formula Engine ───
function evaluateFormula(formula: string): string {
  try { return String(parseExpr(formula.substring(1))) } catch { return formula }
}

function parseExpr(expr: string): any {
  expr = expr.trim()
  if (expr.startsWith('"') && expr.endsWith('"')) return expr.slice(1, -1)
  if (/^-?\d+(\.\d+)?$/.test(expr)) return parseFloat(expr)
  if (expr.toUpperCase() === 'TRUE') return true
  if (expr.toUpperCase() === 'FALSE') return false
  const fnMatch = expr.match(/^([A-Z]+)\((.+)\)$/is)
  if (fnMatch) return callFn(fnMatch[1].toUpperCase(), fnMatch[2])
  if (/^[A-Z]+\d+:[A-Z]+\d+$/.test(expr.toUpperCase())) return getRange(expr.toUpperCase())
  const cellM = expr.match(/^([A-Z]+)(\d+)$/i)
  if (cellM) return getCellVal(cellM[1].toUpperCase(), parseInt(cellM[2]))
  if (/[+\-*/()><=!]/.test(expr)) return safeCalc(expr)
  return expr
}

function splitArgs(s: string): string[] {
  const args: string[] = []; let depth = 0, cur = ''
  for (let i = 0; i < s.length; i++) {
    const ch = s[i]
    if (ch === '(') depth++; else if (ch === ')') depth--
    if (ch === ',' && depth === 0) { args.push(cur.trim()); cur = '' }
    else cur += ch
  }
  if (cur.trim()) args.push(cur.trim())
  return args
}

function callFn(name: string, rawArgs: string): any {
  const args = splitArgs(rawArgs)
  const parsed = args.map(a => parseExpr(a))
  switch (name) {
    case 'SUM': return numArray(args[0]).reduce((a, b) => a + b, 0)
    case 'AVG': case 'AVERAGE': { const a = numArray(args[0]); return a.length ? Math.round(a.reduce((s, v) => s + v, 0) / a.length * 100) / 100 : 0 }
    case 'COUNT': return numArray(args[0], true).length
    case 'MAX': { const a = numArray(args[0]); return a.length ? Math.max(...a) : 0 }
    case 'MIN': { const a = numArray(args[0]); return a.length ? Math.min(...a) : 0 }
    case 'COUNTA': return strArray(args[0]).filter(v => v !== '').length
    case 'COUNTIF': return strArray(args[0]).filter(v => testCond(v, parsed[1])).length
    case 'SUMIF': return numArray(args[0], false, true).filter((_, i) => testCond(strArray(args[0], true)[i], parsed[1])).reduce((a, b) => a + b, 0)
    case 'AVERAGEIF': { const s = numArray(args[0]), c = parsed[1]; const f = s.filter((_, i) => testCond(strArray(args[0], true)[i], c)); return f.length ? Math.round(f.reduce((a, b) => a + b, 0) / f.length * 100) / 100 : 0 }
    case 'ABS': return Math.abs(parsed[0])
    case 'ROUND': return Math.round(parsed[0] * Math.pow(10, parsed[1] || 0)) / Math.pow(10, parsed[1] || 0)
    case 'CEIL': case 'CEILING': return Math.ceil(parsed[0])
    case 'FLOOR': return Math.floor(parsed[0])
    case 'INT': return Math.trunc(parsed[0])
    case 'POWER': case 'POW': return Math.pow(parsed[0], parsed[1])
    case 'MOD': return parsed[0] % parsed[1]
    case 'SQRT': return Math.sqrt(parsed[0])
    case 'LOG': return parsed[1] ? Math.log(parsed[0]) / Math.log(parsed[1]) : Math.log10(parsed[0])
    case 'LN': return Math.log(parsed[0])
    case 'EXP': return Math.exp(parsed[0])
    case 'PI': return Math.PI
    case 'RAND': return Math.random()
    case 'RANDBETWEEN': return Math.floor(Math.random() * (parsed[1] - parsed[0] + 1)) + parsed[0]
    case 'SIGN': return Math.sign(parsed[0])
    case 'TRUNC': return Math.trunc(parsed[0])
    case 'PRODUCT': return numArray(args[0]).reduce((a, b) => a * b, 1)
    case 'MEDIAN': { const a = numArray(args[0]).sort((x, y) => x - y); const m = Math.floor(a.length / 2); return a.length % 2 ? a[m] : (a[m - 1] + a[m]) / 2 }
    case 'STDEV': { const a = numArray(args[0]); const avg = a.reduce((s, v) => s + v, 0) / a.length; return Math.sqrt(a.reduce((s, v) => s + (v - avg) ** 2, 0) / (a.length - 1)) }
    case 'VAR': { const a = numArray(args[0]); const avg = a.reduce((s, v) => s + v, 0) / a.length; return a.reduce((s, v) => s + (v - avg) ** 2, 0) / (a.length - 1) }
    case 'IF': return parsed[0] ? parsed[1] : parsed[2]
    case 'AND': return parsed.every(v => !!v)
    case 'OR': return parsed.some(v => !!v)
    case 'NOT': return !parsed[0]
    case 'XOR': return parsed.filter(v => !!v).length === 1
    case 'IFS': { for (let i = 0; i < parsed.length - 1; i += 2) { if (parsed[i]) return parsed[i + 1] } return parsed.length % 2 ? parsed[parsed.length - 1] : '#N/A' }
    case 'SWITCH': { const val = parsed[0]; for (let i = 1; i < parsed.length - 1; i += 2) { if (val == parsed[i]) return parsed[i + 1] } return parsed.length % 2 === 0 ? parsed[parsed.length - 1] : '#N/A' }
    case 'ISBLANK': return parsed[0] === '' || parsed[0] === undefined || parsed[0] === null
    case 'ISNUMBER': return !isNaN(Number(parsed[0])) && parsed[0] !== ''
    case 'ISTEXT': return typeof parsed[0] === 'string' && isNaN(Number(parsed[0]))
    case 'CONCAT': case 'CONCATENATE': return parsed.join('')
    case 'LEN': return String(parsed[0]).length
    case 'LEFT': return String(parsed[0]).substring(0, parsed[1] || 1)
    case 'RIGHT': return String(parsed[0]).slice(-(parsed[1] || 1))
    case 'MID': return String(parsed[0]).substring(parsed[1] - 1, parsed[1] - 1 + parsed[2])
    case 'UPPER': return String(parsed[0]).toUpperCase()
    case 'LOWER': return String(parsed[0]).toLowerCase()
    case 'TRIM': return String(parsed[0]).trim()
    case 'SUBSTITUTE': case 'REPLACE': return String(parsed[0]).split(String(parsed[1])).join(String(parsed[2]))
    case 'REPT': return String(parsed[0]).repeat(parsed[1])
    case 'FIND': return String(parsed[1]).indexOf(String(parsed[0])) + 1
    case 'TEXT': return String(parsed[0])
    case 'VALUE': return parseFloat(String(parsed[0])) || 0
    case 'EXACT': return String(parsed[0]) === String(parsed[1])
    case 'JOIN': return strArray(args[1]).join(String(parsed[0]))
    case 'NOW': return new Date().toLocaleString('zh-CN')
    case 'TODAY': return new Date().toLocaleDateString('zh-CN')
    case 'YEAR': return new Date(String(parsed[0])).getFullYear()
    case 'MONTH': return new Date(String(parsed[0])).getMonth() + 1
    case 'DAY': return new Date(String(parsed[0])).getDate()
    case 'HOUR': return new Date(String(parsed[0])).getHours()
    case 'MINUTE': return new Date(String(parsed[0])).getMinutes()
    case 'DATEDIF': { const d1 = new Date(String(parsed[0])), d2 = new Date(String(parsed[1])); const diff = (d2.getTime() - d1.getTime()) / 86400000; return parsed[2] === 'Y' ? Math.floor(diff / 365.25) : parsed[2] === 'M' ? Math.floor(diff / 30.44) : Math.floor(diff) }
    case 'WEEKDAY': return new Date(String(parsed[0])).getDay() + 1
    case 'WEEKNUM': { const d = new Date(String(parsed[0])); d.setHours(0, 0, 0, 0); d.setDate(d.getDate() + 3 - (d.getDay() + 6) % 7); const w1 = new Date(d.getFullYear(), 0, 4); return 1 + Math.round(((d.getTime() - w1.getTime()) / 86400000 - 3 + (w1.getDay() + 6) % 7) / 7) }
    case 'VLOOKUP': { const key = String(parsed[0]); const vals = strArray(args[1], true); const col = parsed[2] - 1; const data = getRangeData(args[1]); const idx = vals.indexOf(key); return idx >= 0 && data[idx] ? getCellValByRC(data[idx].r, data[idx].c + col) : '#N/A' }
    case 'INDEX': { const arr = numArray(args[0], false, true); return arr[parsed[1] - 1] ?? '#N/A' }
    case 'MATCH': { const arr = strArray(args[0], true); const idx = arr.indexOf(String(parsed[1])); return idx >= 0 ? idx + 1 : '#N/A' }
    case 'CHOOSE': return parsed[parsed[0]] ?? '#N/A'
    default: return '#NAME?'
  }
}

// ─── Formula Helpers ───
function getCellVal(col: string, row: number): any {
  const c = colIndex(col), r = row - 1
  const v = rows.value[r]?.[c]
  if (v === undefined || v === '') return ''
  const n = parseFloat(v)
  return isNaN(n) ? v : n
}
function getCellValByRC(r: number, c: number): any {
  const v = rows.value[r]?.[c]
  if (v === undefined || v === '') return ''
  const n = parseFloat(v)
  return isNaN(n) ? v : n
}
function getRange(rangeStr: string): any[] {
  const m = rangeStr.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (!m) return []
  const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1
  const result: any[] = []
  for (let r = r1; r <= r2; r++)
    for (let c = c1; c <= c2; c++)
      result.push(rows.value[r]?.[c] || '')
  return result
}
interface RC { r: number; c: number }
function getRangeData(arg: string): RC[] {
  const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (!m) return []
  const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1
  const result: RC[] = []
  for (let r = r1; r <= r2; r++)
    for (let c = c1; c <= c2; c++)
      result.push({ r, c })
  return result
}
function numArray(arg: string, countNonNum = false, raw = false): number[] {
  const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (m) {
    const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1
    const result: number[] = []
    for (let r = r1; r <= r2; r++)
      for (let c = c1; c <= c2; c++) {
        const v = rows.value[r]?.[c]
        if (countNonNum) { if (v !== undefined && v !== '') result.push(raw ? parseFloat(v) || 0 : 1) }
        else { const n = parseFloat(v); if (!isNaN(n)) result.push(n) }
      }
    return result
  }
  return arg.split(',').map(v => parseFloat(v.trim())).filter(v => !isNaN(v))
}
function strArray(arg: string, keepAll = false): string[] {
  const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (m) {
    const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1
    const result: string[] = []
    for (let r = r1; r <= r2; r++)
      for (let c = c1; c <= c2; c++) {
        const v = rows.value[r]?.[c]
        if (keepAll || (v !== undefined && v !== '')) result.push(v || '')
      }
    return result
  }
  return arg.split(',').map(v => v.trim())
}
function testCond(value: string, cond: any): boolean {
  const cs = String(cond)
  const opMatch = cs.match(/^(>=|<=|!=|>|<|=)/)
  if (!opMatch) return value === cs
  const op = opMatch[1], target = cs.slice(op.length)
  const a = parseFloat(value), b = parseFloat(target)
  if (!isNaN(a) && !isNaN(b)) {
    if (op === '>') return a > b; if (op === '<') return a < b
    if (op === '>=') return a >= b; if (op === '<=') return a <= b
    if (op === '!=') return a !== b; if (op === '=') return a === b
  }
  if (op === '=') return value === target
  if (op === '!=') return value !== target
  return false
}
function safeCalc(expr: string): number {
  let safe = expr.replace(/([A-Z]+)(\d+)/gi, (_, col, row) => {
    const v = getCellVal(col.toUpperCase(), parseInt(row))
    return isNaN(v) ? '0' : String(v)
  })
  safe = safe.replace(/[^0-9+\-*/.() ><=!]/g, '')
  try { return Function('"use strict"; return (' + safe + ')')() } catch { return NaN }
}
function colIndex(name: string): number {
  let idx = 0
  for (let i = 0; i < name.length; i++) idx = idx * 26 + (name.charCodeAt(i) - 64)
  return idx - 1
}

// ─── Row/Col Operations ───
function insertRowAt(index: number) {
  pushUndo()
  rows.value.splice(index, 0, makeRow(colCount.value))
  adjustSelection(); emitChange()
}
function deleteRowAt(index: number) {
  if (rows.value.length <= 1) return
  pushUndo()
  rows.value.splice(index, 1)
  adjustSelection(); emitChange()
}
function insertColAt(index: number) {
  pushUndo()
  colCount.value++
  colWidths.value.splice(index, 0, 120)
  colTypes.value.splice(index, 0, 'auto')
  rows.value.forEach(row => row.splice(index, 0, ''))
  adjustSelection(); emitChange()
}
function deleteColAt(index: number) {
  if (colCount.value <= 1) return
  pushUndo()
  colCount.value--
  colWidths.value.splice(index, 1)
  colTypes.value.splice(index, 1)
  rows.value.forEach(row => row.splice(index, 1))
  adjustSelection(); emitChange()
}
function adjustSelection() {
  if (!selection.value) return
  selection.value.endRow = Math.min(selection.value.endRow, rows.value.length - 1)
  selection.value.endCol = Math.min(selection.value.endCol, colCount.value - 1)
}

// Toolbar shortcuts
function addRowAbove() { insertRowAt(selection.value?.startRow ?? 0) }
function addRowBelow() { insertRowAt((selection.value?.startRow ?? rows.value.length - 1) + 1) }
function addColLeft() { insertColAt(selection.value?.startCol ?? 0) }
function addColRight() { insertColAt((selection.value?.startCol ?? colCount.value - 1) + 1) }
function deleteRow() { deleteRowAt(selection.value?.startRow ?? 0) }
function deleteCol() { deleteColAt(selection.value?.startCol ?? 0) }

// ─── Context Menu ───
function showContextMenu(e: MouseEvent) {
  contextMenu.value = { show: true, x: e.clientX, y: e.clientY }
}
function hideContextMenu() { contextMenu.value.show = false }
function ctxInsertRowAbove() { hideContextMenu(); addRowAbove() }
function ctxInsertRowBelow() { hideContextMenu(); addRowBelow() }
function ctxInsertColLeft() { hideContextMenu(); addColLeft() }
function ctxInsertColRight() { hideContextMenu(); addColRight() }
function ctxDeleteRow() { hideContextMenu(); deleteRow() }
function ctxDeleteCol() { hideContextMenu(); deleteCol() }
function ctxCopy() { hideContextMenu(); clipCopy() }
function ctxCut() { hideContextMenu(); clipCut() }
function ctxPaste() { hideContextMenu(); clipPaste() }
function ctxMergeToggle() { hideContextMenu(); toggleMerge() }
function ctxClearCells() {
  hideContextMenu()
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++)
      rows.value[r][c] = ''
  emitChange()
}

// ─── Column Resize ───
function startColResize(col: number, e: MouseEvent) {
  e.preventDefault()
  const startX = e.clientX, startWidth = colWidths.value[col] || 120
  const onMove = (ev: MouseEvent) => { colWidths.value[col] = Math.max(40, startWidth + ev.clientX - startX) }
  const onUp = () => { document.removeEventListener('mousemove', onMove); document.removeEventListener('mouseup', onUp); emitChange() }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

// ─── Edit Key ───
function handleEditKey(e: KeyboardEvent) {
  if (e.key === 'ArrowUp') { e.preventDefault(); finishEdit(); moveUp() }
  if (e.key === 'ArrowDown') { e.preventDefault(); finishEdit(); moveDown() }
}
function moveUp() {
  if (!selection.value || selection.value.startRow <= 0) return
  selectCell(selection.value.startRow - 1, selection.value.startCol); updateFormula()
}
function moveDown() {
  if (!selection.value) return
  if (selection.value.startRow >= rows.value.length - 1) rows.value.push(makeRow(colCount.value))
  selectCell(selection.value.startRow + 1, selection.value.startCol); updateFormula()
}

// ─── Emit ───
function emitChange() {
  const metaArr = Array.from(cellMeta.value.entries())
  emit('change', JSON.stringify({
    rows: rows.value, cols: colCount.value, colWidths: colWidths.value,
    colTypes: colTypes.value, cellMeta: metaArr,
    freezeRows: freezeRows.value, freezeCols: freezeCols.value,
    condRules: condRules.value
  }))
}
function getData(): string {
  const metaArr = Array.from(cellMeta.value.entries())
  return JSON.stringify({
    rows: rows.value, cols: colCount.value, colWidths: colWidths.value,
    colTypes: colTypes.value, cellMeta: metaArr,
    freezeRows: freezeRows.value, freezeCols: freezeCols.value,
    condRules: condRules.value
  })
}

// ─── Global Keyboard ───
function onGlobalKeydown(e: KeyboardEvent) {
  // Undo/Redo
  if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) { e.preventDefault(); undo(); return }
  if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.key === 'z' && e.shiftKey))) { e.preventDefault(); redo(); return }
  // Copy/Cut/Paste
  if ((e.ctrlKey || e.metaKey) && e.key === 'c') { if (editingCell.value) return; e.preventDefault(); clipCopy(); return }
  if ((e.ctrlKey || e.metaKey) && e.key === 'x') { if (editingCell.value) return; e.preventDefault(); clipCut(); return }
  if ((e.ctrlKey || e.metaKey) && e.key === 'v') { if (editingCell.value) return; e.preventDefault(); clipPaste(); return }

  if (editingCell.value) return
  if (!selection.value) return

  if (e.key === 'Delete' || e.key === 'Backspace') {
    pushUndo()
    const { startRow, startCol, endRow, endCol } = selection.value
    for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
      for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++)
        rows.value[r][c] = ''
    emitChange(); e.preventDefault()
  } else if (e.key === 'Enter') { startEdit(selection.value.startRow, selection.value.startCol); e.preventDefault() }
  else if (e.key === 'Tab') { moveNext(); e.preventDefault() }
  else if (e.key === 'ArrowUp') { moveUp() }
  else if (e.key === 'ArrowDown') { moveDown() }
  else if (e.key === 'ArrowLeft') { if (selection.value.startCol > 0) { selectCell(selection.value.startRow, selection.value.startCol - 1); updateFormula() } }
  else if (e.key === 'ArrowRight') { if (selection.value.startCol < colCount.value - 1) { selectCell(selection.value.startRow, selection.value.startCol + 1); updateFormula() } }
  else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey) {
    editingValue.value = e.key
    startEdit(selection.value.startRow, selection.value.startCol)
  }
}

// ─── Lifecycle ───
loadData()

onMounted(() => {
  document.addEventListener('keydown', onGlobalKeydown)
})
onUnmounted(() => {
  document.removeEventListener('keydown', onGlobalKeydown)
})

defineExpose({ getData })
</script>

<style scoped>
.sheet-container { display: flex; flex-direction: column; height: 100%; background: #fff; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }

/* Formula bar */
.formula-bar { display: flex; align-items: center; border-bottom: 1px solid #d0d3d8; height: 32px; font-size: 13px; }
.cell-ref { width: 80px; text-align: center; border-right: 1px solid #d0d3d8; font-weight: 500; color: #333; height: 100%; display: flex; align-items: center; justify-content: center; background: #f8f9fa; }
.formula-divider { padding: 0 8px; color: #666; font-style: italic; border-right: 1px solid #d0d3d8; height: 100%; display: flex; align-items: center; background: #f8f9fa; }
.formula-input { flex: 1; border: none; outline: none; padding: 0 8px; height: 100%; font-size: 13px; }

/* Toolbar */
.sheet-toolbar { display: flex; align-items: center; gap: 4px; padding: 4px 8px; border-bottom: 1px solid #d0d3d8; background: #f8f9fa; flex-wrap: wrap; }
.sheet-info { margin-left: auto; color: #999; font-size: 12px; }

/* Table */
.sheet-scroll { flex: 1; overflow: auto; }
.sheet-table { border-collapse: collapse; table-layout: fixed; }
.sheet-table th, .sheet-table td { border: 1px solid #d0d3d8; height: 26px; font-size: 13px; position: relative; }

/* Headers */
.corner { background: #eef0f4; width: 40px; position: sticky; top: 0; left: 0; z-index: 5; }
.col-header { background: #eef0f4; font-weight: 500; color: #555; text-align: center; position: sticky; top: 0; z-index: 4; cursor: pointer; user-select: none; }
.col-header:hover { background: #dde0e6; }
.col-header.selected { background: #c8ddf0; color: #1a73e8; }
.col-header.sorted { color: #1a73e8; }
.col-menu-trigger { cursor: pointer; font-size: 10px; margin-left: 2px; opacity: 0.5; }
.col-menu-trigger:hover { opacity: 1; }
.sort-icon { font-size: 10px; color: #1a73e8; }
.filter-icon { font-size: 10px; color: #e6a23c; }
.col-resize-handle { position: absolute; right: -2px; top: 0; bottom: 0; width: 5px; cursor: col-resize; }
.row-header { background: #eef0f4; text-align: center; color: #555; font-weight: 500; position: sticky; left: 0; z-index: 2; cursor: pointer; user-select: none; }
.row-header:hover { background: #dde0e6; }
.row-header.selected { background: #c8ddf0; color: #1a73e8; }

/* Freeze */
.frozen-col-header { z-index: 6 !important; }
.frozen-corner { z-index: 7 !important; }
.frozen-row-header { z-index: 3 !important; }
.frozen-row-cell { position: sticky; top: 0; z-index: 3 !important; }
.frozen-col-cell { position: sticky; left: 0; z-index: 2 !important; background: #fff; }
.frozen-corner-cell { position: sticky; top: 0; left: 0; z-index: 7 !important; background: #eef0f4; }

/* Cells */
.cell { padding: 0; cursor: cell; overflow: hidden; }
.cell.selected { background: #e8f0fe; }
.cell.selected-head { outline: 2px solid #1a73e8; outline-offset: -1px; z-index: 1; }
.cell.editing { padding: 0; }
.cell-display { display: block; padding: 0 6px; line-height: 26px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cell-edit-input { width: 100%; height: 100%; border: none; outline: none; padding: 0 6px; font-size: 13px; font-family: inherit; background: #fff; }

/* Fill handle */
.fill-handle { position: absolute; right: -3px; bottom: -3px; width: 8px; height: 8px; background: #1a73e8; cursor: crosshair; z-index: 2; border-radius: 1px; }

/* Context menu */
.context-menu { position: fixed; background: #fff; border: 1px solid #d0d3d8; border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.12); z-index: 1000; min-width: 180px; padding: 4px 0; }
.menu-item { padding: 6px 16px; font-size: 13px; cursor: pointer; color: #333; }
.menu-item:hover { background: #f0f5ff; color: #1a73e8; }
.menu-divider { height: 1px; background: #e8e8e8; margin: 4px 0; }

/* Chart panel */
.chart-panel { border-top: 1px solid #d0d3d8; background: #fafafa; padding: 8px; }
.chart-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.chart-title-input { border: 1px solid #d0d3d8; border-radius: 4px; padding: 2px 8px; font-size: 13px; width: 120px; outline: none; }
.chart-title-input:focus { border-color: #409eff; }
.chart-body { display: flex; justify-content: center; }
.chart-body canvas { border: 1px solid #e8e8e8; border-radius: 4px; background: #fff; }

/* Filter panel */
.filter-panel { position: fixed; background: #fff; border: 1px solid #d0d3d8; border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.12); z-index: 1001; width: 200px; max-height: 300px; }
.filter-panel-header { padding: 8px 12px; border-bottom: 1px solid #e8e8e8; }
.filter-panel-list { max-height: 200px; overflow-y: auto; padding: 4px 12px; }
.filter-panel-item { padding: 2px 0; }
.filter-panel-footer { padding: 8px 12px; border-top: 1px solid #e8e8e8; display: flex; gap: 8px; }

/* Conditional format dialog */
.cond-rule-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.cond-new-rule { display: flex; align-items: center; gap: 8px; }
</style>
