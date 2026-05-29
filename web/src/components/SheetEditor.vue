<template>
  <div class="sheet-container">
    <!-- 公式栏 -->
    <div class="formula-bar">
      <div class="cell-ref">{{ currentCellRef }}</div>
      <div class="formula-divider">fx</div>
      <input
        class="formula-input"
        v-model="formulaValue"
        @keydown.enter="applyFormula"
        @keydown.escape="cancelFormula"
        placeholder="输入内容..."
      />
    </div>

    <!-- 工具栏 -->
    <div class="sheet-toolbar">
      <el-button-group>
        <el-button size="small" @click="addRowAbove" title="上方插入行">
          <el-icon><Top /></el-icon>
        </el-button>
        <el-button size="small" @click="addRowBelow" title="下方插入行">
          <el-icon><Bottom /></el-icon>
        </el-button>
        <el-button size="small" @click="deleteRow" title="删除行">
          <el-icon><Delete /></el-icon>
        </el-button>
      </el-button-group>
      <el-button-group style="margin-left:8px">
        <el-button size="small" @click="addColLeft" title="左侧插入列">
          <el-icon><Back /></el-icon>
        </el-button>
        <el-button size="small" @click="addColRight" title="右侧插入列">
          <el-icon><Right /></el-icon>
        </el-button>
        <el-button size="small" @click="deleteCol" title="删除列">
          <el-icon><Delete /></el-icon>
        </el-button>
      </el-button-group>
      <span class="sheet-info">{{ rows.length }} 行 × {{ colCount }} 列</span>
    </div>

    <!-- 表格 -->
    <div class="sheet-scroll" ref="scrollRef" @contextmenu.prevent="showContextMenu">
      <table class="sheet-table" ref="tableRef">
        <colgroup>
          <col style="width: 40px" />
          <col v-for="(w, c) in colWidths" :key="c" :style="{ width: w + 'px' }" />
        </colgroup>
        <thead>
          <tr>
            <th class="corner"></th>
            <th
              v-for="c in colCount"
              :key="c"
              class="col-header"
              :class="{ selected: isColSelected(c - 1) }"
              @click="selectCol(c - 1)"
              @mousedown.stop="startColResize(c - 1, $event)"
            >
              {{ colName(c - 1) }}
              <div class="col-resize-handle" @mousedown.stop="startColResize(c - 1, $event)"></div>
            </th>
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
              v-for="c in colCount"
              :key="c"
              class="cell"
              :class="{
                selected: isSelected(ri, c - 1),
                'selected-head': isSelectionHead(ri, c - 1),
                editing: editingCell?.row === ri && editingCell?.col === c - 1
              }"
              @click="selectCell(ri, c - 1, $event)"
              @dblclick="startEdit(ri, c - 1)"
            >
              <template v-if="editingCell?.row === ri && editingCell?.col === c - 1">
                <input
                  ref="editInput"
                  class="cell-edit-input"
                  v-model="rows[ri][c - 1]"
                  @keydown.enter.prevent="finishEdit"
                  @keydown.tab.prevent="finishEdit; moveNext()"
                  @keydown.escape="cancelEdit"
                  @keydown="handleEditKey"
                />
              </template>
              <template v-else>
                <span class="cell-display">{{ getCellDisplay(ri, c - 1) }}</span>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 右键菜单 -->
    <div
      v-if="contextMenu.show"
      class="context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
    >
      <div class="menu-item" @click="ctxInsertRowAbove">上方插入行</div>
      <div class="menu-item" @click="ctxInsertRowBelow">下方插入行</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxInsertColLeft">左侧插入列</div>
      <div class="menu-item" @click="ctxInsertColRight">右侧插入列</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxDeleteRow">删除行</div>
      <div class="menu-item" @click="ctxDeleteCol">删除列</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxClearCells">清空单元格</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Top, Bottom, Back, Right, Delete } from '@element-plus/icons-vue'

