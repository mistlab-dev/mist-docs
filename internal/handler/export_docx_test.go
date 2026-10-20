package handler

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"
)

// Helper to inspect the exported docx zip file
func inspectDocx(t *testing.T, data []byte) (documentXML string, relsXML string) {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	foundParts := make(map[string]bool)
	for _, f := range r.File {
		foundParts[f.Name] = true
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content, _ := io.ReadAll(rc)
		rc.Close()

		// Validate well-formed XML for all XML files
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			var dummy any
			if err := xml.Unmarshal(content, &dummy); err != nil {
				t.Errorf("%s is not well-formed XML: %v", f.Name, err)
			}
		}

		if f.Name == "word/document.xml" {
			documentXML = string(content)
		} else if f.Name == "word/_rels/document.xml.rels" {
			relsXML = string(content)
		}
	}

	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/styles.xml", "word/_rels/document.xml.rels"} {
		if !foundParts[required] {
			t.Errorf("missing required docx part: %s", required)
		}
	}

	return documentXML, relsXML
}

func TestHtmlToDocx_NoDoubleUnescape(t *testing.T) {
	// B1: &amp;lt;div&amp;gt; in HTML is literally "&lt;div&gt;"
	input := `<p>use &amp;lt;div&amp;gt;</p>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx double unescape: %v", err)
	}
	docXML, _ := inspectDocx(t, data)
	// Should contain &amp;lt;div&amp;gt; in the XML (which represents literal &lt;div&gt;)
	// NOT <w:t>&lt;div&gt;</w:t> (which would be literal <div>)
	if strings.Contains(docXML, `<w:t xml:space="preserve">&lt;div&gt;</w:t>`) {
		t.Errorf("entity was double-unescaped into literal <div>: %s", docXML)
	}
	if !strings.Contains(docXML, `&amp;lt;div&amp;gt;`) {
		t.Errorf("expected &amp;lt;div&amp;gt; in XML text: %s", docXML)
	}
}

func TestHtmlToDocx_UnknownBlocksAndPre(t *testing.T) {
	// B2: <pre><code> and text inside <div> must not be dropped
	input := `
<div>裸露在div里的文本</div>
<pre><code class="language-bash">echo "hello world"
line 2</code></pre>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx unknown blocks: %v", err)
	}
	docXML, _ := inspectDocx(t, data)
	if !strings.Contains(docXML, "裸露在div里的文本") {
		t.Errorf("text inside div was dropped: %s", docXML)
	}
	if !strings.Contains(docXML, "echo") || !strings.Contains(docXML, "hello world") || !strings.Contains(docXML, "line 2") {
		t.Errorf("code block content was dropped: %s", docXML)
	}
	if !strings.Contains(docXML, "<w:br/>") {
		t.Errorf("newline in pre was not converted to w:br")
	}
}

func TestHtmlToDocx_DeterministicZipOrder(t *testing.T) {
	// B3: [Content_Types].xml MUST be the first zip entry
	data, err := htmlToDocx("<p>test</p>")
	if err != nil {
		t.Fatalf("htmlToDocx deterministic order: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if len(r.File) == 0 || r.File[0].Name != "[Content_Types].xml" {
		t.Errorf("first zip entry must be [Content_Types].xml, got: %v", r.File[0].Name)
	}
}

func TestHtmlToDocx_DangerousLinkScheme(t *testing.T) {
	// S1: javascript: should not be made into a relationship
	input := `<p><a href="javascript:alert(1)">危险链接</a></p>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx dangerous link: %v", err)
	}
	docXML, relsXML := inspectDocx(t, data)
	if strings.Contains(docXML, "<w:hyperlink") {
		t.Errorf("javascript: link should not produce w:hyperlink")
	}
	if strings.Contains(relsXML, "javascript:") {
		t.Errorf("javascript: should not be in rels XML")
	}
	if !strings.Contains(docXML, "危险链接") {
		t.Errorf("text of dangerous link should still be preserved")
	}
}

func TestHtmlToDocx_Empty(t *testing.T) {
	data, err := htmlToDocx("")
	if err != nil {
		t.Fatalf("htmlToDocx empty: %v", err)
	}
	docXML, _ := inspectDocx(t, data)
	if !strings.Contains(docXML, "<w:p>") {
		t.Errorf("empty docx must contain at least an empty paragraph")
	}
}

func TestHtmlToDocx_Headings(t *testing.T) {
	input := `<h1>一级标题</h1><h2>二级标题</h2><h3>三级标题</h3><h4>四级标题</h4><h5>五级标题</h5><h6>六级标题</h6>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx headings: %v", err)
	}
	docXML, _ := inspectDocx(t, data)
	for i := 1; i <= 6; i++ {
		expectedStyle := fmt.Sprintf(`w:pStyle w:val="Heading%d"`, i)
		if !strings.Contains(docXML, expectedStyle) {
			t.Errorf("missing heading style: %s", expectedStyle)
		}
	}
	if !strings.Contains(docXML, "一级标题") || !strings.Contains(docXML, "六级标题") {
		t.Errorf("missing heading text in XML: %s", docXML)
	}
}

