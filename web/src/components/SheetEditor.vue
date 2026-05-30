<template>
  <div class="sheet-container" tabindex="0" ref="containerRef" @keydown="onGlobalKeydown">
    <!-- 公式栏 -->
    <div class="formula-bar">
      <div class="cell-ref">{{ currentCellRef }}</div>
      <button class="formula-fx-btn" @click="toggleFxPanel" :class="{active: showFxPanel}">fx</button>
      <div class="formula-input-wrap">
        <input class="formula-input" v-model="formulaValue" @input="onFormulaInput" @keydown="onFormulaKeydown" @focus="onFormulaFocus" @blur="onFormulaBlur"
          @keydown.enter="applyFormula" @keydown.escape="cancelFormula" placeholder="输入内容或公式..." />
        <!-- 函数自动补全 -->
        <div v-if="showFxPanel" class="fx-panel">
          <div class="fx-search">
            <input v-model="fxSearch" placeholder="搜索函数..." @keydown="onFxSearchKey" ref="fxSearchRef" />
          </div>
          <div class="fx-list">
            <div v-for="(fn, i) in filteredFunctions" :key="fn.name" class="fx-item" :class="{active: fxIndex === i}" @click="insertFunction(fn)" @mouseenter="fxIndex = i">
              <span class="fx-name">{{ fn.name }}</span>
              <span class="fx-desc">{{ fn.desc }}</span>
            </div>
            <div v-if="!filteredFunctions.length" class="fx-empty">没有匹配的函数</div>
          </div>
          <!-- 参数提示 -->
          <div v-if="fxHint" class="fx-hint">
            <strong>{{ fxHint.name }}</strong>({{ fxHint.args }})
            <p>{{ fxHint.desc }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- 工具栏（Excel Ribbon 两行布局） -->
    <div class="ribbon">
      <!-- 第一行：剪贴板 | 字体 | 对齐 -->
      <div class="ribbon-row">
        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <button class="rb-btn" :disabled="!canUndo" @click="undo" title="撤销 (Ctrl+Z)">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M3 8h10a3 3 0 010 6H8"/><path d="M6 5L3 8l3 3"/></svg>
            </button>
            <button class="rb-btn" :disabled="!canRedo" @click="redo" title="重做 (Ctrl+Y)">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M17 8H7a3 3 0 000 6h5"/><path d="M14 5l3 3-3 3"/></svg>
            </button>
          </div>
          <div class="ribbon-section-label">撤销</div>
        </div>
        <div class="rb-sep" />

        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <el-select size="small" v-model="cellFontFamily" @change="applyFontFamily" class="rb-select" style="width:108px">
              <el-option v-for="f in fontList" :key="f" :label="f" :value="f" :style="{fontFamily:f}" />
            </el-select>
            <el-select size="small" v-model="cellFontSize" @change="applyFontSize" class="rb-select" style="width:52px">
              <el-option v-for="s in fontSizes" :key="s" :label="s" :value="s" />
            </el-select>
          </div>
          <div class="ribbon-section-buttons">
            <button class="rb-btn" :class="{active:getMetaProp('bold')}" @click="toggleFormat('bold')" title="加粗 (Ctrl+B)" style="font-weight:800;font-size:14px">B</button>
            <button class="rb-btn" :class="{active:getMetaProp('italic')}" @click="toggleFormat('italic')" title="斜体 (Ctrl+I)" style="font-style:italic;font-family:serif;font-size:14px">I</button>
            <button class="rb-btn" :class="{active:getMetaProp('underline')}" @click="toggleFormat('underline')" title="下划线 (Ctrl+U)" style="text-decoration:underline;font-size:13px">U</button>
            <button class="rb-btn" :class="{active:getMetaProp('strike')}" @click="toggleFormat('strike')" title="删除线" style="text-decoration:line-through;font-size:12px">ab</button>
            <div class="rb-vsep" />
            <div class="color-btn-wrap" title="字体颜色">
              <button class="rb-btn" @click="($refs.textColorPicker as any)?.show()" style="font-weight:800;font-size:13px">A
                <span class="color-indicator" :style="{background: cellTextColor || '#FF0000'}" />
              </button>
              <el-color-picker ref="textColorPicker" v-model="cellTextColor" @change="applyTextColor" size="small" :predefine="['#000000','#444444','#888888','#FF0000','#FF6600','#FFCC00','#33CC33','#00B0F0','#3366FF','#9933FF','#CC00CC','#C00000']" class="hidden-picker" />
            </div>
            <div class="color-btn-wrap" title="填充颜色">
              <button class="rb-btn" @click="($refs.bgColorPicker as any)?.show()" style="padding:0">
                <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 14l4-10h1l4 10"/><path d="M4.5 11h5"/><rect x="13" y="4" width="4" height="6" rx="0.5" :fill="cellBgColor || '#FFFF00'" stroke="#888" stroke-width="0.8"/></svg>
              </button>
              <el-color-picker ref="bgColorPicker" v-model="cellBgColor" @change="applyBgColor" size="small" :predefine="['#FFFFFF','#FFFF00','#CCFFCC','#CCFFFF','#FFCCCC','#FFCCFF','#FFE0B2','#B3E5FC','#D1C4E9','#F8BBD0','#C8E6C9','#FFD700']" class="hidden-picker" />
            </div>
          </div>
          <div class="ribbon-section-label">字体</div>
        </div>
        <div class="rb-sep" />

        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <el-dropdown trigger="click" @command="applyBorder" title="边框">
              <button class="rb-btn">
                <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="3" y="3" width="14" height="14" rx="0.5"/><line x1="3" y1="10" x2="17" y2="10"/><line x1="10" y1="3" x2="10" y2="17"/></svg>
              </button>
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
            <button class="rb-btn" @click="increaseIndent" title="增加缩进">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><line x1="7" y1="4" x2="17" y2="4"/><line x1="7" y1="8" x2="17" y2="8"/><line x1="7" y1="12" x2="17" y2="12"/><line x1="7" y1="16" x2="17" y2="16"/><path d="M2 4v12"/><path d="M2 10l3-3M2 10l3 3"/></svg>
            </button>
            <button class="rb-btn" @click="decreaseIndent" title="减少缩进">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><line x1="7" y1="4" x2="17" y2="4"/><line x1="7" y1="8" x2="17" y2="8"/><line x1="7" y1="12" x2="17" y2="12"/><line x1="7" y1="16" x2="17" y2="16"/><path d="M5 4v12"/><path d="M5 10l-3-3M5 10l-3 3"/></svg>
            </button>
          </div>
          <div class="ribbon-section-label">边框</div>
        </div>
        <div class="rb-sep" />

        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <button class="rb-btn" :class="{active:getMetaProp('align')==='left'}" @click="setAlign('left')" title="左对齐">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><line x1="3" y1="5" x2="17" y2="5"/><line x1="3" y1="10" x2="13" y2="10"/><line x1="3" y1="15" x2="15" y2="15"/></svg>
            </button>
            <button class="rb-btn" :class="{active:getMetaProp('align')==='center'}" @click="setAlign('center')" title="居中对齐">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><line x1="3" y1="5" x2="17" y2="5"/><line x1="5" y1="10" x2="15" y2="10"/><line x1="4" y1="15" x2="16" y2="15"/></svg>
            </button>
            <button class="rb-btn" :class="{active:getMetaProp('align')==='right'}" @click="setAlign('right')" title="右对齐">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><line x1="3" y1="5" x2="17" y2="5"/><line x1="7" y1="10" x2="17" y2="10"/><line x1="5" y1="15" x2="17" y2="15"/></svg>
            </button>
            <button class="rb-btn" :class="{active:getMetaProp('wrap')}" @click="toggleWrap" title="自动换行">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M4 5h12M4 10h8M4 15h10"/><path d="M16 10v0" stroke-width="2"/></svg>
            </button>
            <button class="rb-btn" @click="toggleBorder('all')" title="所有边框">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="3" y="3" width="14" height="14"/><line x1="10" y1="3" x2="10" y2="17"/><line x1="3" y1="10" x2="17" y2="10"/></svg>
            </button>
            <button class="rb-btn" @click="toggleBorder('outer')" title="外边框">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="3" y="3" width="14" height="14" stroke-width="2"/></svg>
            </button>
            <button class="rb-btn" @click="toggleBorder('none')" title="清除边框">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="3" y="3" width="14" height="14" stroke-dasharray="2 2"/></svg>
            </button>
            <button class="rb-btn" @click="setRotate(-90)" title="竖排文字">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M10 17V3"/><path d="M7 6l3-3 3 3"/></svg>
            </button>
            <button class="rb-btn" @click="setRotate(-45)" title="倾斜45°">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M4 16L16 4"/></svg>
            </button>
            <button class="rb-btn" @click="setRotate(0)" title="正常角度">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 10h14"/><path d="M14 7l3 3-3 3"/></svg>
            </button>
            <div class="rb-vsep" />
            <button class="rb-btn" :class="{active:getMetaProp('valign')==='top'}" @click="setVAlign('top')" title="顶端对齐" style="font-size:10px">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><line x1="3" y1="3" x2="17" y2="3"/><rect x="5" y="5" width="10" height="4" rx="0.5"/></svg>
            </button>
            <button class="rb-btn" :class="{active:getMetaProp('valign')==='middle'||!getMetaProp('valign')}" @click="setVAlign('middle')" title="垂直居中" style="font-size:10px">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><line x1="3" y1="3" x2="17" y2="3"/><rect x="5" y="8" width="10" height="4" rx="0.5"/></svg>
            </button>
            <button class="rb-btn" :class="{active:getMetaProp('valign')==='bottom'}" @click="setVAlign('bottom')" title="底端对齐" style="font-size:10px">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><line x1="3" y1="3" x2="17" y2="3"/><rect x="5" y="11" width="10" height="4" rx="0.5"/></svg>
            </button>
          </div>
          <div class="ribbon-section-label">对齐方式</div>
        </div>
        <div class="rb-sep" />

        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <el-select size="small" v-model="currentColType" @change="setColType" class="rb-select" style="width:72px">
              <el-option label="常规" value="auto" />
              <el-option label="文本" value="text" />
              <el-option label="数字" value="number" />
              <el-option label="百分比" value="percent" />
              <el-option label="货币 ¥" value="currency" />
              <el-option label="日期" value="date" />
              <el-option label="科学" value="scientific" />
            </el-select>
          </div>
          <div class="ribbon-section-buttons">
            <button class="rb-btn" @click="changePrecision(-1)" title="减少小数位" style="font-size:11px">.0→</button>
            <button class="rb-btn" @click="changePrecision(1)" title="增加小数位" style="font-size:11px">.00→</button>
          </div>
          <div class="ribbon-section-label">数字</div>
        </div>
        <div class="rb-sep" />

        <!-- 样式 -->
        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <el-dropdown trigger="click" @command="applyStylePreset" title="单元格样式">
              <button class="rb-btn" style="flex-direction:column;gap:0;padding:1px 4px">
                <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.2" style="width:18px;height:18px"><rect x="2" y="2" width="16" height="7" fill="currentColor" opacity="0.15"/><rect x="2" y="11" width="16" height="7"/></svg>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-for="(_, name) in stylePresets" :key="name" :command="name">{{ name }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <button class="rb-btn" @click="showCondDialog = true" title="条件格式">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="2" y="2" width="16" height="16" rx="1"/><line x1="2" y1="7.5" x2="18" y2="7.5"/><line x1="2" y1="13" x2="18" y2="13"/><rect x="2" y="2" width="10" height="5.5" fill="currentColor" opacity="0.15"/></svg>
            </button>
          </div>
          <div class="ribbon-section-label">样式</div>
        </div>
        <div class="rb-sep" />

        <!-- 单元格 -->
        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <el-dropdown trigger="click" title="插入">
              <button class="rb-btn">
                <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="2" width="16" height="16" rx="1"/><line x1="10" y1="6" x2="10" y2="14"/><line x1="6" y1="10" x2="14" y2="10"/></svg>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="addRowAbove">上方插入行</el-dropdown-item>
                  <el-dropdown-item @click="addRowBelow">下方插入行</el-dropdown-item>
                  <el-dropdown-item @click="addColLeft" divided>左侧插入列</el-dropdown-item>
                  <el-dropdown-item @click="addColRight">右侧插入列</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <button class="rb-btn" @click="deleteRow" title="删除行">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="2" width="16" height="16" rx="1"/><line x1="6" y1="10" x2="14" y2="10"/></svg>
            </button>
            <button class="rb-btn" @click="toggleMerge" title="合并单元格">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="2" y="4" width="7" height="5" rx="0.5"/><rect x="11" y="4" width="7" height="5" rx="0.5"/><rect x="2" y="11" width="16" height="5" rx="0.5"/></svg>
            </button>
          </div>
          <div class="ribbon-section-label">单元格</div>
        </div>
        <div class="rb-sep" />

        <!-- 编辑 -->
        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <button class="rb-btn" :class="{active:showSearchDialog}" @click="showSearchDialog = !showSearchDialog" title="查找替换 (Ctrl+F)">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><circle cx="9" cy="9" r="5.5"/><line x1="13" y1="13" x2="17" y2="17"/></svg>
            </button>
            <el-dropdown trigger="click" @command="toggleFreeze" title="冻结窗格">
              <button class="rb-btn">
                <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="2" y="2" width="16" height="16" rx="0.5"/><line x1="2" y1="7" x2="18" y2="7"/><line x1="7" y1="2" x2="7" y2="18"/></svg>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="row">{{ freezeRows > 0 && freezeCols === 0 ? '✓ ' : '' }}冻结首行</el-dropdown-item>
                  <el-dropdown-item command="col">{{ freezeCols > 0 && freezeRows === 0 ? '✓ ' : '' }}冻结首列</el-dropdown-item>
                  <el-dropdown-item command="here">冻结到当前位置</el-dropdown-item>
                  <el-dropdown-item command="none" divided>取消冻结</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
          <div class="ribbon-section-label">编辑</div>
        </div>
        <div class="rb-sep" />

        <!-- 数据 -->
        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <button class="rb-btn" @click="openSplitColDialog" title="分列">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="3" width="3" height="14" rx="0.3"/><line x1="8" y1="5" x2="8" y2="15" stroke-dasharray="2 1.5"/><rect x="11" y="3" width="3" height="14" rx="0.3"/><rect x="16" y="3" width="3" height="14" rx="0.3"/></svg>
            </button>
            <button class="rb-btn" @click="removeDuplicates" title="删除重复项">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="3" width="16" height="14" rx="1"/><line x1="2" y1="8" x2="18" y2="8"/><line x1="2" y1="13" x2="18" y2="13"/><line x1="8" y1="5" x2="12" y2="17" stroke="red" stroke-width="1"/></svg>
            </button>
            <button class="rb-btn" @click="openPivotDialog" title="数据透视">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="2" y="2" width="8" height="8" fill="currentColor" opacity="0.15"/><rect x="10" y="2" width="8" height="8"/><rect x="2" y="10" width="8" height="8"/><rect x="10" y="10" width="8" height="8" fill="currentColor" opacity="0.15"/></svg>
            </button>
          </div>
          <div class="ribbon-section-label">数据</div>
        </div>
        <div class="rb-sep" />

        <!-- 图表/导出 -->
        <div class="ribbon-section">
          <div class="ribbon-section-buttons">
            <button class="rb-btn" :class="{active:showChart}" @click="showChart = !showChart" title="插入图表">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6"><rect x="2" y="11" width="3.5" height="7"/><rect x="8" y="6" width="3.5" height="12"/><rect x="14" y="3" width="3.5" height="15"/></svg>
            </button>
            <button class="rb-btn" @click="exportCSV" title="导出CSV">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M4 3h8l4 4v10a2 2 0 01-2 2H6a2 2 0 01-2-2V3z"/><path d="M12 3v4h4"/><line x1="7" y1="11" x2="13" y2="11"/><line x1="7" y1="14" x2="11" y2="14"/></svg>
            </button>
            <button class="rb-btn" @click="printSheet" title="打印">
              <svg class="rb-svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="4" y="7" width="12" height="8" rx="1"/><path d="M6 7V3h8v4"/><path d="M6 11h8"/><path d="M6 15v2h8v-2"/></svg>
            </button>
          </div>
          <div class="ribbon-section-label">图表</div>
        </div>
      </div>
    </div>

    <!-- Sheet Tab 右键菜单 -->
    <div v-if="tabMenu.show" class="ctx-menu" :style="{ left: tabMenu.x + 'px', top: tabMenu.y + 'px' }">
      <div class="ctx-item" @click="renameSheet(tabMenu.idx); tabMenu.show = false">重命名</div>
      <div class="ctx-item" @click="setTabColor('#1a73e8')">🔵 蓝色</div>
      <div class="ctx-item" @click="setTabColor('#34a853')">🟢 绿色</div>
      <div class="ctx-item" @click="setTabColor('#ea4335')">🔴 红色</div>
      <div class="ctx-item" @click="setTabColor('#fbbc04')">🟡 黄色</div>
      <div class="ctx-item" @click="setTabColor('#9334e6')">🟣 紫色</div>
      <div class="ctx-item" @click="setTabColor('')">无颜色</div>
    </div>

    <!-- 表格区域 -->
    <div class="grid-area" ref="scrollRef" @contextmenu.prevent="showContextMenu" @click="hideContextMenu">
      <table class="grid-table" ref="tableRef">
        <colgroup>
          <col style="width:46px" />
          <col v-for="(w, c) in colWidths" :key="c" :style="{ width: hiddenCols.has(c) ? '0px' : w + 'px' }" />
        </colgroup>
        <thead>
          <tr>
            <th class="corner-cell"></th>
            <th v-for="c in colCount" :key="c"
              v-show="!hiddenCols.has(c - 1)"
              class="col-hdr"
              :class="{ sel: isColSelected(c - 1), sorted: sortCol === c - 1, 'frozen-col-hdr': freezeRows > 0, 'drag-over': colDragOver === c - 1 }"
              @click="selectCol(c - 1)"
              @dblclick="autoFitCol(c - 1)"
              @mousedown="startColDrag(c - 1, $event)"
            >
              <div class="col-hdr-inner">
                <span class="col-letter">{{ colName(c - 1) }}</span>
                <span v-if="sortCol === c - 1" class="sort-arrow">{{ sortDir === 'asc' ? '▲' : '▼' }}</span>
                <span v-if="filterActiveCols.has(c - 1)" class="filter-dot">◆</span>
                <el-dropdown trigger="click" @command="(cmd: string) => handleColMenu(cmd, c - 1)" :hide-on-click="false" size="small">
                  <span class="hdr-menu" @click.stop>▾</span>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="sort-asc">↑ 升序排列</el-dropdown-item>
                      <el-dropdown-item command="sort-desc">↓ 降序排列</el-dropdown-item>
                      <el-dropdown-item command="sort-multi" divided>多列排序...</el-dropdown-item>
                      <el-dropdown-item command="sort-clear">取消排序</el-dropdown-item>
                      <el-dropdown-item command="filter" divided>{{ filterActiveCols.has(c - 1) ? '✓ ' : '' }}筛选此列</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
                <div class="col-resize" @mousedown.stop="startColResize(c - 1, $event)" @dblclick.stop="autoFitCol(c - 1)" />
              </div>
            </th>
          </tr>
        </thead>
        <tbody>
          <template v-for="(row, ri) in currentSheetRows" :key="ri">
            <tr v-show="!isRowFiltered(ri) && !hiddenRows.has(ri)" :style="{ height: (rowHeights[ri] || 26) + 'px' }" :class="{'wrap-row': hasRowWrap(ri)}">
              <td class="row-hdr" :class="{ sel: isRowSelected(ri), 'drag-over': rowDragOver === ri }" @click="selectRow(ri)" @mousedown.prevent="startRowDrag(ri, $event)">
                <div class="row-hdr-inner">
                  {{ ri + 1 }}
                  <div class="row-resize" @mousedown.stop="startRowResize(ri, $event)" />
                </div>
              </td>
              <td v-for="c in colCount" :key="c"
                v-show="!hiddenCols.has(c - 1)"
                class="cell"
                :class="{
                  sel: isSelected(ri, c - 1),
                  'sel-head': isSelectionHead(ri, c - 1),
                  editing: editingCell?.row === ri && editingCell?.col === c - 1,
                  'has-comment': getComment(ri, c - 1),
                  frozen: freezeRows > 0 && ri < freezeRows,
                }"
                :style="getCellStyle(ri, c - 1)"
                :colspan="getColspan(ri, c - 1)"
                :rowspan="getRowspan(ri, c - 1)"
                @mousedown.prevent="onCellMouseDown(ri, c - 1, $event)"
                @mouseenter="onCellMouseEnter(ri, c - 1, $event); showCellComment(ri, c - 1, $event)"
                @dblclick="startEdit(ri, c - 1)"
                @mouseleave="hideCellComment()"
              >
                <template v-if="editingCell?.row === ri && editingCell?.col === c - 1">
                  <input ref="editInput" class="cell-input" v-model="editingValue"
                    @keydown.enter.prevent="onEditEnter($event)"
                    @keydown.tab.prevent="finishEdit(); moveNext()"
                    @keydown.escape="cancelEdit"
                    @keydown="handleEditKey"
                    @input="updateAutocomplete" />
                  <!-- 输入联想 -->
                  <div v-if="acItems.length" class="ac-dropdown">
                    <div v-for="(item, i) in acItems" :key="i" class="ac-item" :class="{'ac-active': i === acIndex}" @mousedown.prevent="acceptAutocomplete(item)">{{ item }}</div>
                  </div>
                </template>
                <template v-else>
                  <span class="cell-val" :style="getCellTextStyle(ri, c - 1)">{{ getCellDisplay(ri, c - 1) }}</span>
                </template>
                <div v-if="isSelectionHead(ri, c - 1) && !editingCell" class="fill-h" @mousedown.stop="startFill($event)" @dblclick.stop="autoFillDown" />
                <div v-if="isSelectionHead(ri, c - 1) && !editingCell && selection && isMultiCellSelection"
                  class="move-h" @mousedown.stop="startMove($event)" />
                <div v-if="getComment(ri, c - 1)" class="comment-flag" />
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- 悬浮批注 -->
    <div v-if="hoverComment.show" class="comment-popup" :style="{ left: hoverComment.x + 'px', top: hoverComment.y + 'px' }">
      {{ hoverComment.text }}
    </div>

    <!-- 右键菜单 -->
    <div v-if="contextMenu.show" class="ctx-menu" :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }">
      <div class="ctx-item" @click="ctxCut"><span class="ctx-icon">✂</span> 剪切<span class="ctx-key">Ctrl+X</span></div>
      <div class="ctx-item" @click="ctxCopy"><span class="ctx-icon">📋</span> 复制<span class="ctx-key">Ctrl+C</span></div>
      <div class="ctx-item" @click="ctxPaste"><span class="ctx-icon">📄</span> 粘贴<span class="ctx-key">Ctrl+V</span></div>
      <div class="ctx-item" @click="ctxPasteValues">只粘贴值</div>
      <div class="ctx-item" @click="ctxPasteFormat">只粘贴格式</div>
      <div class="ctx-item" @click="ctxPasteTranspose">转置粘贴</div>
      <div class="ctx-sep" />
      <div class="ctx-item" @click="ctxInsertRowAbove">↑ 上方插入行</div>
      <div class="ctx-item" @click="ctxInsertRowBelow">↓ 下方插入行</div>
      <div class="ctx-item" @click="ctxInsertColLeft">← 左侧插入列</div>
      <div class="ctx-item" @click="ctxInsertColRight">→ 右侧插入列</div>
      <div class="ctx-sep" />
      <div class="ctx-item" @click="ctxDeleteRow">🗑 删除行</div>
      <div class="ctx-item" @click="ctxDeleteCol">🗑 删除列</div>
      <div class="ctx-item" @click="ctxHideRows">隐藏行</div>
      <div class="ctx-item" @click="ctxHideCols">隐藏列</div>
      <div class="ctx-item" @click="ctxUnhideAll">显示所有隐藏</div>
      <div class="ctx-sep" />
      <div class="ctx-item" @click="ctxClearCells">清空内容</div>
      <div class="ctx-item" @click="ctxMergeToggle">{{ hasMerge ? '取消合并' : '合并单元格' }}</div>
      <div class="ctx-sep" />
      <div class="ctx-item" @click="ctxAddComment">💬 {{ getComment(ctxRow, ctxCol) ? '编辑批注' : '添加批注' }}</div>
      <div class="ctx-item" @click="ctxSetLink">🔗 {{ getCellMeta(ctxRow, ctxCol).link ? '编辑链接' : '插入链接' }}</div>
      <div v-if="getComment(ctxRow, ctxCol)" class="ctx-item" @click="ctxDeleteComment">🗑 删除批注</div>
      <div class="ctx-sep" />
      <div class="ctx-item" @click="ctxSetValidation">📝 数据验证</div>
      <div class="ctx-item" @click="ctxToggleLock">{{ getCellMeta(ctxRow, ctxCol).locked ? '🔓 解锁单元格' : '🔒 锁定单元格' }}</div>
      <div class="ctx-item" @click="ctxToggleProtect">{{ sheet.protected ? '🔓 取消保护' : '🔐 保护工作表' }}</div>
      <div class="ctx-item" @click="ctxGroupRows">📁 分组折叠</div>
    </div>

    <!-- 图表面板 -->
    <div v-if="showChart" class="chart-panel">
      <div class="chart-bar">
        <el-select v-model="chartType" size="small" style="width:100px">
          <el-option label="柱状图" value="bar" /><el-option label="折线图" value="line" />
          <el-option label="饼图" value="pie" /><el-option label="散点图" value="scatter" />
          <el-option label="面积图" value="area" />
        </el-select>
        <el-select v-model="chartDataRange" size="small" style="width:120px">
          <el-option label="当前列" value="col" /><el-option label="选中区域" value="selection" />
          <el-option label="全部" value="all" />
        </el-select>
        <input v-model="chartTitle" placeholder="图表标题" class="chart-title" />
        <button class="tb-btn" @click="exportChart">导出PNG</button>
        <button class="tb-btn" @click="showChart = false">✕</button>
      </div>
      <div class="chart-canvas-wrap">
        <canvas ref="chartCanvas" width="700" height="360" @mousemove="onChartHover" @mouseleave="chartTooltip.show = false" />
        <div v-if="chartTooltip.show" class="chart-tip" :style="{ left: chartTooltip.x + 'px', top: chartTooltip.y + 'px' }">{{ chartTooltip.text }}</div>
      </div>
    </div>

    <!-- 条件格式对话框 -->
    <el-dialog v-model="showCondDialog" title="条件格式" width="480px">
      <el-tabs>
        <el-tab-pane label="规则">
          <div v-for="(rule, i) in condRules" :key="i" style="display:flex;align-items:center;gap:8px;padding:4px 0">
            <span style="flex:1">当 {{ rule.condition }} {{ rule.value }} 时</span>
            <el-color-picker v-model="rule.bgColor" size="small" />
            <el-button size="small" text @click="condRules.splice(i, 1)">删除</el-button>
          </div>
          <el-divider />
          <div style="display:flex;align-items:center;gap:8px">
            <el-select v-model="newCond.condition" size="small" style="width:100px">
              <el-option label="大于" value=">" /><el-option label="小于" value="<" />
              <el-option label="等于" value="=" /><el-option label="不等于" value="!=" />
              <el-option label="包含" value="contains" />
            </el-select>
            <el-input v-model="newCond.value" size="small" style="width:80px" placeholder="值" />
            <el-color-picker v-model="newCond.bgColor" size="small" />
            <el-button size="small" type="primary" @click="addCondRule">添加</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="色阶">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:12px">
            <span>2色：</span><el-color-picker v-model="condScale2Min" size="small" /><span>→</span><el-color-picker v-model="condScale2Max" size="small" />
            <el-button size="small" type="primary" @click="applyCondScale2">应用</el-button>
          </div>
          <div style="display:flex;align-items:center;gap:8px">
            <span>3色：</span><el-color-picker v-model="condScale3Min" size="small" /><span>→</span><el-color-picker v-model="condScale3Mid" size="small" /><span>→</span><el-color-picker v-model="condScale3Max" size="small" />
            <el-button size="small" type="primary" @click="applyCondScale">应用</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="数据条">
          <div style="display:flex;align-items:center;gap:8px">
            <span>颜色：</span><el-color-picker v-model="condDataBarColor" size="small" />
            <el-button size="small" type="primary" @click="applyDataBar">应用到选区</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="图标集">
          <div style="display:flex;gap:8px;flex-wrap:wrap">
            <el-button size="small" @click="applyIconSet('arrows')">🟢↑ 🟡→ 🔴↓ 箭头</el-button>
            <el-button size="small" @click="applyIconSet('flags')">🚩 旗帜</el-button>
            <el-button size="small" @click="applyIconSet('traffic')">🟢🟡🔴 红绿灯</el-button>
          </div>
          <div style="color:#999;font-size:12px;margin-top:8px">根据数值大小自动分配图标，应用到当前选区</div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- 查找替换 -->
    <el-dialog v-model="showSearchDialog" title="查找和替换" width="400px" :close-on-click-modal="false">
      <el-input v-model="searchText" placeholder="查找内容" size="small" style="margin-bottom:8px" @keydown.enter="findNext">
        <template #append><el-button @click="findNext" size="small">查找下一个</el-button></template>
      </el-input>
      <el-input v-model="replaceText" placeholder="替换为" size="small">
        <template #append>
          <el-button @click="replaceOne" size="small">替换</el-button>
          <el-button @click="replaceAll" size="small">全部替换</el-button>
        </template>
      </el-input>
      <div v-if="searchResult" style="color:#999;font-size:12px;margin-top:4px">{{ searchResult }}</div>
    </el-dialog>

    <!-- 批注编辑 -->
    <el-dialog v-model="showCommentDialog" title="批注" width="360px">
      <el-input v-model="commentText" type="textarea" :rows="4" placeholder="批注内容..." />
      <template #footer>
        <el-button size="small" @click="showCommentDialog = false">取消</el-button>
        <el-button size="small" type="primary" @click="saveComment">保存</el-button>
      </template>
    </el-dialog>

    <!-- 链接编辑 -->
    <el-dialog v-model="showLinkDialog" title="插入链接" width="400px">
      <el-input v-model="linkText" placeholder="https://..." />
      <template #footer>
        <el-button size="small" @click="showLinkDialog = false">取消</el-button>
        <el-button size="small" @click="removeLink" type="danger">删除链接</el-button>
        <el-button size="small" type="primary" @click="saveLink">保存</el-button>
      </template>
    </el-dialog>

    <!-- 数据验证 -->
    <el-dialog v-model="showValidationDialog" title="数据验证" width="400px">
      <el-select v-model="validationType" size="small" style="width:100%;margin-bottom:8px">
        <el-option label="无" value="none" /><el-option label="下拉列表" value="list" /><el-option label="数字范围" value="number" />
      </el-select>
      <el-input v-if="validationType === 'list'" v-model="validationOptions" size="small" placeholder="选项,逗号分隔" />
      <div v-if="validationType === 'number'" style="display:flex;gap:8px">
        <el-input-number v-model="validationMin" size="small" placeholder="最小" />
        <el-input-number v-model="validationMax" size="small" placeholder="最大" />
      </div>
      <template #footer>
        <el-button size="small" @click="showValidationDialog = false">取消</el-button>
        <el-button size="small" type="primary" @click="saveValidation">保存</el-button>
      </template>
    </el-dialog>

    <!-- 筛选面板 -->
    <div v-if="showFilterPanel" class="filter-panel" :style="{ left: '200px', top: '200px' }">
      <div style="padding:6px 10px;border-bottom:1px solid #e8e8e8">
        <el-checkbox v-model="filterSelectAll" @change="toggleFilterAll">全选</el-checkbox>
      </div>
      <div style="max-height:180px;overflow-y:auto;padding:4px 10px">
        <div v-for="val in filterUniqueValues" :key="val" style="padding:1px 0">
          <el-checkbox :model-value="filterSelectedValues.has(val)" @change="(v: boolean) => toggleFilterValue(val, v)">{{ val || '(空)' }}</el-checkbox>
        </div>
      </div>
      <div style="padding:6px 10px;border-top:1px solid #e8e8e8;display:flex;gap:6px">
        <el-button size="small" @click="applyFilter(true)">确定</el-button>
        <el-button size="small" @click="applyFilter(false)">取消</el-button>
      </div>
    </div>

    <!-- 分列对话框 -->
    <el-dialog v-model="showSplitColDialog" title="分列" width="440px">
      <div style="margin-bottom:12px;color:#666">将选定列的内容拆分为多列</div>
      <div style="margin-bottom:12px">
        <el-radio-group v-model="splitMode" size="small">
          <el-radio-button value="delimiter">分隔符号</el-radio-button>
          <el-radio-button value="fixed">固定宽度</el-radio-button>
        </el-radio-group>
      </div>
      <div v-if="splitMode === 'delimiter'" style="margin-bottom:12px;display:flex;align-items:center;gap:8px">
        <span>分隔符：</span>
        <el-select v-model="splitDelimiter" size="small" style="width:120px">
          <el-option label="Tab" value="\t" />
          <el-option label="分号 (;)" value=";" />
          <el-option label="逗号 (,)" value="," />
          <el-option label="空格" value=" " />
          <el-option label="竖线 (|)" value="|" />
          <el-option label="自定义" value="__custom__" />
        </el-select>
        <el-input v-if="splitDelimiter === '__custom__'" v-model="splitCustomDelim" size="small" style="width:80px" placeholder="输入分隔符" />
        <el-checkbox v-model="splitConsecutive" size="small" style="margin-left:8px">连续分隔符视为单个</el-checkbox>
      </div>
      <div v-if="splitMode === 'fixed'" style="margin-bottom:12px">
        <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
          <span>位置列表（用逗号分隔）：</span>
        </div>
        <el-input v-model="splitFixedPositions" size="small" placeholder="例如: 3,8,15" />
        <div style="color:#999;font-size:12px;margin-top:4px">从第几个字符处切分，多列用逗号隔开</div>
      </div>
      <div v-if="splitPreview.length" style="margin-bottom:12px">
        <div style="font-size:12px;color:#999;margin-bottom:4px">预览：</div>
        <table style="border-collapse:collapse;font-size:12px;width:100%">
          <tr v-for="(row, i) in splitPreview" :key="i" style="border-bottom:1px solid #eee">
            <td v-for="(cell, j) in row" :key="j" style="padding:2px 8px;border:1px solid #ddd">{{ cell }}</td>
          </tr>
        </table>
        <div style="font-size:11px;color:#999;margin-top:2px">（仅显示前 5 行）</div>
      </div>
      <div style="color:#999;font-size:12px">源列：{{ colName(splitCol) }}</div>
      <template #footer>
        <el-button size="small" @click="showSplitColDialog = false">取消</el-button>
        <el-button size="small" type="primary" @click="doSplitCol" :disabled="!splitPreview.length">确定</el-button>
      </template>
    </el-dialog>

    <!-- 多列排序对话框 -->
    <el-dialog v-model="showMultiSortDialog" title="多列排序" width="460px">
      <div v-for="(rule, i) in multiSortRules" :key="i" style="display:flex;gap:8px;margin-bottom:8px;align-items:center">
        <el-select v-model="rule.col" size="small" placeholder="选择列" style="width:160px">
          <el-option v-for="c in colCount" :key="c" :label="colName(c - 1)" :value="c - 1" />
        </el-select>
        <el-select v-model="rule.dir" size="small" style="width:120px">
          <el-option label="升序 ↑" value="asc" />
          <el-option label="降序 ↓" value="desc" />
        </el-select>
        <el-button size="small" text type="danger" @click="multiSortRules.splice(i, 1)" :disabled="multiSortRules.length <= 1">✕</el-button>
      </div>
      <el-button size="small" @click="multiSortRules.push({ col: 0, dir: 'asc' })">+ 添加级别</el-button>
      <template #footer>
        <el-button size="small" @click="showMultiSortDialog = false">取消</el-button>
        <el-button size="small" type="primary" @click="doMultiSort">排序</el-button>
      </template>
    </el-dialog>

    <!-- 数据透视对话框 -->
    <el-dialog v-model="showPivotDialog" title="数据透视" width="500px">
      <div style="display:flex;gap:12px;margin-bottom:12px">
        <div style="flex:1">
          <div style="font-size:12px;color:#999;margin-bottom:4px">分组列</div>
          <el-select v-model="pivotGroupCol" size="small" style="width:100%">
            <el-option v-for="c in colCount" :key="c" :label="colName(c-1)" :value="c-1" />
          </el-select>
        </div>
        <div style="flex:1">
          <div style="font-size:12px;color:#999;margin-bottom:4px">值列</div>
          <el-select v-model="pivotValueCol" size="small" style="width:100%">
            <el-option v-for="c in colCount" :key="c" :label="colName(c-1)" :value="c-1" />
          </el-select>
        </div>
      </div>
      <el-button size="small" type="primary" @click="doPivot" style="margin-bottom:12px">生成透视表</el-button>
      <div v-if="pivotResult.length" style="max-height:300px;overflow:auto">
        <table style="width:100%;border-collapse:collapse;font-size:13px">
          <thead><tr style="background:#f0f0f0">
            <th style="padding:6px;border:1px solid #ddd">分组</th>
            <th style="padding:6px;border:1px solid #ddd">求和</th>
            <th style="padding:6px;border:1px solid #ddd">计数</th>
            <th style="padding:6px;border:1px solid #ddd">平均值</th>
          </tr></thead>
          <tbody>
            <tr v-for="row in pivotResult" :key="row.group">
              <td style="padding:6px;border:1px solid #ddd">{{ row.group }}</td>
              <td style="padding:6px;border:1px solid #ddd">{{ row.sum }}</td>
              <td style="padding:6px;border:1px solid #ddd">{{ row.count }}</td>
              <td style="padding:6px;border:1px solid #ddd">{{ row.avg }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #footer>
        <el-button size="small" @click="showPivotDialog = false">关闭</el-button>
        <el-button v-if="pivotResult.length" size="small" type="primary" @click="insertPivotResult">插入到表格</el-button>
      </template>
    </el-dialog>

    <!-- 底部Sheet标签栏 -->
    <div class="sheet-tabs">
      <div class="tabs-scroll">
        <div v-for="(sh, si) in sheets" :key="si"
          class="tab" :class="{ active: activeSheet === si }"
          :style="{ borderTopColor: sh.tabColor || undefined }"
          @click="switchSheet(si)" @dblclick="renameSheet(si)" @contextmenu.prevent="showTabMenu($event, si)">
          <span v-if="sh.tabColor" class="tab-dot" :style="{ background: sh.tabColor }"></span>
          {{ sh.name }}
          <span v-if="sheets.length > 1" class="tab-x" @click.stop="deleteSheet(si)">×</span>
        </div>
        <button class="tab-add" @click="addSheet" title="新建工作表">+</button>
      </div>
      <div class="tabs-info">{{ currentSheetRows.length }}×{{ colCount }}</div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'


const props = defineProps<{ initialData?: string }>()
const emit = defineEmits<{ (e: 'change', data: string): void }>()

interface CellMeta {
  bold?: boolean; italic?: boolean; underline?: boolean; strike?: boolean
  color?: string; bgColor?: string; align?: string; valign?: string; wrap?: boolean
  fontFamily?: string; fontSize?: number; precision?: number; indent?: number
  border?: { top?: boolean; right?: boolean; bottom?: boolean; left?: boolean }
  comment?: string; locked?: boolean; link?: string; rotate?: number
  validation?: { type: 'list'|'number'|'text'; options?: string; min?: number; max?: number }
}
interface SheetData {
  name: string; rows: string[][]; colCount: number
  colWidths: number[]; colTypes: string[]; rowHeights: number[]
  cellMeta: Record<string, CellMeta>
  merges: { row: number; col: number; rowspan: number; colspan: number }[]
  frozenRows: number; frozenCols: number; protected?: boolean
  hiddenRows: Set<number>; hiddenCols: Set<number>
  groups: { type: 'row'|'col'; start: number; end: number; collapsed: boolean }[]
  tabColor?: string
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
    frozenRows: 0, frozenCols: 0, hiddenRows: new Set(), hiddenCols: new Set(), groups: [] }
}

