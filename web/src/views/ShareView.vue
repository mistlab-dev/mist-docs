<template>
  <div class="share-page">
    <!-- 加载 -->
    <div v-if="loading" class="state-center">
      <div class="loader-ring" />
      <p class="loader-text">加载中...</p>
    </div>

    <!-- 错误 -->
    <div v-else-if="error" class="state-center">
      <div class="error-icon"><svg viewBox="0 0 20 20" fill="currentColor" width="48" height="48"><path d="M10 2a4 4 0 00-4 4v2H5a1 1 0 00-1 1v8a1 1 0 001 1h10a1 1 0 001-1V9a1 1 0 00-1-1h-1V6a4 4 0 00-4-4zm2 6H8V6a2 2 0 114 0v2z"/></svg></div>
      <h2 class="error-title">{{ error }}</h2>
      <p class="error-desc">请确认链接是否正确，或联系分享者</p>
    </div>

    <!-- 密码验证 -->
    <div v-else-if="needPassword" class="state-center">
      <div class="password-card">
        <div class="pw-icon"><svg viewBox="0 0 20 20" fill="currentColor" width="20" height="20"><path d="M10 2a4 4 0 00-4 4v2H5a1 1 0 00-1 1v8a1 1 0 001 1h10a1 1 0 001-1V9a1 1 0 00-1-1h-1V6a4 4 0 00-4-4zm2 6H8V6a2 2 0 114 0v2z"/></svg></div>
        <h3 class="pw-title">{{ shareInfo?.title || '受保护的文档' }}</h3>
        <p class="pw-desc">此文档需要密码才能访问</p>
        <el-input v-model="password" type="password" placeholder="输入访问密码" size="large" show-password @keyup.enter="accessWithPassword" />
        <el-button type="primary" size="large" class="pw-btn" @click="accessWithPassword" :loading="verifying">
          访问文档
        </el-button>
      </div>
    </div>

    <!-- 文档内容 -->
    <div v-else class="share-content">
      <div class="content-inner">
        <div class="share-header">
          <div class="header-badge">
            <div class="badge-icon" :class="doc.type === 'sheet' ? 'sheet' : 'doc'">
              <el-icon :size="16"><Document v-if="doc.type !== 'sheet'" /><Grid v-else /></el-icon>
            </div>
            <span>{{ doc.type === 'sheet' ? '表格' : '文档' }}</span>
          </div>
          <h1 class="doc-title">{{ doc.title }}</h1>
          <div class="share-meta">
            <el-icon :size="14"><Link /></el-icon>
            <span>通过分享链接访问</span>
          </div>
        </div>
        <el-divider />
        <!-- 文档类型 -->
        <div v-if="doc.type !== 'sheet'" class="doc-body" v-html="doc.content"></div>
        <!-- 表格类型 -->
        <div v-else class="sheet-preview">
          <div v-for="(sheet, si) in parsedSheets" :key="si" class="sheet-section">
            <div v-if="parsedSheets.length > 1" class="sheet-tab">{{ sheet.name || ('工作表' + (si + 1)) }}</div>
            <div class="table-wrapper">
              <table class="sheet-table">
                <tbody>
                  <tr v-for="(row, ri) in sheet.renderedRows" :key="ri">
                    <td v-for="(cell, ci) in row" :key="ci" :class="getCellClass(sheet, ri, ci)">{{ cell }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div v-if="!parsedSheets.length" class="empty-sheet">空表格</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'

const route = useRoute()
const token = route.params.token as string

const loading = ref(true)
const verifying = ref(false)
const error = ref('')
const needPassword = ref(false)
const shareInfo = ref<any>(null)
const doc = ref<any>({})
const password = ref('')

// Parse sheet data for rendering
const parsedSheets = computed(() => {
  if (doc.value?.type !== 'sheet' || !doc.value?.content) return []
  try {
    const raw = typeof doc.value.content === 'string' ? JSON.parse(doc.value.content) : doc.value.content
    const sheets = Array.isArray(raw) ? raw : [raw]
    return sheets.map((sheet: any) => {
      const rows = sheet.rows || []
      // Simple formula evaluation for display
      const renderedRows = rows.map((row: string[]) =>
        (row || []).map((cell: string) => evalCell(cell, rows))
      )
      return { name: sheet.name, renderedRows, merges: sheet.merges || [] }
    })
  } catch {
    return []
  }
})

function evalCell(cell: string, allRows: string[][]): string {
  if (!cell || !cell.startsWith('=')) return cell || ''
  try {
    const expr = cell.substring(1).toUpperCase()
    // SUM(A1:B3)
    const sumMatch = expr.match(/^SUM\(([A-Z])(\d+):([A-Z])(\d+)\)$/)
    if (sumMatch) {
      const c1 = sumMatch[1].charCodeAt(0) - 65
      const r1 = parseInt(sumMatch[2]) - 1
      const c2 = sumMatch[3].charCodeAt(0) - 65
      const r2 = parseInt(sumMatch[4]) - 1
      let sum = 0
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          const v = parseFloat(allRows[r]?.[c] || '0')
          sum += isNaN(v) ? 0 : v
        }
      }
      return String(Math.round(sum * 1e6) / 1e6)
    }
    // AVERAGE
    const avgMatch = expr.match(/^AVERAGE\(([A-Z])(\d+):([A-Z])(\d+)\)$/)
    if (avgMatch) {
      const c1 = avgMatch[1].charCodeAt(0) - 65
      const r1 = parseInt(avgMatch[2]) - 1
      const c2 = avgMatch[3].charCodeAt(0) - 65
      const r2 = parseInt(avgMatch[4]) - 1
      let sum = 0, count = 0
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          const v = parseFloat(allRows[r]?.[c] || '0')
          if (!isNaN(v)) { sum += v; count++ }
        }
      }
      return count ? String(Math.round(sum / count * 1e6) / 1e6) : '0'
    }
    // COUNT
    const countMatch = expr.match(/^COUNT\(([A-Z])(\d+):([A-Z])(\d+)\)$/)
    if (countMatch) {
      const c1 = countMatch[1].charCodeAt(0) - 65
      const r1 = parseInt(countMatch[2]) - 1
      const c2 = countMatch[3].charCodeAt(0) - 65
      const r2 = parseInt(countMatch[4]) - 1
      let count = 0
      for (let r = r1; r <= r2; r++) {
        for (let c = c1; c <= c2; c++) {
          const v = allRows[r]?.[c]
          if (v && !isNaN(parseFloat(v))) count++
        }
      }
      return String(count)
    }
    // Simple cell ref like =A1 or =A1+B1
    let resolved = expr
    const refRe = /([A-Z])(\d+)/g
    resolved = resolved.replace(refRe, (_: string, col: string, row: string) => {
      const c = col.charCodeAt(0) - 65
      const r = parseInt(row) - 1
      const v = allRows[r]?.[c] || '0'
      return v.startsWith('=') ? '0' : v
    })
    // Safe eval for simple arithmetic
    const result = Function('"use strict"; return (' + resolved + ')')()
    if (typeof result === 'number' && !isNaN(result)) return String(Math.round(result * 1e6) / 1e6)
    return String(result)
  } catch {
    return cell
  }
}