const props = defineProps<{ initialData?: string }>()
const emit = defineEmits<{ (e: 'change', data: string): void }>()

const rows = ref<string[][]>([])
const colCount = ref(10)
const colWidths = ref<number[]>([])
const scrollRef = ref<HTMLElement | null>(null)
const tableRef = ref<HTMLElement | null>(null)
const editInput = ref<any[]>([])

// 选中状态
const selection = ref<{ startRow: number; startCol: number; endRow: number; endCol: number } | null>(null)
const editingCell = ref<{ row: number; col: number } | null>(null)

// 公式栏
const formulaValue = ref('')
const currentCellRef = computed(() => {
  if (!selection.value) return ''
  return `${colName(selection.value.startCol)}${selection.value.startRow + 1}`
})

// 右键菜单
const contextMenu = ref<{ show: boolean; x: number; y: number; row: number; col: number }>({
  show: false, x: 0, y: 0, row: -1, col: -1
})

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

function makeRow(cols: number): string[] {
  return new Array(cols).fill('')
}

function initEmpty(numRows = 50, numCols = 10) {
  colCount.value = numCols
  colWidths.value = Array(numCols).fill(120)
  rows.value = Array.from({ length: numRows }, () => makeRow(numCols))
}

// 加载数据
function loadData() {
  if (props.initialData && props.initialData !== '{}') {
    try {
      const parsed = JSON.parse(props.initialData)
      if (parsed.rows && parsed.rows.length) {
        rows.value = parsed.rows
        colCount.value = parsed.cols || parsed.rows[0]?.length || 10
        colWidths.value = parsed.colWidths || Array(colCount.value).fill(120)
        return
      }
    } catch { /* ignore */ }
  }
  initEmpty()
}

// 选中
function selectCell(row: number, col: number, e?: MouseEvent) {
  if (e?.shiftKey && selection.value) {
    selection.value = {
      ...selection.value,
      endRow: row,
      endCol: col,
    }
  } else {
    selection.value = { startRow: row, startCol: col, endRow: row, endCol: col }
  }
  editingCell.value = null
  updateFormula()
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
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  return row >= r1 && row <= r2 && col >= c1 && col <= c2
}

function isSelectionHead(row: number, col: number): boolean {
  if (!selection.value) return false
  return selection.value.startRow === row && selection.value.startCol === col
}

function isRowSelected(row: number): boolean {
  if (!selection.value) return false
  const r1 = Math.min(selection.value.startRow, selection.value.endRow)
  const r2 = Math.max(selection.value.startRow, selection.value.endRow)
  return row >= r1 && row <= r2
}

function isColSelected(col: number): boolean {
  if (!selection.value) return false
  const c1 = Math.min(selection.value.startCol, selection.value.endCol)
  const c2 = Math.max(selection.value.startCol, selection.value.endCol)
  return col >= c1 && col <= c2
}

// 编辑
function startEdit(row: number, col: number) {
  editingCell.value = { row, col }
  formulaValue.value = rows.value[row]?.[col] || ''
  nextTick(() => {
    const inputs = editInput.value
    if (inputs && inputs.length) {
      const el = inputs[0]?.$el || inputs[0]
      el?.focus()
    }
  })
}

function finishEdit() {
  if (!editingCell.value) return
  const { row, col } = editingCell.value
  rows.value[row][col] = formulaValue.value
  editingCell.value = null
  emitChange()
}

function cancelEdit() {
  editingCell.value = null
  updateFormula()
}

function moveNext() {
  if (!selection.value) return
  const { startRow, startCol } = selection.value
  const nextCol = startCol + 1
  if (nextCol < colCount.value) {
    selectCell(startRow, nextCol)
  } else if (startRow + 1 < rows.value.length) {
    selectCell(startRow + 1, 0)
  }
}

