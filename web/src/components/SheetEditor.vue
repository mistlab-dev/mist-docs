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

      <!-- 字体字号 -->
      <el-select size="small" v-model="cellFontFamily" @change="applyFontFamily" style="width:110px" title="字体">
        <el-option v-for="f in fontList" :key="f" :label="f" :value="f" :style="{fontFamily:f}" />
      </el-select>
      <el-select size="small" v-model="cellFontSize" @change="applyFontSize" style="width:70px" title="字号">
        <el-option v-for="s in fontSizes" :key="s" :label="s + 'px'" :value="s" />
      </el-select>

      <el-divider direction="vertical" />

      <!-- 格式 -->
      <el-button-group>
        <el-button size="small" :type="getMetaProp('bold') ? 'primary' : ''" @click="toggleFormat('bold')" title="加粗"><b>B</b></el-button>
        <el-button size="small" :type="getMetaProp('italic') ? 'primary' : ''" @click="toggleFormat('italic')" title="斜体"><i>I</i></el-button>
        <el-button size="small" :type="getMetaProp('underline') ? 'primary' : ''" @click="toggleFormat('underline')" title="下划线"><u>U</u></el-button>
        <el-button size="small" :type="getMetaProp('strike') ? 'primary' : ''" @click="toggleFormat('strike')" title="删除线"><s>S</s></el-button>
      </el-button-group>

      <el-divider direction="vertical" />

      <!-- 颜色 -->
      <el-color-picker size="small" v-model="cellTextColor" @change="applyTextColor" title="文字颜色" />
      <el-color-picker size="small" v-model="cellBgColor" @change="applyBgColor" title="背景颜色" />

      <el-divider direction="vertical" />

      <!-- 边框 -->
      <el-dropdown trigger="click" @command="applyBorder" title="边框">
        <el-button size="small"><el-icon><Grid /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="all">全部边框</el-dropdown-item>
            <el-dropdown-item command="outer">外边框</el-dropdown-item>
            <el-dropdown-item command="none">无边框</el-dropdown-item>
            <el-dropdown-item command="top" divided>上边框</el-dropdown-item>
            <el-dropdown-item command="bottom">下边框</el-dropdown-item>
            <el-dropdown-item command="left">左边框</el-dropdown-item>
            <el-dropdown-item command="right">右边框</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <el-divider direction="vertical" />

      <!-- 对齐 -->
      <el-button-group>
        <el-button size="small" :type="getMetaProp('align')==='left'?'primary':''" @click="setAlign('left')" title="左对齐">☰</el-button>
        <el-button size="small" :type="getMetaProp('align')==='center'?'primary':''" @click="setAlign('center')" title="居中">☱</el-button>
        <el-button size="small" :type="getMetaProp('align')==='right'?'primary':''" @click="setAlign('right')" title="右对齐">☷</el-button>
      </el-button-group>
      <el-button size="small" :type="getMetaProp('wrap')?'primary':''" @click="toggleWrap" title="自动换行">
        <el-icon><DocumentCopy /></el-icon>
      </el-button>

      <el-divider direction="vertical" />

      <!-- 数字精度 -->
      <el-button-group>
        <el-button size="small" @click="changePrecision(-1)" title="减少小数位">.0→0</el-button>
        <el-button size="small" @click="changePrecision(1)" title="增加小数位">0→.0</el-button>
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
        <el-button size="small" title="冻结"><el-icon><Lock /></el-icon></el-button>
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

      <!-- 查找替换 -->
      <el-button size="small" @click="showSearchDialog = !showSearchDialog" title="查找替换 Ctrl+F">
        <el-icon><Search /></el-icon>
      </el-button>

      <span class="sheet-info">{{ currentSheetRows.length }} 行 × {{ colCount }} 列</span>
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
              @dblclick="autoFitCol(c - 1)"
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
          <template v-for="(row, ri) in currentSheetRows" :key="ri">
            <tr v-show="!isRowFiltered(ri)">
              <td class="row-header"
                :class="{
                  selected: isRowSelected(ri),
                  'frozen-row-header': freezeCols > 0,
                }"
                :style="{ height: (rowHeights[ri] || 26) + 'px' }"
                @click="selectRow(ri)"
              >
                {{ ri + 1 }}
                <div class="row-resize-handle" @mousedown.stop="startRowResize(ri, $event)"></div>
              </td>
              <td v-for="c in colCount" :key="c"
                class="cell"
                :class="{
                  selected: isSelected(ri, c - 1),
                  'selected-head': isSelectionHead(ri, c - 1),
                  editing: editingCell?.row === ri && editingCell?.col === c - 1,
                  'has-comment': getComment(ri, c - 1),
                  'has-validation': getValidation(ri, c - 1),
                }"
                :style="getCellStyle(ri, c - 1)"
                :colspan="getColspan(ri, c - 1)"
                :rowspan="getRowspan(ri, c - 1)"
                v-show="!isCellHidden(ri, c - 1)"
                @click="selectCell(ri, c - 1, $event)"
                @dblclick="startEdit(ri, c - 1)"
                @mouseenter="showCellComment(ri, c - 1, $event)"
                @mouseleave="hideCellComment()"
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
                <!-- 拖拽移动柄 -->
                <div v-if="isSelectionHead(ri, c - 1) && !editingCell && selection && isMultiCellSelection"
                  class="move-handle" @mousedown.stop="startMove($event)"></div>
                <!-- 批注标记 -->
                <div v-if="getComment(ri, c - 1)" class="comment-marker"></div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- 悬浮批注 -->
    <div v-if="hoverComment.show" class="hover-comment"
      :style="{ left: hoverComment.x + 'px', top: hoverComment.y + 'px' }">
      {{ hoverComment.text }}
    </div>

    <!-- 右键菜单 -->
    <div v-if="contextMenu.show" class="context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }">
      <div class="menu-item" @click="ctxCut">剪切 Ctrl+X</div>
      <div class="menu-item" @click="ctxCopy">复制 Ctrl+C</div>
      <div class="menu-item" @click="ctxPaste">粘贴 Ctrl+V</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxInsertRowAbove">上方插入行</div>
      <div class="menu-item" @click="ctxInsertRowBelow">下方插入行</div>
      <div class="menu-item" @click="ctxInsertColLeft">左侧插入列</div>
      <div class="menu-item" @click="ctxInsertColRight">右侧插入列</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxDeleteRow">删除行</div>
      <div class="menu-item" @click="ctxDeleteCol">删除列</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxClearCells">清空单元格</div>
      <div class="menu-item" @click="ctxMergeToggle">{{ hasMerge ? '取消合并' : '合并单元格' }}</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxAddComment">{{ getComment(ctxRow, ctxCol) ? '编辑批注' : '添加批注' }}</div>
      <div v-if="getComment(ctxRow, ctxCol)" class="menu-item" @click="ctxDeleteComment">删除批注</div>
      <div class="menu-divider"></div>
      <div class="menu-item" @click="ctxSetValidation">数据验证/下拉列表</div>
      <div class="menu-item" @click="ctxGroupRows">分组折叠行</div>
    </div>

    <!-- 图表面板 -->
    <div v-if="showChart" class="chart-panel">
      <div class="chart-header">
        <el-select v-model="chartType" size="small" style="width:100px">
          <el-option label="柱状图" value="bar" />
          <el-option label="折线图" value="line" />
          <el-option label="饼图" value="pie" />
          <el-option label="散点图" value="scatter" />
          <el-option label="面积图" value="area" />
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
        <canvas ref="chartCanvas" width="700" height="380"
          @mousemove="onChartHover" @mouseleave="chartTooltip.show = false" />
        <div v-if="chartTooltip.show" class="chart-tooltip"
          :style="{ left: chartTooltip.x + 'px', top: chartTooltip.y + 'px' }">
          {{ chartTooltip.text }}
        </div>
      </div>
    </div>

    <!-- 条件格式对话框 -->
    <el-dialog v-model="showCondDialog" title="条件格式" width="500px">
      <el-tabs>
        <el-tab-pane label="规则">
          <div v-for="(rule, i) in condRules" :key="i" class="cond-rule-item">
            <span>当 {{ rule.condition }} {{ rule.value }} 时</span>
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
        </el-tab-pane>
        <el-tab-pane label="色阶">
          <div class="cond-scale-row">
            <span>2色渐变：</span>
            <el-color-picker v-model="condScale2Min" size="small" />
            <span>→</span>
            <el-color-picker v-model="condScale2Max" size="small" />
            <el-button size="small" type="primary" @click="applyCondScale2">应用到选区</el-button>
          </div>
          <div class="cond-scale-row" style="margin-top:12px">
            <span>3色渐变：</span>
            <el-color-picker v-model="condScale3Min" size="small" />
            <span>→</span>
            <el-color-picker v-model="condScale3Mid" size="small" />
            <span>→</span>
            <el-color-picker v-model="condScale3Max" size="small" />
            <el-button size="small" type="primary" @click="applyCondScale">应用到选区</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="数据条">
          <div class="cond-scale-row">
            <span>颜色：</span>
            <el-color-picker v-model="condDataBarColor" size="small" />
            <el-button size="small" type="primary" @click="applyDataBar">应用到选区</el-button>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- 查找替换对话框 -->
    <el-dialog v-model="showSearchDialog" title="查找和替换" width="420px" :close-on-click-modal="false">
      <el-input v-model="searchText" placeholder="查找内容" size="small" style="margin-bottom:8px" @keydown.enter="findNext">
        <template #append><el-button @click="findNext" size="small">查找下一个</el-button></template>
      </el-input>
      <el-input v-model="replaceText" placeholder="替换为" size="small" style="margin-bottom:8px">
        <template #append>
          <el-button @click="replaceOne" size="small">替换</el-button>
          <el-button @click="replaceAll" size="small">全部替换</el-button>
        </template>
      </el-input>
      <div v-if="searchResult" style="color:#999;font-size:12px">{{ searchResult }}</div>
    </el-dialog>

    <!-- 批注编辑对话框 -->
    <el-dialog v-model="showCommentDialog" title="编辑批注" width="360px">
      <el-input v-model="commentText" type="textarea" :rows="4" placeholder="输入批注内容..." />
      <template #footer>
        <el-button size="small" @click="showCommentDialog = false">取消</el-button>
        <el-button size="small" type="primary" @click="saveComment">保存</el-button>
      </template>
    </el-dialog>

    <!-- 数据验证对话框 -->
    <el-dialog v-model="showValidationDialog" title="数据验证" width="400px">
      <el-select v-model="validationType" size="small" style="width:100%;margin-bottom:8px">
        <el-option label="无验证" value="none" />
        <el-option label="下拉列表" value="list" />
        <el-option label="数字范围" value="number" />
      </el-select>
      <div v-if="validationType === 'list'">
        <el-input v-model="validationOptions" size="small" placeholder="选项，用逗号分隔 (如: 是,否,待定)" />
      </div>
      <div v-if="validationType === 'number'" style="display:flex;gap:8px">
        <el-input-number v-model="validationMin" size="small" placeholder="最小值" />
        <el-input-number v-model="validationMax" size="small" placeholder="最大值" />
      </div>
      <template #footer>
        <el-button size="small" @click="showValidationDialog = false">取消</el-button>
        <el-button size="small" type="primary" @click="saveValidation">保存</el-button>
      </template>
    </el-dialog>

    <!-- 筛选面板 -->
    <div v-if="showFilterPanel" class="filter-panel"
      :style="{ left: '200px', top: '200px' }">
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

    <!-- 多Sheet标签栏 -->
    <div class="sheet-tabs-bar">
      <div class="sheet-tabs-scroll">
        <div v-for="(sh, si) in sheets" :key="si"
          class="sheet-tab"
          :class="{ active: activeSheet === si }"
          @click="switchSheet(si)"
          @dblclick="renameSheet(si)"
          @contextmenu.prevent="renameSheet(si)"
        >
          {{ sh.name }}
          <span v-if="sheets.length > 1" class="tab-close" @click.stop="deleteSheet(si)">✕</span>
        </div>
      </div>
      <el-button size="small" text @click="addSheet" title="新增Sheet">+</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { RefreshLeft, RefreshRight, Delete, Top, Bottom, Back, Right, Lock, MagicStick, TrendCharts, Search, Grid, DocumentCopy } from '@element-plus/icons-vue'