const sheet = computed(() => sheets.value[activeSheet.value] || makeSheet('Sheet1'))
const rows = computed(() => sheet.value.rows)
const colCount = computed(() => sheet.value.colCount)
const colWidths = computed(() => sheet.value.colWidths)
const colTypes = computed(() => sheet.value.colTypes)
const rowHeights = computed(() => sheet.value.rowHeights)
const freezeRows = computed(() => sheet.value.frozenRows)
const freezeCols = computed(() => sheet.value.frozenCols)
const hiddenRows = computed(() => sheet.value.hiddenRows || new Set<number>())
const hiddenCols = computed(() => sheet.value.hiddenCols || new Set<number>())
const currentSheetRows = computed(() => rows.value)

const selection = ref<{ startRow: number; startCol: number; endRow: number; endCol: number } | null>(null)
const editingCell = ref<{ row: number; col: number } | null>(null)
const editingValue = ref('')
const formulaValue = ref('')

const scrollRef = ref<HTMLElement>()
const containerRef = ref<HTMLElement>()
const chartCanvas = ref<HTMLCanvasElement>()
const editInput = ref<HTMLInputElement[]>()

const showChart = ref(false)
const showCondDialog = ref(false)
const showSearchDialog = ref(false)
const showFilterPanel = ref(false)
const showCommentDialog = ref(false)
const showLinkDialog = ref(false)
const linkText = ref('')
let linkRow = 0, linkCol = 0
const showValidationDialog = ref(false)

