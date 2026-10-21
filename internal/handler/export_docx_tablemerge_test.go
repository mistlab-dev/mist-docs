package handler

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// docxTableForTest returns the <w:tbl>…</w:tbl> fragment of the first table
// in the generated document.xml for html.
func docxTableForTest(t *testing.T, html string) string {
	t.Helper()
	doc := docxDocumentXMLForTest(t, html)
	start := strings.Index(doc, "<w:tbl>")
	end := strings.Index(doc, "</w:tbl>")
	if start < 0 || end < 0 {
		t.Fatalf("no <w:tbl> in document.xml")
	}
	return doc[start : end+len("</w:tbl>")]
}

// docxRowCount returns the number of <w:tr> in tbl.
func docxRowCount(tbl string) int {
	return strings.Count(tbl, "<w:tr>")
}

// docxRowSums parses each w:tr and returns, per row, the sum of gridSpan of
// its w:tc elements — the conservation invariant: every row must sum to the
// table's grid column count.
func docxRowSums(t *testing.T, tbl string) []int {
	t.Helper()
	trRe := regexp.MustCompile(`(?s)<w:tr>.*?</w:tr>`)
	tcRe := regexp.MustCompile(`(?s)<w:tc>.*?</w:tc>`)
	spanRe := regexp.MustCompile(`<w:gridSpan w:val="(\d+)"/>`)
	var sums []int
	for _, tr := range trRe.FindAllString(tbl, -1) {
		sum := 0
		for _, tc := range tcRe.FindAllString(tr, -1) {
			span := 1
			if m := spanRe.FindStringSubmatch(tc); m != nil {
				span, _ = strconv.Atoi(m[1])
			}
			sum += span
		}
		sums = append(sums, sum)
	}
	return sums
}

// TestHtmlToDocx_TableColspan guards acceptance #55-1: colspan=N maps to
// <w:gridSpan w:val="N"/>, one w:tc per real cell (not per covered column),
// and every row's gridSpan sum equals the grid column count (conservation).
func TestHtmlToDocx_TableColspan(t *testing.T) {
	html := `<table>
	  <thead><tr><th colspan="2">成绩汇总</th><th>备注</th></tr></thead>
	  <tbody>
	    <tr><td>张三</td><td>95</td><td>优秀</td></tr>
	    <tr><td colspan="3">合计：2 人</td></tr>
	  </tbody>
	</table>`
	tbl := docxTableForTest(t, html)

	// gridSpan values: header 2 + body-row-3 → exactly two gridSpan elements.
	if got := strings.Count(tbl, `<w:gridSpan w:val="2"/>`); got != 1 {
		t.Errorf("gridSpan val=2 count = %d, want 1", got)
	}
	if got := strings.Count(tbl, `<w:gridSpan w:val="3"/>`); got != 1 {
		t.Errorf("gridSpan val=3 count = %d, want 1", got)
	}
	// 3 grid columns (2 + 1 header, 3 body cells, 3 spanned).
	gridCols := strings.Count(tbl, "<w:gridCol ")
	if gridCols != 3 {
		t.Errorf("gridCol count = %d, want 3", gridCols)
	}
	// Conservation: every row's gridSpan sum == 3.
	for i, sum := range docxRowSums(t, tbl) {
		if sum != 3 {
			t.Errorf("row %d gridSpan sum = %d, want 3", i, sum)
		}
	}
	// Chinese text intact (no mojibake / no dropped chars).
	for _, want := range []string{"成绩汇总", "合计：2 人", "张三", "优秀"} {
		if !strings.Contains(tbl, want) {
			t.Errorf("missing Chinese text %q", want)
		}
	}
	// One w:tc per real cell: 1+1 header, 3 body, 1 footer = 5 w:tc.
	if got := strings.Count(tbl, "<w:tc>"); got != 6 {
		t.Errorf("w:tc count = %d, want 6 (one per real cell)", got)
	}
}