const props = defineProps<{ initialData?: string }>()
const emit = defineEmits<{ (e: 'change', data: string): void }>()

interface CellMeta {
  bold?: boolean; italic?: boolean; underline?: boolean; strike?: boolean
  color?: string; bgColor?: string; align?: string; wrap?: boolean
  fontFamily?: string; fontSize?: number; precision?: number
  border?: { top?: boolean; right?: boolean; bottom?: boolean; left?: boolean }
  comment?: string
  validation?: { type: 'list'|'number'|'text'; options?: string; min?: number; max?: number }
}
interface SheetData {
  name: string; rows: string[][]; colCount: number
  colWidths: number[]; colTypes: string[]; rowHeights: number[]
  cellMeta: Record<string, CellMeta>
  merges: { row: number; col: number; rowspan: number; colspan: number }[]
  frozenRows: number; frozenCols: number
  groups: { type: 'row'|'col'; start: number; end: number; collapsed: boolean }[]
}
interface CondRule { condition: string; value: string; bgColor: string }

const fontList = ['Arial','Courier New','Georgia','Times New Roman','Verdana','Microsoft YaHei','SimSun','SimHei','KaiTi','FangSong','monospace','serif','sans-serif']
const fontSizes = [10,11,12,13,14,16,18,20,24,28,32,36,48,64,72]
const weekDays = ['周一','周二','周三','周四','周五','周六','周日']
const months = ['1月','2月','3月','4月','5月','6月','7月','8月','9月','10月','11月','12月']

const sheets = ref<SheetData[]>([])
const activeSheet = ref(0)

function makeSheet(name: string, rows = 50, cols = 26): SheetData {
  return { name, rows: Array.from({ length: rows }, () => Array(cols).fill('')),
    colCount: cols, colWidths: Array(cols).fill(120), colTypes: Array(cols).fill('auto'),
    rowHeights: Array(rows).fill(26), cellMeta: {}, merges: [],
    frozenRows: 0, frozenCols: 0, groups: [] }
}

const sheet = computed(() => sheets.value[activeSheet.value] || makeSheet('Sheet1'))
const rows = computed(() => sheet.value.rows)
const colCount = computed(() => sheet.value.colCount)
const colWidths = computed(() => sheet.value.colWidths)
const colTypes = computed(() => sheet.value.colTypes)
const rowHeights = computed(() => sheet.value.rowHeights)
const freezeRows = computed(() => sheet.value.frozenRows)
const freezeCols = computed(() => sheet.value.frozenCols)
const currentSheetRows = computed(() => rows.value)

const selection = ref<{ startRow: number; startCol: number; endRow: number; endCol: number } | null>(null)
const editingCell = ref<{ row: number; col: number } | null>(null)
const editingValue = ref('')
const formulaValue = ref('')

const scrollRef = ref<HTMLElement>()
const chartCanvas = ref<HTMLCanvasElement>()
const editInput = ref<HTMLInputElement[]>()

const showChart = ref(false)
const showCondDialog = ref(false)
const showSearchDialog = ref(false)
const showFilterPanel = ref(false)
const showCommentDialog = ref(false)
const showValidationDialog = ref(false)

const chartType = ref('bar')
const chartDataRange = ref('col')
const chartTitle = ref('')
const chartTooltip = reactive({ show: false, x: 0, y: 0, text: '' })

const contextMenu = reactive({ show: false, x: 0, y: 0 })
let ctxRow = 0, ctxCol = 0

const sortCol = ref(-1)
const sortDir = ref<'asc'|'desc'>('asc')
const filterActiveCols = ref(new Set<number>())
const filterUniqueValues = ref<string[]>([])
const filterSelectedValues = ref(new Set<string>())
const filterSelectAll = ref(true)
let filterTargetCol = 0

const condRules = ref<CondRule[]>([])
const newCond = reactive({ condition: '>', value: '', bgColor: '#ffcccc' })
const condScale = ref('2')
const condScale2Min = ref('#ffffff')
const condScale2Max = ref('#4caf50')
const condScale3Min = ref('#f44336')
const condScale3Mid = ref('#ffff00')
const condScale3Max = ref('#4caf50')
const condDataBarColor = ref('#4caf50')

const cellTextColor = ref('')
const cellBgColor = ref('')
const cellFontFamily = ref('Arial')
const cellFontSize = ref(13)
const currentColType = ref('auto')

const searchText = ref('')
const replaceText = ref('')
const searchResult = reactive({ count: 0, current: 0 })
const commentText = ref('')
let commentRow = 0, commentCol = 0

const validationType = ref<'list'|'number'|'text'>('list')
const validationOptions = ref('')
const validationMin = ref<number|undefined>(undefined)
const validationMax = ref<number|undefined>(undefined)
let validationRow = 0, validationCol = 0

const hoverComment = reactive({ show: false, x: 0, y: 0, text: '' })

// Undo/Redo
const undoStack: string[] = []
const redoStack: string[] = []
const canUndo = computed(() => undoStack.length > 0)
const canRedo = computed(() => redoStack.length > 0)
function snapshot(): string { return JSON.stringify(sheets.value) }
function pushUndo() { undoStack.push(snapshot()); if (undoStack.length > 50) undoStack.shift(); redoStack.length = 0 }
function undo() { if (!undoStack.length) return; redoStack.push(snapshot()); sheets.value = JSON.parse(undoStack.pop()!) }
function redo() { if (!redoStack.length) return; undoStack.push(snapshot()); sheets.value = JSON.parse(redoStack.pop()!) }