const chartType = ref('bar')
const chartDataRange = ref('col')
const chartTitle = ref('')
const chartTooltip = reactive({ show: false, x: 0, y: 0, text: '' })

const contextMenu = reactive({ show: false, x: 0, y: 0 })
const tabMenu = reactive({ show: false, x: 0, y: 0, idx: 0 })
let ctxRow = 0, ctxCol = 0

const sortCol = ref(-1)
const sortDir = ref<'asc'|'desc'>('asc')
const showMultiSortDialog = ref(false)
const multiSortRules = reactive<{ col: number; dir: 'asc' | 'desc' }[]>([{ col: 0, dir: 'asc' }])
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

const cellTextColor = ref<string|null>(null)
const cellBgColor = ref<string|null>(null)
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
    const oldRows = d.rows || []
    const cols = d.cols || d.colCount || 26
    // 旧数据行数为0时用默认50行
    const rowCount = oldRows.length > 0 ? oldRows.length : 50
    const rows = oldRows.length > 0 ? oldRows : Array.from({ length: 50 }, () => Array(cols).fill(''))
    const s: SheetData = {
      name: 'Sheet1', rows, colCount: cols,
      colWidths: d.colWidths || Array(cols).fill(120),
      colTypes: d.colTypes || Array(cols).fill('auto'),
      rowHeights: d.rowHeights || Array(rowCount).fill(26),
      cellMeta: {}, merges: d.merges || [],
      frozenRows: d.freezeRows || 0, frozenCols: d.freezeCols || 0,
      hiddenRows: new Set(Array.isArray(d.hiddenRows) ? d.hiddenRows : []), hiddenCols: new Set(Array.isArray(d.hiddenCols) ? d.hiddenCols : []),
      protected: d.protected || false, groups: d.groups || []
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
  // Ctrl+点击链接时打开
  if (e?.ctrlKey || e?.metaKey) { const link = getCellMeta(r, c).link; if (link) { window.open(link, '_blank'); return } }
  if (editingCell.value) finishEdit()
  if (e?.shiftKey && selection.value) { selection.value.endRow = r; selection.value.endCol = c }
  else { selection.value = { startRow: r, startCol: c, endRow: r, endCol: c } }
  ctxRow = r; ctxCol = c; editingCell.value = null; updateFormula(); updateToolbarState()
  containerRef.value?.focus()
}
function selectRow(ri: number) { if (editingCell.value) finishEdit(); selection.value = { startRow: ri, startCol: 0, endRow: ri, endCol: colCount.value - 1 }; updateFormula() }
function selectCol(ci: number) { if (editingCell.value) finishEdit(); selection.value = { startRow: 0, startCol: ci, endRow: rows.value.length - 1, endCol: ci }; updateFormula() }