function getCellClass(sheet: any, ri: number, ci: number): string {
  if (!sheet.merges?.length) return ''
  for (const m of sheet.merges) {
    if (ri === m.row && ci === m.col) return 'merge-origin'
    if (ri >= m.row && ri < m.row + m.rowspan && ci >= m.col && ci < m.col + m.colspan) return 'merge-hidden'
  }
  return ''
}

async function loadShare() {
  try {
    const { data } = await axios.get(`/api/s/${token}`)
    doc.value = data
  } catch (e: any) {
    const resp = e.response?.data
    if (resp?.need_password) {
      needPassword.value = true
      shareInfo.value = { title: '受保护的文档' }
      try {
        const { data: info } = await axios.get(`/api/s/${token}/info`)
        shareInfo.value = info
      } catch {}
    } else if (e.response?.status === 410) {
      error.value = '分享链接已过期'
    } else {
      error.value = resp?.error || '分享链接不存在'
    }
  } finally {
    loading.value = false
  }
}

async function accessWithPassword() {
  if (!password.value) return
  verifying.value = true
  try {
    const { data } = await axios.get(`/api/s/${token}?password=${encodeURIComponent(password.value)}`)
    doc.value = data
    needPassword.value = false
  } catch (e: any) {
    error.value = e.response?.data?.error || '密码错误'
  } finally {
    verifying.value = false
  }
}

onMounted(loadShare)
</script>