func TestHtmlToDocx_InlineFormattingAndChinese(t *testing.T) {
	input := `<p>这是<strong>粗体</strong>，这是<em>斜体</em>，这是<u>下划线</u>，这是<s>删除线</s>，这是<code>代码片段</code>，含中文。</p>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx inline: %v", err)
	}
	docXML, _ := inspectDocx(t, data)

	if !strings.Contains(docXML, "<w:b/>") {
		t.Errorf("missing bold tag")
	}
	if !strings.Contains(docXML, "<w:i/>") {
		t.Errorf("missing italic tag")
	}
	if !strings.Contains(docXML, `<w:u w:val="single"/>`) {
		t.Errorf("missing underline tag")
	}
	if !strings.Contains(docXML, "<w:strike/>") {
		t.Errorf("missing strike tag")
	}
	if !strings.Contains(docXML, `Courier New`) {
		t.Errorf("missing code font Courier New")
	}
	if !strings.Contains(docXML, "这是") || !strings.Contains(docXML, "粗体") || !strings.Contains(docXML, "代码片段") {
		t.Errorf("chinese content corrupted")
	}
}

func TestHtmlToDocx_Lists(t *testing.T) {
	input := `
<ul>
  <li>无序项 1</li>
  <li>无序项 2
    <ul>
      <li>嵌套项 2.1</li>
    </ul>
  </li>
</ul>
<ol>
  <li>有序项 1</li>
  <li>有序项 2</li>
</ol>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx lists: %v", err)
	}
	docXML, _ := inspectDocx(t, data)

	if !strings.Contains(docXML, "无序项 1") || !strings.Contains(docXML, "•") {
		t.Errorf("missing bullet item")
	}
	if !strings.Contains(docXML, "有序项 1") || !strings.Contains(docXML, "1.") {
		t.Errorf("missing numbered item")
	}
	// Level 2 indent is 420 * 2 = 840
	if !strings.Contains(docXML, `<w:ind w:left="840"/>`) {
		t.Errorf("missing nested list indentation w:left=840")
	}
	if !strings.Contains(docXML, "嵌套项 2.1") {
		t.Errorf("missing nested list text")
	}
}

func TestHtmlToDocx_Table(t *testing.T) {
	input := `
<table>
  <thead>
    <tr><th>列一</th><th>列二</th></tr>
  </thead>
  <tbody>
    <tr><td>数据A</td><td>数据B</td></tr>
  </tbody>
</table>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx table: %v", err)
	}
	docXML, _ := inspectDocx(t, data)

	if !strings.Contains(docXML, "<w:tbl>") || !strings.Contains(docXML, "</w:tbl>") {
		t.Errorf("missing table element")
	}
	if !strings.Contains(docXML, "<w:tr>") || !strings.Contains(docXML, "<w:tc>") {
		t.Errorf("missing table row / cell element")
	}
	if !strings.Contains(docXML, "列一") || !strings.Contains(docXML, "数据A") {
		t.Errorf("missing table text")
	}
	// Header cell should have bold and shading
	if !strings.Contains(docXML, `<w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/>`) {
		t.Errorf("missing header background shading")
	}
}

func TestHtmlToDocx_Hyperlinks(t *testing.T) {
	input := `<p>欢迎访问 <a href="https://mistlab.dev">MistLab 官网</a> 了解详情。</p>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx hyperlink: %v", err)
	}
	docXML, relsXML := inspectDocx(t, data)

	if !strings.Contains(docXML, "<w:hyperlink") {
		t.Errorf("missing w:hyperlink in document.xml")
	}
	if !strings.Contains(docXML, "MistLab 官网") {
		t.Errorf("missing link anchor text")
	}
	if !strings.Contains(relsXML, `Target="https://mistlab.dev"`) {
		t.Errorf("missing hyperlink target in document.xml.rels: %s", relsXML)
	}
	if !strings.Contains(relsXML, `TargetMode="External"`) {
		t.Errorf("missing TargetMode=External in document.xml.rels")
	}
}