// ── 行拖拽排序 ──
const rowDragOver = ref(-1)
const colDragOver = ref(-1)
function startRowDrag(ri: number, e: MouseEvent) {
  if (e.button !== 0) return
  selectRow(ri)
  const startY = e.clientY, startRi = ri
  const mv = (ev: MouseEvent) => {
    // 根据鼠标位置计算目标行
    const tbody = containerRef.value?.querySelector('tbody')
    if (!tbody) return
    const rows_ = tbody.querySelectorAll('tr')
    for (let i = 0; i < rows_.length; i++) {
      const rect = rows_[i].getBoundingClientRect()
      if (ev.clientY >= rect.top && ev.clientY <= rect.bottom) { rowDragOver.value = i; break }
    }
  }
  const up = () => {
    document.removeEventListener('mousemove', mv); document.removeEventListener('mouseup', up)
    const target = rowDragOver.value
    rowDragOver.value = -1
    if (target < 0 || target === startRi) return
    pushUndo()
    // 移动行
    const movedRows = rows.value.splice(startRi, 1)
    rows.value.splice(target, 0, ...movedRows)
    const movedHeights = rowHeights.value.splice(startRi, 1)
    rowHeights.value.splice(target, 0, ...movedHeights)
    selection.value = { startRow: target, startCol: 0, endRow: target, endCol: colCount.value - 1 }
    emitChange()
  }
  document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up)
}
// ── 列拖拽排序 ──
function startColDrag(ci: number, e: MouseEvent) {
  // 不拦截 dropdown 按钮
  if ((e.target as HTMLElement).closest('.hdr-menu, .el-dropdown')) return
  if (e.button !== 0) return
  selectCol(ci)
  const startX = e.clientX, startCi = ci
  const mv = (ev: MouseEvent) => {
    const thead = containerRef.value?.querySelector('thead')
    if (!thead) return
    const ths = thead.querySelectorAll('th')
    for (let i = 1; i < ths.length; i++) { // skip corner
      const rect = ths[i].getBoundingClientRect()
      if (ev.clientX >= rect.left && ev.clientX <= rect.right) { colDragOver.value = i - 1; break }
    }
  }
  const up = () => {
    document.removeEventListener('mousemove', mv); document.removeEventListener('mouseup', up)
    const target = colDragOver.value
    colDragOver.value = -1
    if (target < 0 || target === startCi) return
    pushUndo()
    // 移动列：数据 + 宽度 + 类型
    rows.value.forEach(r => { const v = r.splice(startCi, 1)[0]; r.splice(target, 0, v) })
    const w = colWidths.value.splice(startCi, 1)[0]; colWidths.value.splice(target, 0, w)
    const t = colTypes.value.splice(startCi, 1)[0]; colTypes.value.splice(target, 0, t)
    selection.value = { startRow: 0, startCol: target, endRow: rows.value.length - 1, endCol: target }
    emitChange()
  }
  document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up)
}
let isDragging = false
let formulaRangeMode = false // 公式范围选择模式
let formulaInsertPos = { start: 0, end: 0 } // 插入位置

function isFormulaRangeEditing(): boolean {
  if (!formulaValue.value.startsWith('=')) return false
  // 检查公式栏是否聚焦或正在输入
  return formulaRangeMode
}

function onCellMouseDown(r: number, c: number, e: MouseEvent) {
  if (e.button !== 0) return
  // 公式范围选择模式：拖选单元格直接填入范围引用
  if (formulaRangeMode) {
    const parts = getFormulaInsertParts()
    formulaInsertPos = parts
    const ref = colName(c) + (r + 1)
    formulaValue.value = parts.start + ref
    isDragging = true
    document.addEventListener('mouseup', stopDrag, { once: true })
    return
  }
  if (e.shiftKey && selection.value) {
    selection.value.endRow = r
    selection.value.endCol = c
  } else {
    selectCell(r, c, e)
  }
  isDragging = true
  document.addEventListener('mouseup', stopDrag, { once: true })
}

function onCellMouseEnter(r: number, c: number, _e: MouseEvent) {
  if (!isDragging) return
  // 公式范围模式：扩展范围引用
  if (formulaRangeMode) {
    // 找到第一个被选中的起始单元格（上一次 mousedown 的）
    const startRef = formulaValue.value.match(/([A-Z]+\d+)$/)?.[1]
    if (startRef) {
      const m = startRef.match(/^([A-Z]+)(\d+)$/)
      if (m) {
        const sc = colIndex(m[1]), sr = parseInt(m[2]) - 1
        const endRef = colName(c) + (r + 1)
        const rangeStr = colName(Math.min(sc, c)) + (Math.min(sr, r) + 1) + ':' + colName(Math.max(sc, c)) + (Math.max(sr, r) + 1)
        // 替换末尾的引用为范围
        formulaValue.value = formulaValue.value.replace(/([A-Z]+\d+)$/, rangeStr)
      }
    }
    return
  }
  if (!selection.value) return
  selection.value.endRow = r
  selection.value.endCol = c
}

function stopDrag() {
  isDragging = false
  // 公式范围模式：补上后缀（如 ) ）并退出
  if (formulaRangeMode && formulaInsertPos.end) {
    const val = formulaValue.value
    if (!val.endsWith(formulaInsertPos.end)) formulaValue.value = val + formulaInsertPos.end
  }
  if (formulaRangeMode) {
    formulaRangeMode = false
    // 自动应用公式
    applyFormula()
  }
}
function updateFormula() { if (!selection.value) return; formulaValue.value = rows.value[selection.value.startRow]?.[selection.value.startCol] || '' }
function updateToolbarState() {
  if (!selection.value) return
  const m = getCellMeta(selection.value.startRow, selection.value.startCol)
  cellTextColor.value = m.color || null; cellBgColor.value = m.bgColor || null
  cellFontFamily.value = m.fontFamily || 'Arial'; cellFontSize.value = m.fontSize || 13
  currentColType.value = colTypes.value[selection.value.startCol] || 'auto'
}
function moveNext() { if (!selection.value) return; const nc = selection.value.startCol + 1; if (nc >= colCount.value) selectCell(selection.value.startRow + 1, 0); else selectCell(selection.value.startRow, nc); updateFormula() }

// Editing
function startEdit(r: number, c: number) {
  if (editingCell.value?.row === r && editingCell.value?.col === c) return
  if (sheet.value.protected && getCellMeta(r, c).locked) return
  finishEdit(); editingCell.value = { row: r, col: c }; editingValue.value = rows.value[r]?.[c] || ''
  nextTick(() => { editInput.value?.[0]?.focus() })
}
function finishEdit() {
  acItems.value = []
  if (!editingCell.value) return
  const { row, col } = editingCell.value
  const oldVal = rows.value[row]?.[col] || ''
  if (editingValue.value !== oldVal) { pushUndo() }
  if (!rows.value[row]) rows.value[row] = Array(colCount.value).fill('')
  rows.value[row][col] = editingValue.value
  editingCell.value = null
  if (editingValue.value !== oldVal) emitChange()
  containerRef.value?.focus()
}
function cancelEdit() { editingCell.value = null; updateFormula() }
function onEditEnter(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && isMultiCellSelection.value) {
    // Ctrl+Enter：填入选区所有格
    if (!editingCell.value || !selection.value) return
    const val = editingValue.value
    pushUndo()
    const { startRow, startCol, endRow, endCol } = selection.value
    for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
      for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) rows.value[r][c] = val
    editingCell.value = null; emitChange()
  } else {
    finishEdit()
  }
}
function applyFormula() {
  if (editingCell.value) { editingCell.value = null } // 结束编辑
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) rows.value[r][c] = formulaValue.value
  emitChange()
}
function cancelFormula() { updateFormula(); showFxPanel.value = false }

// ── 函数辅助面板 ──
const showFxPanel = ref(false)
const fxSearch = ref('')
const fxIndex = ref(0)
const fxSearchRef = ref<HTMLInputElement | null>(null)
const acItems = ref<string[]>([])
const acIndex = ref(0)
const fxHint = ref<{name:string;args:string;desc:string}|null>(null)