function updateFormula() {
  if (!selection.value) { formulaValue.value = ''; return }
  const { startRow, startCol } = selection.value
  formulaValue.value = rows.value[startRow]?.[startCol] || ''
}

function applyFormula() {
  if (!selection.value) return
  const { startRow, startCol } = selection.value
  rows.value[startRow][startCol] = formulaValue.value
  editingCell.value = null
  emitChange()
}

function cancelFormula() {
  updateFormula()
}

function getCellDisplay(row: number, col: number): string {
  const val = rows.value[row]?.[col] || ''
  // 简单公式求值
  if (val.startsWith('=')) {
    return evaluateFormula(val)
  }
  return val
}

function evaluateFormula(formula: string): string {
  try {
    const expr = formula.substring(1).toUpperCase()
    // SUM(A1:B3)
    const sumMatch = expr.match(/^SUM\(([A-Z]+)(\d+):([A-Z]+)(\d+)\)$/)
    if (sumMatch) {
      const c1 = colIndex(sumMatch[1]), r1 = parseInt(sumMatch[2]) - 1
      const c2 = colIndex(sumMatch[3]), r2 = parseInt(sumMatch[4]) - 1
      let sum = 0
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          sum += parseFloat(rows.value[r]?.[c]) || 0
        }
      }
      return String(sum)
    }
    // AVG
    const avgMatch = expr.match(/^AVG\(([A-Z]+)(\d+):([A-Z]+)(\d+)\)$/)
    if (avgMatch) {
      const c1 = colIndex(avgMatch[1]), r1 = parseInt(avgMatch[2]) - 1
      const c2 = colIndex(avgMatch[3]), r2 = parseInt(avgMatch[4]) - 1
      let sum = 0, count = 0
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          const v = parseFloat(rows.value[r]?.[c])
          if (!isNaN(v)) { sum += v; count++ }
        }
      }
      return count ? String(Math.round(sum / count * 100) / 100) : '0'
    }
    // COUNT
    const countMatch = expr.match(/^COUNT\(([A-Z]+)(\d+):([A-Z]+)(\d+)\)$/)
    if (countMatch) {
      const c1 = colIndex(countMatch[1]), r1 = parseInt(countMatch[2]) - 1
      const c2 = colIndex(countMatch[3]), r2 = parseInt(countMatch[4]) - 1
      let count = 0
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          const v = rows.value[r]?.[c]
          if (v && v.trim()) count++
        }
      }
      return String(count)
    }
    // MAX
    const maxMatch = expr.match(/^MAX\(([A-Z]+)(\d+):([A-Z]+)(\d+)\)$/)
    if (maxMatch) {
      const c1 = colIndex(maxMatch[1]), r1 = parseInt(maxMatch[2]) - 1
      const c2 = colIndex(maxMatch[3]), r2 = parseInt(maxMatch[4]) - 1
      let max = -Infinity
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          const v = parseFloat(rows.value[r]?.[c])
          if (!isNaN(v) && v > max) max = v
        }
      }
      return max === -Infinity ? '0' : String(max)
    }
    // MIN
    const minMatch = expr.match(/^MIN\(([A-Z]+)(\d+):([A-Z]+)(\d+)\)$/)
    if (minMatch) {
      const c1 = colIndex(minMatch[1]), r1 = parseInt(minMatch[2]) - 1
      const c2 = colIndex(minMatch[3]), r2 = parseInt(minMatch[4]) - 1
      let min = Infinity
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          const v = parseFloat(rows.value[r]?.[c])
          if (!isNaN(v) && v < min) min = v
        }
      }
      return min === Infinity ? '0' : String(min)
    }
    // IF(condition, true_val, false_val) - supports simple comparisons
    const ifMatch = expr.match(/^IF\(([^,]+),([^,]+),([^)]+)\)$/)
    if (ifMatch) {
      const cond = resolveValue(ifMatch[1].trim())
      const trueVal = resolveValue(ifMatch[2].trim())
      const falseVal = resolveValue(ifMatch[3].trim())
      // Evaluate condition
      const condStr = String(cond)
      let result = false
      if (condStr.includes('>=')) { const [a, b] = condStr.split('>='); result = parseFloat(a) >= parseFloat(b) }
      else if (condStr.includes('<=')) { const [a, b] = condStr.split('<='); result = parseFloat(a) <= parseFloat(b) }
      else if (condStr.includes('!=')) { const [a, b] = condStr.split('!='); result = a !== b }
      else if (condStr.includes('>')) { const [a, b] = condStr.split('>'); result = parseFloat(a) > parseFloat(b) }
      else if (condStr.includes('<')) { const [a, b] = condStr.split('<'); result = parseFloat(a) < parseFloat(b) }
      else if (condStr.includes('=')) { const [a, b] = condStr.split('='); result = a === b }
      else { result = !!cond && cond !== '0' && cond !== '' }
      return result ? trueVal : falseVal
    }
    // 简单四则运算
    const calcMatch = expr.match(/^([\d+\-*/(). ]+)$/)
    if (calcMatch) {
      // 安全计算：只允许数字和运算符
      const safe = expr.replace(/[^0-9+\-*/().]/g, '')
      if (safe === expr) {
        const result = Function('"use strict"; return (' + expr + ')')()
        return String(result)
      }
    }
    // 单元格引用 A1
    const cellMatch = expr.match(/^([A-Z]+)(\d+)$/)
    if (cellMatch) {
      const c = colIndex(cellMatch[1]), r = parseInt(cellMatch[2]) - 1
      return rows.value[r]?.[c] || ''
    }
    return formula
  } catch {
    return formula
  }
}