// Data Load
function loadData() {
  if (!props.initialData) { sheets.value = [makeSheet('Sheet1')]; return }
  try {
    const d = JSON.parse(props.initialData)
    if (Array.isArray(d)) { sheets.value = d; return }
    const s: SheetData = {
      name: 'Sheet1', rows: d.rows || [], colCount: d.cols || 26,
      colWidths: d.colWidths || Array(d.cols || 26).fill(120),
      colTypes: d.colTypes || Array(d.cols || 26).fill('auto'),
      rowHeights: d.rowHeights || Array((d.rows || []).length).fill(26),
      cellMeta: {}, merges: d.merges || [],
      frozenRows: d.freezeRows || 0, frozenCols: d.freezeCols || 0, groups: []
    }
    if (d.cellMeta && Array.isArray(d.cellMeta)) { (d.cellMeta as [string, CellMeta][]).forEach(([k, v]) => s.cellMeta[k] = v) }
    else if (d.cellMeta && typeof d.cellMeta === 'object') { s.cellMeta = d.cellMeta }
    if (d.condRules) condRules.value = d.condRules
    sheets.value = [s]
  } catch { sheets.value = [makeSheet('Sheet1')] }
}

// Cell Meta
function metaKey(r: number, c: number): string { return `${r},${c}` }
function getCellMeta(r: number, c: number): CellMeta { return sheet.value.cellMeta[metaKey(r, c)] || {} }
function setCellMeta(r: number, c: number, partial: Partial<CellMeta>) {
  const k = metaKey(r, c)
  if (!sheet.value.cellMeta[k]) sheet.value.cellMeta[k] = {}
  Object.assign(sheet.value.cellMeta[k], partial)
}
function getMetaProp(prop: keyof CellMeta): any {
  if (!selection.value) return undefined
  return getCellMeta(selection.value.startRow, selection.value.startCol)[prop]
}
function getComment(r: number, c: number): string { return getCellMeta(r, c).comment || '' }
function getValidation(r: number, c: number) { return getCellMeta(r, c).validation }

// Column Name
function colName(idx: number): string { let n = ''; let i = idx; while (i >= 0) { n = String.fromCharCode(65 + (i % 26)) + n; i = Math.floor(i / 26) - 1 }; return n }
function colIndex(name: string): number { let idx = 0; for (let i = 0; i < name.length; i++) idx = idx * 26 + (name.charCodeAt(i) - 64); return idx - 1 }
const currentCellRef = computed(() => { if (!selection.value) return ''; return colName(selection.value.startCol) + (selection.value.startRow + 1) })

// Selection
const isMultiCellSelection = computed(() => {
  if (!selection.value) return false
  const { startRow, startCol, endRow, endCol } = selection.value
  return startRow !== endRow || startCol !== endCol
})
function isSelected(r: number, c: number): boolean {
  if (!selection.value) return false
  const { startRow, startCol, endRow, endCol } = selection.value
  return r >= Math.min(startRow, endRow) && r <= Math.max(startRow, endRow) && c >= Math.min(startCol, endCol) && c <= Math.max(startCol, endCol)
}
function isSelectionHead(r: number, c: number): boolean { return selection.value?.startRow === r && selection.value?.startCol === c }
function isRowSelected(ri: number): boolean { if (!selection.value) return false; return ri >= Math.min(selection.value.startRow, selection.value.endRow) && ri <= Math.max(selection.value.startRow, selection.value.endRow) }
function isColSelected(ci: number): boolean { if (!selection.value) return false; return ci >= Math.min(selection.value.startCol, selection.value.endCol) && ci <= Math.max(selection.value.startCol, selection.value.endCol) }
function selectCell(r: number, c: number, e?: MouseEvent) {
  if (e?.shiftKey && selection.value) { selection.value.endRow = r; selection.value.endCol = c }
  else { selection.value = { startRow: r, startCol: c, endRow: r, endCol: c } }
  ctxRow = r; ctxCol = c; editingCell.value = null; updateFormula(); updateToolbarState()
}
function selectRow(ri: number) { selection.value = { startRow: ri, startCol: 0, endRow: ri, endCol: colCount.value - 1 }; updateFormula() }
function selectCol(ci: number) { selection.value = { startRow: 0, startCol: ci, endRow: rows.value.length - 1, endCol: ci }; updateFormula() }
function updateFormula() { if (!selection.value) return; formulaValue.value = rows.value[selection.value.startRow]?.[selection.value.startCol] || '' }
function updateToolbarState() {
  if (!selection.value) return
  const m = getCellMeta(selection.value.startRow, selection.value.startCol)
  cellTextColor.value = m.color || ''; cellBgColor.value = m.bgColor || ''
  cellFontFamily.value = m.fontFamily || 'Arial'; cellFontSize.value = m.fontSize || 13
  currentColType.value = colTypes.value[selection.value.startCol] || 'auto'
}
function moveNext() { if (!selection.value) return; const nc = selection.value.startCol + 1; if (nc >= colCount.value) selectCell(selection.value.startRow + 1, 0); else selectCell(selection.value.startRow, nc); updateFormula() }

// Editing
function startEdit(r: number, c: number) {
  if (editingCell.value?.row === r && editingCell.value?.col === c) return
  finishEdit(); editingCell.value = { row: r, col: c }; editingValue.value = rows.value[r]?.[c] || ''
  nextTick(() => { editInput.value?.[0]?.focus() })
}
function finishEdit() {
  if (!editingCell.value) return; const { row, col } = editingCell.value; pushUndo()
  if (!rows.value[row]) rows.value[row] = Array(colCount.value).fill('')
  rows.value[row][col] = editingValue.value; editingCell.value = null; emitChange()
}
function cancelEdit() { editingCell.value = null; updateFormula() }
function applyFormula() {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) rows.value[r][c] = formulaValue.value
  emitChange()
}
function cancelFormula() { updateFormula() }
function handleEditKey(e: KeyboardEvent) {
  if (e.key === 'ArrowUp') { e.preventDefault(); finishEdit(); if (selection.value && selection.value.startRow > 0) selectCell(selection.value.startRow - 1, selection.value.startCol) }
  if (e.key === 'ArrowDown') { e.preventDefault(); finishEdit(); if (selection.value) selectCell(Math.min(selection.value.startRow + 1, rows.value.length - 1), selection.value.startCol) }
}

// Cell Display
function getCellDisplay(r: number, c: number): string {
  const raw = rows.value[r]?.[c] ?? ''
  if (raw.startsWith('=')) return computeFormula(raw)
  const t = colTypes.value[c] || 'auto'
  if (t === 'percent' && raw !== '') { const n = parseFloat(raw); return isNaN(n) ? raw : (n * 100).toFixed(2) + '%' }
  if (t === 'currency' && raw !== '') { const n = parseFloat(raw); return isNaN(n) ? raw : '¥' + n.toFixed(2) }
  if (t === 'number' && raw !== '') { const n = parseFloat(raw); return isNaN(n) ? raw : n.toLocaleString() }
  return raw
}
function getCellTextStyle(r: number, c: number): Record<string, string> {
  const m = getCellMeta(r, c); const s: Record<string, string> = {}
  if (m.bold) s.fontWeight = 'bold'; if (m.italic) s.fontStyle = 'italic'
  if (m.underline && m.strike) s.textDecoration = 'underline line-through'
  else if (m.underline) s.textDecoration = 'underline'; else if (m.strike) s.textDecoration = 'line-through'
  if (m.color) s.color = m.color; if (m.fontFamily) s.fontFamily = m.fontFamily
  if (m.fontSize) s.fontSize = m.fontSize + 'px'; if (m.align) s.textAlign = m.align
  if (m.wrap) { s.whiteSpace = 'normal'; s.wordBreak = 'break-all' }
  return s
}
function getCellStyle(r: number, c: number): Record<string, string> {
  const m = getCellMeta(r, c); const s: Record<string, string> = {}
  if (m.bgColor) s.background = m.bgColor
  const b = m.border; const brd = '1px solid #333'
  if (b) { if (b.top) s.borderTop = brd; if (b.bottom) s.borderBottom = brd; if (b.left) s.borderLeft = brd; if (b.right) s.borderRight = brd }
  for (const rule of condRules.value) { const v = rows.value[r]?.[c] || ''; if (testCond(v, rule)) { s.background = rule.bgColor; break } }
  return s
}
function getColspan(r: number, c: number): number | undefined { const mg = sheet.value.merges.find(m => m.row === r && m.col === c); return mg?.colspan }
function getRowspan(r: number, c: number): number | undefined { const mg = sheet.value.merges.find(m => m.row === r && m.col === c); return mg?.rowspan }
function isCellHidden(r: number, c: number): boolean {
  return sheet.value.merges.some(m => { if (m.row === r && m.col === c) return false; return r >= m.row && r < m.row + m.rowspan && c >= m.col && c < m.col + m.colspan })
}
const hasMerge = computed(() => {
  if (!selection.value) return false
  const { startRow, startCol, endRow, endCol } = selection.value
  return sheet.value.merges.some(m => m.row >= Math.min(startRow, endRow) && m.row <= Math.max(startRow, endRow) && m.col >= Math.min(startCol, endCol) && m.col <= Math.max(startCol, endCol))
})
function toggleMerge() {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow), c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  const idx = sheet.value.merges.findIndex(m => m.row === r1 && m.col === c1)
  if (idx >= 0) sheet.value.merges.splice(idx, 1)
  else sheet.value.merges.push({ row: r1, col: c1, rowspan: r2 - r1 + 1, colspan: c2 - c1 + 1 })
  emitChange()
}

