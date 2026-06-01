package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"

	"fmt"
	"strings"
	"testing"
)

// ==================== 真实 xlsx 构造器 ====================
// 模拟 Excel 生成的 xlsx 文件结构，用于端到端测试

// xlsxBuilder 构造接近真实 Excel 的 xlsx 文件
type xlsxBuilder struct {
	sharedStrings []string
	sharedMap     map[string]int
	rows          map[int]map[int]string // rowNum(1-based) → colIdx(0-based) → value
	numericCells  map[int]map[int]string // rowNum → colIdx → raw numeric string
	maxRow        int
	maxCol        int
}

func newXlsxBuilder() *xlsxBuilder {
	return &xlsxBuilder{
		sharedMap:    make(map[string]int),
		rows:         make(map[int]map[int]string),
		numericCells: make(map[int]map[int]string),
	}
}

// SetString 设置字符串单元格
func (b *xlsxBuilder) SetString(row, col int, value string) {
	if _, ok := b.sharedMap[value]; !ok {
		b.sharedMap[value] = len(b.sharedStrings)
		b.sharedStrings = append(b.sharedStrings, value)
	}
	if b.rows[row] == nil {
		b.rows[row] = make(map[int]string)
	}
	b.rows[row][col] = value
	if row > b.maxRow {
		b.maxRow = row
	}
	if col > b.maxCol {
		b.maxCol = col
	}
}

// SetNumber 设置数值单元格
func (b *xlsxBuilder) SetNumber(row, col int, value string) {
	if b.numericCells[row] == nil {
		b.numericCells[row] = make(map[int]string)
	}
	b.numericCells[row][col] = value
	if row > b.maxRow {
		b.maxRow = row
	}
	if col > b.maxCol {
		b.maxCol = col
	}
}

