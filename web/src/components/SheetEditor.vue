<template>
  <div class="sheet-wrapper">
    <!-- 公式栏 -->
    <div class="formula-bar">
      <div class="cell-ref">{{ activeCell }}</div>
      <input
        class="formula-input"
        :value="activeCellValue"
        @input="onFormulaInput($event)"
        @keydown.enter="commitFormula"
        @keydown.escape="cancelFormula"
        placeholder="输入内容..."
      />
    </div>
    <!-- 工具栏 -->
    <div class="sheet-toolbar">
      <el-button-group>
        <el-button size="small" @click="addRowAbove">↑ 插入行</el-button>
        <el-button size="small" @click="addRowBelow">↓ 插入行</el-button>
        <el-button size="small" type="danger" @click="deleteRow" :disabled="rows.length <= 1">删除行</el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="addColLeft">← 插入列</el-button>
        <el-button size="small" @click="addColRight">→ 插入列</el-button>
        <el-button size="small" type="danger" @click="deleteCol" :disabled="cols <= 1">删除列</el-button>
      </el-button-group>
      <div style="margin-left:auto;display:flex;gap:6px;align-items:center">
        <el-button size="small" @click="exportCSV">导出 CSV</el-button>
        <el-upload :auto-upload="false" :show-file-list="false" accept=".csv" :on-change="importCSV">
          <el-button size="small">导入 CSV</el-button>
        </el-upload>
        <span class="sheet-info">{{ rows.length }} 行 × {{ cols }} 列</span>
      </div>
    </div>
    <!-- 表格主体 -->
    <div class="sheet-body" ref="sheetBody" @keydown="onKeyDown" tabindex="0">
      <div class="sheet-scroll">
        <table class="sheet-table" @mousedown.prevent>
          <colgroup>
            <col style="width:48px" />
            <col v-for="c in cols" :key="c" style="width:100px" />
          </colgroup>
          <thead>
            <tr>
              <th class="corner"></th>
              <th
                v-for="c in cols"
                :key="c"
                class="col-header"
                :class="{ selected: isColSelected(c - 1) }"
                @click="selectCol(c - 1)"
                @contextmenu.prevent="headerContext($event, 'col', c - 1)"
              >{{ colName(c - 1) }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, ri) in rows" :key="ri">
              <td
                class="row-header"
                :class="{ selected: isRowSelected(ri) }"
                @click="selectRow(ri)"
              >{{ ri + 1 }}</td>
              <td
                v-for="c in cols"
                :key="c"
                class="cell"
                :class="{
                  selected: isActive(ri, c - 1),
                  inRange: isInRange(ri, c - 1),
                }"
                @click="selectCell(ri, c - 1, $event)"
                @dblclick="startEdit(ri, c - 1)"
              >
                <template v-if="editing && editing.r === ri && editing.c === c - 1">
                  <input
                    ref="editInput"
                    class="cell-edit"
                    v-model="editing.val"
                    @keydown.enter="commitEdit"
                    @keydown.tab.prevent="commitAndMove(0, 1)"
                    @keydown.escape="cancelEdit"
                    @blur="commitEdit"
                  />
                </template>
                <template v-else>
                  <div class="cell-display">{{ getCell(ri, c - 1) }}</div>
                </template>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <!-- 右键菜单 -->
    <div
      v-if="contextMenu.show"
      class="context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
    >
      <div class="ctx-item" @click="ctxInsertRowAbove">在上方插入行</div>
      <div class="ctx-item" @click="ctxInsertRowBelow">在下方插入行</div>
      <div class="ctx-item" @click="ctxInsertColLeft">在左侧插入列</div>
      <div class="ctx-item" @click="ctxInsertColRight">在右侧插入列</div>
      <div class="ctx-sep"></div>
      <div class="ctx-item danger" @click="ctxDeleteRow">删除行</div>
      <div class="ctx-item danger" @click="ctxDeleteCol">删除列</div>
      <div class="ctx-sep"></div>
      <div class="ctx-item" @click="ctxClearCells">清空内容</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, computed, onMounted, onUnmounted, watch } from 'vue'

const props = defineProps<{ initialData?: string }>()
const emit = defineEmits<{ (e: 'change', data: string): void }>()

const cols = ref(10)
const rows = ref<string[][]>([])
const sheetBody = ref<HTMLElement | null>(null)

