package handler

import (
	"strings"
	"testing"
)

// ==================== htmlToMarkdown 黑盒测试 ====================

// TC1: 基础段落
func TestHTMLToMarkdown_BasicParagraph(t *testing.T) {
	html := "<p>Hello World</p>"
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "Hello World") {
		t.Errorf("got: %s", md)
	}
}

// TC2: 标题 h1-h6
func TestHTMLToMarkdown_Headings(t *testing.T) {
	html := "<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>"
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "# Title") {
		t.Error("missing h1")
	}
	if !strings.Contains(md, "## Subtitle") {
		t.Error("missing h2")
	}
	if !strings.Contains(md, "### Section") {
		t.Error("missing h3")
	}
}

// TC3: 代码块
func TestHTMLToMarkdown_CodeBlock(t *testing.T) {
	html := `<pre><code>func main() {
    fmt.Println("hello")
}</code></pre>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "```") {
		t.Errorf("missing code fence: %s", md)
	}
	if !strings.Contains(md, "func main()") {
		t.Errorf("missing code content: %s", md)
	}
}

// TC4: 代码块带语言
func TestHTMLToMarkdown_CodeBlockWithLanguage(t *testing.T) {
	html := `<pre><code class="language-go">package main</code></pre>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "```go") {
		t.Errorf("missing language hint: %s", md)
	}
}

// TC5: 内联代码
func TestHTMLToMarkdown_InlineCode(t *testing.T) {
	html := `<p>Use <code>fmt.Println</code> to print</p>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "`fmt.Println`") {
		t.Errorf("missing inline code: %s", md)
	}
}

// TC6: 简单表格
func TestHTMLToMarkdown_Table(t *testing.T) {
	html := `<table><tr><th>Name</th><th>Value</th></tr><tr><td>A</td><td>1</td></tr></table>`
	md := htmlToMarkdown(html)
	// Should have header row
	if !strings.Contains(md, "| Name | Value |") {
		t.Errorf("missing header row: %s", md)
	}
	// Should have separator
	if !strings.Contains(md, "| --- |") {
		t.Errorf("missing separator: %s", md)
	}
	// Should have data row
	if !strings.Contains(md, "| A | 1 |") {
		t.Errorf("missing data row: %s", md)
	}
}

// TC7: 无序列表
func TestHTMLToMarkdown_UnorderedList(t *testing.T) {
	html := `<ul><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "- Item 1") {
		t.Errorf("missing list item: %s", md)
	}
	if !strings.Contains(md, "- Item 2") {
		t.Errorf("missing list item 2: %s", md)
	}
}

// TC8: 有序列表
func TestHTMLToMarkdown_OrderedList(t *testing.T) {
	html := `<ol><li>First</li><li>Second</li><li>Third</li></ol>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "1. First") {
		t.Errorf("missing ordered item: %s", md)
	}
	if !strings.Contains(md, "2. Second") {
		t.Errorf("missing ordered item 2: %s", md)
	}
}

// TC9: 引用块
func TestHTMLToMarkdown_Blockquote(t *testing.T) {
	html := `<blockquote>This is a quote</blockquote>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "> This is a quote") {
		t.Errorf("missing blockquote: %s", md)
	}
}

// TC10: 粗体和斜体
func TestHTMLToMarkdown_BoldItalic(t *testing.T) {
	html := `<p><strong>bold</strong> and <em>italic</em></p>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "**bold**") {
		t.Errorf("missing bold: %s", md)
	}
	if !strings.Contains(md, "*italic*") {
		t.Errorf("missing italic: %s", md)
	}
}

// TC11: 删除线
func TestHTMLToMarkdown_Strikethrough(t *testing.T) {
	html := `<p><s>deleted</s> text</p>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "~deleted~") || !strings.Contains(md, "~~deleted~~") {
		t.Errorf("missing strikethrough: %s", md)
	}
}

// TC12: 链接
func TestHTMLToMarkdown_Link(t *testing.T) {
	html := `<a href="https://example.com">Example</a>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "[Example](https://example.com)") {
		t.Errorf("missing link: %s", md)
	}
}

// TC13: 图片
func TestHTMLToMarkdown_Image(t *testing.T) {
	html := `<img src="image.png" alt="Logo">`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "![Logo](image.png)") {
		t.Errorf("missing image: %s", md)
	}
}

// TC14: 混合内容
func TestHTMLToMarkdown_MixedContent(t *testing.T) {
	html := `<h1>Document</h1>
<p>Some <strong>bold</strong> text.</p>
<pre><code>code block</code></pre>
<ul><li>item</li></ul>`
	md := htmlToMarkdown(html)
	if !strings.Contains(md, "# Document") {
		t.Error("missing heading")
	}
	if !strings.Contains(md, "**bold**") {
		t.Error("missing bold")
	}
	if !strings.Contains(md, "```") {
		t.Error("missing code block")
	}
	if !strings.Contains(md, "- item") {
		t.Error("missing list")
	}
}

// TC15: HTML 实体解码
func TestHTMLToMarkdown_EntityDecode(t *testing.T) {
	html := `<p>&amp; &lt; &gt; &quot;</p>`
	md := htmlToMarkdown(html)
	// Should decode entities
	if !strings.Contains(md, "&") {
		t.Errorf("missing decoded amp: %s", md)
	}
}

// ==================== htmlToText 测试 ====================

func TestHTMLToText_Basic(t *testing.T) {
	html := `<p>Hello</p><p>World</p>`
	txt := htmlToText(html)
	if !strings.Contains(txt, "Hello") || !strings.Contains(txt, "World") {
		t.Errorf("got: %s", txt)
	}
}

func TestHTMLToText_StripsTags(t *testing.T) {
	html := `<h1>Title</h1><div><span>text</span></div>`
	txt := htmlToText(html)
	if strings.Contains(txt, "<") {
		t.Errorf("should not contain tags: %s", txt)
	}
}

// ==================== wrapHTML 测试 ====================

func TestWrapHTML_Structure(t *testing.T) {
	html := wrapHTML("Test Title", "<p>Content</p>")
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("missing doctype")
	}
	if !strings.Contains(html, "<title>Test Title</title>") {
		t.Error("missing title")
	}
	if !strings.Contains(html, "<style>") {
		t.Error("missing style")
	}
}

// ==================== wrapWordHTML 测试 ====================

func TestWrapWordHTML_ChineseFont(t *testing.T) {
	html := wrapWordHTML("中文文档", "<p>内容</p>")
	if !strings.Contains(html, "SimSun") && !strings.Contains(html, "Microsoft YaHei") {
		t.Error("missing Chinese font")
	}
}

// ==================== sanitizeFilename 测试 ====================

func TestSanitizeFilename_Basic(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"simple.txt", "simple.txt"},
		{"file name.doc", "file_name.doc"},
		{"中文标题", "____"},
		{"file<>:\"/\\|?*.txt", "file_________.txt"},
	}

	for _, tt := range tests {
		out := sanitizeFilename(tt.in)
		if out != tt.out {
			t.Errorf("sanitizeFilename(%s) = %s, want %s", tt.in, out, tt.out)
		}
		if len(out) > 100 {
			t.Errorf("filename too long: %d", len(out))
		}
	}
}