// Formatting
function toggleFormat(prop: 'bold'|'italic'|'underline'|'strike') {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) setCellMeta(r, c, { [prop]: !getCellMeta(r, c)[prop] })
  emitChange()
}
function applyTextColor(c: string) { applyToSelection('color', c) }
function applyBgColor(c: string) { applyToSelection('bgColor', c) }
function applyFontFamily(f: string) { applyToSelection('fontFamily', f) }
function applyFontSize(s: number) { applyToSelection('fontSize', s) }
function setAlign(a: string) { applyToSelection('align', a) }
function toggleWrap() { if (!selection.value) return; applyToSelection('wrap', !getMetaProp('wrap')) }
function applyToSelection(prop: string, val: any) {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) setCellMeta(r, c, { [prop]: val })
  emitChange()
}
function applyBorder(cmd: string) {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) {
      const b: Record<string, boolean> = {}
      if (cmd === 'all') b.top = b.right = b.bottom = b.left = true
      else if (cmd === 'outer') { if (r === Math.min(startRow, endRow)) b.top = true; if (r === Math.max(startRow, endRow)) b.bottom = true; if (c === Math.min(startCol, endCol)) b.left = true; if (c === Math.max(startCol, endCol)) b.right = true }
      else if (cmd === 'top') b.top = true; else if (cmd === 'bottom') b.bottom = true
      else if (cmd === 'left') b.left = true; else if (cmd === 'right') b.right = true
      setCellMeta(r, c, { border: b as any })
    }
  emitChange()
}
function changePrecision(delta: number) {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) setCellMeta(r, c, { precision: Math.max(0, Math.min(10, (getCellMeta(r, c).precision ?? 2) + delta)) })
  emitChange()
}
function setColType(t: string) { if (!selection.value) return; const { startCol, endCol } = selection.value; for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) colTypes.value[c] = t; emitChange() }

// Row/Col Operations
function makeRow(cols: number): string[] { return Array(cols).fill('') }
function insertRowAt(idx: number) { pushUndo(); rows.value.splice(idx, 0, makeRow(colCount.value)); rowHeights.value.splice(idx, 0, 26); emitChange() }
function deleteRowAt(idx: number) { if (rows.value.length <= 1) return; pushUndo(); rows.value.splice(idx, 1); rowHeights.value.splice(idx, 1); emitChange() }
function insertColAt(idx: number) { pushUndo(); sheet.value.colCount++; colWidths.value.splice(idx, 0, 120); colTypes.value.splice(idx, 0, 'auto'); rows.value.forEach(r => r.splice(idx, 0, '')); emitChange() }
function deleteColAt(idx: number) { if (colCount.value <= 1) return; pushUndo(); sheet.value.colCount--; colWidths.value.splice(idx, 1); colTypes.value.splice(idx, 1); rows.value.forEach(r => r.splice(idx, 1)); emitChange() }
function addRowAbove() { insertRowAt(selection.value?.startRow ?? 0) }
function addRowBelow() { insertRowAt((selection.value?.startRow ?? rows.value.length - 1) + 1) }
function addColLeft() { insertColAt(selection.value?.startCol ?? 0) }
function addColRight() { insertColAt((selection.value?.startCol ?? colCount.value - 1) + 1) }
function deleteRow() { deleteRowAt(selection.value?.startRow ?? 0) }
function deleteCol() { deleteColAt(selection.value?.startCol ?? 0) }

// Resize
function startColResize(col: number, e: MouseEvent) {
  e.preventDefault(); const startX = e.clientX, sw = colWidths.value[col] || 120
  const mv = (ev: MouseEvent) => { colWidths.value[col] = Math.max(40, sw + ev.clientX - startX) }
  const up = () => { document.removeEventListener('mousemove', mv); document.removeEventListener('mouseup', up); emitChange() }
  document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up)
}
function startRowResize(row: number, e: MouseEvent) {
  e.preventDefault(); const startY = e.clientY, sh = rowHeights.value[row] || 26
  const mv = (ev: MouseEvent) => { rowHeights.value[row] = Math.max(18, sh + ev.clientY - startY) }
  const up = () => { document.removeEventListener('mousemove', mv); document.removeEventListener('mouseup', up); emitChange() }
  document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up)
}
function autoFitCol(col: number) { let maxW = 40; for (const row of rows.value) { const w = (row[col] || '').length * 9 + 16; if (w > maxW) maxW = w }; colWidths.value[col] = Math.min(maxW, 400); emitChange() }

// Freeze
function toggleFreeze(cmd: string) {
  if (cmd === 'row') sheet.value.frozenRows = sheet.value.frozenRows > 0 ? 0 : 1
  else if (cmd === 'col') sheet.value.frozenCols = sheet.value.frozenCols > 0 ? 0 : 1
  else { sheet.value.frozenRows = 0; sheet.value.frozenCols = 0 }; emitChange()
}

// Sort/Filter
function handleColMenu(cmd: string, col: number) {
  if (cmd === 'sort-asc') { sortCol.value = col; sortDir.value = 'asc'; sortRows() }
  else if (cmd === 'sort-desc') { sortCol.value = col; sortDir.value = 'desc'; sortRows() }
  else if (cmd === 'sort-clear') { sortCol.value = -1 }
  else if (cmd === 'filter') openFilter(col)
}
function sortRows() {
  if (sortCol.value < 0) return; pushUndo()
  const c = sortCol.value, d = sortDir.value === 'asc' ? 1 : -1
  rows.value.sort((a, b) => { const va = a[c] || '', vb = b[c] || ''; const na = parseFloat(va), nb = parseFloat(vb); if (!isNaN(na) && !isNaN(nb)) return (na - nb) * d; return va.localeCompare(vb) * d })
  emitChange()
}
function isRowFiltered(ri: number): boolean {
  if (!filterActiveCols.value.size) return false
  for (const c of filterActiveCols.value) if (!filterSelectedValues.value.has(rows.value[ri]?.[c] || '')) return true
  return false
}
function openFilter(col: number) {
  if (filterActiveCols.value.has(col)) { filterActiveCols.value.delete(col); emitChange(); return }
  filterActiveCols.value.add(col); const vals = new Set<string>()
  rows.value.forEach(r => vals.add(r?.[col] || '')); filterUniqueValues.value = Array.from(vals)
  filterSelectedValues.value = new Set(vals); filterSelectAll.value = true; filterTargetCol = col; showFilterPanel.value = true
}
function toggleFilterAll(checked: boolean) { if (checked) filterSelectedValues.value = new Set(filterUniqueValues.value); else filterSelectedValues.value = new Set() }
function toggleFilterValue(val: string, checked: boolean) { if (checked) filterSelectedValues.value.add(val); else filterSelectedValues.value.delete(val) }
function applyFilter(checked: boolean) { if (checked) filterActiveCols.value.add(filterTargetCol); else filterActiveCols.value.delete(filterTargetCol); showFilterPanel.value = false; emitChange() }

