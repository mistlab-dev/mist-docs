package handler

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// ==================== markdownToHTML 黑盒测试 ====================
// 不看实现细节，从用户角度测试

// MD1: 基础段落
func TestMarkdown_BasicParagraph(t *testing.T) {
	html := markdownToHTML("Hello World")
	if !strings.Contains(html, "<p>Hello World</p>") {
		t.Errorf("got: %s", html)
	}
}

// MD2: 标题
func TestMarkdown_Headings(t *testing.T) {
	html := markdownToHTML("# H1\n## H2\n### H3")
	if !strings.Contains(html, "<h1>H1</h1>") {
		t.Error("missing h1")
	}
	if !strings.Contains(html, "<h2>H2</h2>") {
		t.Error("missing h2")
	}
	if !strings.Contains(html, "<h3>H3</h3>") {
		t.Error("missing h3")
	}
}

// MD3: 列表
func TestMarkdown_List(t *testing.T) {
	html := markdownToHTML("- item1\n- item2\n- item3")
	if !strings.Contains(html, "<ul>") || !strings.Contains(html, "</ul>") {
		t.Errorf("missing ul tags: %s", html)
	}
	if !strings.Contains(html, "<li>item1</li>") {
		t.Error("missing li")
	}
}

// MD4: 代码块
func TestMarkdown_CodeBlock(t *testing.T) {
	html := markdownToHTML("```\ncode here\n```")
	if !strings.Contains(html, "<pre><code>") || !strings.Contains(html, "</code></pre>") {
		t.Errorf("missing code block: %s", html)
	}
}

// MD5: 混合：标题+段落+列表+代码
func TestMarkdown_Mixed(t *testing.T) {
	md := `# Title

Some text

- item1
- item2

More text

` + "```" + `
code
` + "```"

	html := markdownToHTML(md)
	if !strings.Contains(html, "<h1>Title</h1>") {
		t.Error("missing h1")
	}
	if !strings.Contains(html, "<li>item1</li>") {
		t.Error("missing li")
	}
	if !strings.Contains(html, "<pre><code>") {
		t.Error("missing code block")
	}
}

// MD6: 空 markdown
func TestMarkdown_Empty(t *testing.T) {
	html := markdownToHTML("")
	if html != "" {
		// 空输入应该返回空
		t.Errorf("empty input should return empty, got: %q", html)
	}
}

// MD7: 只有空行
func TestMarkdown_OnlyBlankLines(t *testing.T) {
	html := markdownToHTML("\n\n\n")
	if html != "" {
		t.Errorf("blank lines should produce empty output, got: %q", html)
	}
}

// MD8: 列表后紧跟代码块
func TestMarkdown_ListThenCode(t *testing.T) {
	md := "- item1\n- item2\n```\ncode\n```"
	html := markdownToHTML(md)
	if !strings.Contains(html, "</ul>") {
		t.Error("list should be closed")
	}
	if !strings.Contains(html, "<pre><code>") {
		t.Error("code block should exist")
	}
}

// MD9: 代码块未关闭
func TestMarkdown_UnclosedCodeBlock(t *testing.T) {
	html := markdownToHTML("```\nopen code")
	if !strings.Contains(html, "<pre><code>") {
		t.Error("should open code block")
	}
	if !strings.Contains(html, "</code></pre>") {
		t.Error("should close code block at end")
	}
}

// MD10: Markdown 里的 HTML 特殊字符没转义
func TestMarkdown_NoHTMLEscape(t *testing.T) {
	html := markdownToHTML("A < B & C > D")
	// markdownToHTML 没有转义！如果直接输出就是 XSS
	if strings.Contains(html, "&lt;") || strings.Contains(html, "&amp;") {
		t.Log("OK: HTML chars escaped")
	} else {
		t.Errorf("POTENTIAL BUG: HTML special chars not escaped in markdown output: %s", html)
	}
}

// MD11: markdown 列表项包含 HTML
func TestMarkdown_ListItemWithHTML(t *testing.T) {
	html := markdownToHTML("- <script>alert(1)</script>")
	if strings.Contains(html, "<script>") {
		t.Errorf("POTENTIAL BUG: XSS in list item not escaped: %s", html)
	}
}

// ==================== textToHTML 黑盒测试 ====================

// TXT1: 基础文本
func TestTextToHTML_Basic(t *testing.T) {
	html := textToHTML("Hello")
	if !strings.Contains(html, "<p>Hello</p>") {
		t.Errorf("got: %s", html)
	}
}