// TestHtmlToDocx_TableRowspan guards acceptance #55-2: rowspan=N starts with
// <w:vMerge w:val="restart"/> and each covered row gets an empty
// <w:tc><w:tcPr><w:vMerge/> continuation cell, so the w:tr sequence stays
// intact and every row keeps its full column count.
func TestHtmlToDocx_TableRowspan(t *testing.T) {
	html := `<table>
	  <tr><td rowspan="2">组别A</td><td>第一行</td></tr>
	  <tr><td>第二行</td></tr>
	  <tr><td>组别B</td><td>第三行</td></tr>
	</table>`
	tbl := docxTableForTest(t, html)

	// Exactly one restart, exactly one continuation placeholder.
	if got := strings.Count(tbl, `<w:vMerge w:val="restart"/>`); got != 1 {
		t.Errorf("vMerge restart count = %d, want 1", got)
	}
	if got := strings.Count(tbl, `<w:vMerge/>`); got != 1 {
		t.Errorf("vMerge continuation count = %d, want 1", got)
	}
	// w:tr sequence intact: one per HTML row.
	if got := docxRowCount(tbl); got != 3 {
		t.Errorf("w:tr count = %d, want 3", got)
	}
	// The continuation cell is <w:tc><w:tcPr><w:vMerge/> + tcW + <w:p/>.
	if !strings.Contains(tbl, `<w:tc><w:tcPr><w:vMerge/><w:tcW w:w="0" w:type="auto"/></w:tcPr><w:p/></w:tc>`) {
		t.Errorf("missing <w:vMerge/> continuation cell")
	}
	// Every row keeps both grid columns (2 gridCol, span sum 2 per row).
	if got := strings.Count(tbl, "<w:gridCol "); got != 2 {
		t.Errorf("gridCol count = %d, want 2", got)
	}
	for i, sum := range docxRowSums(t, tbl) {
		if sum != 2 {
			t.Errorf("row %d gridSpan sum = %d, want 2", i, sum)
		}
	}
	// The starting cell carries restart; no content leaks into the placeholder.
	if !strings.Contains(tbl, `<w:vMerge w:val="restart"/>`) ||
		!strings.Contains(tbl, "组别A") || !strings.Contains(tbl, "第二行") {
		t.Errorf("missing rowspan anchor/continuation content")
	}
}

// TestHtmlToDocx_TableMixedMergeRagged guards acceptance #55-3: colspan and
// rowspan combined with irregular rows. The grid must stay rectangular (all
// rows span the same number of columns) and the gridCol count must equal the
// maximum column count.
func TestHtmlToDocx_TableMixedMergeRagged(t *testing.T) {
	html := `<table>
	  <thead><tr><th rowspan="2">名称</th><th colspan="2">数据</th></tr>
	           <tr><th>东部</th><th>西部</th></tr></thead>
	  <tbody>
	    <tr><td>甲</td><td>1</td><td>2</td></tr>
	    <tr><td rowspan="2" colspan="2">乙（合并）</td><td>3</td></tr>
	    <tr><td>4</td></tr>
	    <tr><td>丙</td><td>5</td></tr>
	  </tbody>
	</table>`
	tbl := docxTableForTest(t, html)

	// 3 grid columns; the ragged last row (2 cells) still spans all 3.
	if got := strings.Count(tbl, "<w:gridCol "); got != 3 {
		t.Errorf("gridCol count = %d, want 3 (max column count)", got)
	}
	// Header spans: 名称 rowspan → 1 restart + 1 continuation; 数据 colspan=2.
	// gridSpan val=2 appears 3 times: 数据 (start), 乙 restart, and 乙's
	// continuation tc — a colspan+rowspan continuation is ONE tc carrying
	// both <w:gridSpan> and <w:vMerge/>, not N bare vMerge placeholders.
	if got := strings.Count(tbl, `<w:gridSpan w:val="2"/>`); got != 3 {
		t.Errorf("gridSpan val=2 count = %d, want 3 (数据 + 乙 restart + 乙 cont)", got)
	}
	// 乙: rowspan=2 + colspan=2 → 1 restart + 1 merged continuation.
	if got := strings.Count(tbl, `<w:vMerge w:val="restart"/>`); got != 2 {
		t.Errorf("vMerge restart count = %d, want 2 (名称 + 乙)", got)
	}
	if got := strings.Count(tbl, `<w:vMerge/>`); got != 2 {
		t.Errorf("vMerge continuation count = %d, want 2 (名称×1 + 乙×1)", got)
	}
	// The 乙 continuation tc must carry gridSpan=2 together with vMerge.
	if !strings.Contains(tbl, `<w:tc><w:tcPr><w:gridSpan w:val="2"/><w:vMerge/>`) {
		t.Errorf("missing merged continuation tc with gridSpan+vMerge")
	}
	// Conservation on every row: gridSpan sums all equal 3.
	for i, sum := range docxRowSums(t, tbl) {
		if sum != 3 {
			t.Errorf("row %d gridSpan sum = %d, want 3", i, sum)
		}
	}
	// w:tr sequence intact: 2 header + 4 body rows.
	if got := docxRowCount(tbl); got != 6 {
		t.Errorf("w:tr count = %d, want 6", got)
	}
	for _, want := range []string{"名称", "数据", "东部", "西部", "甲", "乙（合并）", "丙"} {
		if !strings.Contains(tbl, want) {
			t.Errorf("missing Chinese text %q", want)
		}
	}
	// CT_Tbl ordering: tblPr → tblGrid → first tr.
	pr := strings.Index(tbl, "</w:tblPr>")
	grid := strings.Index(tbl, "<w:tblGrid>")
	tr := strings.Index(tbl, "<w:tr>")
	if !(pr < grid && grid < tr) {
		t.Errorf("CT_Tbl order broken: tblPr@%d tblGrid@%d tr@%d", pr, grid, tr)
	}
}