interface FxDef { name: string; args: string; desc: string; template: string }
const FX_LIST: FxDef[] = [
  { name: 'SUM', args: 'num1, num2, ...', desc: '求和', template: 'SUM()' },
  { name: 'AVERAGE', args: 'num1, num2, ...', desc: '求平均值', template: 'AVERAGE()' },
  { name: 'COUNT', args: 'value1, value2, ...', desc: '计数（数字）', template: 'COUNT()' },
  { name: 'COUNTA', args: 'value1, value2, ...', desc: '计数（非空）', template: 'COUNTA()' },
  { name: 'MAX', args: 'num1, num2, ...', desc: '最大值', template: 'MAX()' },
  { name: 'MIN', args: 'num1, num2, ...', desc: '最小值', template: 'MIN()' },
  { name: 'IF', args: '条件, 真值, 假值', desc: '条件判断', template: 'IF(, , )' },
  { name: 'AND', args: '条件1, 条件2, ...', desc: '所有为真', template: 'AND(, )' },
  { name: 'OR', args: '条件1, 条件2, ...', desc: '任一为真', template: 'OR(, )' },
  { name: 'NOT', args: '条件', desc: '取反', template: 'NOT()' },
  { name: 'ROUND', args: '数值, 小数位', desc: '四舍五入', template: 'ROUND(, 2)' },
  { name: 'CEILING', args: '数值', desc: '向上取整', template: 'CEILING()' },
  { name: 'FLOOR', args: '数值', desc: '向下取整', template: 'FLOOR()' },
  { name: 'ABS', args: '数值', desc: '绝对值', template: 'ABS()' },
  { name: 'MOD', args: '被除数, 除数', desc: '取余', template: 'MOD(, )' },
  { name: 'POWER', args: '底数, 指数', desc: '幂运算', template: 'POWER(, 2)' },
  { name: 'SQRT', args: '数值', desc: '平方根', template: 'SQRT()' },
  { name: 'CONCATENATE', args: '文本1, 文本2, ...', desc: '拼接文本', template: 'CONCATENATE(, )' },
  { name: 'LEFT', args: '文本, 字符数', desc: '取左侧字符', template: 'LEFT(, 1)' },
  { name: 'RIGHT', args: '文本, 字符数', desc: '取右侧字符', template: 'RIGHT(, 1)' },
  { name: 'MID', args: '文本, 起始位, 长度', desc: '截取文本', template: 'MID(, 1, 1)' },
  { name: 'LEN', args: '文本', desc: '文本长度', template: 'LEN()' },
  { name: 'UPPER', args: '文本', desc: '转大写', template: 'UPPER()' },
  { name: 'LOWER', args: '文本', desc: '转小写', template: 'LOWER()' },
  { name: 'TRIM', args: '文本', desc: '去除空格', template: 'TRIM()' },
  { name: 'SUBSTITUTE', args: '文本, 旧文本, 新文本', desc: '替换文本', template: 'SUBSTITUTE(, , )' },
  { name: 'VALUE', args: '文本', desc: '文本转数字', template: 'VALUE()' },
  { name: 'NOW', args: '', desc: '当前日期时间', template: 'NOW()' },
  { name: 'TODAY', args: '', desc: '当前日期', template: 'TODAY()' },
  { name: 'YEAR', args: '日期', desc: '提取年份', template: 'YEAR()' },
  { name: 'MONTH', args: '日期', desc: '提取月份', template: 'MONTH()' },
  { name: 'DAY', args: '日期', desc: '提取日', template: 'DAY()' },
  { name: 'DATEDIF', args: '日期1, 日期2, 单位', desc: '日期差', template: 'DATEDIF(, , "D")' },
  { name: 'VLOOKUP', args: '查找值, 范围, 列号', desc: '纵向查找', template: 'VLOOKUP(, , 2)' },
  { name: 'INDEX', args: '范围, 行号', desc: '按位置取值', template: 'INDEX(, 1)' },
  { name: 'MATCH', args: '查找值, 范围', desc: '查找位置', template: 'MATCH(, )' },
  { name: 'RAND', args: '', desc: '随机数 0~1', template: 'RAND()' },
  { name: 'RANDBETWEEN', args: '最小值, 最大值', desc: '随机整数', template: 'RANDBETWEEN(1, 100)' },
  { name: 'ISBLANK', args: '单元格', desc: '是否为空', template: 'ISBLANK()' },
  { name: 'ISNUMBER', args: '值', desc: '是否为数字', template: 'ISNUMBER()' },
]

const filteredFunctions = computed(() => {
  const q = fxSearch.value.toUpperCase()
  if (!q) return FX_LIST.slice(0, 15)
  return FX_LIST.filter(f => f.name.includes(q) || f.desc.includes(fxSearch.value)).slice(0, 15)
})

function toggleFxPanel() {
  showFxPanel.value = !showFxPanel.value
  if (showFxPanel.value) {
    fxSearch.value = ''
    fxIndex.value = 0
    if (!formulaValue.value.startsWith('=')) formulaValue.value = '='
    nextTick(() => fxSearchRef.value?.focus())
  }
}

function onFormulaInput() {
  updateFormulaRangeMode()
  if (formulaValue.value.startsWith('=')) {
    const after = formulaValue.value.slice(1)
    // 提取正在输入的函数名
    const m = after.match(/([A-Z]+)$/i)
    if (m && m[1].length >= 1) {
      fxSearch.value = m[1]
      showFxPanel.value = true
      fxIndex.value = 0
    } else {
      showFxPanel.value = false
    }
  } else {
    showFxPanel.value = false
    formulaRangeMode = false
  }
}

function updateFormulaRangeMode() {
  const val = formulaValue.value
  if (!val.startsWith('=')) { formulaRangeMode = false; return }
  const trimmed = val.replace(/\s+$/, '')
  const lastChar = trimmed[trimmed.length - 1]
  if (lastChar === '(' || lastChar === ',') {
    formulaRangeMode = true
  } else if (/\(\)$/.test(trimmed)) {
    // 空括号如 SUM()，替换末尾 ) 为范围引用
    formulaRangeMode = true
  } else if (/[A-Z]+\d+$/i.test(trimmed)) {
    formulaRangeMode = true
  } else {
    formulaRangeMode = false
  }
}

function onFormulaFocus() {
  updateFormulaRangeMode()
  if (formulaValue.value.startsWith('=')) {
    const after = formulaValue.value.slice(1)
    const m = after.match(/([A-Z]+)$/i)
    if (m) {
      fxSearch.value = m[1]
      showFxPanel.value = true
      fxIndex.value = 0
    }
  }
}

function onFormulaBlur() {
  // 拖选进行中不退出 range mode
  if (isDragging) {
    setTimeout(() => { if (!isDragging) formulaRangeMode = false }, 300)
    return
  }
  formulaRangeMode = false
}

// 当公式以 ( 或 , 结尾时，记录插入点用于范围选择
function getFormulaInsertParts(): { start: string; end: string } {
  const val = formulaValue.value
  if (!val.startsWith('=')) return { start: val, end: '' }
  const lastOpen = val.lastIndexOf('(')
  const lastComma = val.lastIndexOf(',')
  const splitAt = Math.max(lastOpen, lastComma)
  if (splitAt < 0) return { start: val, end: '' }
  const afterSplit = val.slice(splitAt + 1)
  // 空括号: afterSplit === ')'
  if (afterSplit === ')') return { start: val.slice(0, splitAt + 1), end: ')' }
  // 已有引用: A1 或 A1:B2
  const refMatch = afterSplit.match(/^([A-Z]+\d+(?::[A-Z]+\d+)?)?$/i)
  if (refMatch) return { start: val.slice(0, splitAt + 1), end: '' }
  return { start: val, end: '' }
}

function insertFunction(fn: FxDef) {
  const val = formulaValue.value
  if (!val.startsWith('=')) {
    formulaValue.value = '=' + fn.template
  } else {
    // 替换末尾正在输入的函数名
    const m = val.match(/([A-Z]+)$/i)
    if (m) {
      formulaValue.value = val.slice(0, val.length - m[1].length) + fn.template
    } else {
      formulaValue.value = val + fn.template
    }
  }
  showFxPanel.value = false
  fxHint.value = fn
  updateFormulaRangeMode()
}

function onFormulaKeydown(e: KeyboardEvent) {
  if (!showFxPanel.value) return
  if (e.key === 'ArrowDown') { e.preventDefault(); fxIndex.value = Math.min(fxIndex.value + 1, filteredFunctions.value.length - 1); updateFxHint() }
  else if (e.key === 'ArrowUp') { e.preventDefault(); fxIndex.value = Math.max(fxIndex.value - 1, 0); updateFxHint() }
  else if (e.key === 'Tab' || e.key === 'Enter') {
    if (filteredFunctions.value.length) { e.preventDefault(); insertFunction(filteredFunctions.value[fxIndex.value]) }
  }
  else if (e.key === 'Escape') { showFxPanel.value = false }
}

function onFxSearchKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') { e.preventDefault(); fxIndex.value = Math.min(fxIndex.value + 1, filteredFunctions.value.length - 1); updateFxHint() }
  else if (e.key === 'ArrowUp') { e.preventDefault(); fxIndex.value = Math.max(fxIndex.value - 1, 0); updateFxHint() }
  else if (e.key === 'Enter') { if (filteredFunctions.value.length) insertFunction(filteredFunctions.value[fxIndex.value]) }
  else if (e.key === 'Escape') { showFxPanel.value = false }
}

function updateFxHint() {
  const fn = filteredFunctions.value[fxIndex.value]
  fxHint.value = fn || null
}

function handleEditKey(e: KeyboardEvent) {
  if (acItems.value.length) {
    if (e.key === 'ArrowDown') { e.preventDefault(); acIndex.value = Math.min(acIndex.value + 1, acItems.value.length - 1); return }
    if (e.key === 'ArrowUp') { e.preventDefault(); acIndex.value = Math.max(acIndex.value - 1, 0); return }
    if (e.key === 'Enter' || e.key === 'Tab') { acceptAutocomplete(acItems.value[acIndex.value]); return }
  }
  if (e.key === 'ArrowUp') { e.preventDefault(); finishEdit(); if (selection.value && selection.value.startRow > 0) selectCell(selection.value.startRow - 1, selection.value.startCol) }
  if (e.key === 'ArrowDown') { e.preventDefault(); finishEdit(); if (selection.value) selectCell(Math.min(selection.value.startRow + 1, rows.value.length - 1), selection.value.startCol) }
}
function updateAutocomplete() {
  acItems.value = []; acIndex.value = 0
  if (!editingCell.value || !editingValue.value || editingValue.value.length < 1) return
  const c = editingCell.value.col, val = editingValue.value.toLowerCase()
  const seen = new Set<string>()
  for (let r = 0; r < rows.value.length; r++) {
    if (r === editingCell.value.row) continue
    const cv = rows.value[r]?.[c]
    if (cv && cv.toLowerCase().startsWith(val) && !seen.has(cv)) { seen.add(cv); acItems.value.push(cv) }
    if (acItems.value.length >= 8) break
  }
}
function acceptAutocomplete(val: string) {
  if (val) editingValue.value = val
  acItems.value = []; acIndex.value = 0
}

