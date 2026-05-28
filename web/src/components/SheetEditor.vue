<template>
  <div class="sheet-container">
    <div class="sheet-toolbar">
      <el-button size="small" @click="addRow">+ 行</el-button>
      <el-button size="small" @click="addCol">+ 列</el-button>
      <el-button size="small" type="danger" @click="removeRow">- 行</el-button>
      <el-button size="small" type="danger" @click="removeCol">- 列</el-button>
      <span class="sheet-info">{{ rows.length }} 行 × {{ cols }} 列</span>
    </div>
    <div class="sheet-scroll">
      <table class="sheet-table">
        <thead>
          <tr>
            <th class="row-header"></th>
            <th v-for="c in cols" :key="c">{{ colName(c - 1) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, ri) in rows" :key="ri">
            <td class="row-header">{{ ri + 1 }}</td>
            <td v-for="c in cols" :key="c">
              <input
                v-model="rows[ri][c - 1]"
                @input="onChange"
                class="cell-input"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{ initialData?: string }>()
const emit = defineEmits<{ (e: 'change', data: string): void }>()

const cols = ref(10)
const rows = ref<string[][]>([])

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

function initRows(numRows: number, numCols: number) {
  const result: string[][] = []
  for (let r = 0; r < numRows; r++) {
    const row: string[] = []
    for (let c = 0; c < numCols; c++) {
      row.push('')
    }
    result.push(row)
  }
  return result
}

function addRow() {
  rows.value.push(new Array(cols.value).fill(''))
  onChange()
}

function addCol() {
  cols.value++
  rows.value.forEach(row => row.push(''))
  onChange()
}

function removeRow() {
  if (rows.value.length > 1) {
    rows.value.pop()
    onChange()
  }
}

function removeCol() {
  if (cols.value > 1) {
    cols.value--
    rows.value.forEach(row => row.pop())
    onChange()
  }
}

function onChange() {
  emit('change', JSON.stringify({ rows: rows.value, cols: cols.value }))
}

function getData(): string {
  return JSON.stringify({ rows: rows.value, cols: cols.value })
}

// 加载初始数据
if (props.initialData) {
  try {
    const parsed = JSON.parse(props.initialData)
    if (parsed.rows) {
      rows.value = parsed.rows
      cols.value = parsed.cols || parsed.rows[0]?.length || 10
    } else {
      rows.value = initRows(20, cols.value)
    }
  } catch {
    rows.value = initRows(20, cols.value)
  }
} else {
  rows.value = initRows(20, cols.value)
}

defineExpose({ getData })
</script>

<style scoped>
.sheet-container { display: flex; flex-direction: column; height: 100%; }
.sheet-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-bottom: 1px solid #e8e8e8;
  background: #fafafa;
}
.sheet-info { margin-left: auto; color: #999; font-size: 12px; }
.sheet-scroll { flex: 1; overflow: auto; }
.sheet-table {
  border-collapse: collapse;
  width: max-content;
  min-width: 100%;
}
.sheet-table th,
.sheet-table td {
  border: 1px solid #e0e0e0;
  min-width: 100px;
  height: 28px;
}
.sheet-table th {
  background: #f5f5f5;
  font-weight: normal;
  font-size: 12px;
  color: #666;
  text-align: center;
  position: sticky;
  top: 0;
  z-index: 1;
}
.row-header {
  background: #f5f5f5;
  font-size: 12px;
  color: #666;
  text-align: center;
  min-width: 40px !important;
  width: 40px;
  position: sticky;
  left: 0;
  z-index: 2;
}
.sheet-table thead th:first-child {
  z-index: 3;
}
.cell-input {
  width: 100%;
  height: 100%;
  border: none;
  outline: none;
  padding: 2px 6px;
  font-size: 13px;
  background: transparent;
}
.cell-input:focus {
  background: #fff;
  box-shadow: inset 0 0 0 2px #409eff;
}
</style>