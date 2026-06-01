package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ==================== colLetterToIdx 单元测试 ====================

func TestColLetterToIdx(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		// 单列 A-Z → index 0-25
		{"A", 0}, {"B", 1}, {"C", 2}, {"D", 3}, {"E", 4},
		{"F", 5}, {"G", 6}, {"H", 7}, {"I", 8}, {"J", 9},
		{"K", 10}, {"L", 11}, {"M", 12}, {"N", 13}, {"O", 14},
		{"P", 15}, {"Q", 16}, {"R", 17}, {"S", 18}, {"T", 19},
		{"U", 20}, {"V", 21}, {"W", 22}, {"X", 23}, {"Y", 24},
		{"Z", 25},
		// 双列
		{"AA", 26}, {"AB", 27}, {"AC", 28}, {"AZ", 51},
		{"BA", 52}, {"BB", 53}, {"CA", 78},
		{"ZA", 676}, {"ZZ", 701},
		// 三列
		{"AAA", 702}, {"AAB", 703},
	}

	for _, tt := range tests {
		got := colLetterToIdx(tt.input)
		if got != tt.want {
			t.Errorf("colLetterToIdx(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// ==================== splitCellRef 测试 ====================

func TestSplitCellRef(t *testing.T) {
	tests := []struct {
		input   string
		wantCol string
		wantRow string
	}{
		{"A1", "A", "1"},
		{"Z26", "Z", "26"},
		{"AA1", "AA", "1"},
		{"AB12", "AB", "12"},
		{"AZ100", "AZ", "100"},
	}
	for _, tt := range tests {
		col, row := splitCellRef(tt.input)
		if col != tt.wantCol || row != tt.wantRow {
			t.Errorf("splitCellRef(%q) = (%q,%q), want (%q,%q)",
				tt.input, col, row, tt.wantCol, tt.wantRow)
		}
	}
}

// ==================== parseSimpleInt 测试 ====================

func TestParseSimpleInt(t *testing.T) {
	if n, _ := parseSimpleInt("42"); n != 42 {
		t.Errorf("parseSimpleInt(42) = %d", n)
	}
	if n, _ := parseSimpleInt("0"); n != 0 {
		t.Errorf("parseSimpleInt(0) = %d", n)
	}
	if _, err := parseSimpleInt(""); err == nil {
		t.Error("empty string should error")
	}
	if _, err := parseSimpleInt("abc"); err == nil {
		t.Error("non-digit should error")
	}
}

// ==================== xlsx 辅助 ====================

const testContentTypes = `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
</Types>`

func idxToColLetter(idx int) string {
	idx++ // 1-based
	s := ""
	for idx > 0 {
		idx--
		s = string(rune('A'+idx%26)) + s
		idx /= 26
	}
	return s
}

// buildXLSX 构造最小 xlsx 文件
// cells: "A1"→"value"，值为空字符串则生成数值型单元格
func buildXLSX(t *testing.T, cells map[string]string) []byte {
	t.Helper()

	// 收集 sharedStrings（仅非空字符串值）
	ssArr := []string{}
	ssMap := map[string]int{}
	numericCells := map[string]string{} // ref → numeric value

	for ref, v := range cells {
		// 判断是否数字
		if isNumeric(v) {
			numericCells[ref] = v
		} else {
			if _, ok := ssMap[v]; !ok {
				ssMap[v] = len(ssArr)
				ssArr = append(ssArr, v)
			}
		}
	}

	// sharedStrings.xml
	var sst strings.Builder
	sst.WriteString(`<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	for _, s := range ssArr {
		escaped := strings.ReplaceAll(s, "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		sst.WriteString(`<si><t>` + escaped + `</t></si>`)
	}
	sst.WriteString(`</sst>`)

	// sheet1.xml
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	sheet.WriteString(`<row r="1">`)
	for ref, v := range cells {
		if nv, ok := numericCells[ref]; ok {
			sheet.WriteString(fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, nv))
		} else {
			sheet.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, ssMap[v]))
		}
	}
	sheet.WriteString(`</row></sheetData></worksheet>`)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("[Content_Types].xml")
	f.Write([]byte(testContentTypes))
	f, _ = w.Create("xl/sharedStrings.xml")
	f.Write([]byte(sst.String()))
	f, _ = w.Create("xl/worksheets/sheet1.xml")
	f.Write([]byte(sheet.String()))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func isNumeric(s string) bool {
	hasDigit := false
	for _, c := range s {
		if (c >= '0' && c <= '9') || c == '.' || c == '-' {
			hasDigit = true
		} else {
			return false
		}
	}
	return hasDigit
}

func parseSheetResult(t *testing.T, jsonStr string) [][]string {
	t.Helper()
	var rows [][]string
	if err := json.Unmarshal([]byte(jsonStr), &rows); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	return rows
}

// ==================== xlsxToSheet 集成测试 ====================

func TestXlsxToSheetBasic3Cols(t *testing.T) {
	result, err := xlsxToSheet(buildXLSX(t, map[string]string{
		"A1": "姓名", "B1": "年龄", "C1": "城市",
		"A2": "张三", "B2": "25",   "C2": "北京",
	}))
	if err != nil {
		t.Fatal(err)
	}
	rows := parseSheetResult(t, result)

	if rows[0][0] != "姓名" { t.Error("A1 mismatch") }
	if rows[0][1] != "年龄" { t.Error("B1 mismatch") }
	if rows[0][2] != "城市" { t.Error("C1 mismatch") }
	if rows[1][0] != "张三" { t.Error("A2 mismatch") }
}

func TestXlsxToSheet6Cols(t *testing.T) {
	result, err := xlsxToSheet(buildXLSX(t, map[string]string{
		"A1": "C1", "B1": "C2", "C1": "C3", "D1": "C4", "E1": "C5", "F1": "C6",
	}))
	if err != nil {
		t.Fatal(err)
	}
	rows := parseSheetResult(t, result)

	want := []string{"C1", "C2", "C3", "C4", "C5", "C6"}
	for i, w := range want {
		if rows[0][i] != w {
			t.Errorf("col %d (%s): got %q, want %q", i, idxToColLetter(i), rows[0][i], w)
		}
	}
}

func TestXlsxToSheetBeyondZ(t *testing.T) {
	// AA=26, AB=27, AZ=51 列
	result, err := xlsxToSheet(buildXLSX(t, map[string]string{
		"Z1": "ColZ", "AA1": "ColAA", "AB1": "ColAB", "AZ1": "ColAZ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	rows := parseSheetResult(t, result)

	if rows[0][25] != "ColZ" {
		t.Errorf("Z (idx 25): got %q", rows[0][25])
	}
	if len(rows[0]) <= 26 || rows[0][26] != "ColAA" {
		t.Errorf("AA (idx 26): got %q, cols=%d", safeGet(rows, 0, 26), len(rows[0]))
	}
	if len(rows[0]) <= 27 || rows[0][27] != "ColAB" {
		t.Errorf("AB (idx 27): got %q, cols=%d", safeGet(rows, 0, 27), len(rows[0]))
	}
	if len(rows[0]) <= 51 || rows[0][51] != "ColAZ" {
		t.Errorf("AZ (idx 51): got %q, cols=%d", safeGet(rows, 0, 51), len(rows[0]))
	}
}

func TestXlsxToSheetSparse(t *testing.T) {
	// 只有 A1 和 Z1，中间全空
	result, err := xlsxToSheet(buildXLSX(t, map[string]string{
		"A1": "First", "Z1": "Last",
	}))
	if err != nil {
		t.Fatal(err)
	}
	rows := parseSheetResult(t, result)

	if rows[0][0] != "First" { t.Error("A1") }
	if rows[0][25] != "Last" { t.Error("Z1") }
	for i := 1; i < 25; i++ {
		if rows[0][i] != "" {
			t.Errorf("col %d should be empty, got %q", i, rows[0][i])
		}
	}
}

func TestXlsxToSheetNumeric(t *testing.T) {
	// 纯数值不走 sharedStrings
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	sheet.WriteString(`<row r="1">`)
	sheet.WriteString(`<c r="A1"><v>42</v></c>`)
	sheet.WriteString(`<c r="B1"><v>3.14</v></c>`)
	sheet.WriteString(`</row></sheetData></worksheet>`)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("[Content_Types].xml")
	f.Write([]byte(testContentTypes))
	f, _ = w.Create("xl/sharedStrings.xml")
	f.Write([]byte(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`))
	f, _ = w.Create("xl/worksheets/sheet1.xml")
	f.Write([]byte(sheet.String()))
	w.Close()

	result, err := xlsxToSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	rows := parseSheetResult(t, result)

	if rows[0][0] != "42" { t.Errorf("A1: %q", rows[0][0]) }
	if rows[0][1] != "3.14" { t.Errorf("B1: %q", rows[0][1]) }
}

func safeGet(rows [][]string, r, c int) string {
	if r < len(rows) && c < len(rows[r]) {
		return rows[r][c]
	}
	return "(out of range)"
}