function resolveValue(expr: string): string {
  // Cell reference like A1
  const cellMatch = expr.match(/^([A-Z]+)(\d+)$/)
  if (cellMatch) {
    const c = colIndex(cellMatch[1]), r = parseInt(cellMatch[2]) - 1
    return rows.value[r]?.[c] || ''
  }
  return expr
}

function colIndex(name: string): number {
  let idx = 0
  for (let i = 0; i < name.length; i++) {
    idx = idx * 26 + (name.charCodeAt(i) - 64)
  }
  return idx - 1
}

// 行列操作
function insertRowAt(index: number) {
  rows.value.splice(index, 0, makeRow(colCount.value))
  adjustSelection()
  emitChange()
}

function deleteRowAt(index: number) {
  if (rows.value.length <= 1) return
  rows.value.splice(index, 1)
  adjustSelection()
  emitChange()
}

function insertColAt(index: number) {
  colCount.value++
  colWidths.value.splice(index, 0, 120)
  rows.value.forEach(row => row.splice(index, 0, ''))
  adjustSelection()
  emitChange()
}

function deleteColAt(index: number) {
  if (colCount.value <= 1) return
  colCount.value--
  colWidths.value.splice(index, 1)
  rows.value.forEach(row => row.splice(index, 1))
  adjustSelection()
  emitChange()
}

function adjustSelection() {
  if (!selection.value) return
  selection.value.endRow = Math.min(selection.value.endRow, rows.value.length - 1)
  selection.value.endCol = Math.min(selection.value.endCol, colCount.value - 1)
}

// 工具栏按钮
function addRowAbove() {
  const row = selection.value?.startRow ?? 0
  insertRowAt(row)
}

function addRowBelow() {
  const row = selection.value?.startRow ?? rows.value.length - 1
  insertRowAt(row + 1)
}

function addColLeft() {
  const col = selection.value?.startCol ?? 0
  insertColAt(col)
}

function addColRight() {
  const col = selection.value?.startCol ?? colCount.value - 1
  insertColAt(col + 1)
}

function deleteRow() {
  const row = selection.value?.startRow ?? 0
  deleteRowAt(row)
}

function deleteCol() {
  const col = selection.value?.startCol ?? 0
  deleteColAt(col)
}

