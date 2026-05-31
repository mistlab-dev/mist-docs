/**
 * Sheet formula engine — pure functions for testing.
 * These are extracted from SheetEditor.vue to allow unit testing.
 */

// ─── helpers ───

function colIndex(name: string): number {
  let idx = 0
  for (let i = 0; i < name.length; i++) idx = idx * 26 + (name.charCodeAt(i) - 64)
  return idx - 1
}

function splitArgs(s: string): string[] {
  const args: string[] = []
  let depth = 0, cur = '', inStr = false
  for (let i = 0; i < s.length; i++) {
    const ch = s[i]
    if (ch === '"' && (i === 0 || s[i - 1] !== '\\')) inStr = !inStr
    if (!inStr) { if (ch === '(') depth++; if (ch === ')') depth-- }
    if (ch === ',' && depth === 0 && !inStr) { args.push(cur); cur = '' } else cur += ch
  }
  if (cur.trim()) args.push(cur)
  return args
}

// ─── sheet data interface ───

export interface SheetData {
  rows: string[][]
}

/**
 * Create a simple sheet from a 2D string array for testing.
 * Each cell is stored as a raw string.
 */
export function makeSheet(cells: string[][]): SheetData {
  return { rows: cells }
}

// ─── resolve cell value (handles formula cells) ───

function resolveCellRaw(rows: string[][], r: number, c: number, sheets?: SheetData[], sheetIdx?: number): string {
  const sourceRows = (sheetIdx !== undefined && sheets) ? sheets[sheetIdx].rows : rows
  const v = sourceRows[r]?.[c]
  if (v === undefined || v === '') return ''
  if (v.startsWith('=')) return computeFormulaRaw(v, rows, sheets, sheetIdx)
  return v
}

function getCellVal(rows: string[][], col: string, row: number, sheets?: SheetData[]): any {
  const c = colIndex(col), r = row - 1
  const v = resolveCellRaw(rows, r, c, sheets)
  if (v === '') return ''
  const n = parseFloat(v)
  return isNaN(n) ? v : n
}

function getCellValByRC(rows: string[][], r: number, c: number): any {
  const v = resolveCellRaw(rows, r, c)
  if (v === '') return ''
  const n = parseFloat(v)
  return isNaN(n) ? v : n
}

function getRange(rows: string[][], rangeStr: string): any[] {
  const m = rangeStr.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (!m) return []
  const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1
  const res: any[] = []
  for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) res.push(resolveCellRaw(rows, r, c))
  return res
}

function numArray(rows: string[][], arg: string, countNonNum = false, countAll = false): number[] {
  const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (m) {
    const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1
    const res: number[] = []
    for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) {
      const v = resolveCellRaw(rows, r, c)
      if (countAll) { if (v !== '') res.push(1) }
      else if (countNonNum) { const n = parseFloat(v); if (!isNaN(n)) res.push(n) }
      else { const n = parseFloat(v); if (!isNaN(n)) res.push(n) }
    }
    return res
  }
  return arg.split(',').map(v => parseFloat(v.trim())).filter(v => !isNaN(v))
}

function strArray(rows: string[][], arg: string, keepAll = false): string[] {
  const m = arg.trim().toUpperCase().match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (m) {
    const c1 = colIndex(m[1]), r1 = parseInt(m[2]) - 1, c2 = colIndex(m[3]), r2 = parseInt(m[4]) - 1
    const res: string[] = []
    for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) {
      const v = resolveCellRaw(rows, r, c)
      if (keepAll || v !== '') res.push(v)
    }
    return res
  }
  return arg.split(',').map(v => v.trim())
}

function safeCalc(rows: string[][], expr: string): number {
  let safe = expr.replace(/([A-Z]+)(\d+)/gi, (_, col, row) => {
    const v = getCellVal(rows, col.toUpperCase(), parseInt(row))
    // console.log('[safeCalc]', col+row, '->', v, typeof v)
    return (typeof v === 'number') ? String(v) : (isNaN(Number(v)) ? '0' : String(v))
  })
  safe = safe.replace(/[^0-9+\-*/.() <>!=&]/g, '')
  try { return Function('"use strict"; return (' + safe + ')')() } catch { return NaN }
}

// ─── main eval engine ───