// Conditional Format
function addCondRule() { condRules.value.push({ ...newCond }); newCond.value = ''; newCond.bgColor = '#ffcccc' }
function testCond(value: string, rule: CondRule): boolean {
  const n = parseFloat(value), tv = parseFloat(rule.value)
  if (rule.condition === '>') return !isNaN(n) && n > tv; if (rule.condition === '<') return !isNaN(n) && n < tv
  if (rule.condition === '=') return value === rule.value; if (rule.condition === '!=') return value !== rule.value
  if (rule.condition === 'contains') return value.includes(rule.value); return false
}
function hexToRgb(hex: string): { r: number; g: number; b: number } {
  const h = hex.replace('#', ''); return { r: parseInt(h.substring(0, 2), 16), g: parseInt(h.substring(2, 4), 16), b: parseInt(h.substring(4, 6), 16) }
}
function applyCondScale2() {
  if (!selection.value) return; pushUndo()
  const minC = hexToRgb(condScale2Min.value), maxC = hexToRgb(condScale2Max.value)
  const { startRow, startCol, endRow, endCol } = selection.value
  let minV = Infinity, maxV = -Infinity
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) { const n = parseFloat(rows.value[r]?.[c] || ''); if (!isNaN(n)) { if (n < minV) minV = n; if (n > maxV) maxV = n } }
  if (minV === Infinity) return
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) {
      const n = parseFloat(rows.value[r]?.[c] || ''); if (isNaN(n)) continue
      const t = maxV === minV ? 0.5 : (n - minV) / (maxV - minV)
      setCellMeta(r, c, { bgColor: `rgb(${Math.round(minC.r + (maxC.r - minC.r) * t)},${Math.round(minC.g + (maxC.g - minC.g) * t)},${Math.round(minC.b + (maxC.b - minC.b) * t)})` })
    }
  emitChange()
}
function applyCondScale() { applyCondScale2() }
function applyDataBar() {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  let minV = Infinity, maxV = -Infinity
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) { const n = parseFloat(rows.value[r]?.[c] || ''); if (!isNaN(n)) { if (n < minV) minV = n; if (n > maxV) maxV = n } }
  if (minV === Infinity) return
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) {
      const n = parseFloat(rows.value[r]?.[c] || ''); if (isNaN(n)) continue
      const pct = maxV === minV ? 100 : Math.round(((n - minV) / (maxV - minV)) * 100)
      setCellMeta(r, c, { bgColor: `linear-gradient(90deg, ${condDataBarColor.value} ${pct}%, transparent ${pct}%)` })
    }
  emitChange()
}

// Copy/Paste/Cut
let clipData: { data: string[][]; cut?: boolean } | null = null
function clipCopy() {
  if (!selection.value) return
  const { startRow, startCol, endRow, endCol } = selection.value; const data: string[][] = []
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++) { const row: string[] = []; for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) row.push(rows.value[r]?.[c] || ''); data.push(row) }
  clipData = { data }; navigator.clipboard?.writeText(data.map(r => r.join('\t')).join('\n'))
}
function clipCut() { clipCopy(); if (clipData) clipData.cut = true }
function clipPaste() {
  if (!selection.value) return; pushUndo(); const r0 = selection.value.startRow, c0 = selection.value.startCol
  if (clipData) {
    for (let r = 0; r < clipData.data.length; r++) for (let c = 0; c < clipData.data[r].length; c++) { if (!rows.value[r0 + r]) rows.value[r0 + r] = makeRow(colCount.value); rows.value[r0 + r][c0 + c] = clipData.data[r][c] }
    if (clipData.cut) clipData.cut = false
  } else { navigator.clipboard?.readText().then(t => { const lines = t.split('\n'); for (let r = 0; r < lines.length; r++) { const cells = lines[r].split('\t'); for (let c = 0; c < cells.length; c++) { if (!rows.value[r0 + r]) rows.value[r0 + r] = makeRow(colCount.value); rows.value[r0 + r][c0 + c] = cells[c] } }; emitChange() }); return }
  emitChange()
}

// Fill Handle
function startFill(e: MouseEvent) {
  if (!selection.value) return; e.preventDefault()
  const sr = selection.value.startRow, sc = selection.value.startCol, er = selection.value.endRow, ec = selection.value.endCol
  const srcR1 = Math.min(sr, er), srcR2 = Math.max(sr, er), srcC1 = Math.min(sc, ec), srcC2 = Math.max(sc, ec)
  const startY = e.clientY
  const mv = (ev: MouseEvent) => { const dy = ev.clientY - startY; selection.value!.endRow = srcR2 + Math.round(dy / 26) }
  const up = () => {
    document.removeEventListener('mousemove', mv); document.removeEventListener('mouseup', up)
    if (!selection.value) return; pushUndo()
    const tgtR2 = Math.max(sr, selection.value.endRow)
    for (let r = srcR1; r <= tgtR2; r++) for (let c = srcC1; c <= srcC2; c++) {
      if (r >= srcR1 && r <= srcR2) continue
      const srcR = srcR1 + ((r - srcR1) % (srcR2 - srcR1 + 1))
      const sv = rows.value[srcR]?.[c] || ''
      const sn = parseFloat(sv)
      if (!isNaN(sn) && sv === String(sn)) rows.value[r][c] = String(sn + (r - srcR1))
      else { const wIdx = weekDays.indexOf(sv); if (wIdx >= 0) rows.value[r][c] = weekDays[(wIdx + r - srcR1) % 7]; else { const mIdx = months.indexOf(sv); if (mIdx >= 0) rows.value[r][c] = months[(mIdx + r - srcR1) % 12]; else rows.value[r][c] = sv } }
    }
    emitChange()
  }
  document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up)
}

// Move Handle
function startMove(e: MouseEvent) {
  if (!selection.value) return; e.preventDefault()
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow), c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  const data: string[][] = []
  for (let r = r1; r <= r2; r++) { const row: string[] = []; for (let c = c1; c <= c2; c++) row.push(rows.value[r]?.[c] || ''); data.push(row) }
  const startY = e.clientY, startX = e.clientX
  const up = (ev: MouseEvent) => {
    document.removeEventListener('mousemove', () => {}); document.removeEventListener('mouseup', up)
    pushUndo()
    for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) rows.value[r][c] = ''
    const dr = Math.round((ev.clientY - startY) / 26), dc = Math.round((ev.clientX - startX) / (colWidths.value[c1] || 120))
    const tr = r1 + dr, tc = c1 + dc
    for (let r = 0; r < data.length; r++) for (let c = 0; c < data[r].length; c++) { if (!rows.value[tr + r]) rows.value[tr + r] = makeRow(colCount.value); rows.value[tr + r][tc + c] = data[r][c] }
    selection.value = { startRow: tr, startCol: tc, endRow: tr + data.length - 1, endCol: tc + (data[0]?.length || 1) - 1 }; emitChange()
  }
  document.addEventListener('mousemove', () => {}); document.addEventListener('mouseup', up)
}

// Comments
function showCellComment(r: number, c: number, e: MouseEvent) { const cm = getComment(r, c); if (!cm) return; hoverComment.show = true; hoverComment.text = cm; hoverComment.x = e.clientX + 12; hoverComment.y = e.clientY + 12 }
function hideCellComment() { hoverComment.show = false }
function ctxAddComment() { hideContextMenu(); commentRow = ctxRow; commentCol = ctxCol; commentText.value = getComment(ctxRow, ctxCol) || ''; showCommentDialog.value = true }
function ctxDeleteComment() { hideContextMenu(); pushUndo(); const k = metaKey(ctxRow, ctxCol); if (sheet.value.cellMeta[k]) { delete sheet.value.cellMeta[k].comment }; emitChange() }
function saveComment() { pushUndo(); setCellMeta(commentRow, commentCol, { comment: commentText.value || undefined }); showCommentDialog.value = false; emitChange() }

// Validation
function ctxSetValidation() {
  hideContextMenu(); validationRow = ctxRow; validationCol = ctxCol
  const v = getValidation(ctxRow, ctxCol)
  if (v) { validationType.value = v.type; validationOptions.value = v.options || ''; validationMin.value = v.min; validationMax.value = v.max }
  else { validationType.value = 'list'; validationOptions.value = ''; validationMin.value = undefined; validationMax.value = undefined }
  showValidationDialog.value = true
}
function saveValidation() { pushUndo(); setCellMeta(validationRow, validationCol, { validation: { type: validationType.value, options: validationOptions.value, min: validationMin.value, max: validationMax.value } }); showValidationDialog.value = false; emitChange() }

// Grouping
function ctxGroupRows() { hideContextMenu(); if (!selection.value) return; pushUndo(); const { startRow, endRow } = selection.value; sheet.value.groups.push({ type: 'row', start: Math.min(startRow, endRow), end: Math.max(startRow, endRow), collapsed: false }); emitChange() }

// Search/Replace
function findNext() {
  if (!searchText.value) return; const s = selection.value; let sr = s?.startRow ?? 0, sc = (s?.startCol ?? -1) + 1
  searchResult.count = 0; let found = false
  for (let pass = 0; pass < 2; pass++) for (let r = (pass === 0 ? sr : 0); r < rows.value.length; r++) for (let c = (pass === 0 && r === sr ? sc : 0); c < colCount.value; c++) { if ((rows.value[r]?.[c] || '').includes(searchText.value)) { searchResult.count++; if (!found) { selectCell(r, c); found = true } } }
}
function replaceOne() { if (!selection.value || !searchText.value) return; pushUndo(); const r = selection.value.startRow, c = selection.value.startCol, v = rows.value[r]?.[c] || ''; if (v.includes(searchText.value)) { rows.value[r][c] = v.replace(searchText.value, replaceText.value); emitChange() }; findNext() }
function replaceAll() { if (!searchText.value) return; pushUndo(); let count = 0; rows.value.forEach((row, ri) => row.forEach((v, ci) => { if (v.includes(searchText.value)) { row[ci] = v.split(searchText.value).join(replaceText.value); count++ } })); emitChange(); searchResult.count = count }