// TXT2: 多行
func TestTextToHTML_MultiLine(t *testing.T) {
	html := textToHTML("Line1\nLine2")
	if !strings.Contains(html, "<p>Line1</p>") || !strings.Contains(html, "<p>Line2</p>") {
		t.Errorf("got: %s", html)
	}
}

// TXT3: 空行 → <br>
func TestTextToHTML_BlankLine(t *testing.T) {
	html := textToHTML("Line1\n\nLine2")
	if !strings.Contains(html, "<br>") {
		t.Errorf("blank line should produce <br>: %s", html)
	}
}

// TXT4: HTML 特殊字符没转义
func TestTextToHTML_NoHTMLEscape(t *testing.T) {
	html := textToHTML("A < B & C > D")
	if strings.Contains(html, "&lt;") || strings.Contains(html, "&amp;") {
		t.Log("OK: HTML chars escaped")
	} else {
		t.Errorf("POTENTIAL BUG: HTML special chars not escaped in text output: %s", html)
	}
}

// TXT5: XSS
func TestTextToHTML_XSS(t *testing.T) {
	html := textToHTML("<script>alert('xss')</script>")
	if strings.Contains(html, "<script>") {
		t.Errorf("POTENTIAL BUG: XSS not escaped: %s", html)
	}
}

// ==================== xlsxToSheet 更多边界 ====================

// XLSX: 没有工作表
func TestXlsxImport_NoWorksheet(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", "<Types/>")
	w.Close()

	_, err := xlsxToSheet(buf.Bytes())
	if err == nil {
		t.Error("expected error for no worksheet")
	}
}

// XLSX: sharedStrings 里有多段文本 (rich text)
// 真实 Excel 的 sharedStrings 可能包含多个 <r><t> 块
func TestXlsxImport_SharedStringsRichText(t *testing.T) {
	// 真实 Excel 的 sharedStrings 可能长这样:
	// <si><r><t>Hello</t></r><r><t> World</t></r></si>
	// 当前解析只取第一个 <t>，可能丢失内容
	sstXML := `<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<si><r><t>Hello</t></r><r><t> World</t></r></si>
</sst>`

	sheetXML := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData><row r="1"><c r="A1" t="s"><v>0</v></c></row></sheetData></worksheet>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", testContentTypes)
	writeZipEntry(t, w, "xl/sharedStrings.xml", sstXML)
	writeZipEntry(t, w, "xl/worksheets/sheet1.xml", sheetXML)
	w.Close()

	result, err := xlsxToSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	rows := parseRows(t, result)

	// BUG? 只解析到 "Hello" 而不是 "Hello World"？
	if rows[0][0] != "Hello World" {
		t.Errorf("POTENTIAL BUG: rich text shared string = %q, want %q. Rich text (multiple <r> runs) may not be fully parsed.", rows[0][0], "Hello World")
	}
}

// XLSX: 空单元格（有 <c> 但没有 <v>）
func TestXlsxImport_EmptyCellWithRef(t *testing.T) {
	sheetXML := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData><row r="1">
<c r="A1" t="s"><v>0</v></c>
<c r="B1"/>
<c r="C1" t="s"><v>1</v></c>
</row></sheetData></worksheet>`

	sstXML := `<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<si><t>A</t></si><si><t>C</t></si></sst>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", testContentTypes)
	writeZipEntry(t, w, "xl/sharedStrings.xml", sstXML)
	writeZipEntry(t, w, "xl/worksheets/sheet1.xml", sheetXML)
	w.Close()

	result, err := xlsxToSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	rows := parseRows(t, result)

	assertCell(t, rows, 0, 0, "A")
	assertCell(t, rows, 0, 1, "") // 空单元格
	assertCell(t, rows, 0, 2, "C")
}

// XLSX: 公式单元格（有 <f> 和 <v>）
func TestXlsxImport_FormulaCell(t *testing.T) {
	sheetXML := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData><row r="1">
<c r="A1"><v>10</v></c>
<c r="B1"><v>20</v></c>
<c r="C1"><f>SUM(A1:B1)</f><v>30</v></c>
</row></sheetData></worksheet>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", testContentTypes)
	writeZipEntry(t, w, "xl/sharedStrings.xml", `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`)
	writeZipEntry(t, w, "xl/worksheets/sheet1.xml", sheetXML)
	w.Close()

	result, err := xlsxToSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	rows := parseRows(t, result)

	assertCell(t, rows, 0, 0, "10")
	assertCell(t, rows, 0, 1, "20")
	assertCell(t, rows, 0, 2, "30") // 应该取 <v> 的值
}

