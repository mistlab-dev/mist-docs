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
    const result = parseExpr(formula.substring(1))
    return String(result)
  } catch {
    return formula
  }
}

// ─── 递归公式解析器 ───
function parseExpr(expr: string): any {
  expr = expr.trim()
  // 字符串字面量
  if (expr.startsWith('"') && expr.endsWith('"')) return expr.slice(1, -1)
  // 数字
  if (/^-?\d+(\.\d+)?$/.test(expr)) return parseFloat(expr)
  // 布尔
  if (expr.toUpperCase() === 'TRUE') return true
  if (expr.toUpperCase() === 'FALSE') return false
  // 函数调用
  const fnMatch = expr.match(/^([A-Z]+)\((.+)\)$/is)
  if (fnMatch) return callFn(fnMatch[1].toUpperCase(), fnMatch[2])
  // 范围 A1:B3
  if (/^[A-Z]+\d+:[A-Z]+\d+$/.test(expr.toUpperCase())) return getRange(expr.toUpperCase())
  // 单元格引用 A1
  const cellM = expr.match(/^([A-Z]+)(\d+)$/i)
  if (cellM) return getCellVal(cellM[1].toUpperCase(), parseInt(cellM[2]))
  // 四则运算 + 比较
  if (/[+\-*/()><=!]/.test(expr)) {
    return safeCalc(expr)
  }
  return expr
}

// 拆分多参数（支持括号嵌套）
function splitArgs(s: string): string[] {
  const args: string[] = []
  let depth = 0, cur = ''
  for (let i = 0; i < s.length; i++) {
    const ch = s[i]
    if (ch === '(') depth++
    else if (ch === ')') depth--
    if (ch === ',' && depth === 0) { args.push(cur.trim()); cur = '' }
    else cur += ch
  }
  if (cur.trim()) args.push(cur.trim())
  return args
}