// Build 生成 xlsx 字节
func (b *xlsxBuilder) Build(t *testing.T) []byte {
	t.Helper()

	// [Content_Types].xml
	ctXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

	// _rels/.rels
	relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

	// xl/_rels/workbook.xml.rels
	wbRelsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

	// xl/workbook.xml
	wbXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>
</workbook>`

	// xl/styles.xml (minimal)
	stylesXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<counts><numFmts count="1"/></counts></styleSheet>`

	// xl/sharedStrings.xml
	var sst strings.Builder
	sst.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sst.WriteString(fmt.Sprintf(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`,
		len(b.sharedStrings), len(b.sharedStrings)))
	for _, s := range b.sharedStrings {
		escaped := xmlEscape(s)
		sst.WriteString(`<si><t>` + escaped + `</t></si>`)
	}
	sst.WriteString(`</sst>`)

	// xl/worksheets/sheet1.xml
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	sheet.WriteString(`<sheetData>`)

	// 按 row 顺序输出（从 1 到 maxRow）
	for r := 1; r <= b.maxRow; r++ {
		rowHasData := b.rows[r] != nil || b.numericCells[r] != nil
		if !rowHasData {
			continue
		}
		sheet.WriteString(fmt.Sprintf(`<row r="%d">`, r))
		// 字符串单元格
		for c, v := range b.rows[r] {
			ref := idxToColLetter(c) + fmt.Sprintf("%d", r)
			sheet.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, b.sharedMap[v]))
		}
		// 数值单元格
		for c, v := range b.numericCells[r] {
			ref := idxToColLetter(c) + fmt.Sprintf("%d", r)
			sheet.WriteString(fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, v))
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	// 打包 zip
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", ctXML)
	writeZipEntry(t, w, "_rels/.rels", relsXML)
	writeZipEntry(t, w, "xl/workbook.xml", wbXML)
	writeZipEntry(t, w, "xl/_rels/workbook.xml.rels", wbRelsXML)
	writeZipEntry(t, w, "xl/styles.xml", stylesXML)
	writeZipEntry(t, w, "xl/sharedStrings.xml", sst.String())
	writeZipEntry(t, w, "xl/worksheets/sheet1.xml", sheet.String())
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeZipEntry(t *testing.T, w *zip.Writer, name, content string) {
	t.Helper()
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// parseRows 解析 xlsxToSheet 返回的 JSON
func parseRows(t *testing.T, jsonStr string) [][]string {
	t.Helper()
	var rows [][]string
	if err := json.Unmarshal([]byte(jsonStr), &rows); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	return rows
}

// ==================== 测试用例 ====================

// TC1: 基础 3 列 × 2 行（姓名表）
func TestXlsxImport_Basic3x2(t *testing.T) {
	b := newXlsxBuilder()
	b.SetString(1, 0, "姓名")
	b.SetString(1, 1, "年龄")
	b.SetString(1, 2, "城市")
	b.SetString(2, 0, "张三")
	b.SetNumber(2, 1, "25")
	b.SetString(2, 2, "北京")

	rows := parseRows(t, mustXlsx(t, b))

	// 验证尺寸
	if len(rows) < 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if len(rows[0]) < 3 {
		t.Fatalf("expected 3 cols, got %d", len(rows[0]))
	}

	// 验证内容
	assertCell(t, rows, 0, 0, "姓名")
	assertCell(t, rows, 0, 1, "年龄")
	assertCell(t, rows, 0, 2, "城市")
	assertCell(t, rows, 1, 0, "张三")
	assertCell(t, rows, 1, 1, "25")
	assertCell(t, rows, 1, 2, "北京")
}

// TC2: 5 列宽表
func TestXlsxImport_5Columns(t *testing.T) {
	b := newXlsxBuilder()
	headers := []string{"序号", "姓名", "部门", "职位", "入职日期"}
	for i, h := range headers {
		b.SetString(1, i, h)
	}
	b.SetNumber(2, 0, "1")
	b.SetString(2, 1, "李四")
	b.SetString(2, 2, "研发部")
	b.SetString(2, 3, "高级工程师")
	b.SetString(2, 4, "2024-01-15")

	rows := parseRows(t, mustXlsx(t, b))

	for i, h := range headers {
		assertCell(t, rows, 0, i, h)
	}
	assertCell(t, rows, 1, 0, "1")
	assertCell(t, rows, 1, 1, "李四")
	assertCell(t, rows, 1, 4, "2024-01-15")
}

// TC3: 10 列大表
func TestXlsxImport_10Columns(t *testing.T) {
	b := newXlsxBuilder()
	for i := 0; i < 10; i++ {
		b.SetString(1, i, fmt.Sprintf("Column%d", i+1))
		b.SetString(2, i, fmt.Sprintf("R2C%d", i+1))
	}

	rows := parseRows(t, mustXlsx(t, b))

	if len(rows[0]) < 10 {
		t.Fatalf("expected 10 cols, got %d", len(rows[0]))
	}
	for i := 0; i < 10; i++ {
		assertCell(t, rows, 0, i, fmt.Sprintf("Column%d", i+1))
		assertCell(t, rows, 1, i, fmt.Sprintf("R2C%d", i+1))
	}
}

// TC4: 26 列（A-Z 完整）
func TestXlsxImport_26ColumnsAZ(t *testing.T) {
	b := newXlsxBuilder()
	for i := 0; i < 26; i++ {
		col := idxToColLetter(i)
		b.SetString(1, i, "H_"+col)
		b.SetString(2, i, "D_"+col)
	}

	rows := parseRows(t, mustXlsx(t, b))

	if len(rows[0]) < 26 {
		t.Fatalf("expected 26 cols, got %d", len(rows[0]))
	}
	// 验证头尾
	assertCell(t, rows, 0, 0, "H_A")
	assertCell(t, rows, 0, 25, "H_Z")
	assertCell(t, rows, 1, 0, "D_A")
	assertCell(t, rows, 1, 25, "D_Z")
}

// TC5: 超过 Z 列 — AA, AB, AZ 列
func TestXlsxImport_BeyondZColumn(t *testing.T) {
	b := newXlsxBuilder()
	// Z(25), AA(26), AB(27), AZ(51)
	b.SetString(1, 25, "Col_Z")
	b.SetString(1, 26, "Col_AA")
	b.SetString(1, 27, "Col_AB")
	b.SetString(1, 51, "Col_AZ")

	rows := parseRows(t, mustXlsx(t, b))

	assertCell(t, rows, 0, 25, "Col_Z")
	assertCell(t, rows, 0, 26, "Col_AA")
	assertCell(t, rows, 0, 27, "Col_AB")
	assertCell(t, rows, 0, 51, "Col_AZ")
	// 中间空列应该是空字符串
	assertCell(t, rows, 0, 0, "")
	assertCell(t, rows, 0, 28, "")
}

// TC6: 稀疏数据 — 只有 A1 和 D4
func TestXlsxImport_SparseData(t *testing.T) {
	b := newXlsxBuilder()
	b.SetString(1, 0, "TopLeft")
	b.SetString(4, 3, "D4")

	rows := parseRows(t, mustXlsx(t, b))

	assertCell(t, rows, 0, 0, "TopLeft")
	assertCell(t, rows, 3, 3, "D4")
	// 中间都是空的
	assertCell(t, rows, 1, 0, "")
	assertCell(t, rows, 2, 1, "")
}

// TC7: 单列数据
func TestXlsxImport_SingleColumn(t *testing.T) {
	b := newXlsxBuilder()
	for i := 1; i <= 5; i++ {
		b.SetString(i, 0, fmt.Sprintf("Row%d", i))
	}

	rows := parseRows(t, mustXlsx(t, b))

	if len(rows) < 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	for i := 0; i < 5; i++ {
		assertCell(t, rows, i, 0, fmt.Sprintf("Row%d", i+1))
	}
}

// TC8: 单行数据（1行×多列）
func TestXlsxImport_SingleRow(t *testing.T) {
	b := newXlsxBuilder()
	cols := []string{"A", "B", "C", "D", "E"}
	for i, c := range cols {
		b.SetString(1, i, c)
	}

	rows := parseRows(t, mustXlsx(t, b))

	if len(rows) < 1 {
		t.Fatal("expected at least 1 row")
	}
	for i, c := range cols {
		assertCell(t, rows, 0, i, c)
	}
}

// TC9: 全是数值
func TestXlsxImport_AllNumeric(t *testing.T) {
	b := newXlsxBuilder()
	b.SetString(1, 0, "数值1")
	b.SetString(1, 1, "数值2")
	b.SetNumber(2, 0, "100")
	b.SetNumber(2, 1, "3.14159")
	b.SetNumber(3, 0, "-42")
	b.SetNumber(3, 1, "0")

	rows := parseRows(t, mustXlsx(t, b))

	assertCell(t, rows, 1, 0, "100")
	assertCell(t, rows, 1, 1, "3.14159")
	assertCell(t, rows, 2, 0, "-42")
	assertCell(t, rows, 2, 1, "0")
}

// TC10: 空表（只有 header）
func TestXlsxImport_HeaderOnly(t *testing.T) {
	b := newXlsxBuilder()
	b.SetString(1, 0, "Col1")
	b.SetString(1, 1, "Col2")

	rows := parseRows(t, mustXlsx(t, b))

	if len(rows) < 1 {
		t.Fatal("expected at least 1 row")
	}
	assertCell(t, rows, 0, 0, "Col1")
	assertCell(t, rows, 0, 1, "Col2")
}

// TC11: 包含特殊字符（&, <, >, 中文, 日文, emoji）
func TestXlsxImport_SpecialChars(t *testing.T) {
	b := newXlsxBuilder()
	b.SetString(1, 0, "Tom & Jerry")
	b.SetString(1, 1, "A<B>C")
	b.SetString(2, 0, "中文测试")
	b.SetString(2, 1, "こんにちは")
	b.SetString(3, 0, "café résumé")

	rows := parseRows(t, mustXlsx(t, b))

	assertCell(t, rows, 0, 0, "Tom & Jerry")
	assertCell(t, rows, 0, 1, "A<B>C")
	assertCell(t, rows, 1, 0, "中文测试")
	assertCell(t, rows, 1, 1, "こんにちは")
	assertCell(t, rows, 2, 0, "café résumé")
}

// TC12: 长文本单元格
func TestXlsxImport_LongText(t *testing.T) {
	b := newXlsxBuilder()
	longText := strings.Repeat("这是一段很长的文本用于测试。", 50) // ~450 chars
	b.SetString(1, 0, longText)

	rows := parseRows(t, mustXlsx(t, b))

	if rows[0][0] != longText {
		t.Errorf("long text mismatch: got %d chars, want %d", len(rows[0][0]), len(longText))
	}
}

// TC13: 跳跃行号（Excel 里行号可以不连续）
func TestXlsxImport_SkippedRows(t *testing.T) {
	b := newXlsxBuilder()
	b.SetString(1, 0, "Row1")
	// 跳过第 2 行
	b.SetString(3, 0, "Row3")
	b.SetString(5, 0, "Row5")

	rows := parseRows(t, mustXlsx(t, b))

	assertCell(t, rows, 0, 0, "Row1")
	assertCell(t, rows, 2, 0, "Row3")
	assertCell(t, rows, 4, 0, "Row5")
	// 空行应该是空字符串
	assertCell(t, rows, 1, 0, "")
	assertCell(t, rows, 3, 0, "")
}

// TC14: 30 列（A-AD），验证多列完整性
func TestXlsxImport_30Columns(t *testing.T) {
	b := newXlsxBuilder()
	for i := 0; i < 30; i++ {
		b.SetString(1, i, fmt.Sprintf("C%d_%s", i+1, idxToColLetter(i)))
	}

	rows := parseRows(t, mustXlsx(t, b))

	if len(rows[0]) < 30 {
		t.Fatalf("expected 30 cols, got %d", len(rows[0]))
	}
	for i := 0; i < 30; i++ {
		expected := fmt.Sprintf("C%d_%s", i+1, idxToColLetter(i))
		assertCell(t, rows, 0, i, expected)
	}
}

// TC15: 52 列（A-AZ），跨双字母列
func TestXlsxImport_52Columns(t *testing.T) {
	b := newXlsxBuilder()
	for i := 0; i < 52; i++ {
		b.SetString(1, i, idxToColLetter(i))
	}

	rows := parseRows(t, mustXlsx(t, b))

	if len(rows[0]) < 52 {
		t.Fatalf("expected 52 cols, got %d", len(rows[0]))
	}
	// 验证关键列
	assertCell(t, rows, 0, 0, "A")
	assertCell(t, rows, 0, 25, "Z")
	assertCell(t, rows, 0, 26, "AA")
	assertCell(t, rows, 0, 51, "AZ")
}

// ==================== 辅助函数 ====================

func mustXlsx(t *testing.T, b *xlsxBuilder) string {
	t.Helper()
	result, err := xlsxToSheet(b.Build(t))
	if err != nil {
		t.Fatalf("xlsxToSheet error: %v", err)
	}
	return result
}

func assertCell(t *testing.T, rows [][]string, r, c int, expected string) {
	t.Helper()
	if r >= len(rows) {
		t.Errorf("row %d out of range (total %d rows), expected %q", r, len(rows), expected)
		return
	}
	if c >= len(rows[r]) {
		t.Errorf("col %d out of range (total %d cols in row %d), expected %q", c, len(rows[r]), r, expected)
		return
	}
	if rows[r][c] != expected {
		t.Errorf("cell[%d][%d] = %q, want %q", r, c, rows[r][c], expected)
	}
}