// 选区
const active = ref<{ r: number; c: number }>({ r: 0, c: 0 })
const rangeEnd = ref<{ r: number; c: number } | null>(null)
const editing = ref<{ r: number; c: number; val: string } | null>(null)
const editInput = ref<HTMLInputElement[] | null>(null)

// 右键菜单
const contextMenu = ref<{ show: boolean; x: number; y: number; r: number; c: number }>({
  show: false, x: 0, y: 0, r: 0, c: 0
})

// 公式栏
const activeCell = computed(() => `${colName(active.value.c)}${active.value.r + 1}`)
const activeCellValue = computed({
  get: () => editing.value ? editing.value.val : getCell(active.value.r, active.value.c),
  set: (v: string) => { /* handled by onFormulaInput */ }
})

function onFormulaInput(e: Event) {
  const val = (e.target as HTMLInputElement).value
  if (!editing.value) {
    startEdit(active.value.r, active.value.c)
  }
  if (editing.value) {
    editing.value.val = val
  }
}
function commitFormula() {
  commitEdit()
}
function cancelFormula() {
  cancelEdit()
}

// Cell helpers
function colName(i: number): string {
  let name = ''
  i++
  while (i > 0) {
    i--
    name = String.fromCharCode(65 + (i % 26)) + name
    i = Math.floor(i / 26)
  }
  return name
}

function getCell(r: number, c: number): string {
  return rows.value[r]?.[c] || ''
}

function setCell(r: number, c: number, val: string) {
  if (rows.value[r]) rows.value[r][c] = val
  onChange()
}

function ensureSize(numRows: number, numCols: number) {
  while (cols.value < numCols) { cols.value++; rows.value.forEach(row => row.push('')) }
  while (rows.value.length < numRows) rows.value.push(new Array(cols.value).fill(''))
}

// Selection
function isActive(r: number, c: number) {
  return active.value.r === r && active.value.c === c
}

function isInRange(r: number, c: number) {
  if (!rangeEnd.value) return false
  const r1 = Math.min(active.value.r, rangeEnd.value.r)
  const r2 = Math.max(active.value.r, rangeEnd.value.r)
  const c1 = Math.min(active.value.c, rangeEnd.value.c)
  const c2 = Math.max(active.value.c, rangeEnd.value.c)
  return r >= r1 && r <= r2 && c >= c1 && c <= c2
}

function isRowSelected(r: number) { return active.value.r === r }
function isColSelected(c: number) { return active.value.c === c }

function selectCell(r: number, c: number, e?: MouseEvent) {
  if (e?.shiftKey) {
    rangeEnd.value = { r, c }
  } else {
    rangeEnd.value = null
  }
  if (editing.value) commitEdit()
  active.value = { r, c }
  nextTick(() => sheetBody.value?.focus())
}

function selectRow(r: number) {
  if (editing.value) commitEdit()
  active.value = { r, c: 0 }
  rangeEnd.value = { r, c: cols.value - 1 }
}

function selectCol(c: number) {
  if (editing.value) commitEdit()
  active.value = { r: 0, c }
  rangeEnd.value = { r: rows.value.length - 1, c }
}

// Editing
function startEdit(r: number, c: number) {
  if (editing.value) commitEdit()
  editing.value = { r, c, val: getCell(r, c) }
  nextTick(() => {
    const inputs = editInput.value
    if (inputs && inputs.length > 0) inputs[0]?.focus()
  })
}

function commitEdit() {
  if (!editing.value) return
  setCell(editing.value.r, editing.value.c, editing.value.val)
  editing.value = null
}

function cancelEdit() {
  editing.value = null
}

function commitAndMove(dr: number, dc: number) {
  commitEdit()
  const nr = Math.max(0, Math.min(rows.value.length - 1, active.value.r + dr))
  const nc = Math.max(0, Math.min(cols.value - 1, active.value.c + dc))
  active.value = { r: nr, c: nc }
  nextTick(() => sheetBody.value?.focus())
}