// TestHtmlToDocx_TableGridWidthsFromColgroup: colgroup/width info wins over
// the equal-split fallback, and the widths still sum to 9000 twips.
func TestHtmlToDocx_TableGridWidthsFromColgroup(t *testing.T) {
	html := `<table><colgroup><col style="width:20%"><col style="width:80%"></colgroup>
	  <tr><td>甲</td><td>乙</td></tr></table>`
	tbl := docxTableForTest(t, html)
	re := regexp.MustCompile(`<w:gridCol w:w="(\d+)"/>`)
	var widths []int
	for _, m := range re.FindAllStringSubmatch(tbl, -1) {
		w, _ := strconv.Atoi(m[1])
		widths = append(widths, w)
	}
	if len(widths) != 2 || widths[0] != 1800 || widths[1] != 7200 {
		t.Errorf("colgroup widths = %v, want [1800 7200]", widths)
	}
	sum := 0
	for _, w := range widths {
		sum += w
	}
	if sum != docxTableWidthTwips {
		t.Errorf("width sum = %d, want %d", sum, docxTableWidthTwips)
	}
}

// TestHtmlToDocx_TableEqualSplitFallback: without colgroup info every column
// gets an equal share of 9000 twips (#51 behaviour preserved).
func TestHtmlToDocx_TableEqualSplitFallback(t *testing.T) {
	html := `<table><tr><td>甲</td><td>乙</td><td>丙</td></tr></table>`
	tbl := docxTableForTest(t, html)
	re := regexp.MustCompile(`<w:gridCol w:w="(\d+)"/>`)
	var widths []int
	for _, m := range re.FindAllStringSubmatch(tbl, -1) {
		w, _ := strconv.Atoi(m[1])
		widths = append(widths, w)
	}
	if len(widths) != 3 || widths[0] != 3000 || widths[1] != 3000 || widths[2] != 3000 {
		t.Errorf("fallback widths = %v, want [3000 3000 3000]", widths)
	}
}

// TestHtmlToDocx_TableRaggedShortFirstRow: when a later row widens the grid,
// the earlier short row must be padded so every row's span sum still equals
// the grid column count (the conservation invariant).
func TestHtmlToDocx_TableRaggedShortFirstRow(t *testing.T) {
	html := `<table>
	  <tr><td>A</td></tr>
	  <tr><td>B</td><td>C</td></tr>
	</table>`
	tbl := docxTableForTest(t, html)

	if got := strings.Count(tbl, "<w:gridCol "); got != 2 {
		t.Errorf("gridCol count = %d, want 2", got)
	}
	sums := docxRowSums(t, tbl)
	if len(sums) != 2 {
		t.Fatalf("row count = %d, want 2", len(sums))
	}
	for i, sum := range sums {
		if sum != 2 {
			t.Errorf("row %d gridSpan sum = %d, want 2 (short row must pad)", i, sum)
		}
	}
}

// TestHtmlToDocx_TableContAndEmptyHaveParagraph: CT_Tc requires at least one
// block-level child; every continuation and empty tc must end with <w:p/>.
func TestHtmlToDocx_TableContAndEmptyHaveParagraph(t *testing.T) {
	html := `<table>
	  <tr><td rowspan="2">左</td><td>1</td></tr>
	  <tr><td>2</td><td>3</td></tr>
	</table>`
	tbl := docxTableForTest(t, html)

	// Continuation tc: <w:vMerge/> followed by tcW + <w:p/>.
	if strings.Contains(tbl, `<w:vMerge/></w:tcPr></w:tc>`) {
		t.Errorf("continuation tc missing <w:p/>")
	}
	// Empty pad tc must also carry <w:p/>.
	if strings.Contains(tbl, `<w:tcW w:w="0" w:type="auto"/></w:tcPr></w:tc>`) {
		t.Errorf("empty pad tc missing <w:p/>")
	}
	if !strings.Contains(tbl, `<w:vMerge/><w:tcW w:w="0" w:type="auto"/></w:tcPr><w:p/></w:tc>`) {
		t.Errorf("continuation tc should be <w:vMerge/> + tcW + <w:p/>")
	}
	// Conservation: row2 = 1 cont + 2 cells = 3.
	for i, sum := range docxRowSums(t, tbl) {
		if sum != 3 {
			t.Errorf("row %d gridSpan sum = %d, want 3", i, sum)
		}
	}
}

// TestHtmlToDocx_TableSpanClamped: colspan/rowspan come from user HTML and
// are clamped to docxMaxTableSpan so a hostile colspan="50000" cannot blow
// up the exported XML.
func TestHtmlToDocx_TableSpanClamped(t *testing.T) {
	html := `<table><tr><td colspan="50000">宽</td></tr></table>`
	tbl := docxTableForTest(t, html)

	if got := strings.Count(tbl, "<w:gridCol "); got != docxMaxTableSpan {
		t.Errorf("gridCol count = %d, want clamped %d", got, docxMaxTableSpan)
	}
	want := `<w:gridSpan w:val="` + strconv.Itoa(docxMaxTableSpan) + `"/>`
	if !strings.Contains(tbl, want) {
		t.Errorf("missing clamped %s", want)
	}
}