function callFn(name: string, rawArgs: string): any {
  const args = splitArgs(rawArgs)
  // 解析参数时，范围参数特殊处理
  const parsed = args.map(a => parseExpr(a))

  switch (name) {
    // ─── 聚合 ───
    case 'SUM': return numArray(args[0]).reduce((a, b) => a + b, 0)
    case 'AVG': case 'AVERAGE': { const arr = numArray(args[0]); return arr.length ? Math.round(arr.reduce((a, b) => a + b, 0) / arr.length * 100) / 100 : 0 }
    case 'COUNT': return numArray(args[0], true).length
    case 'MAX': { const arr = numArray(args[0]); return arr.length ? Math.max(...arr) : 0 }
    case 'MIN': { const arr = numArray(args[0]); return arr.length ? Math.min(...arr) : 0 }
    case 'COUNTA': return strArray(args[0]).filter(v => v !== '').length

    // ─── 条件聚合 ───
    case 'COUNTIF': { const vals = strArray(args[0]), cond = parsed[1]; return vals.filter(v => testCond(v, cond)).length }
    case 'SUMIF': { const vals = numArray(args[0], false, true), criteria = parsed[1]; return vals.filter((_, i) => testCond(strArray(args[0], true)[i], criteria)).reduce((a, b) => a + b, 0) }
    case 'AVERAGEIF': { const s = numArray(args[0]), c = parsed[1]; const f = s.filter((_, i) => testCond(strArray(args[0], true)[i], c)); return f.length ? Math.round(f.reduce((a, b) => a + b, 0) / f.length * 100) / 100 : 0 }

    // ─── 数学 ───
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
    case 'MEDIAN': { const arr = numArray(args[0]).sort((a, b) => a - b); const m = Math.floor(arr.length / 2); return arr.length % 2 ? arr[m] : (arr[m - 1] + arr[m]) / 2 }
    case 'STDEV': { const arr = numArray(args[0]); const avg = arr.reduce((a, b) => a + b, 0) / arr.length; return Math.sqrt(arr.reduce((s, v) => s + (v - avg) ** 2, 0) / (arr.length - 1)) }
    case 'VAR': { const arr = numArray(args[0]); const avg = arr.reduce((a, b) => a + b, 0) / arr.length; return arr.reduce((s, v) => s + (v - avg) ** 2, 0) / (arr.length - 1) }

    // ─── 逻辑 ───
    case 'IF': return parsed[0] ? parsed[1] : parsed[2]
    case 'AND': return parsed.every(v => !!v)
    case 'OR': return parsed.some(v => !!v)
    case 'NOT': return !parsed[0]
    case 'XOR': return parsed.filter(v => !!v).length === 1
    case 'IFS': { for (let i = 0; i < parsed.length - 1; i += 2) { if (parsed[i]) return parsed[i + 1] } return parsed.length % 2 ? parsed[parsed.length - 1] : '#N/A' }
    case 'SWITCH': { const val = parsed[0]; for (let i = 1; i < parsed.length - 1; i += 2) { if (val == parsed[i]) return parsed[i + 1] } return parsed.length % 2 === 0 ? parsed[parsed.length - 1] : '#N/A' }
    case 'ISBLANK': return parsed[0] === '' || parsed[0] === undefined || parsed[0] === null
    case 'ISNUMBER': return !isNaN(parsed[0]) && parsed[0] !== ''
    case 'ISTEXT': return typeof parsed[0] === 'string' && isNaN(parsed[0])

    // ─── 文本 ───
    case 'CONCAT': case 'CONCATENATE': return parsed.join('')
    case 'LEN': return String(parsed[0]).length
    case 'LEFT': return String(parsed[0]).substring(0, parsed[1] || 1)
    case 'RIGHT': return String(parsed[0]).slice(-(parsed[1] || 1))
    case 'MID': return String(parsed[0]).substring(parsed[1] - 1, parsed[1] - 1 + parsed[2])
    case 'UPPER': return String(parsed[0]).toUpperCase()
    case 'LOWER': return String(parsed[0]).toLowerCase()
    case 'TRIM': return String(parsed[0]).trim()
    case 'SUBSTITUTE': case 'REPLACE': return String(parsed[0]).replaceAll(String(parsed[1]), String(parsed[2]))
    case 'REPT': return String(parsed[0]).repeat(parsed[1])
    case 'FIND': return String(parsed[1]).indexOf(String(parsed[0])) + 1
    case 'TEXT': return String(parsed[0])
    case 'VALUE': return parseFloat(String(parsed[0])) || 0
    case 'EXACT': return String(parsed[0]) === String(parsed[1])
    case 'JOIN': return strArray(args[1]).join(String(parsed[0]))

    // ─── 日期 ───
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

    // ─── 查找 ───
    case 'VLOOKUP': { const key = String(parsed[0]); const vals = strArray(args[1], true); const col = parsed[2] - 1; const data = getRangeData(args[1]); const idx = vals.indexOf(key); return idx >= 0 && data[idx] ? getCellValByRC(data[idx].r, data[idx].c + col) : '#N/A' }
    case 'INDEX': { const arr = numArray(args[0], false, true); return arr[parsed[1] - 1] ?? '#N/A' }
    case 'MATCH': { const arr = strArray(args[0], true); const idx = arr.indexOf(String(parsed[1])); return idx >= 0 ? idx + 1 : '#N/A' }
    case 'CHOOSE': return parsed[parsed[0]] ?? '#N/A'

    default: return '#NAME?'
  }
}

// ─── 辅助 ───
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
  // comma-separated values
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
  if (cs.startsWith('>') || cs.startsWith('<') || cs.startsWith('!=') || cs.startsWith('=')) {
    const op = cs.match(/^(>=|<=|!=|>|<|=)/)![1]
    const target = cs.slice(op.length)
    const a = parseFloat(value), b = parseFloat(target)
    if (!isNaN(a) && !isNaN(b)) {
      if (op === '>') return a > b; if (op === '<') return a < b
      if (op === '>=') return a >= b; if (op === '<=') return a <= b
      if (op === '!=') return a !== b; if (op === '=') return a === b
    }
    if (op === '=') return value === target
    if (op === '!=') return value !== target
  }
  return value === cs
}

function safeCalc(expr: string): number {
  // Replace cell refs with values
  let safe = expr.replace(/([A-Z]+)(\d+)/gi, (_, col, row) => {
    const v = getCellVal(col.toUpperCase(), parseInt(row))
    return isNaN(v) ? '0' : String(v)
  })
  safe = safe.replace(/[^0-9+\-*/.() ><=!]/g, '')
  try { return Function('"use strict"; return (' + safe + ')')() } catch { return NaN }
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