<style scoped>
.share-page { min-height: calc(var(--vh, 1vh) * 100); background: #f5f7fa; }

/* 居中状态 */
.state-center {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; min-height: calc(var(--vh, 1vh) * 100); padding: 40px 20px;
}

/* 加载动画 */
.loader-ring {
  width: 40px; height: 40px; border-radius: 50%;
  border: 3px solid #e8ecf0; border-top-color: #4f6ef7;
  animation: spin 0.8s linear infinite;
}
.loader-text { margin-top: 16px; color: #909399; font-size: 14px; }
@keyframes spin { to { transform: rotate(360deg); } }

/* 错误 */
.error-icon { font-size: 56px; margin-bottom: 16px; }
.error-title { font-size: 18px; color: #303133; margin: 0 0 8px; }
.error-desc { font-size: 14px; color: #909399; margin: 0; }

/* 密码卡片 */
.password-card {
  width: 380px; padding: 36px; background: #fff;
  border-radius: 16px; box-shadow: 0 8px 32px rgba(0,0,0,0.08);
  display: flex; flex-direction: column; align-items: center;
}
.pw-icon { font-size: 40px; margin-bottom: 12px; }
.pw-title { font-size: 18px; color: #303133; margin: 0 0 6px; text-align: center; }
.pw-desc { font-size: 13px; color: #909399; margin: 0 0 20px; text-align: center; }
.password-card :deep(.el-input) { width: 100%; }
.password-card :deep(.el-input__wrapper) { border-radius: 10px; }
.pw-btn { width: 100%; margin-top: 14px; border-radius: 10px; font-size: 15px; }

/* 文档内容 */
.share-content {
  display: flex; justify-content: center; padding: 32px 20px;
}
.content-inner {
  width: 100%; max-width: 800px; background: #fff;
  border-radius: 16px; padding: 40px 48px; min-height: calc(var(--vh, 1vh) * 100 - 64px);
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
}
.share-header { margin-bottom: 8px; }
.header-badge {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 4px 12px; background: #f5f7fa; border-radius: 20px;
  font-size: 12px; color: #606266; margin-bottom: 12px;
}
.badge-icon {
  width: 22px; height: 22px; border-radius: 6px;
  display: flex; align-items: center; justify-content: center;
}
.badge-icon.doc { background: #e8f0fe; color: #4f6ef7; }
.badge-icon.sheet { background: #e6f7f0; color: #36b37e; }

.doc-title { font-size: 26px; font-weight: 700; color: #1a1a2e; margin: 0 0 10px; }
.share-meta {
  display: flex; align-items: center; gap: 6px;
  color: #909399; font-size: 13px;
}
.share-header + :deep(.el-divider) { margin: 16px 0 24px; }

/* 富文本样式 */
.doc-body { line-height: 1.8; color: #333; }
.doc-body :deep(h1) { font-size: 24px; margin: 32px 0 16px; }
.doc-body :deep(h2) { font-size: 20px; margin: 28px 0 12px; }
.doc-body :deep(h3) { font-size: 17px; margin: 20px 0 10px; }
.doc-body :deep(p) { margin: 0 0 12px; }
.doc-body :deep(img) { max-width: 100%; border-radius: 8px; margin: 8px 0; }
.doc-body :deep(table) { border-collapse: collapse; width: 100%; margin: 12px 0; }
.doc-body :deep(th), .doc-body :deep(td) { border: 1px solid #e8ecf0; padding: 10px 14px; }
.doc-body :deep(th) { background: #fafbfc; font-weight: 600; }
.doc-body :deep(pre) { background: #f6f8fa; padding: 16px; border-radius: 10px; overflow-x: auto; font-size: 13px; }
.doc-body :deep(code) { background: #f0f2f5; padding: 2px 6px; border-radius: 4px; font-size: 13px; }
.doc-body :deep(blockquote) { border-left: 4px solid #4f6ef7; padding-left: 16px; color: #606266; margin: 12px 0; }
.doc-body :deep(ul), .doc-body :deep(ol) { padding-left: 24px; }
.doc-body :deep(a) { color: #4f6ef7; text-decoration: none; }
.doc-body :deep(a:hover) { text-decoration: underline; }

/* 表格分享预览 */
.sheet-preview { padding: 0; }
.sheet-section { margin-bottom: 20px; }
.sheet-tab {
  display: inline-block; padding: 4px 14px; background: #4f6ef7;
  color: #fff; border-radius: 6px 6px 0 0; font-size: 13px; font-weight: 500;
}
.table-wrapper { overflow-x: auto; border: 1px solid #e8ecf0; border-radius: 0 6px 6px 6px; }
.sheet-table { border-collapse: collapse; width: 100%; font-size: 13px; }
.sheet-table td {
  border: 1px solid #e8ecf0; padding: 6px 10px; white-space: nowrap;
  min-width: 60px; max-width: 300px; overflow: hidden; text-overflow: ellipsis;
}
.sheet-table td.merge-hidden { display: none; }
.sheet-table tr:nth-child(even) td { background: #fafbfc; }
.empty-sheet { text-align: center; color: #c0c4cc; padding: 40px; }

@media (max-width: 768px) {
  .content-inner { padding: 24px 16px; border-radius: 0; }
  .doc-title { font-size: 20px; }
  .share-content { padding: 0; }
}
</style>