// Multi-Sheet
function switchSheet(idx: number) { activeSheet.value = idx; selection.value = null; editingCell.value = null; updateToolbarState() }
function addSheet() { sheets.value.push(makeSheet('Sheet' + (sheets.value.length + 1))); activeSheet.value = sheets.value.length - 1 }
function renameSheet(idx: number) {
  const name = prompt('重命名Sheet:', sheets.value[idx]?.name || '')
  if (name) sheets.value[idx].name = name; emitChange()
}
function deleteSheet(idx: number) { if (sheets.value.length <= 1) return; sheets.value.splice(idx, 1); activeSheet.value = Math.min(activeSheet.value, sheets.value.length - 1) }

// Context Menu
function showContextMenu(e: MouseEvent) { ctxRow = selection.value?.startRow ?? 0; ctxCol = selection.value?.startCol ?? 0; contextMenu.show = true; contextMenu.x = e.clientX; contextMenu.y = e.clientY }
function hideContextMenu() { contextMenu.show = false }
function ctxCut() { hideContextMenu(); clipCut() }
function ctxCopy() { hideContextMenu(); clipCopy() }
function ctxPaste() { hideContextMenu(); clipPaste() }
function ctxInsertRowAbove() { hideContextMenu(); addRowAbove() }
function ctxInsertRowBelow() { hideContextMenu(); addRowBelow() }
function ctxInsertColLeft() { hideContextMenu(); addColLeft() }
function ctxInsertColRight() { hideContextMenu(); addColRight() }
function ctxDeleteRow() { hideContextMenu(); deleteRow() }
function ctxDeleteCol() { hideContextMenu(); deleteCol() }
function ctxClearCells() { hideContextMenu(); if (!selection.value) return; pushUndo(); const { startRow, startCol, endRow, endCol } = selection.value; for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++) for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) rows.value[r][c] = ''; emitChange() }
function ctxMergeToggle() { hideContextMenu(); toggleMerge() }

// Formula Engine
function computeFormula(expr: string): string { if (!expr.startsWith('=')) return expr; try { return evalFormula(expr.slice(1)) } catch { return '#ERROR!' } }
function evalFormula(expr: string): string {
  const m = expr.match(/^(\w+)\((.*)\)$/s); if (!m) return String(safeCalc(expr))
  const fn = m[1].toUpperCase(), args = splitArgs(m[2]), parsed = args.map(a => evalArg(a))
  switch (fn) {
    case 'SUM': return String(numArray(args.join(',')).reduce((a, b) => a + b, 0))
    case 'AVG': case 'AVERAGE': { const arr = numArray(args.join(',')); return arr.length ? String(arr.reduce((a, b) => a + b, 0) / arr.length) : '0' }
    case 'COUNT': return String(numArray(args.join(','), true).length)
    case 'MAX': { const arr = numArray(args.join(',')); return arr.length ? String(Math.max(...arr)) : '0' }
    case 'MIN': { const arr = numArray(args.join(',')); return arr.length ? String(Math.min(...arr)) : '0' }
    case 'IF': return parsed[0] ? String(parsed[1] ?? '') : String(parsed[2] ?? '')
    case 'AND': return String(parsed.slice(0, -1).every(Boolean))
    case 'OR': return String(parsed.some(Boolean))
    case 'NOT': return String(!parsed[0])
    case 'CONCAT': case 'CONCATENATE': return parsed.join('')
    case 'LEFT': return String(parsed[0]).slice(0, Number(parsed[1]) || 1)
    case 'RIGHT': return String(parsed[0]).slice(-(Number(parsed[1]) || 1))
    case 'MID': return String(parsed[0]).slice(Number(parsed[1]) - 1, Number(parsed[1]) - 1 + Number(parsed[2]))
    case 'LEN': case 'LENGTH': return String(String(parsed[0]).length)
    case 'UPPER': return String(parsed[0]).toUpperCase()
    case 'LOWER': return String(parsed[0]).toLowerCase()
    case 'TRIM': return String(parsed[0]).trim()
    case 'SUBSTITUTE': case 'REPLACE': return String(parsed[0]).split(String(parsed[1])).join(String(parsed[2]))
    case 'TEXT': return String(parsed[0])
    case 'VALUE': return String(parseFloat(String(parsed[0])) || 0)
    case 'ROUND': return String(Math.round(Number(parsed[0]) * Math.pow(10, Number(parsed[1]) || 0)) / Math.pow(10, Number(parsed[1]) || 0))
    case 'CEIL': case 'CEILING': return String(Math.ceil(Number(parsed[0])))
    case 'FLOOR': return String(Math.floor(Number(parsed[0])))
    case 'ABS': return String(Math.abs(Number(parsed[0])))
    case 'MOD': return String(Number(parsed[0]) % Number(parsed[1]))
    case 'POWER': case 'POW': return String(Math.pow(Number(parsed[0]), Number(parsed[1])))
    case 'SQRT': return String(Math.sqrt(Number(parsed[0])))
    case 'LOG': return String(Math.log(Number(parsed[0])))
    case 'LOG10': return String(Math.log10(Number(parsed[0])))
    case 'EXP': return String(Math.exp(Number(parsed[0])))
    case 'PI': return String(Math.PI)
    case 'INT': return String(Math.floor(Number(parsed[0])))
    case 'RAND': return String(Math.random())
    case 'RANDBETWEEN': return String(Math.floor(Math.random() * (Number(parsed[1]) - Number(parsed[0]) + 1)) + Number(parsed[0]))
    case 'NOW': return new Date().toLocaleString('zh-CN')
    case 'TODAY': return new Date().toLocaleDateString('zh-CN')
    case 'YEAR': return String(new Date(String(parsed[0])).getFullYear())
    case 'MONTH': return String(new Date(String(parsed[0])).getMonth() + 1)
    case 'DAY': return String(new Date(String(parsed[0])).getDate())
    case 'DATEDIF': { const d1 = new Date(String(parsed[0])), d2 = new Date(String(parsed[1])); const diff = (d2.getTime() - d1.getTime()) / 86400000; return String(parsed[2] === 'Y' ? Math.floor(diff / 365.25) : parsed[2] === 'M' ? Math.floor(diff / 30.44) : Math.floor(diff)) }
    case 'WEEKDAY': return String(new Date(String(parsed[0])).getDay() + 1)
    case 'ISBLANK': return String(parsed[0] === '' || parsed[0] === undefined || parsed[0] === null)
    case 'ISNUMBER': return String(!isNaN(Number(parsed[0])) && parsed[0] !== '')
    case 'ISTEXT': return String(typeof parsed[0] === 'string' && isNaN(Number(parsed[0])))
    case 'VLOOKUP': { const key = String(parsed[0]); const data = getRangeData(args[1]); const col = Number(parsed[2]) - 1; const row = data.find(rc => String(getCellValByRC(rc.r, rc.c)) === key); return row ? String(getCellValByRC(row.r, row.c + col)) : '#N/A' }
    case 'INDEX': { const arr = numArray(args[0], false, true); return String(arr[Number(parsed[1]) - 1] ?? '#N/A') }
    case 'MATCH': { const arr = strArray(args[0], true); const idx = arr.indexOf(String(parsed[1])); return String(idx >= 0 ? idx + 1 : '#N/A') }
    case 'CHOOSE': return String(parsed[Number(parsed[0])] ?? '#N/A')
    default: return '#NAME?'
  }
}
function evalArg(a: string): any {
  a = a.trim()
  if ((a.startsWith('"') && a.endsWith('"')) || (a.startsWith("'") && a.endsWith("'"))) return a.slice(1, -1)
  const crossRef = a.match(/^(\w+)!(.+)$/i)
  if (crossRef) { const si = sheets.value.findIndex(s => s.name.toLowerCase() === crossRef[1].toLowerCase()); if (si >= 0) { const s = sheets.value[si]; const cr = crossRef[2].match(/^([A-Z]+)(\d+)$/); if (cr) { return s.rows[parseInt(cr[2]) - 1]?.[colIndex(cr[1])] || '' } }; return '#REF!' }
  const cr = a.match(/^([A-Z]+)(\d+)$/); if (cr) return getCellVal(cr[1], parseInt(cr[2]))
  const rng = a.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/); if (rng) return getRange(a)
  if (!isNaN(Number(a))) return Number(a)
  if (a.toUpperCase() === 'TRUE') return true; if (a.toUpperCase() === 'FALSE') return false
  return a
}
function getCellVal(col: string, row: number): any { const c = colIndex(col), r = row - 1; const v = rows.value[r]?.[c]; if (v === undefined || v === '') return ''; const n = parseFloat(v); return isNaN(n) ? v : n }
function getCellValByRC(r: number, c: number): any { const v = rows.value[r]?.[c]; if (v === undefined || v === '') return ''; const n = parseFloat(v); return isNaN(n) ? v : n }
function getRange(rangeStr: string): any[] { const m = rangeStr.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/); if (!m) return []; const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1; const res: any[] = []; for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) res.push(rows.value[r]?.[c] || ''); return res }
interface RC { r: number; c: number }
function getRangeData(arg: string): RC[] { const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/); if (!m) return []; const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1; const res: RC[] = []; for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) res.push({ r, c }); return res }
function numArray(arg: string, countNonNum = false, raw = false): number[] {
  const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (m) { const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1; const res: number[] = []; for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) { const v = rows.value[r]?.[c]; if (countNonNum) { if (v !== undefined && v !== '') res.push(raw ? parseFloat(v) || 0 : 1) } else { const n = parseFloat(v); if (!isNaN(n)) res.push(n) } }; return res }
  return arg.split(',').map(v => parseFloat(v.trim())).filter(v => !isNaN(v))
}
function strArray(arg: string, keepAll = false): string[] {
  const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (m) { const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1; const res: string[] = []; for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) { const v = rows.value[r]?.[c]; if (keepAll || (v !== undefined && v !== '')) res.push(v || '') }; return res }
  return arg.split(',').map(v => v.trim())
}
function splitArgs(s: string): string[] { const args: string[] = []; let depth = 0, cur = '', inStr = false; for (let i = 0; i < s.length; i++) { const ch = s[i]; if (ch === '"' && (i === 0 || s[i - 1] !== '\\')) inStr = !inStr; if (!inStr) { if (ch === '(') depth++; if (ch === ')') depth-- } if (ch === ',' && depth === 0 && !inStr) { args.push(cur); cur = '' } else cur += ch }; if (cur.trim()) args.push(cur); return args }
function safeCalc(expr: string): number { let safe = expr.replace(/([A-Z]+)(\d+)/gi, (_, col, row) => { const v = getCellVal(col.toUpperCase(), parseInt(row)); return isNaN(v) ? '0' : String(v) }); safe = safe.replace(/[^0-9+\-*/.() ]/g, ''); try { return Function('"use strict"; return (' + safe + ')')() } catch { return NaN } }