// 右键菜单
function showContextMenu(e: MouseEvent) {
  contextMenu.value = {
    show: true,
    x: e.clientX,
    y: e.clientY,
    row: selection.value?.startRow ?? 0,
    col: selection.value?.startCol ?? 0,
  }
}

function hideContextMenu() {
  contextMenu.value.show = false
}

function ctxInsertRowAbove() { hideContextMenu(); addRowAbove() }
function ctxInsertRowBelow() { hideContextMenu(); addRowBelow() }
function ctxInsertColLeft() { hideContextMenu(); addColLeft() }
function ctxInsertColRight() { hideContextMenu(); addColRight() }
function ctxDeleteRow() { hideContextMenu(); deleteRow() }
function ctxDeleteCol() { hideContextMenu(); deleteCol() }
function ctxClearCells() {
  hideContextMenu()
  if (!selection.value) return
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  for (let r = r1; r <= r2; r++) {
    for (let c = c1; c <= c2; c++) {
      rows.value[r][c] = ''
    }
  }
  emitChange()
}

// 键盘
function handleEditKey(e: KeyboardEvent) {
  if (e.key === 'ArrowUp') { e.preventDefault(); finishEdit(); moveUp() }
  if (e.key === 'ArrowDown') { e.preventDefault(); finishEdit(); moveDown() }
}

function moveUp() {
  if (!selection.value || selection.value.startRow <= 0) return
  selectCell(selection.value.startRow - 1, selection.value.startCol)
  updateFormula()
}

function moveDown() {
  if (!selection.value) return
  if (selection.value.startRow >= rows.value.length - 1) {
    // 自动追加行
    rows.value.push(makeRow(colCount.value))
  }
  selectCell(selection.value.startRow + 1, selection.value.startCol)
  updateFormula()
}

// 列宽拖拽
function startColResize(col: number, e: MouseEvent) {
  e.preventDefault()
  const startX = e.clientX
  const startWidth = colWidths.value[col] || 120
  const minWidth = 40

  function onMove(ev: MouseEvent) {
    const delta = ev.clientX - startX
    colWidths.value[col] = Math.max(minWidth, startWidth + delta)
  }

  function onUp() {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
    emitChange()
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

function emitChange() {
  emit('change', JSON.stringify({ rows: rows.value, cols: colCount.value, colWidths: colWidths.value }))
}

function getData(): string {
  return JSON.stringify({ rows: rows.value, cols: colCount.value, colWidths: colWidths.value })
}

// 全局键盘
function onGlobalKeydown(e: KeyboardEvent) {
  if (editingCell.value) return
  if (!selection.value) return

  if (e.key === 'Delete' || e.key === 'Backspace') {
    const { startRow, startCol, endRow, endCol } = selection.value
    const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
    const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
    for (let r = r1; r <= r2; r++) {
      for (let c = c1; c <= c2; c++) {
        rows.value[r][c] = ''
      }
    }
    emitChange()
    e.preventDefault()
  } else if (e.key === 'Enter') {
    startEdit(selection.value.startRow, selection.value.startCol)
    e.preventDefault()
  } else if (e.key === 'Tab') {
    moveNext()
    e.preventDefault()
  } else if (e.key === 'ArrowUp') { moveUp() }
  else if (e.key === 'ArrowDown') { moveDown() }
  else if (e.key === 'ArrowLeft') {
    if (selection.value.startCol > 0) {
      selectCell(selection.value.startRow, selection.value.startCol - 1)
      updateFormula()
    }
  } else if (e.key === 'ArrowRight') {
    if (selection.value.startCol < colCount.value - 1) {
      selectCell(selection.value.startRow, selection.value.startCol + 1)
      updateFormula()
    }
  } else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey) {
    // 直接输入
    formulaValue.value = e.key
    startEdit(selection.value.startRow, selection.value.startCol)
  }
}

// 点击外部关闭右键菜单
function onGlobalClick() {
  hideContextMenu()
}

loadData()

onMounted(() => {
  document.addEventListener('keydown', onGlobalKeydown)
  document.addEventListener('click', onGlobalClick)
})

onUnmounted(() => {
  document.removeEventListener('keydown', onGlobalKeydown)
  document.removeEventListener('click', onGlobalClick)
})

defineExpose({ getData })
</script>

<style scoped>
.sheet-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #fff;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

/* 公式栏 */
.formula-bar {
  display: flex;
  align-items: center;
  border-bottom: 1px solid #d0d3d8;
  height: 32px;
  font-size: 13px;
}
.cell-ref {
  width: 80px;
  text-align: center;
  border-right: 1px solid #d0d3d8;
  font-weight: 500;
  color: #333;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f8f9fa;
}
.formula-divider {
  padding: 0 8px;
  color: #666;
  font-style: italic;
  border-right: 1px solid #d0d3d8;
  height: 100%;
  display: flex;
  align-items: center;
  background: #f8f9fa;
}
.formula-input {
  flex: 1;
  border: none;
  outline: none;
  padding: 0 8px;
  height: 100%;
  font-size: 13px;
}

/* 工具栏 */
.sheet-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  border-bottom: 1px solid #d0d3d8;
  background: #f8f9fa;
}
.sheet-info {
  margin-left: auto;
  color: #999;
  font-size: 12px;
}