// Keyboard navigation
function onKeyDown(e: KeyboardEvent) {
  if (editing.value) return

  const { r, c } = active.value
  switch (e.key) {
    case 'ArrowUp': e.preventDefault(); active.value = { r: Math.max(0, r - 1), c }; break
    case 'ArrowDown': e.preventDefault(); active.value = { r: Math.min(rows.value.length - 1, r + 1), c }; break
    case 'ArrowLeft': e.preventDefault(); active.value = { r, c: Math.max(0, c - 1) }; break
    case 'ArrowRight': e.preventDefault(); active.value = { r, c: Math.min(cols.value - 1, c + 1) }; break
    case 'Tab': e.preventDefault(); commitAndMove(0, e.shiftKey ? -1 : 1); break
    case 'Enter': e.preventDefault(); startEdit(r, c); break
    case 'Delete':
    case 'Backspace':
      e.preventDefault()
      clearSelection()
      break
    default:
      if (e.key.length === 1 && !e.ctrlKey && !e.metaKey) {
        startEdit(r, c)
        editing.value!.val = e.key
        nextTick(() => {
          const inputs = editInput.value
          if (inputs && inputs.length > 0) inputs[0]?.focus()
        })
      }
  }
}

function clearSelection() {
  if (rangeEnd.value) {
    const r1 = Math.min(active.value.r, rangeEnd.value.r)
    const r2 = Math.max(active.value.r, rangeEnd.value.r)
    const c1 = Math.min(active.value.c, rangeEnd.value.c)
    const c2 = Math.max(active.value.c, rangeEnd.value.c)
    for (let r = r1; r <= r2; r++)
      for (let c = c1; c <= c2; c++)
        rows.value[r][c] = ''
  } else {
    rows.value[active.value.r][active.value.c] = ''
  }
  onChange()
}

// Row/Col operations
function addRowAbove() {
  const r = active.value.r
  rows.value.splice(r, 0, new Array(cols.value).fill(''))
  active.value = { r, c: active.value.c }
  onChange()
}
function addRowBelow() {
  const r = active.value.r + 1
  rows.value.splice(r, 0, new Array(cols.value).fill(''))
  active.value = { r, c: active.value.c }
  onChange()
}
function addColLeft() {
  const c = active.value.c
  cols.value++
  rows.value.forEach(row => row.splice(c, 0, ''))
  onChange()
}
function addColRight() {
  const c = active.value.c + 1
  cols.value++
  rows.value.forEach(row => row.splice(c, 0, ''))
  onChange()
}
function deleteRow() {
  if (rows.value.length <= 1) return
  rows.value.splice(active.value.r, 1)
  active.value = { r: Math.min(active.value.r, rows.value.length - 1), c: active.value.c }
  onChange()
}
function deleteCol() {
  if (cols.value <= 1) return
  cols.value--
  rows.value.forEach(row => row.splice(active.value.c, 1))
  active.value = { r: active.value.r, c: Math.min(active.value.c, cols.value - 1) }
  onChange()
}

// Right-click menu
function headerContext(e: MouseEvent, type: string, idx: number) {
  contextMenu.value = { show: true, x: e.clientX, y: e.clientY, r: active.value.r, c: idx }
}
function closeContext() { contextMenu.value.show = false }
function ctxInsertRowAbove() { closeContext(); addRowAbove() }
function ctxInsertRowBelow() { closeContext(); addRowBelow() }
function ctxInsertColLeft() { closeContext(); addColLeft() }
function ctxInsertColRight() { closeContext(); addColRight() }
function ctxDeleteRow() { closeContext(); deleteRow() }
function ctxDeleteCol() { closeContext(); deleteCol() }
function ctxClearCells() {
  closeContext()
  clearSelection()
}

