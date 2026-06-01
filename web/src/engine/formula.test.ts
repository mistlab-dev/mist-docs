import { describe, it, expect } from 'vitest'
import { evalExpr } from './formula.testable'

// grid builds a 2D array: each argument is one ROW
// grid(['10','20'], ['30','40']) = 2 rows x 2 cols
//   A1=10 B1=20  A2=30 B2=40
function grid(...rows: string[][]): string[][] { return rows }

// shorthand: column vector (each value = one row, col A only)
function col(...vals: string[]): string[][] {
  return vals.map(v => [v])
}

describe('formula engine', () => {
  // ─── basic ───
  it('plain number', () => {
    expect(evalExpr('=42', grid(['10', '20']))).toBe('42')
  })

  it('simple arithmetic', () => {
    expect(evalExpr('=1+2+3', grid([]))).toBe('6')
  })

  it('cell reference arithmetic', () => {
    const g = col('10', '20', '30')
    expect(evalExpr('=A1+A2+A3', g)).toBe('60')
  })

  // ─── SUM ───
  it('SUM range of plain numbers', () => {
    const g = col('10', '20', '30')
    expect(evalExpr('=SUM(A1:A3)', g)).toBe('60')
  })

  it('SUM range with empty cells', () => {
    const g = col('10', '', '30')
    expect(evalExpr('=SUM(A1:A3)', g)).toBe('40')
  })

  it('SUM range with text cells (skipped)', () => {
    const g = col('10', 'hello', '30')
    expect(evalExpr('=SUM(A1:A3)', g)).toBe('40')
  })

  it('SUM with literal args', () => {
    expect(evalExpr('=SUM(1,2,3)', grid([]))).toBe('6')
  })

  it('SUM multi-column range', () => {
    const g = grid(['10', '20'], ['30', '40'])
    expect(evalExpr('=SUM(A1:B2)', g)).toBe('100')
  })

  // ─── formula referencing formula cells ───
  it('SUM range where some cells are formulas', () => {
    const g = col('10', '=A1*2', '30')
    expect(evalExpr('=SUM(A1:A3)', g)).toBe('60')
  })

  it('nested formula chain: A2=A1+10, A3=A2*2', () => {
    const g = col('5', '=A1+10', '=A2*2')
    expect(evalExpr('=A3', g)).toBe('30')
  })

  it('SUM of formula cells', () => {
    const g = col('5', '=A1*2', '=A1+5')
    expect(evalExpr('=SUM(A1:A3)', g)).toBe('25')
  })

  // ─── AVERAGE ───
  it('AVERAGE', () => {
    const g = col('10', '20', '30')
    expect(evalExpr('=AVERAGE(A1:A3)', g)).toBe('20')
  })

  it('AVG alias', () => {
    const g = col('10', '20', '30')
    expect(evalExpr('=AVG(A1:A3)', g)).toBe('20')
  })

  // ─── COUNT / COUNTA ───
  it('COUNT numbers only', () => {
    const g = col('10', 'hello', '30', '')
    expect(evalExpr('=COUNT(A1:A4)', g)).toBe('2')
  })

  it('COUNTA (non-empty)', () => {
    const g = col('10', 'hello', '30', '')
    expect(evalExpr('=COUNTA(A1:A4)', g)).toBe('3')
  })

  // ─── MAX / MIN ───
  it('MAX', () => {
    const g = col('10', '5', '30', '2')
    expect(evalExpr('=MAX(A1:A4)', g)).toBe('30')
  })

  it('MIN', () => {
    const g = col('10', '5', '30', '2')
    expect(evalExpr('=MIN(A1:A4)', g)).toBe('2')
  })

  // ─── IF ───
  it('IF true branch', () => {
    expect(evalExpr('=IF(1>0,"yes","no")', grid([]))).toBe('yes')
  })

  it('IF false branch', () => {
    expect(evalExpr('=IF(1<0,"yes","no")', grid([]))).toBe('no')
  })

  it('IF with cell ref condition', () => {
    const g = col('100')
    expect(evalExpr('=IF(A1>50,"big","small")', g)).toBe('big')
  })

  // ─── text functions ───
  it('CONCATENATE', () => {
    expect(evalExpr('=CONCATENATE("hello"," ","world")', grid([]))).toBe('hello world')
  })

  it('LEFT', () => {
    expect(evalExpr('=LEFT("hello",3)', grid([]))).toBe('hel')
  })

  it('RIGHT', () => {
    expect(evalExpr('=RIGHT("hello",2)', grid([]))).toBe('lo')
  })

  it('MID', () => {
    expect(evalExpr('=MID("hello",2,3)', grid([]))).toBe('ell')
  })

  it('LEN', () => {
    expect(evalExpr('=LEN("hello")', grid([]))).toBe('5')
  })

  it('UPPER', () => {
    expect(evalExpr('=UPPER("hello")', grid([]))).toBe('HELLO')
  })

  it('LOWER', () => {
    expect(evalExpr('=LOWER("HELLO")', grid([]))).toBe('hello')
  })

  it('TRIM', () => {
    expect(evalExpr('=TRIM("  hi  ")', grid([]))).toBe('hi')
  })

  it('SUBSTITUTE', () => {
    expect(evalExpr('=SUBSTITUTE("aabaa","b","c")', grid([]))).toBe('aacaa')
  })

  it('VALUE', () => {
    expect(evalExpr('=VALUE("42")', grid([]))).toBe('42')
  })

  // ─── math functions ───
  it('ROUND', () => {
    expect(evalExpr('=ROUND(3.14159,2)', grid([]))).toBe('3.14')
  })

  it('CEILING', () => {
    expect(evalExpr('=CEILING(3.14)', grid([]))).toBe('4')
  })

  it('FLOOR', () => {
    expect(evalExpr('=FLOOR(3.86)', grid([]))).toBe('3')
  })

  it('ABS', () => {
    expect(evalExpr('=ABS(-5)', grid([]))).toBe('5')
  })

  it('MOD', () => {
    expect(evalExpr('=MOD(10,3)', grid([]))).toBe('1')
  })

  it('POWER', () => {
    expect(evalExpr('=POWER(2,10)', grid([]))).toBe('1024')
  })

  it('SQRT', () => {
    expect(evalExpr('=SQRT(144)', grid([]))).toBe('12')
  })

  it('INT', () => {
    expect(evalExpr('=INT(3.99)', grid([]))).toBe('3')
  })

  // ─── edge cases ───
  it('empty range returns 0 for SUM', () => {
    const g = col('', '', '')
    expect(evalExpr('=SUM(A1:A3)', g)).toBe('0')
  })

  it('unknown function returns #NAME?', () => {
    expect(evalExpr('=FOOBAR(1)', grid([]))).toBe('#NAME?')
  })

  it('non-formula passes through', () => {
    expect(evalExpr('hello', grid([]))).toBe('hello')
  })

  // ─── real-world: invoice-style calculation ───
  it('invoice total: price * qty across rows, then SUM', () => {
    // A=qty, B=price, C=A*B
    const g = grid(
      ['2', '50', '=A1*B1'],   // row 1: 100
      ['3', '20', '=A2*B2'],   // row 2: 60
      ['1', '100', '=A3*B3'],  // row 3: 100
    )
    expect(evalExpr('=C1', g)).toBe('100')
    expect(evalExpr('=C2', g)).toBe('60')
    expect(evalExpr('=C3', g)).toBe('100')
    expect(evalExpr('=SUM(C1:C3)', g)).toBe('260')
  })

  // ─── nested SUM referencing SUM ───
  it('SUM where source cell itself uses SUM', () => {
    const g = grid(
      ['10', '=SUM(A1:A3)'],   // row1 B1=60
      ['20', '=SUM(A4:A5)'],   // row2 B2=90
      ['30', ''],
      ['40', ''],
      ['50', ''],
    )
    expect(evalExpr('=B1', g)).toBe('60')
    expect(evalExpr('=B2', g)).toBe('90')
    expect(evalExpr('=SUM(B1:B2)', g)).toBe('150')
  })

  // ─── logical ───
  it('AND all true', () => {
    expect(evalExpr('=AND(1,1,1)', grid([]))).toBe('true')
  })

  it('AND one false', () => {
    expect(evalExpr('=AND(1,0,1)', grid([]))).toBe('false')
  })

  it('OR all false', () => {
    expect(evalExpr('=OR(0,0,0)', grid([]))).toBe('false')
  })

  it('OR one true', () => {
    expect(evalExpr('=OR(0,1,0)', grid([]))).toBe('true')
  })

  it('NOT', () => {
    expect(evalExpr('=NOT(0)', grid([]))).toBe('true')
    expect(evalExpr('=NOT(1)', grid([]))).toBe('false')
  })
})

  // ─── multi-column selection: 选4列求和 ───
  it('SUM across 4 columns in one row', () => {
    // A1=1, B1=2, C1=3, D1=4 → SUM(A1:D1) = 10
    const g = [['1', '2', '3', '4']]
    expect(evalExpr('=SUM(A1:D1)', g)).toBe('10')
  })

  it('SUM across 5 columns in one row', () => {
    const g = [['1', '2', '3', '4', '5']]
    expect(evalExpr('=SUM(A1:E1)', g)).toBe('15')
  })

  it('AVERAGE across 4 columns in one row', () => {
    const g = [['10', '20', '30', '40']]
    expect(evalExpr('=AVERAGE(A1:D1)', g)).toBe('25')
  })

  it('SUM 4x3 block', () => {
    // 4列 x 3行
    const g = [
      ['1', '2', '3', '4'],     // 10
      ['5', '6', '7', '8'],     // 26
      ['9', '10', '11', '12'],  // 42
    ]
    expect(evalExpr('=SUM(A1:D3)', g)).toBe('78')
  })

  it('SUM selecting only columns B through E', () => {
    const g = [['a', '1', '2', '3', '4', 'b']]
    expect(evalExpr('=SUM(B1:E1)', g)).toBe('10')
  })

  // ─── 选中4列但数据是字符串数字 ───
  it('SUM 4 columns with string numbers in cells', () => {
    const g = [['1', '2', '3', '4']]
    expect(evalExpr('=SUM(A1:D1)', g)).toBe('10')
  })

  // 选中4列，有公式单元格
  it('SUM 4 columns where some cells are formulas', () => {
    const g = [['10', '=A1*2', '=A1+20', '=A1+30']]
    // 10 + 20 + 30 + 40 = 100
    expect(evalExpr('=SUM(A1:D1)', g)).toBe('100')
  })

  // 选整列4列
  it('SUM full column selection A:D', () => {
    const g = [
      ['1', '2', '3', '4'],
      ['5', '6', '7', '8'],
    ]
    expect(evalExpr('=SUM(A1:D2)', g)).toBe('36')
  })

  // 选4列但单元格值为数字字符串带空格
  it('SUM 4 columns with spaces in values', () => {
    const g = [[' 10 ', ' 20 ', ' 30 ', ' 40 ']]
    // parseFloat(' 10 ') = 10, 应该OK
    expect(evalExpr('=SUM(A1:D1)', g)).toBe('100')
  })