/* 表格 */
.sheet-scroll {
  flex: 1;
  overflow: auto;
}
.sheet-table {
  border-collapse: collapse;
  table-layout: fixed;
}
.sheet-table th,
.sheet-table td {
  border: 1px solid #d0d3d8;
  height: 26px;
  font-size: 13px;
  position: relative;
}

/* 行列头 */
.corner {
  background: #eef0f4;
  width: 40px;
  position: sticky;
  top: 0;
  left: 0;
  z-index: 3;
}
.col-header {
  background: #eef0f4;
  font-weight: 500;
  color: #555;
  text-align: center;
  position: sticky;
  top: 0;
  z-index: 2;
  cursor: pointer;
  user-select: none;
}
.col-header:hover { background: #dde0e6; }
.col-header.selected { background: #c8ddf0; color: #1a73e8; }
.col-resize-handle {
  position: absolute;
  right: -2px;
  top: 0;
  bottom: 0;
  width: 5px;
  cursor: col-resize;
}
.row-header {
  background: #eef0f4;
  text-align: center;
  color: #555;
  font-weight: 500;
  position: sticky;
  left: 0;
  z-index: 1;
  cursor: pointer;
  user-select: none;
}
.row-header:hover { background: #dde0e6; }
.row-header.selected { background: #c8ddf0; color: #1a73e8; }

/* 单元格 */
.cell {
  padding: 0;
  cursor: cell;
  overflow: hidden;
}
.cell.selected {
  background: #e8f0fe;
}
.cell.selected-head {
  outline: 2px solid #1a73e8;
  outline-offset: -1px;
  z-index: 1;
}
.cell.editing {
  padding: 0;
}
.cell-display {
  display: block;
  padding: 0 6px;
  line-height: 26px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cell-edit-input {
  width: 100%;
  height: 100%;
  border: none;
  outline: none;
  padding: 0 6px;
  font-size: 13px;
  font-family: inherit;
  background: #fff;
}

/* 右键菜单 */
.context-menu {
  position: fixed;
  background: #fff;
  border: 1px solid #d0d3d8;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.12);
  z-index: 1000;
  min-width: 160px;
  padding: 4px 0;
}
.menu-item {
  padding: 6px 16px;
  font-size: 13px;
  cursor: pointer;
  color: #333;
}
.menu-item:hover {
  background: #f0f5ff;
  color: #1a73e8;
}
.menu-divider {
  height: 1px;
  background: #e8e8e8;
  margin: 4px 0;
}
</style>