// Chart
function drawChart() { const canvas = chartCanvas.value; if (!canvas) return; const ctx = canvas.getContext('2d'); if (!ctx) return; ctx.clearRect(0, 0, canvas.width, canvas.height); const data = getChartData(); if (!data.length) return; const t = chartType.value; if (t === 'bar') drawBarChart(ctx, data, canvas); else if (t === 'line') drawLineChart(ctx, data, canvas); else if (t === 'pie') drawPieChart(ctx, data, canvas); else if (t === 'scatter') drawScatterChart(ctx, data, canvas); else if (t === 'area') drawAreaChart(ctx, data, canvas) }
function getChartData(): number[] {
  if (!selection.value) return []; const range = chartDataRange.value; let nums: number[] = []
  if (range === 'col') { const c = selection.value.startCol; for (const row of rows.value) { const n = parseFloat(row[c]); if (!isNaN(n)) nums.push(n) } }
  else if (range === 'selection') { const { startRow, startCol, endRow, endCol } = selection.value; for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++) for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) { const n = parseFloat(rows.value[r]?.[c]); if (!isNaN(n)) nums.push(n) } }
  else { rows.value.forEach(row => row.forEach(v => { const n = parseFloat(v); if (!isNaN(n)) nums.push(n) })) }
  return nums.slice(0, 50)
}
function drawBarChart(ctx: CanvasRenderingContext2D, data: number[], canvas: HTMLCanvasElement) { const w = canvas.width, h = canvas.height, pad = 50, maxV = Math.max(...data, 1), bw = Math.max(4, (w - pad * 2) / data.length - 4); if (chartTitle.value) { ctx.fillStyle = '#333'; ctx.font = '14px sans-serif'; ctx.fillText(chartTitle.value, w / 2 - 40, 20) }; data.forEach((v, i) => { const x = pad + i * (bw + 4), bh = (v / maxV) * (h - pad * 2); ctx.fillStyle = `hsl(${(i * 360 / data.length) % 360},70%,55%)`; ctx.fillRect(x, h - pad - bh, bw, bh) }) }
function drawLineChart(ctx: CanvasRenderingContext2D, data: number[], canvas: HTMLCanvasElement) { const w = canvas.width, h = canvas.height, pad = 50, maxV = Math.max(...data, 1); ctx.strokeStyle = '#4caf50'; ctx.lineWidth = 2; ctx.beginPath(); data.forEach((v, i) => { const x = pad + (i / (data.length - 1 || 1)) * (w - pad * 2), y = h - pad - (v / maxV) * (h - pad * 2); i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y) }); ctx.stroke() }
function drawPieChart(ctx: CanvasRenderingContext2D, data: number[], canvas: HTMLCanvasElement) { const cx = canvas.width / 2, cy = canvas.height / 2, r = Math.min(cx, cy) - 50; const total = data.reduce((a, b) => a + Math.abs(b), 0) || 1; let angle = -Math.PI / 2; data.forEach((v, i) => { const slice = (Math.abs(v) / total) * Math.PI * 2; ctx.fillStyle = `hsl(${(i * 360 / data.length) % 360},70%,55%)`; ctx.beginPath(); ctx.moveTo(cx, cy); ctx.arc(cx, cy, r, angle, angle + slice); ctx.closePath(); ctx.fill(); angle += slice }) }
function drawScatterChart(ctx: CanvasRenderingContext2D, data: number[], canvas: HTMLCanvasElement) { const w = canvas.width, h = canvas.height, pad = 50, maxV = Math.max(...data, 1); ctx.fillStyle = '#1a73e8'; data.forEach((v, i) => { const x = pad + (i / (data.length - 1 || 1)) * (w - pad * 2), y = h - pad - (v / maxV) * (h - pad * 2); ctx.beginPath(); ctx.arc(x, y, 4, 0, Math.PI * 2); ctx.fill() }) }
function drawAreaChart(ctx: CanvasRenderingContext2D, data: number[], canvas: HTMLCanvasElement) { const w = canvas.width, h = canvas.height, pad = 50, maxV = Math.max(...data, 1), baseY = h - pad; ctx.fillStyle = 'rgba(76,175,80,0.3)'; ctx.beginPath(); ctx.moveTo(pad, baseY); data.forEach((v, i) => { ctx.lineTo(pad + (i / (data.length - 1 || 1)) * (w - pad * 2), h - pad - (v / maxV) * (h - pad * 2)) }); ctx.lineTo(pad + (w - pad * 2), baseY); ctx.closePath(); ctx.fill(); ctx.strokeStyle = '#4caf50'; ctx.lineWidth = 2; ctx.beginPath(); data.forEach((v, i) => { const x = pad + (i / (data.length - 1 || 1)) * (w - pad * 2), y = h - pad - (v / maxV) * (h - pad * 2); i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y) }); ctx.stroke() }
function onChartHover(e: MouseEvent) { const canvas = chartCanvas.value; if (!canvas) return; const rect = canvas.getBoundingClientRect(), mx = e.clientX - rect.left, my = e.clientY - rect.top; const data = getChartData(); if (!data.length) return; const pad = 50, w = canvas.width, h = canvas.height, maxV = Math.max(...data, 1); let closest = '', minDist = 20; data.forEach((v, i) => { const x = pad + (i / (data.length - 1 || 1)) * (w - pad * 2), y = h - pad - (v / maxV) * (h - pad * 2); const d = Math.sqrt((mx - x) ** 2 + (my - y) ** 2); if (d < minDist) { minDist = d; closest = `[${i}]: ${v}` } }); if (closest) { chartTooltip.show = true; chartTooltip.text = closest; chartTooltip.x = mx; chartTooltip.y = my - 20 } else chartTooltip.show = false }
function exportChart() { const canvas = chartCanvas.value; if (!canvas) return; const link = document.createElement('a'); link.download = 'chart.png'; link.href = canvas.toDataURL(); link.click() }
watch([showChart, chartType, chartDataRange, chartTitle, selection], () => { if (showChart.value) nextTick(drawChart) })

