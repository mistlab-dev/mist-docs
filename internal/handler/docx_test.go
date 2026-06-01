package handler

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// ==================== docx 构造器 ====================

type docxBuilder struct {
	paragraphs []docxParagraph
}

type docxRun struct {
	text    string
	bold    bool
	italic  bool
	underline bool
	strike  bool
}

type docxParagraph struct {
	style string     // e.g. "Heading1", "" for normal
	runs  []docxRun // text runs
}

func newDocxBuilder() *docxBuilder {
	return &docxBuilder{}
}

func (d *docxBuilder) AddParagraph(style string, text string) {
	d.paragraphs = append(d.paragraphs, docxParagraph{style: style, runs: []docxRun{{text: text}}})
}

func (d *docxBuilder) AddFormattedParagraph(style string, runs []docxRun) {
	d.paragraphs = append(d.paragraphs, docxParagraph{style: style, runs: runs})
}

func (d *docxBuilder) AddEmptyParagraph() {
	d.paragraphs = append(d.paragraphs, docxParagraph{style: "", runs: nil})
}

func (d *docxBuilder) Build(t *testing.T) []byte {
	t.Helper()

	// 构建 word/document.xml
	var doc strings.Builder
	doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	doc.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	doc.WriteString(`<w:body>`)
	for _, p := range d.paragraphs {
		doc.WriteString(`<w:p>`)
		if p.style != "" {
			doc.WriteString(`<w:pPr><w:pStyle w:val="` + xmlEscape(p.style) + `"/></w:pPr>`)
		}
		for _, run := range p.runs {
			doc.WriteString(`<w:r>`)
			if run.bold || run.italic || run.underline || run.strike {
				doc.WriteString(`<w:rPr>`)
				if run.bold {
					doc.WriteString(`<w:b/>`)
				}
				if run.italic {
					doc.WriteString(`<w:i/>`)
				}
				if run.underline {
					doc.WriteString(`<w:u/>`)
				}
				if run.strike {
					doc.WriteString(`<w:strike/>`)
				}
				doc.WriteString(`</w:rPr>`)
			}
			doc.WriteString(`<w:t>`)
			doc.WriteString(xmlEscape(run.text))
			doc.WriteString(`</w:t></w:r>`)
		}
		doc.WriteString(`</w:p>`)
	}
	doc.WriteString(`</w:body></w:document>`)

	// [Content_Types].xml
	ctXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

	// _rels/.rels
	relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", ctXML)
	writeZipEntry(t, w, "_rels/.rels", relsXML)
	writeZipEntry(t, w, "word/document.xml", doc.String())
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func mustDocxHTML(t *testing.T, d *docxBuilder) string {
	t.Helper()
	result, err := docxToHTML(d.Build(t))
	if err != nil {
		t.Fatalf("docxToHTML error: %v", err)
	}
	return result
}

// ==================== docx 导入测试 ====================

// TC1: 基础段落
func TestDocxImport_BasicParagraphs(t *testing.T) {
	d := newDocxBuilder()
	d.AddParagraph("", "Hello World")
	d.AddParagraph("", "第二段文字")

	html := mustDocxHTML(t, d)

	if !strings.Contains(html, "<p>Hello World</p>") {
		t.Errorf("missing paragraph, got: %s", html)
	}
	if !strings.Contains(html, "<p>第二段文字</p>") {
		t.Errorf("missing second paragraph, got: %s", html)
	}
}

// TC2: 标题（H1/H2/H3）
func TestDocxImport_Headings(t *testing.T) {
	d := newDocxBuilder()
	d.AddParagraph("Heading1", "一级标题")
	d.AddParagraph("Heading2", "二级标题")
	d.AddParagraph("Heading3", "三级标题")

	html := mustDocxHTML(t, d)

	if !strings.Contains(html, "<h1>一级标题</h1>") {
		t.Errorf("missing h1, got: %s", html)
	}
	if !strings.Contains(html, "<h2>二级标题</h2>") {
		t.Errorf("missing h2, got: %s", html)
	}
	if !strings.Contains(html, "<h3>三级标题</h3>") {
		t.Errorf("missing h3, got: %s", html)
	}
}

// TC3: 标题样式变体（不同 Word 版本生成的样式名）
func TestDocxImport_HeadingStyleVariants(t *testing.T) {
	tests := []struct {
		style    string
		expected string
	}{
		{"Heading1", "<h1>"},
		{"Titre1", "<h1>"},    // 法文 Word
		{"berschrift1", "<h1>"}, // 德文 Word（不含 Heading 但以 1 结尾）
		{"2", "<h2>"},         // 某些版本只输出数字
		{"3", "<h3>"},
		{"4", "<h3>"},         // h4 映射到 h3
	}

	for _, tt := range tests {
		d := newDocxBuilder()
		d.AddParagraph(tt.style, "Test")
		html := mustDocxHTML(t, d)
		if !strings.Contains(html, tt.expected) {
			t.Errorf("style %q → expected %q in: %s", tt.style, tt.expected, html)
		}
	}
}

// TC4: 空段落 → <p><br></p>
func TestDocxImport_EmptyParagraph(t *testing.T) {
	d := newDocxBuilder()
	d.AddParagraph("", "Text before")
	d.AddEmptyParagraph()
	d.AddParagraph("", "Text after")

	html := mustDocxHTML(t, d)

	if !strings.Contains(html, "<p><br></p>") {
		t.Errorf("empty paragraph should be <p><br></p>, got: %s", html)
	}
}

// TC5: HTML 特殊字符转义
func TestDocxImport_HTMLEscape(t *testing.T) {
	d := newDocxBuilder()
	d.AddParagraph("", "A < B > C & D")
	d.AddParagraph("", "<script>alert('xss')</script>")

	html := mustDocxHTML(t, d)

	if !strings.Contains(html, "A &lt; B &gt; C &amp; D") {
		t.Errorf("HTML escape failed, got: %s", html)
	}
	if strings.Contains(html, "<script>") {
		t.Errorf("XSS not escaped, got: %s", html)
	}
}

// TC6: 中文内容
func TestDocxImport_ChineseContent(t *testing.T) {
	d := newDocxBuilder()
	d.AddParagraph("Heading1", "项目报告")
	d.AddParagraph("", "本次会议讨论了以下内容：需求分析、系统设计、开发计划。")
	d.AddParagraph("", "负责人：张三、李四、王五")

	html := mustDocxHTML(t, d)

	if !strings.Contains(html, "<h1>项目报告</h1>") {
		t.Errorf("missing Chinese heading, got: %s", html)
	}
	if !strings.Contains(html, "张三") {
		t.Errorf("missing Chinese text, got: %s", html)
	}
}

// TC7: 空文档
func TestDocxImport_EmptyDocument(t *testing.T) {
	d := newDocxBuilder()
	// 不添加任何段落

	html := mustDocxHTML(t, d)

	if !strings.Contains(html, "空文档") {
		t.Errorf("empty doc should show placeholder, got: %s", html)
	}
}

// TC8: 混合标题和段落
func TestDocxImport_MixedContent(t *testing.T) {
	d := newDocxBuilder()
	d.AddParagraph("Heading1", "第一章")
	d.AddParagraph("", "这是第一章的内容。")
	d.AddParagraph("Heading2", "1.1 子标题")
	d.AddParagraph("", "这是子标题下的内容。")
	d.AddParagraph("Heading1", "第二章")
	d.AddParagraph("", "第二章内容。")

	html := mustDocxHTML(t, d)

	// 验证结构完整
	countH1 := strings.Count(html, "<h1>")
	countH2 := strings.Count(html, "<h2>")
	countP := strings.Count(html, "<p>")

	if countH1 != 2 {
		t.Errorf("expected 2 h1, got %d", countH1)
	}
	if countH2 != 1 {
		t.Errorf("expected 1 h2, got %d", countH2)
	}
	if countP != 3 {
		t.Errorf("expected 3 p, got %d", countP)
	}
}

// TC9: 长段落
func TestDocxImport_LongParagraph(t *testing.T) {
	d := newDocxBuilder()
	longText := strings.Repeat("这是一段很长的文本。", 100) // ~1200 chars
	d.AddParagraph("", longText)

	html := mustDocxHTML(t, d)

	if !strings.Contains(html, longText[:50]) {
		t.Errorf("long text truncated, got: %s...", html[:100])
	}
}

// TC10: 缺少 word/document.xml
func TestDocxImport_NoDocumentXML(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeZipEntry(t, w, "[Content_Types].xml", "<Types/>")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := docxToHTML(buf.Bytes())
	if err == nil {
		t.Error("expected error for missing document.xml")
	}
}

// TC11: 非法 zip 文件
func TestDocxImport_InvalidZip(t *testing.T) {
	_, err := docxToHTML([]byte("not a zip file"))
	if err == nil {
		t.Error("expected error for invalid zip")
	}
}

// TC12: 格式化文本（粗体/斜体/下划线/删除线）
func TestDocxImport_FormattedRuns(t *testing.T) {
	d := newDocxBuilder()
	d.AddFormattedParagraph("", []docxRun{
		{text: "正常 "},
		{text: "粗体", bold: true},
		{text: " "},
		{text: "斜体", italic: true},
		{text: " "},
		{text: "粗斜体", bold: true, italic: true},
		{text: " "},
		{text: "删除线", strike: true},
		{text: " "},
		{text: "下划线", underline: true},
	})

	html := mustDocxHTML(t, d)

	checks := []struct{ expected string }{
		{"正常"},
		{"<strong>粗体</strong>"},
		{"<em>斜体</em>"},
		{"<strong><em>粗斜体</em></strong>"},
		{"<s>删除线</s>"},
		{"<u>下划线</u>"},
	}
	for _, c := range checks {
		if !strings.Contains(html, c.expected) {
			t.Errorf("missing %q in: %s", c.expected, html)
		}
	}
}

// ==================== docx XML 结构验证 ====================

// 验证构造的 docx 能被正确解析
func TestDocxBuilder_XMLRoundTrip(t *testing.T) {
	d := newDocxBuilder()
	d.AddParagraph("Heading1", "Test Title")
	d.AddParagraph("", "Paragraph text")

	data := d.Build(t)

	// 验证 zip 结构
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			found = true
			rc, _ := f.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()

			var doc wDocument
			if err := xml.Unmarshal(content, &doc); err != nil {
				t.Fatalf("xml unmarshal: %v", err)
			}
			if len(doc.Body.Paragraphs) != 2 {
				t.Errorf("expected 2 paragraphs, got %d", len(doc.Body.Paragraphs))
			}
			if doc.Body.Paragraphs[0].PPr.PStyle.Val != "Heading1" {
				t.Errorf("first paragraph style = %q", doc.Body.Paragraphs[0].PPr.PStyle.Val)
			}
		}
	}
	if !found {
		t.Error("word/document.xml not found in zip")
	}
}