// Cell Display
function getCellDisplay(r: number, c: number): string {
  const raw = rows.value[r]?.[c] ?? ''
  if (raw.startsWith('=')) return computeFormula(raw)
  const t = colTypes.value[c] || 'auto'
  if (t === 'percent' && raw !== '') { const n = parseFloat(raw); return isNaN(n) ? raw : (n * 100).toFixed(2) + '%' }
  if (t === 'currency' && raw !== '') { const n = parseFloat(raw); return isNaN(n) ? raw : '¥' + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
  if (t === 'number' && raw !== '') { const n = parseFloat(raw); return isNaN(n) ? raw : n.toLocaleString() }
  if (t === 'scientific' && raw !== '') { const n = parseFloat(raw); return isNaN(n) ? raw : n.toExponential(2) }
  return raw
}
function hasRowWrap(r: number): boolean {
  for (let c = 0; c < colCount.value; c++) { if (getCellMeta(r, c).wrap) return true } return false
}

function getCellTextStyle(r: number, c: number): Record<string, string> {
  const m = getCellMeta(r, c); const s: Record<string, string> = {}
  if (m.bold) s.fontWeight = 'bold'; if (m.italic) s.fontStyle = 'italic'
  if (m.underline && m.strike) s.textDecoration = 'underline line-through'
  else if (m.underline) s.textDecoration = 'underline'; else if (m.strike) s.textDecoration = 'line-through'
  if (m.color) s.color = m.color; if (m.fontFamily) s.fontFamily = m.fontFamily
  if (m.fontSize) s.fontSize = m.fontSize + 'px'; if (m.align) s.textAlign = m.align
  if (m.wrap) { s.whiteSpace = 'normal'; s.wordBreak = 'break-all' }
  // 边框样式
  if (m.border) {
    const bStyle = '1px solid #333'
    if (m.border.top) s.borderTop = bStyle; if (m.border.right) s.borderRight = bStyle
    if (m.border.bottom) s.borderBottom = bStyle; if (m.border.left) s.borderLeft = bStyle
  }
  if (m.locked && sheet.value.protected) s.background = '#f0f0f0'
  if (m.link) { s.color = '#1a73e8'; s.textDecoration = (s.textDecoration ? s.textDecoration + ' ' : '') + 'underline'; s.cursor = 'pointer' }
  if (m.rotate) { s.transform = `rotate(${m.rotate}deg)`; s.transformOrigin = 'center center' }
  if ((m as any).indent) s.paddingLeft = ((m as any).indent * 16) + 'px'
  return s
}
function getCellStyle(r: number, c: number): Record<string, string> {
  const m = getCellMeta(r, c); const s: Record<string, string> = {}
  if (m.bgColor) s.background = m.bgColor
  if ((m as any).valign) s.verticalAlign = (m as any).valign as string
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
function applyTextColor(c: string | null) { applyToSelection('color', c || '') }
function applyBgColor(c: string | null) { applyToSelection('bgColor', c || '') }
function applyFontFamily(f: string) { applyToSelection('fontFamily', f) }
function applyFontSize(s: number) { applyToSelection('fontSize', s) }
function setAlign(a: string) { applyToSelection('align', a) }
function setVAlign(v: string) { applyToSelection('valign', v) }
function toggleWrap() { if (!selection.value) return; applyToSelection('wrap', !getMetaProp('wrap')) }
function toggleBorder(mode: 'all' | 'outer' | 'none') {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) {
    if (mode === 'none') { setCellMeta(r, c, { border: undefined }) }
    else if (mode === 'all') { setCellMeta(r, c, { border: { top: true, right: true, bottom: true, left: true } }) }
    else if (mode === 'outer') {
      const b: any = {}
      if (r === r1) b.top = true; if (r === r2) b.bottom = true
      if (c === c1) b.left = true; if (c === c2) b.right = true
      setCellMeta(r, c, { border: b })
    }
  }
  emitChange()
}
function setRotate(deg: number) { if (!selection.value) return; applyToSelection('rotate', deg) }
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
  else if (cmd === 'here') {
    if (selection.value) { sheet.value.frozenRows = selection.value.startRow; sheet.value.frozenCols = selection.value.startCol }
  }
  else { sheet.value.frozenRows = 0; sheet.value.frozenCols = 0 }; emitChange()
}

// Sort/Filter
function handleColMenu(cmd: string, col: number) {
  if (cmd === 'sort-asc') { sortCol.value = col; sortDir.value = 'asc'; sortRows() }
  else if (cmd === 'sort-desc') { sortCol.value = col; sortDir.value = 'desc'; sortRows() }
  else if (cmd === 'sort-multi') { multiSortRules.length = 0; multiSortRules.push({ col, dir: 'asc' }); showMultiSortDialog.value = true }
  else if (cmd === 'sort-clear') { sortCol.value = -1 }
  else if (cmd === 'filter') openFilter(col)
}
function sortRows() {
  if (sortCol.value < 0) return; pushUndo()
  const c = sortCol.value, d = sortDir.value === 'asc' ? 1 : -1
  rows.value.sort((a, b) => { const va = a[c] || '', vb = b[c] || ''; const na = parseFloat(va), nb = parseFloat(vb); if (!isNaN(na) && !isNaN(nb)) return (na - nb) * d; return va.localeCompare(vb) * d })
  emitChange()
}
function doMultiSort() {
  if (!multiSortRules.length) return; pushUndo()
  rows.value.sort((a, b) => {
    for (const rule of multiSortRules) {
      const d = rule.dir === 'asc' ? 1 : -1
      const va = a[rule.col] || '', vb = b[rule.col] || ''
      const na = parseFloat(va), nb = parseFloat(vb)
      let cmp: number
      if (!isNaN(na) && !isNaN(nb)) cmp = (na - nb) * d
      else cmp = va.localeCompare(vb) * d
      if (cmp !== 0) return cmp
    }
    return 0
  })
  sortCol.value = multiSortRules[0].col; sortDir.value = multiSortRules[0].dir
  showMultiSortDialog.value = false; emitChange()
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
// ── 填充柄逻辑 ──
function adjustFormulaRefs(formula: string, rowOffset: number, colOffset: number): string {
  // 替换公式中的相对引用 A1 → A(rowOffset+1) 等
  return formula.replace(/([A-Z]+)(\d+)/gi, (match, col, row) => {
    const r = parseInt(row) + rowOffset
    const c = colIndex(col) + colOffset
    return colName(c) + r
  })
}

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
      if (sv.startsWith('=')) {
        // 公式：调整引用
        const rowOff = r - srcR
        rows.value[r][c] = adjustFormulaRefs(sv, rowOff, 0)
      } else {
        const sn = parseFloat(sv)
        if (!isNaN(sn) && sv === String(sn)) rows.value[r][c] = String(sn + (r - srcR1))
        else { const wIdx = weekDays.indexOf(sv); if (wIdx >= 0) rows.value[r][c] = weekDays[(wIdx + r - srcR1) % 7]; else { const mIdx = months.indexOf(sv); if (mIdx >= 0) rows.value[r][c] = months[(mIdx + r - srcR1) % 12]; else rows.value[r][c] = sv } }
      }
    }
    emitChange()
  }
  document.addEventListener('mousemove', mv); document.addEventListener('mouseup', up)
}

// 双击填充柄：自动填充到相邻列最后一个非空行
function autoFillDown() {
  if (!selection.value) return
  const sr = selection.value.startRow, sc = selection.value.startCol, er = selection.value.endRow, ec = selection.value.endCol
  const srcR1 = Math.min(sr, er), srcR2 = Math.max(sr, er), srcC1 = Math.min(sc, ec), srcC2 = Math.max(sc, ec)
  // 找相邻列最后有数据的行
  let lastRow = srcR2
  for (let c = 0; c < colCount.value; c++) {
    if (c >= srcC1 && c <= srcC2) continue
    for (let r = rows.value.length - 1; r > lastRow; r--) {
      if (rows.value[r]?.[c] && rows.value[r][c] !== '') { lastRow = Math.max(lastRow, r); break }
    }
  }
  if (lastRow <= srcR2) return
  pushUndo()
  for (let r = srcR2 + 1; r <= lastRow; r++) for (let c = srcC1; c <= srcC2; c++) {
    const srcR = srcR1 + ((r - srcR1) % (srcR2 - srcR1 + 1))
    const sv = rows.value[srcR]?.[c] || ''
    if (sv.startsWith('=')) {
      rows.value[r][c] = adjustFormulaRefs(sv, r - srcR, 0)
    } else {
      const sn = parseFloat(sv)
      if (!isNaN(sn) && sv === String(sn)) rows.value[r][c] = String(sn + (r - srcR1))
      else { const wIdx = weekDays.indexOf(sv); if (wIdx >= 0) rows.value[r][c] = weekDays[(wIdx + r - srcR1) % 7]; else { const mIdx = months.indexOf(sv); if (mIdx >= 0) rows.value[r][c] = months[(mIdx + r - srcR1) % 12]; else rows.value[r][c] = sv } }
    }
  }
  selection.value = { startRow: srcR1, startCol: srcC1, endRow: lastRow, endCol: srcC2 }
  emitChange()
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
function ctxSetLink() { hideContextMenu(); linkRow = ctxRow; linkCol = ctxCol; linkText.value = getCellMeta(ctxRow, ctxCol).link || ''; showLinkDialog.value = true }
function saveLink() { pushUndo(); setCellMeta(linkRow, linkCol, { link: linkText.value || undefined }); showLinkDialog.value = false; emitChange() }
function removeLink() { pushUndo(); setCellMeta(linkRow, linkCol, { link: undefined }); showLinkDialog.value = false; emitChange() }

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
function ctxToggleLock() { hideContextMenu(); pushUndo(); const locked = !getCellMeta(ctxRow, ctxCol).locked; applyToSelection('locked', locked); emitChange() }
function ctxToggleProtect() { hideContextMenu(); sheet.value.protected = !sheet.value.protected; emitChange() }

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
function hideContextMenu() { contextMenu.show = false; tabMenu.show = false }
function showTabMenu(e: MouseEvent, idx: number) { tabMenu.show = true; tabMenu.x = e.clientX; tabMenu.y = e.clientY; tabMenu.idx = idx }
function setTabColor(color: string) { sheets.value[tabMenu.idx].tabColor = color || undefined; tabMenu.show = false; emitChange() }
function ctxCut() { hideContextMenu(); clipCut() }
function ctxCopy() { hideContextMenu(); clipCopy() }
function ctxPaste() { hideContextMenu(); clipPaste() }
function ctxPasteValues() { hideContextMenu(); if (!clipData || !selection.value) return; pushUndo(); const r0 = selection.value.startRow, c0 = selection.value.startCol; for (let r = 0; r < clipData.data.length; r++) for (let c = 0; c < clipData.data[r].length; c++) { if (!rows.value[r0 + r]) rows.value[r0 + r] = makeRow(colCount.value); rows.value[r0 + r][c0 + c] = clipData.data[r][c] }; emitChange() }
function ctxPasteFormat() { hideContextMenu(); if (!selection.value) return; pushUndo(); const { startRow, startCol, endRow, endCol } = selection.value; const src = clipData ? { r: 0, c: 0 } : null; if (!src) return; const r0 = selection.value.startRow, c0 = selection.value.startCol; for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++) for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) { const sr = r - r0, sc = c - c0; if (sr < clipData!.data.length && sc < clipData!.data[sr].length) { const srcMeta = getCellMeta(r0 + sr, c0 + sc); setCellMeta(r, c, { ...srcMeta }) } }; emitChange() }
function ctxPasteTranspose() { hideContextMenu(); if (!clipData || !selection.value) return; pushUndo(); const r0 = selection.value.startRow, c0 = selection.value.startCol; const transposed = clipData.data[0].map((_, c) => clipData.data.map(row => row[c])); for (let r = 0; r < transposed.length; r++) for (let c = 0; c < transposed[r].length; c++) { if (!rows.value[r0 + r]) rows.value[r0 + r] = makeRow(colCount.value); rows.value[r0 + r][c0 + c] = transposed[r][c] }; emitChange() }
function ctxInsertRowAbove() { hideContextMenu(); addRowAbove() }
function ctxInsertRowBelow() { hideContextMenu(); addRowBelow() }
function ctxInsertColLeft() { hideContextMenu(); addColLeft() }
function ctxInsertColRight() { hideContextMenu(); addColRight() }
function ctxDeleteRow() { hideContextMenu(); deleteRow() }
function ctxDeleteCol() { hideContextMenu(); deleteCol() }
function ctxHideRows() { hideContextMenu(); if (!selection.value) return; pushUndo(); const { startRow, endRow } = selection.value; for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++) hiddenRows.value.add(r); emitChange() }
function ctxHideCols() { hideContextMenu(); if (!selection.value) return; pushUndo(); const { startCol, endCol } = selection.value; for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) hiddenCols.value.add(c); emitChange() }
function ctxUnhideAll() { hideContextMenu(); pushUndo(); sheet.value.hiddenRows = new Set(); sheet.value.hiddenCols = new Set(); emitChange() }
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
    case 'SPARKLINE': { const data = numArray(args[0]); return computeSparkline(data, String(parsed[1] || 'line')) }
    default: return '#NAME?'
  }
}
function evalArg(a: string): any {
  a = a.trim()
  if ((a.startsWith('"') && a.endsWith('"')) || (a.startsWith("'") && a.endsWith("'"))) return a.slice(1, -1)
  const crossRef = a.match(/^(\w+)!(.+)$/i)
  if (crossRef) {
    const si = sheets.value.findIndex(s => s.name.toLowerCase() === crossRef[1].toLowerCase())
    if (si < 0) return '#REF!'
    const s = sheets.value[si]
    const ref = crossRef[2].toUpperCase()
    const rng = ref.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
    if (rng) {
      const c1 = colIndex(rng[1]), r1 = parseInt(rng[2]) - 1, c2 = colIndex(rng[3]), r2 = parseInt(rng[4]) - 1
      const res: any[] = []; for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) { const v = s.rows[r]?.[c]; if (v !== undefined && v !== '') { const n = parseFloat(v); res.push(isNaN(n) ? v : n) } else res.push('') }
      return res
    }
    const cr = ref.match(/^([A-Z]+)(\d+)$/)
    if (cr) { const v = s.rows[parseInt(cr[2]) - 1]?.[colIndex(cr[1])]; if (v === undefined || v === '') return ''; const n = parseFloat(v); return isNaN(n) ? v : n }
    return '#REF!'
  }
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
  const crossRange = arg.trim().match(/^(\w+)!([A-Z]+\d+:[A-Z]+\d+)$/i)
  if (crossRange) {
    const si = sheets.value.findIndex(s => s.name.toLowerCase() === crossRange[1].toLowerCase())
    if (si < 0) return []
    const s = sheets.value[si], rangeStr = crossRange[2].toUpperCase()
    const mr = rangeStr.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
    if (!mr) return []
    const c1 = colIndex(mr[1]), r1 = parseInt(mr[2]) - 1, c2 = colIndex(mr[3]), r2 = parseInt(mr[4]) - 1
    const res: number[] = []
    for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) { const v = s.rows[r]?.[c]; if (countNonNum) { if (v !== undefined && v !== '') res.push(raw ? parseFloat(v) || 0 : 1) } else { const n = parseFloat(v); if (!isNaN(n)) res.push(n) } }
    return res
  }
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
  if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) { if (!editingCell.value) { e.preventDefault(); undo() } return }
  if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.key === 'z' && e.shiftKey))) { if (!editingCell.value) { e.preventDefault(); redo() } return }
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