function evalFormula(rows: string[][], expr: string, sheets?: SheetData[]): string {
  const m = expr.match(/^(\w+)\((.*)\)$/s)
  if (!m) return String(safeCalc(rows, expr))

  const fn = m[1].toUpperCase()
  const rawArgs = m[2]
  const args = splitArgs(rawArgs)
  const parsed = args.map(a => evalArg(rows, a, sheets))

  switch (fn) {
    case 'SUM': return String(numArray(rows, args.join(',')).reduce((a, b) => a + b, 0))
    case 'AVG': case 'AVERAGE': {
      const arr = numArray(rows, args.join(','))
      return arr.length ? String(arr.reduce((a, b) => a + b, 0) / arr.length) : '0'
    }
    case 'COUNT': return String(numArray(rows, args.join(',')).length)
    case 'COUNTA': return String(numArray(rows, args.join(','), false, true).length)
    case 'MAX': { const arr = numArray(rows, args.join(',')); return arr.length ? String(Math.max(...arr)) : '0' }
    case 'MIN': { const arr = numArray(rows, args.join(',')); return arr.length ? String(Math.min(...arr)) : '0' }
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
    case 'VALUE': return String(parseFloat(String(parsed[0])) || 0)
    case 'ROUND': return String(Math.round(Number(parsed[0]) * Math.pow(10, Number(parsed[1]) || 0)) / Math.pow(10, Number(parsed[1]) || 0))
    case 'CEIL': case 'CEILING': return String(Math.ceil(Number(parsed[0])))
    case 'FLOOR': return String(Math.floor(Number(parsed[0])))
    case 'ABS': return String(Math.abs(Number(parsed[0])))
    case 'MOD': return String(Number(parsed[0]) % Number(parsed[1]))
    case 'POWER': case 'POW': return String(Math.pow(Number(parsed[0]), Number(parsed[1])))
    case 'SQRT': return String(Math.sqrt(Number(parsed[0])))
    case 'INT': return String(Math.floor(Number(parsed[0])))
    case 'NOW': return new Date().toLocaleString('zh-CN')
    case 'TODAY': return new Date().toLocaleDateString('zh-CN')
    default: return '#NAME?'
  }
}

function evalArg(rows: string[][], a: string, sheets?: SheetData[]): any {
  a = a.trim()
  if ((a.startsWith('"') && a.endsWith('"')) || (a.startsWith("'") && a.endsWith("'"))) return a.slice(1, -1)

  // cross-sheet ref
  const crossRef = a.match(/^(\w+)!(.+)$/i)
  if (crossRef && sheets) {
    const si = sheets.findIndex(s => (s as any).name?.toLowerCase() === crossRef[1].toLowerCase())
    if (si < 0) return '#REF!'
    const ref = crossRef[2].toUpperCase()
    const rng = ref.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
    if (rng) {
      const c1 = colIndex(rng[1]), r1 = parseInt(rng[2]) - 1, c2 = colIndex(rng[3]), r2 = parseInt(rng[4]) - 1
      const res: any[] = []
      for (let r = r1; r <= r2; r++) for (let c = c1; c <= c2; c++) {
        const v = resolveCellRaw(rows, r, c, sheets, si)
        if (v !== '') { const n = parseFloat(v); res.push(isNaN(n) ? v : n) } else res.push('')
      }
      return res
    }
    const cr = ref.match(/^([A-Z]+)(\d+)$/)
    if (cr) {
      const v = resolveCellRaw(rows, parseInt(cr[2]) - 1, colIndex(cr[1]), sheets, si)
      if (v === '') return ''
      const n = parseFloat(v); return isNaN(n) ? v : n
    }
    return '#REF!'
  }

  // cell ref
  const cr = a.match(/^([A-Z]+)(\d+)$/)
  if (cr) return getCellVal(rows, cr[1], parseInt(cr[2]), sheets)

  // range ref
  const rng = a.match(/^([A-Z]+)(\d+):([A-Z]+)(\d+)$/)
  if (rng) return getRange(rows, a)

  if (!isNaN(Number(a))) return Number(a)
  if (a.toUpperCase() === 'TRUE') return true
  if (a.toUpperCase() === 'FALSE') return false
  // try as expression (e.g. 1<0, A1+5)
  if (/[<>=!+×/÷*-]/.test(a)) {
    const result = safeCalc(rows, a)
    if (!isNaN(result)) return result
  }
  return a
}

/**
 * Top-level: evaluate a formula string like "=SUM(A1:A3)"
 * rows is a 2D string array representing the current sheet.
 */
export function computeFormulaRaw(expr: string, rows: string[][], _sheets?: SheetData[], _sheetIdx?: number): string {
  if (!expr.startsWith('=')) return expr
  try { return evalFormula(rows, expr.slice(1), _sheets) } catch { return '#ERROR!' }
}

/**
 * Convenience: evaluate a formula against a simple 2D grid.
 */
export function evalExpr(expr: string, grid: string[][]): string {
  return computeFormulaRaw(expr, grid)
}