// CSV import/export
function exportCSV() {
  const csvRows = rows.value.map(row =>
    row.map(cell => {
      const val = cell || ''
      return val.includes(',') || val.includes('"') || val.includes('\n')
        ? `"${val.replace(/"/g, '""')}"`
        : val
    }).join(',')
  )
  const blob = new Blob(['\uFEFF' + csvRows.join('\n')], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = 'export.csv'; a.click()
  URL.revokeObjectURL(url)
}

function importCSV(file: any) {
  const reader = new FileReader()
  reader.onload = (e) => {
    const text = e.target?.result as string
    const lines = text.split('\n').filter(l => l.trim())
    const parsed = lines.map(line => parseCSVLine(line))
    const maxCols = Math.max(...parsed.map(r => r.length), cols.value)
    ensureSize(parsed.length, maxCols)
    cols.value = maxCols
    rows.value = parsed.map(row => {
      while (row.length < maxCols) row.push('')
      return row
    })
    onChange()
  }
  reader.readAsText(file.raw)
}

function parseCSVLine(line: string): string[] {
  const cells: string[] = []
  let i = 0
  while (i < line.length) {
    if (line[i] === '"') {
      let val = ''
      i++
      while (i < line.length) {
        if (line[i] === '"' && line[i + 1] === '"') { val += '"'; i += 2 }
        else if (line[i] === '"') { i++; break }
        else { val += line[i]; i++ }
      }
      cells.push(val)
      if (line[i] === ',') i++
    } else {
      const next = line.indexOf(',', i)
      if (next === -1) { cells.push(line.slice(i)); break }
      cells.push(line.slice(i, next))
      i = next + 1
    }
  }
  return cells
}

// Data
function onChange() {
  emit('change', JSON.stringify({ rows: rows.value, cols: cols.value }))
}

function getData(): string {
  return JSON.stringify({ rows: rows.value, cols: cols.value })
}

// Init
function initData(data?: string) {
  if (data && data !== '{}') {
    try {
      const parsed = JSON.parse(data)
      if (parsed.rows && Array.isArray(parsed.rows)) {
        rows.value = parsed.rows
        cols.value = parsed.cols || parsed.rows[0]?.length || 10
        return
      }
    } catch { /* fallthrough */ }
  }
  // Default: 50 rows x 10 cols
  cols.value = 10
  rows.value = Array.from({ length: 50 }, () => new Array(cols.value).fill(''))
}

initData(props.initialData)

// Click outside to close context menu
function onDocClick(e: MouseEvent) {
  if (contextMenu.value.show) closeContext()
}
onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))

defineExpose({ getData })
</script>

<style scoped>
.sheet-wrapper {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #fff;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  font-size: 13px;
}

/* 公式栏 */
.formula-bar {
  display: flex;
  align-items: center;
  border-bottom: 1px solid #e0e0e0;
  background: #fafafa;
  height: 32px;
}
.cell-ref {
  width: 72px;
  text-align: center;
  font-weight: 600;
  color: #333;
  border-right: 1px solid #e0e0e0;
  font-size: 12px;
  line-height: 32px;
  flex-shrink: 0;
}
.formula-input {
  flex: 1;
  border: none;
  outline: none;
  padding: 0 10px;
  font-size: 13px;
  height: 100%;
  background: transparent;
}

/* 工具栏 */
.sheet-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  border-bottom: 1px solid #e0e0e0;
  background: #fff;
  flex-wrap: wrap;
}
.sheet-info {
  color: #999;
  font-size: 12px;
  white-space: nowrap;
}

/* 表格主体 */
.sheet-body {
  flex: 1;
  overflow: auto;
  outline: none;
  position: relative;
}
.sheet-scroll {
  min-width: 100%;
  display: inline-block;
}

.sheet-table {
  border-collapse: collapse;
  table-layout: fixed;
}
.sheet-table th,
.sheet-table td {
  border: 1px solid #d5d5d5;
  height: 26px;
  padding: 0;
  position: relative;
}

/* 表头 */
.col-header, .row-header, .corner {
  background: #f8f9fa;
  color: #5f6368;
  font-weight: 500;
  font-size: 11px;
  text-align: center;
  user-select: none;
  cursor: pointer;
}
.col-header:hover, .row-header:hover { background: #e8f0fe; }
.col-header.selected, .row-header.selected { background: #d3e3fd; }
.corner { width: 48px; }

/* 单元格 */
.cell {
  cursor: cell;
  padding: 0;
}
.cell-display {
  padding: 2px 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  height: 100%;
  line-height: 22px;
}
.cell.selected {
  outline: 2px solid #1a73e8;
  outline-offset: -1px;
  z-index: 1;
}
.cell.inRange {
  background: #e8f0fe;
}

/* 编辑状态 */
.cell-edit {
  width: 100%;
  height: 100%;
  border: none;
  outline: none;
  padding: 2px 6px;
  font-size: 13px;
  font-family: inherit;
  background: #fff;
  box-shadow: inset 0 0 0 2px #1a73e8;
}

/* 右键菜单 */
.context-menu {
  position: fixed;
  background: #fff;
  border: 1px solid #ddd;
  border-radius: 6px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.12);
  padding: 4px 0;
  z-index: 1000;
  min-width: 160px;
}
.ctx-item {
  padding: 6px 16px;
  cursor: pointer;
  font-size: 13px;
  color: #333;
}
.ctx-item:hover { background: #f0f0f0; }
.ctx-item.danger { color: #e53935; }
.ctx-item.danger:hover { background: #fce8e6; }
.ctx-sep { height: 1px; background: #e0e0e0; margin: 4px 0; }
</style>