// ─── Text to Columns (分列) ───
const showSplitColDialog = ref(false)
const splitDelimiter = ref(',')
const splitCustomDelim = ref('')
const splitConsecutive = ref(true)
const splitMode = ref<'delimiter'|'fixed'>('delimiter')
const splitFixedPositions = ref('')
const splitCol = ref(0)

const splitPreview = computed(() => {
  const c = splitCol.value
  const preview: string[][] = []
  const sampleRows = rows.value.slice(0, 5)
  for (const row of sampleRows) {
    const val = row[c] || ''
    if (splitMode.value === 'fixed') {
      const positions = splitFixedPositions.value.split(',').map(s => parseInt(s.trim())).filter(n => n > 0).sort((a, b) => a - b)
      if (!positions.length) { preview.push([val]); continue }
      const parts: string[] = []
      let last = 0
      for (const p of positions) {
        parts.push(val.slice(last, p))
        last = p
      }
      parts.push(val.slice(last))
      preview.push(parts)
    } else {
      const delim = splitDelimiter.value === '__custom__' ? splitCustomDelim.value : splitDelimiter.value
      if (!delim) { preview.push([val]); continue }
      const actualDelim = delim === '\\t' ? '\t' : delim
      let parts: string[]
      if (splitConsecutive.value) {
        // 连续分隔符视为单个
        parts = val.split(new RegExp(actualDelim.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '+'))
      } else {
        parts = val.split(actualDelim)
      }
      preview.push(parts.map(p => p.trim()))
    }
  }
  return preview
})

function openSplitColDialog() {
  if (!selection.value) return
  splitCol.value = selection.value.startCol
  splitMode.value = 'delimiter'
  splitConsecutive.value = true
  splitDelimiter.value = ','
  splitCustomDelim.value = ''
  splitFixedPositions.value = ''
  showSplitColDialog.value = true
}
function doSplitCol() {
  pushUndo()
  const c = splitCol.value
  // Determine max parts from preview logic
  let maxParts = 1
  for (const row of rows.value) {
    const val = row[c] || ''
    if (splitMode.value === 'fixed') {
      const positions = splitFixedPositions.value.split(',').map(s => parseInt(s.trim())).filter(n => n > 0).sort((a, b) => a - b)
      maxParts = Math.max(maxParts, positions.length + 1)
    } else {
      const delim = splitDelimiter.value === '__custom__' ? splitCustomDelim.value : splitDelimiter.value
      if (!delim) continue
      const actualDelim = delim === '\\t' ? '\t' : delim
      let parts: string[]
      if (splitConsecutive.value) {
        parts = val.split(new RegExp(actualDelim.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '+'))
      } else {
        parts = val.split(actualDelim)
      }
      maxParts = Math.max(maxParts, parts.length)
    }
  }
  // Insert new columns
  for (let i = 1; i < maxParts; i++) insertColAt(c + i)
  // Split data
  for (const row of rows.value) {
    const val = row[c] || ''
    if (splitMode.value === 'fixed') {
      const positions = splitFixedPositions.value.split(',').map(s => parseInt(s.trim())).filter(n => n > 0).sort((a, b) => a - b)
      const parts: string[] = []
      let last = 0
      for (const p of positions) { parts.push(val.slice(last, p)); last = p }
      parts.push(val.slice(last))
      for (let i = 0; i < parts.length; i++) row[c + i] = parts[i]
    } else {
      const delim = splitDelimiter.value === '__custom__' ? splitCustomDelim.value : splitDelimiter.value
      if (!delim) continue
      const actualDelim = delim === '\\t' ? '\t' : delim
      let parts: string[]
      if (splitConsecutive.value) {
        parts = val.split(new RegExp(actualDelim.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '+'))
      } else {
        parts = val.split(actualDelim)
      }
      for (let i = 0; i < parts.length; i++) row[c + i] = parts[i].trim()
    }
  }
  showSplitColDialog.value = false
  emitChange()
}

// ─── Remove Duplicates (去重) ───
function removeDuplicates() {
  if (!selection.value) return
  pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const c1 = Math.min(startCol, endCol), c2 = Math.max(startCol, endCol)
  const seen = new Set<string>()
  const toRemove: number[] = []
  for (let r = r1; r <= r2; r++) {
    const key = rows.value[r].slice(c1, c2 + 1).join('\x00')
    if (seen.has(key)) toRemove.push(r)
    else seen.add(key)
  }
  // Remove from bottom up
  for (let i = toRemove.length - 1; i >= 0; i--) {
    rows.value.splice(toRemove[i], 1)
    rowHeights.value.splice(toRemove[i], 1)
  }
  emitChange()
}

// ─── Print (打印) ───
function printSheet() {
  const w = window.open('', '_blank')
  if (!w) return
  let html = '<html><head><title>打印表格</title><style>'
  html += 'body{font-family:Arial,sans-serif;font-size:12px}'
  html += 'table{border-collapse:collapse;width:100%}'
  html += 'th,td{border:1px solid #999;padding:4px 8px;text-align:left}'
  html += 'th{background:#eee;font-weight:bold}'
  html += '</style></head><body><table>'
  // Header row
  html += '<tr><th></th>'
  for (let c = 0; c < colCount.value; c++) html += `<th>${colName(c)}</th>`
  html += '</tr>'
  // Data rows
  for (let r = 0; r < rows.value.length; r++) {
    if (isRowFiltered(r)) continue
    html += `<tr><td style="background:#eee;color:#666">${r + 1}</td>`
    for (let c = 0; c < colCount.value; c++) {
      const v = getCellDisplay(r, c)
      const m = getCellMeta(r, c)
      let style = ''
      if (m.bold) style += 'font-weight:bold;'
      if (m.italic) style += 'font-style:italic;'
      if (m.align) style += `text-align:${m.align};`
      if (m.bgColor) style += `background:${m.bgColor};`
      if (m.color) style += `color:${m.color};`
      html += `<td style="${style}">${v}</td>`
    }
    html += '</tr>'
  }
  html += '</table></body></html>'
  w.document.write(html)
  w.document.close()
  w.setTimeout(() => w.print(), 300)
}

// ─── Sparklines (迷你图) ───
function renderSparklines() {
  // Sparklines rendered via cell display - uses SPARKLINE() formula
}
function computeSparkline(data: number[], type: string): string {
  if (!data.length) return ''
  const canvas = document.createElement('canvas')
  canvas.width = 120; canvas.height = 28
  const ctx = canvas.getContext('2d')!
  const maxV = Math.max(...data), minV = Math.min(...data)
  const range = maxV - minV || 1
  if (type === 'bar') {
    const bw = Math.max(2, 120 / data.length - 1)
    data.forEach((v, i) => {
      const h = Math.max(1, ((v - minV) / range) * 24)
      ctx.fillStyle = '#4caf50'
      ctx.fillRect(i * (bw + 1), 28 - h, bw, h)
    })
  } else {
    ctx.strokeStyle = '#1a73e8'; ctx.lineWidth = 1.5; ctx.beginPath()
    data.forEach((v, i) => {
      const x = (i / (data.length - 1 || 1)) * 120
      const y = 28 - ((v - minV) / range) * 26
      i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)
    })
    ctx.stroke()
  }
  return canvas.toDataURL()
}

// ─── Indent (缩进) ───
function increaseIndent() { if (!selection.value) return; pushUndo(); applyToSelection('indent', (getMetaProp('indent') || 0) + 1) }
function decreaseIndent() { if (!selection.value) return; const cur = getMetaProp('indent') || 0; if (cur <= 0) return; pushUndo(); applyToSelection('indent', cur - 1) }

// ─── Export xlsx placeholder (frontend only, exports CSV) ───
function exportCSV() {
  let csv = ''
  for (let r = 0; r < rows.value.length; r++) {
    if (isRowFiltered(r)) continue
    const cells: string[] = []
    for (let c = 0; c < colCount.value; c++) {
      let v = rows.value[r]?.[c] || ''
      if (v.startsWith('=')) v = computeFormula(v)
      if (v.includes(',') || v.includes('"') || v.includes('\n')) v = '"' + v.replace(/"/g, '""') + '"'
      cells.push(v)
    }
    csv += cells.join(',') + '\n'
  }
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const link = document.createElement('a'); link.download = 'sheet.csv'; link.href = URL.createObjectURL(blob); link.click()
}

// ─── Cell Style Presets ───
const stylePresets: Record<string, Partial<CellMeta>> = {
  '标题': { bold: true, fontSize: 16, align: 'center', bgColor: '#4472c4', color: '#ffffff' },
  '标题2': { bold: true, fontSize: 14, color: '#2f5496' },
  '标题3': { bold: true, fontSize: 12, color: '#2f5496', bgColor: '#d6e4f0' },
  '强调': { bold: true, bgColor: '#ffd7d7' },
  '良好': { bgColor: '#c6efce', color: '#006100' },
  '警告': { bgColor: '#ffeb9c', color: '#9c6500' },
  '计算': { bgColor: '#e2efda', italic: true },
  '输入': { bgColor: '#fff2cc', border: { top: true, right: true, bottom: true, left: true } },
  '清除': {},
}
const showStylePanel = ref(false)
function applyStylePreset(name: string) {
  if (!selection.value) return; pushUndo()
  const preset = stylePresets[name]; if (!preset) return
  const { startRow, startCol, endRow, endCol } = selection.value
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) {
      if (name === '清除') { sheet.value.cellMeta[metaKey(r, c)] = {} }
      else setCellMeta(r, c, { ...preset })
    }
  showStylePanel.value = false; emitChange()
}

// ─── Icon Set Condition Format ───
function applyIconSet(type: 'arrows' | 'flags' | 'traffic') {
  if (!selection.value) return; pushUndo()
  const { startRow, startCol, endRow, endCol } = selection.value
  let minV = Infinity, maxV = -Infinity
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) {
      const n = parseFloat(rows.value[r]?.[c] || ''); if (!isNaN(n)) { if (n < minV) minV = n; if (n > maxV) maxV = n }
    }
  if (minV === Infinity) return
  const range = maxV - minV || 1
  for (let r = Math.min(startRow, endRow); r <= Math.max(startRow, endRow); r++)
    for (let c = Math.min(startCol, endCol); c <= Math.max(startCol, endCol); c++) {
      const n = parseFloat(rows.value[r]?.[c] || ''); if (isNaN(n)) continue
      const pct = (n - minV) / range
      let icon = ''
      if (type === 'arrows') icon = pct > 0.66 ? '🟢↑' : pct > 0.33 ? '🟡→' : '🔴↓'
      else if (type === 'flags') icon = pct > 0.66 ? '🚩' : pct > 0.33 ? '🏁' : '🏳️'
      else icon = pct > 0.66 ? '🟢' : pct > 0.33 ? '🟡' : '🔴'
      setCellMeta(r, c, { comment: icon + ' ' + (rows.value[r][c] || '') })
    }
  emitChange()
}

// ─── Pivot Table (简易数据透视) ───
const showPivotDialog = ref(false)
const pivotGroupCol = ref(0)
const pivotValueCol = ref(1)
const pivotResult = ref<{ group: string; sum: number; count: number; avg: number }[]>([])

function openPivotDialog() {
  if (!selection.value) return
  pivotGroupCol.value = selection.value.startCol
  pivotValueCol.value = Math.min(selection.value.startCol + 1, colCount.value - 1)
  showPivotDialog.value = true
}
function doPivot() {
  const gc = pivotGroupCol.value, vc = pivotValueCol.value
  const { startRow, endRow } = selection.value || { startRow: 0, endRow: rows.value.length - 1 }
  const r1 = Math.min(startRow, endRow), r2 = Math.max(startRow, endRow)
  const groups = new Map<string, { sum: number; count: number }>()
  for (let r = r1; r <= r2; r++) {
    const g = rows.value[r]?.[gc] || '(空)'
    const v = parseFloat(rows.value[r]?.[vc] || '0') || 0
    const cur = groups.get(g) || { sum: 0, count: 0 }
    cur.sum += v; cur.count++; groups.set(g, cur)
  }
  pivotResult.value = Array.from(groups.entries()).map(([group, { sum, count }]) => ({ group, sum, count, avg: count ? Math.round((sum / count) * 100) / 100 : 0 }))
}
function insertPivotResult() {
  pushUndo()
  const startR = rows.value.length
  // Header
  rows.value.push(['分组', '求和', '计数', '平均值'])
  // Data
  for (const row of pivotResult.value) rows.value.push([row.group, String(row.sum), String(row.count), String(row.avg)])
  showPivotDialog.value = false; emitChange()
}

// ─── Extend CellMeta type for indent ───
// indent is already handled via Partial<CellMeta>
// Update getCellTextStyle to include indent
const _origGetCellTextStyle = getCellTextStyle
// We'll handle indent in the template display directly

// Emit
function emitChange() { emit('change', getData()) }
function getData(): string {
  // 序列化时 Set → Array
  const data = sheets.value.map(s => ({
    ...s,
    hiddenRows: Array.from(s.hiddenRows || []),
    hiddenCols: Array.from(s.hiddenCols || []),
  }))
  return JSON.stringify(data)
}