// XLSX: 布尔值单元格
func TestXlsxImport_BooleanCell(t *testing.T) {
	sheetXML := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData><row r="1">
<c r="A1" t="b"><v>1</v></c>
<c r="B1" t="b"><v>0</v></c>
</row></sheetData></worksheet>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", testContentTypes)
	writeZipEntry(t, w, "xl/sharedStrings.xml", `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`)
	writeZipEntry(t, w, "xl/worksheets/sheet1.xml", sheetXML)
	w.Close()

	result, err := xlsxToSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	rows := parseRows(t, result)

	// 布尔类型 t="b"，当前代码不是 "s" 就直接取 <v>
	// 所以结果是 "1" 和 "0"，而不是 "true"/"false"
	t.Logf("Boolean A1=%q B1=%q (raw values, may need true/false conversion)", rows[0][0], rows[0][1])
}

// XLSX: 行号不按顺序（Excel 可能乱序输出）
func TestXlsxImport_RowOutOfOrder(t *testing.T) {
	sheetXML := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData>
<row r="5"><c r="A5" t="s"><v>0</v></c></row>
<row r="1"><c r="A1" t="s"><v>1</v></c></row>
<row r="3"><c r="A3" t="s"><v>2</v></c></row>
</sheetData></worksheet>`

	sstXML := `<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<si><t>Row5</t></si><si><t>Row1</t></si><si><t>Row3</t></si></sst>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", testContentTypes)
	writeZipEntry(t, w, "xl/sharedStrings.xml", sstXML)
	writeZipEntry(t, w, "xl/worksheets/sheet1.xml", sheetXML)
	w.Close()

	result, err := xlsxToSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	rows := parseRows(t, result)

	// 用 r 属性解析行号，应该正确映射
	assertCell(t, rows, 0, 0, "Row1")  // row 1 → index 0
	assertCell(t, rows, 2, 0, "Row3")  // row 3 → index 2
	assertCell(t, rows, 4, 0, "Row5")  // row 5 → index 4
}

// XLSX: 共享字符串中含 XML 命名空间
func TestXlsxImport_SharedStringWithNamespace(t *testing.T) {
	sstXML := `<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="1" uniqueCount="1">
<si><t xml:space="preserve">  spaced text  </t></si>
</sst>`

	sheetXML := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData><row r="1"><c r="A1" t="s"><v>0</v></c></row></sheetData></worksheet>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", testContentTypes)
	writeZipEntry(t, w, "xl/sharedStrings.xml", sstXML)
	writeZipEntry(t, w, "xl/worksheets/sheet1.xml", sheetXML)
	w.Close()

	result, err := xlsxToSheet(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	rows := parseRows(t, result)

	// xml:space="preserve" 应该保留前后空格
	val := rows[0][0]
	if val != "  spaced text  " {
		t.Errorf("POTENTIAL BUG: whitespace not preserved, got %q", val)
	}
}

// ==================== docxToHTML 更多边界 ====================

// DOCX: 多个 run 组成一个段落（Word 经常把一个句子拆成多个 run）
func TestDocxImport_MultipleRuns(t *testing.T) {
	// 真实 Word 可能生成: <w:r><w:t>Hello </w:t></w:r><w:r><w:t>World</w:t></w:r>
	var docXML strings.Builder
	docXML.WriteString(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	docXML.WriteString(`<w:p><w:r><w:t>Hello </w:t></w:r><w:r><w:t>World</w:t></w:r></w:p>`)
	docXML.WriteString(`</w:body></w:document>`)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	writeZipEntry(t, w, "_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	writeZipEntry(t, w, "word/document.xml", docXML.String())
	w.Close()

	html, err := docxToHTML(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, "<p>Hello World</p>") {
		t.Errorf("multiple runs should be concatenated, got: %s", html)
	}
}

// DOCX: 粗体/斜体 run（带 <w:rPr>）
func TestDocxImport_RunWithFormatting(t *testing.T) {
	// Word 带格式的 run: <w:r><w:rPr><w:b/></w:rPr><w:t>bold text</w:t></w:r>
	var docXML strings.Builder
	docXML.WriteString(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	docXML.WriteString(`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Bold text</w:t></w:r></w:p>`)
	docXML.WriteString(`</w:body></w:document>`)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	writeZipEntry(t, w, "_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	writeZipEntry(t, w, "word/document.xml", docXML.String())
	w.Close()

	html, err := docxToHTML(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	// 当前代码不处理粗体/斜体，只提取纯文本
	// 至少应该保留文字内容
	if !strings.Contains(html, "Bold text") {
		t.Errorf("bold text content missing: %s", html)
	}
	t.Logf("Formatting handling: %s (bold/italic tags converted)", html)
}