// Global Keyboard
function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) { e.preventDefault(); undo(); return }
  if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.key === 'z' && e.shiftKey))) { e.preventDefault(); redo(); return }
  if ((e.ctrlKey || e.metaKey) && e.key === 'c') { if (!editingCell.value) { e.preventDefault(); clipCopy() } return }
  if ((e.ctrlKey || e.metaKey) && e.key === 'x') { if (!editingCell.value) { e.preventDefault(); clipCut() } return }
  if ((e.ctrlKey || e.metaKey) && e.key === 'v') { if (!editingCell.value) { e.preventDefault(); clipPaste() } return }
  if ((e.ctrlKey || e.metaKey) && (e.key === 'f' || e.key === 'h')) { e.preventDefault(); showSearchDialog.value = true; return }
  if ((e.ctrlKey || e.metaKey) && e.key === 'a') { if (!editingCell.value) { e.preventDefault(); selection.value = { startRow: 0, startCol: 0, endRow: rows.value.length - 1, endCol: colCount.value - 1 } } return }
  if (editingCell.value) return; if (!selection.value) return
  if (e.key === 'Delete' || e.key === 'Backspace') { pushUndo(); const { startRow, startCol, endRow, endCol } = selection.value; for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++) for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) rows.value[r][c] = ''; emitChange(); e.preventDefault() }
  else if (e.key === 'Enter') { startEdit(selection.value.startRow, selection.value.startCol); e.preventDefault() }
  else if (e.key === 'Tab') { moveNext(); e.preventDefault() }
  else if (e.key === 'ArrowUp') { if (selection.value.startRow > 0) selectCell(selection.value.startRow - 1, selection.value.startCol) }
  else if (e.key === 'ArrowDown') { selectCell(Math.min(selection.value.startRow + 1, rows.value.length - 1), selection.value.startCol) }
  else if (e.key === 'ArrowLeft') { if (selection.value.startCol > 0) selectCell(selection.value.startRow, selection.value.startCol - 1) }
  else if (e.key === 'ArrowRight') { if (selection.value.startCol < colCount.value - 1) selectCell(selection.value.startRow, selection.value.startCol + 1) }
  else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey) { editingValue.value = e.key; startEdit(selection.value.startRow, selection.value.startCol) }
}

// Emit
function emitChange() { emit('change', getData()) }
function getData(): string { return JSON.stringify(sheets.value) }

// Lifecycle
loadData()
onMounted(() => { document.addEventListener('keydown', onGlobalKeydown) })
onUnmounted(() => { document.removeEventListener('keydown', onGlobalKeydown) })
defineExpose({ getData })
</script>

<style scoped>
.sheet-container { display: flex; flex-direction: column; height: 100%; background: #fff; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
.formula-bar { display: flex; align-items: center; border-bottom: 1px solid #d0d3d8; height: 32px; font-size: 13px; }
.cell-ref { width: 80px; text-align: center; border-right: 1px solid #d0d3d8; font-weight: 500; color: #333; height: 100%; display: flex; align-items: center; justify-content: center; background: #f8f9fa; }
.formula-divider { padding: 0 8px; color: #666; font-style: italic; border-right: 1px solid #d0d3d8; height: 100%; display: flex; align-items: center; background: #f8f9fa; }
.formula-input { flex: 1; border: none; outline: none; padding: 0 8px; height: 100%; font-size: 13px; }
.sheet-toolbar { display: flex; align-items: center; gap: 4px; padding: 4px 8px; border-bottom: 1px solid #d0d3d8; background: #f8f9fa; flex-wrap: wrap; }
.sheet-info { margin-left: auto; color: #999; font-size: 12px; white-space: nowrap; }
.sheet-scroll { flex: 1; overflow: auto; }
.sheet-table { border-collapse: collapse; table-layout: fixed; }
.sheet-table th, .sheet-table td { border: 1px solid #d0d3d8; font-size: 13px; position: relative; }
.corner { background: #eef0f4; width: 40px; position: sticky; top: 0; left: 0; z-index: 5; }
.col-header { background: #eef0f4; font-weight: 500; color: #555; text-align: center; position: sticky; top: 0; z-index: 4; cursor: pointer; user-select: none; min-height: 26px; }
.col-header:hover { background: #dde0e6; }
.col-header.selected { background: #c8ddf0; color: #1a73e8; }
.col-header.sorted { color: #1a73e8; }
.col-menu-trigger { cursor: pointer; font-size: 10px; margin-left: 2px; opacity: 0.5; }
.col-menu-trigger:hover { opacity: 1; }
.sort-icon { font-size: 10px; color: #1a73e8; }
.filter-icon { font-size: 10px; color: #e6a23c; }
.col-resize-handle { position: absolute; right: -2px; top: 0; bottom: 0; width: 5px; cursor: col-resize; }
.row-header { background: #eef0f4; text-align: center; color: #555; font-weight: 500; position: sticky; left: 0; z-index: 2; cursor: pointer; user-select: none; position: relative; }
.row-header:hover { background: #dde0e6; }
.row-header.selected { background: #c8ddf0; color: #1a73e8; }
.row-resize-handle { position: absolute; bottom: -2px; left: 0; right: 0; height: 5px; cursor: row-resize; }
.frozen-corner { z-index: 7 !important; }
.frozen-col-header { z-index: 6 !important; }
.frozen-row-header { z-index: 3 !important; }
.cell { padding: 0; cursor: cell; overflow: hidden; position: relative; }
.cell.selected { background: #e8f0fe; }
.cell.selected-head { outline: 2px solid #1a73e8; outline-offset: -1px; z-index: 1; }
.cell.editing { padding: 0; }
.cell-display { display: block; padding: 0 6px; line-height: 26px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cell-edit-input { width: 100%; height: 100%; border: none; outline: none; padding: 0 6px; font-size: 13px; font-family: inherit; background: #fff; }
.fill-handle { position: absolute; right: -3px; bottom: -3px; width: 8px; height: 8px; background: #1a73e8; cursor: crosshair; z-index: 2; border-radius: 1px; }
.move-handle { position: absolute; top: 0; left: 50%; transform: translateX(-50%); width: 20px; height: 4px; background: #1a73e8; cursor: move; z-index: 2; border-radius: 2px; }
.comment-marker { position: absolute; top: 0; right: 0; width: 0; height: 0; border-left: 6px solid transparent; border-top: 6px solid #ff6b6b; z-index: 3; }
.hover-comment { position: fixed; background: #fff; border: 1px solid #ddd; border-radius: 6px; padding: 8px 12px; font-size: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.12); z-index: 2000; max-width: 250px; white-space: pre-wrap; }
.context-menu { position: fixed; background: #fff; border: 1px solid #d0d3d8; border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.12); z-index: 1000; min-width: 180px; padding: 4px 0; }
.menu-item { padding: 6px 16px; font-size: 13px; cursor: pointer; color: #333; }
.menu-item:hover { background: #f0f5ff; color: #1a73e8; }
.menu-divider { height: 1px; background: #e8e8e8; margin: 4px 0; }
.chart-panel { border-top: 1px solid #d0d3d8; background: #fafafa; padding: 8px; }
.chart-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.chart-title-input { border: 1px solid #d0d3d8; border-radius: 4px; padding: 2px 8px; font-size: 13px; width: 120px; outline: none; }
.chart-title-input:focus { border-color: #409eff; }
.chart-body { display: flex; justify-content: center; position: relative; }
.chart-body canvas { border: 1px solid #e8e8e8; border-radius: 4px; background: #fff; }
.chart-tooltip { position: absolute; background: rgba(0,0,0,0.75); color: #fff; padding: 4px 8px; border-radius: 4px; font-size: 12px; pointer-events: none; }
.filter-panel { position: fixed; background: #fff; border: 1px solid #d0d3d8; border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.12); z-index: 1001; width: 200px; max-height: 300px; }
.filter-panel-header { padding: 8px 12px; border-bottom: 1px solid #e8e8e8; }
.filter-panel-list { max-height: 200px; overflow-y: auto; padding: 4px 12px; }
.filter-panel-item { padding: 2px 0; }
.filter-panel-footer { padding: 8px 12px; border-top: 1px solid #e8e8e8; display: flex; gap: 8px; }
.cond-rule-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.cond-new-rule { display: flex; align-items: center; gap: 8px; }
.cond-scale-row { display: flex; align-items: center; gap: 8px; }
.sheet-tabs { display: flex; align-items: center; gap: 2px; padding: 4px 8px; border-top: 1px solid #d0d3d8; background: #f8f9fa; min-height: 32px; }
.sheet-tab { padding: 4px 12px; font-size: 12px; cursor: pointer; border: 1px solid transparent; border-radius: 4px 4px 0 0; color: #555; user-select: none; }
.sheet-tab:hover { background: #e8e8e8; }
.sheet-tab.active { background: #fff; border-color: #d0d3d8; border-bottom-color: #fff; font-weight: 500; color: #1a73e8; }
.search-dialog { position: fixed; top: 60px; right: 20px; z-index: 2000; }
</style>