// Lifecycle
loadData()
onMounted(() => { document.addEventListener('keydown', onGlobalKeydown) })
onUnmounted(() => { document.removeEventListener('keydown', onGlobalKeydown) })
defineExpose({ getData })
</script>

<style scoped>
/* ── Layout ── */
.sheet-container { display: flex; flex-direction: column; height: 100%; background: #fff; font-family: 'Segoe UI', -apple-system, BlinkMacSystemFont, sans-serif; font-size: 13px; color: #333; outline: none; }

/* ── Formula Bar ── */
.formula-bar { display: flex; align-items: center; height: 28px; border-bottom: 1px solid #d6d6d6; background: #f3f3f3; position: relative; }
.cell-ref { width: 72px; text-align: center; font-size: 12px; color: #444; border-right: 1px solid #d6d6d6; height: 100%; display: flex; align-items: center; justify-content: center; background: #fff; font-weight: 500; flex-shrink: 0; }
.formula-fx-btn { padding: 0 8px; color: #555; font-style: italic; font-weight: 600; border-right: 1px solid #d6d6d6; height: 100%; display: flex; align-items: center; background: #f3f3f3; font-size: 12px; cursor: pointer; border: none; }
.formula-fx-btn:hover { background: #e8e8e8; }
.formula-fx-btn.active { background: #e0ecf7; color: #409eff; }
.formula-input-wrap { flex: 1; position: relative; height: 100%; }
.formula-input { width: 100%; border: none; outline: none; padding: 0 8px; height: 100%; font-size: 13px; background: #fff; }

/* 函数面板 */
.fx-panel {
  position: absolute; top: 100%; left: 0; z-index: 200;
  background: #fff; border: 1px solid #d6d6d6; border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.12); width: 360px;
  display: flex; flex-direction: column; max-height: 320px;
}
.fx-search { padding: 8px; border-bottom: 1px solid #eee; }
.fx-search input {
  width: 100%; border: 1px solid #ddd; border-radius: 4px; padding: 5px 8px;
  font-size: 13px; outline: none;
}
.fx-search input:focus { border-color: #409eff; }
.fx-list { overflow-y: auto; max-height: 180px; }
.fx-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 12px; cursor: pointer; transition: background 0.1s;
}
.fx-item:hover, .fx-item.active { background: #ecf5ff; }
.fx-name { font-weight: 600; font-size: 13px; color: #409eff; min-width: 100px; }
.fx-desc { font-size: 12px; color: #999; }
.fx-empty { padding: 16px; text-align: center; color: #c0c4cc; font-size: 13px; }
.fx-hint {
  padding: 8px 12px; background: #f5f7fa; border-top: 1px solid #eee;
  font-size: 12px; color: #666; border-radius: 0 0 8px 8px;
}
.fx-hint strong { color: #409eff; }
.fx-hint p { margin: 4px 0 0; color: #999; }

/* ── Ribbon (Excel style) ── */
.ribbon { border-bottom: 1px solid #c6c6c6; background: #f3f3f3; }
.ribbon-row { display: flex; align-items: stretch; padding: 2px 4px; gap: 0; min-height: 62px; overflow-x: auto; }
.ribbon-section { display: flex; flex-direction: column; align-items: center; padding: 2px 6px 0; min-width: 0; }
.ribbon-section-buttons { display: flex; align-items: center; gap: 1px; min-height: 26px; }
.ribbon-section-label { font-size: 10px; color: #888; margin-top: auto; padding: 1px 0; white-space: nowrap; user-select: none; }
.rb-sep { width: 1px; background: #d0d0d0; margin: 4px 2px; align-self: stretch; }
.rb-vsep { width: 1px; height: 18px; background: #d0d0d0; margin: 0 3px; }
.rb-btn { display: inline-flex; align-items: center; justify-content: center; min-width: 26px; height: 24px; border: 1px solid transparent; border-radius: 2px; background: transparent; cursor: pointer; font-size: 13px; color: #333; padding: 0 4px; transition: all 0.08s; white-space: nowrap; }
.rb-btn:hover:not(:disabled) { background: #c8ddf0; border-color: #90b4d8; }
.rb-btn:active:not(:disabled) { background: #b0ccea; }
.rb-btn.active { background: #c8ddf0; border-color: #6da0cc; color: #1565c0; }
.rb-btn:disabled { opacity: 0.35; cursor: default; }
.rb-svg { width: 16px; height: 16px; display: inline-block; vertical-align: middle; }
.rb-select :deep(.el-input__wrapper) { box-shadow: none !important; background: #fff; border: 1px solid #c0c0c0; border-radius: 2px; }
.rb-select :deep(.el-input__wrapper:hover) { border-color: #90b4d8; }

/* Color picker integration */
.color-btn-wrap { position: relative; display: inline-flex; }
.color-btn-wrap .hidden-picker { position: absolute; bottom: -2px; left: 0; opacity: 0; pointer-events: none; width: 1px; height: 1px; overflow: hidden; }
.color-indicator { display: block; height: 3px; margin-top: 1px; border-radius: 1px; min-width: 14px; }

/* ── Grid ── */
.grid-area { flex: 1; overflow: auto; background: #fff; }
.grid-table { border-collapse: collapse; table-layout: fixed; }
.grid-table th, .grid-table td { border-right: 1px solid #e2e2e2; border-bottom: 1px solid #e2e2e2; }

/* Corner */
.corner-cell { background: linear-gradient(135deg, #f0f0f0, #e8e8e8); width: 46px; position: sticky; top: 0; left: 0; z-index: 5; border-right: 1px solid #c0c0c0; border-bottom: 1px solid #c0c0c0; }

/* Column Headers */
.col-hdr { background: linear-gradient(180deg, #fafafa, #eee); font-weight: 600; color: #555; text-align: center; position: sticky; top: 0; z-index: 4; cursor: pointer; user-select: none; height: 24px; font-size: 12px; border-bottom: 1px solid #c0c0c0; overflow: visible; }
.col-hdr:hover { background: linear-gradient(180deg, #e8e8e8, #ddd); }
.col-hdr.sel { background: linear-gradient(180deg, #d6e4f9, #c2d8f0); color: #1a73e8; }
.col-hdr.sorted { color: #1a73e8; }
.col-letter { font-size: 12px; }
.sort-arrow { font-size: 9px; color: #1a73e8; margin-left: 2px; }
.filter-dot { font-size: 8px; color: #e6a23c; margin-left: 2px; }
.hdr-menu { cursor: pointer; font-size: 10px; margin-left: 1px; opacity: 0.4; }
.hdr-menu:hover { opacity: 1; }
.col-hdr-inner { position: relative; display: inline-flex; align-items: center; justify-content: center; width: 100%; height: 100%; }
.col-resize { position: absolute; right: -2px; top: 0; bottom: 0; width: 5px; cursor: col-resize; z-index: 5; }
.frozen-col-hdr { z-index: 6 !important; }

/* Row Headers */
.row-hdr { background: linear-gradient(90deg, #fafafa, #eee); text-align: center; color: #555; font-weight: 600; position: sticky; left: 0; z-index: 2; cursor: pointer; user-select: none; font-size: 12px; border-right: 1px solid #c0c0c0; min-width: 46px; overflow: visible; }
.row-hdr:hover { background: linear-gradient(90deg, #e8e8e8, #ddd); }
.row-hdr.sel { background: linear-gradient(90deg, #d6e4f9, #c2d8f0); color: #1a73e8; }
.row-hdr.drag-over { border-top: 2px solid #1a73e8; }
.row-hdr-inner { position: relative; width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; }
.row-resize { position: absolute; bottom: -2px; left: 0; right: 0; height: 5px; cursor: row-resize; z-index: 3; }

/* ── Cells ── */
.cell { padding: 0; cursor: cell; overflow: visible; position: relative; height: 26px; }
.cell.sel { background: #e8f0fe !important; }
.cell.sel-head { outline: 2px solid #1a73e8; outline-offset: -1px; z-index: 1; background: #fff !important; }
.cell.editing { padding: 0; }
.cell.has-comment .comment-flag { position: absolute; top: 0; right: 0; width: 0; height: 0; border-left: 6px solid transparent; border-top: 6px solid #e6a23c; z-index: 3; }
.cell.frozen { background: #fafafa; }

.cell-val { display: block; padding: 0 6px; line-height: 26px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.wrap-row { height: auto !important; }
.wrap-row .cell-val { white-space: normal; word-break: break-all; line-height: 1.4; }
.cell-input { width: 100%; height: 100%; border: none; outline: none; padding: 0 6px; font-size: 13px; font-family: inherit; background: #fff; }
.ac-dropdown { position: absolute; top: 100%; left: 0; background: #fff; border: 1px solid #d0d0d0; border-radius: 4px; box-shadow: 0 4px 12px rgba(0,0,0,0.12); z-index: 100; max-height: 200px; overflow-y: auto; min-width: 120px; }
.ac-item { padding: 4px 10px; font-size: 13px; cursor: pointer; color: #333; }
.ac-item:hover, .ac-item.ac-active { background: #e8f0fe; color: #1a73e8; }

/* Fill & Move handles */
.fill-h { position: absolute; right: -4px; bottom: -4px; width: 8px; height: 8px; background: #1a73e8; cursor: crosshair; z-index: 2; border-radius: 0; }
.move-h { position: absolute; left: 50%; top: -4px; transform: translateX(-50%); width: 16px; height: 4px; background: #1a73e8; cursor: move; z-index: 2; border-radius: 2px; opacity: 0.7; }
.comment-flag { position: absolute; top: 0; right: 0; width: 0; height: 0; border-left: 6px solid transparent; border-top: 6px solid #e6a23c; z-index: 3; }

/* ── Comment Popup ── */
.comment-popup { position: fixed; background: #fffbe6; border: 1px solid #ffe58f; border-radius: 4px; padding: 8px 12px; font-size: 12px; box-shadow: 0 2px 8px rgba(0,0,0,0.12); z-index: 2000; max-width: 250px; white-space: pre-wrap; color: #333; }

/* ── Context Menu ── */
.ctx-menu { position: fixed; background: #fff; border: 1px solid #d0d0d0; border-radius: 6px; box-shadow: 0 4px 16px rgba(0,0,0,0.14); z-index: 1000; min-width: 200px; padding: 4px 0; }
.ctx-item { padding: 6px 28px 6px 12px; font-size: 13px; cursor: pointer; color: #333; display: flex; align-items: center; gap: 8px; }
.ctx-item:hover { background: #e8f0fe; color: #1a73e8; }
.ctx-icon { font-size: 14px; width: 18px; text-align: center; }
.ctx-key { margin-left: auto; color: #aaa; font-size: 11px; }
.ctx-sep { height: 1px; background: #e8e8e8; margin: 4px 0; }

/* ── Chart Panel ── */
.chart-panel { border-top: 1px solid #d6d6d6; background: #fafafa; padding: 8px; }
.chart-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.chart-title { border: 1px solid #d6d6d6; border-radius: 4px; padding: 2px 8px; font-size: 13px; width: 120px; outline: none; }
.chart-title:focus { border-color: #1a73e8; }
.chart-canvas-wrap { display: flex; justify-content: center; position: relative; }
.chart-canvas-wrap canvas { border: 1px solid #e0e0e0; border-radius: 4px; background: #fff; }
.chart-tip { position: absolute; background: rgba(0,0,0,0.78); color: #fff; padding: 4px 8px; border-radius: 4px; font-size: 12px; pointer-events: none; }

/* ── Filter Panel ── */
.filter-panel { position: fixed; background: #fff; border: 1px solid #d0d0d0; border-radius: 6px; box-shadow: 0 4px 16px rgba(0,0,0,0.14); z-index: 1001; width: 200px; }

/* ── Sheet Tabs (Excel style) ── */
.sheet-tabs { display: flex; align-items: center; height: 32px; border-top: 1px solid #d6d6d6; background: #f3f3f3; padding: 0 4px; flex-shrink: 0; }
.tabs-scroll { display: flex; align-items: center; gap: 0; flex: 1; overflow-x: auto; }
.tab { display: inline-flex; align-items: center; gap: 4px; padding: 4px 14px; font-size: 12px; cursor: pointer; border: 1px solid transparent; border-bottom: none; border-radius: 4px 4px 0 0; color: #555; user-select: none; background: transparent; height: 26px; transition: all 0.1s; white-space: nowrap; }
.tab:hover { background: #e5e5e5; }
.tab.active { background: #fff; border-color: #d6d6d6; color: #1a73e8; font-weight: 600; border-bottom: 1px solid #fff; margin-bottom: -1px; }
.tab-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.tab-x { font-size: 14px; color: #999; margin-left: 2px; line-height: 1; }
.tab-x:hover { color: #e53935; }
.tab-add { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 26px; border: 1px solid transparent; border-radius: 4px; background: transparent; cursor: pointer; font-size: 16px; color: #666; transition: all 0.1s; }
.tab-add:hover { background: #e5e5e5; border-color: #d0d0d0; color: #1a73e8; }
.tabs-info { font-size: 11px; color: #999; padding: 0 8px; white-space: nowrap; }
</style>
