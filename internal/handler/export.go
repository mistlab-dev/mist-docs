package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

// ExportDocument exports a document in the specified format.
// GET /docs/documents/:id/export?format=markdown|html|txt
func ExportDocument(c *gin.Context) {
	docID := c.Param("id")
	format := c.DefaultQuery("format", "html")

	// Get document info
	var title, docType, deptID string
	err := database.DB.QueryRow(
		"SELECT title, type, department_id FROM md_documents WHERE id = ? AND status = 1",
		docID,
	).Scan(&title, &docType, &deptID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	// Read content
	content, err := store.ReadCurrent(deptID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文档内容失败"})
		return
	}

	htmlContent := string(content)
	fileName := sanitizeFilename(title)

	switch format {
	case "markdown", "md":
		markdown := htmlToMarkdown(htmlContent)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, fileName))
		c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(markdown))

	case "html":
		fullHTML := wrapHTML(title, htmlContent)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.html"`, fileName))
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(fullHTML))

	case "txt":
		text := htmlToText(htmlContent)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, fileName))
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(text))

	case "pdf":
		pdf, err := htmlToPDF(title, htmlContent)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF 生成失败: " + err.Error()})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, fileName))
		c.Data(http.StatusOK, "application/pdf", pdf)

	case "docx":
		// Word-compatible HTML (Office opens .doc HTML natively)
		wordHTML := wrapWordHTML(title, htmlContent)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.doc"`, fileName))
		c.Data(http.StatusOK, "application/msword", []byte(wordHTML))

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的格式，可选: markdown, html, txt, pdf, docx"})
	}

	// Audit
	userName, _ := c.Get("username")
	audit(c, "export", "document", docID, title, fmt.Sprintf("%s 导出文档 (%s)", userName, format))
}

// ==================== Markdown Conversion (Improved) ====================

// htmlToMarkdown converts HTML to Markdown with proper handling of
// nested structures, code blocks, tables, and complex inline formatting.
func htmlToMarkdown(html string) string {
	// Pre-process: normalize whitespace and fix common issues
	s := strings.ReplaceAll(html, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Code blocks (pre) - handle first to avoid inner conversion
	s = convertCodeBlocks(s)

	// Tables
	s = convertTables(s)

	// Blockquotes
	s = convertBlockquotes(s)

	// Lists (handle nesting properly)
	s = convertLists(s)

	// Headers
	s = convertHeaders(s)

	// Paragraphs and breaks
	s = convertParagraphs(s)

	// Inline elements
	s = convertInlineElements(s)

	// Images and links
	s = convertImages(s)
	s = convertLinks(s)

	// Final cleanup
	s = stripRemainingTags(s)
	s = cleanWhitespace(s)

	return strings.TrimSpace(s)
}

// convertCodeBlocks handles <pre><code> blocks.
func convertCodeBlocks(s string) string {
	// <pre><code>...</code></pre> → fenced code block
	re := regexp.MustCompile(`(?s)<pre[^>]*>\s*<code[^>]*>(.*?)</code>\s*</pre>`)
	s = re.ReplaceAllStringFunc(s, func(m string) string {
		// Extract content
		sub := re.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		content := sub[1]
		// Check if it's a code block with language hint
		langClass := ""
		classRe := regexp.MustCompile(`<code[^>]*class="[^"]*language-([^"]+)"[^>]*>`)
		if classRe.MatchString(m) {
			langMatch := classRe.FindStringSubmatch(m)
			if len(langMatch) > 1 {
				langClass = langMatch[1]
			}
		}
		// Decode HTML entities in code
		content = htmlDecode(content)
		return "\n```" + langClass + "\n" + content + "\n```\n"
	})

	// Simple <pre> without <code>
	re2 := regexp.MustCompile(`(?s)<pre[^>]*>(.*?)</pre>`)
	s = re2.ReplaceAllStringFunc(s, func(m string) string {
		sub := re2.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		content := htmlDecode(sub[1])
		return "\n```\n" + content + "\n```\n"
	})

	return s
}

// convertTables converts HTML tables to Markdown tables.
func convertTables(s string) string {
	tableRe := regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)
	s = tableRe.ReplaceAllStringFunc(s, func(tableMatch string) string {
		content := tableRe.FindStringSubmatch(tableMatch)[1]

		// Extract rows
		rowRe := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
		rows := rowRe.FindAllStringSubmatch(content, -1)

		if len(rows) == 0 {
			return ""
		}

		var mdTable strings.Builder

		for i, row := range rows {
			rowContent := row[1]

			// Determine if header row (th or first row with th-like style)
			cellRe := regexp.MustCompile(`(?s)<t[h|d][^>]*>(.*?)</t[h|d]>`)
			cells := cellRe.FindAllStringSubmatch(rowContent, -1)

			if len(cells) == 0 {
				continue
			}

			// Write row
			mdTable.WriteString("|")
			for _, cell := range cells {
				cellText := stripTags(cell[1])
				cellText = strings.TrimSpace(cellText)
				mdTable.WriteString(" " + cellText + " |")
			}
			mdTable.WriteString("\n")

			// Add separator after first row (assume first row is header)
			if i == 0 {
				mdTable.WriteString("|")
				for j := 0; j < len(cells); j++ {
					mdTable.WriteString(" --- |")
				}
				mdTable.WriteString("\n")
			}
		}

		return "\n" + mdTable.String() + "\n"
	})

	return s
}

// convertBlockquotes handles nested blockquotes.
func convertBlockquotes(s string) string {
	// Simple blockquote
	re := regexp.MustCompile(`(?s)<blockquote[^>]*>(.*?)</blockquote>`)
	s = re.ReplaceAllStringFunc(s, func(m string) string {
		content := re.FindStringSubmatch(m)[1]
		content = stripTags(content)
		lines := strings.Split(content, "\n")
		var result strings.Builder
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				result.WriteString("> " + trimmed + "\n")
			}
		}
		return result.String()
	})
	return s
}

// convertLists handles ul/ol with proper nesting.
func convertLists(s string) string {
	// Process nested lists by depth
	maxDepth := 6
	for depth := maxDepth; depth >= 1; depth-- {
		indent := strings.Repeat("  ", depth-1)

		// Unordered lists
		ulRe := regexp.MustCompile(`<ul[^>]*>(.*?)</ul>`)
		s = ulRe.ReplaceAllStringFunc(s, func(m string) string {
			content := ulRe.FindStringSubmatch(m)[1]
			// Process li items
			liRe := regexp.MustCompile(`<li[^>]*>(.*?)</li>`)
			items := liRe.FindAllStringSubmatch(content, -1)
			var result strings.Builder
			for _, item := range items {
				text := stripTags(item[1])
				text = strings.TrimSpace(text)
				result.WriteString(indent + "- " + text + "\n")
			}
			return result.String()
		})

		// Ordered lists - similar approach
		olRe := regexp.MustCompile(`<ol[^>]*>(.*?)</ol>`)
		s = olRe.ReplaceAllStringFunc(s, func(m string) string {
			content := olRe.FindStringSubmatch(m)[1]
			liRe := regexp.MustCompile(`<li[^>]*>(.*?)</li>`)
			items := liRe.FindAllStringSubmatch(content, -1)
			var result strings.Builder
			for i, item := range items {
				text := stripTags(item[1])
				text = strings.TrimSpace(text)
				result.WriteString(indent + fmt.Sprintf("%d. ", i+1) + text + "\n")
			}
			return result.String()
		})
	}
	return s
}

// convertHeaders converts h1-h6.
func convertHeaders(s string) string {
	for i := 6; i >= 1; i-- {
		re := regexp.MustCompile(fmt.Sprintf(`<h%d[^>]*>(.*?)</h%d>`, i, i))
		marker := strings.Repeat("#", i)
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			content := re.FindStringSubmatch(m)[1]
			content = stripTags(content)
			return "\n" + marker + " " + strings.TrimSpace(content) + "\n"
		})
	}
	return s
}

// convertParagraphs handles p, div, br.
func convertParagraphs(s string) string {
	// <p>...</p> → text + newline
	pRe := regexp.MustCompile(`<p[^>]*>(.*?)</p>`)
	s = pRe.ReplaceAllStringFunc(s, func(m string) string {
		content := pRe.FindStringSubmatch(m)[1]
		return strings.TrimSpace(content) + "\n\n"
	})

	// <div>...</div> → newline-separated
	divRe := regexp.MustCompile(`<div[^>]*>(.*?)</div>`)
	s = divRe.ReplaceAllStringFunc(s, func(m string) string {
		content := divRe.FindStringSubmatch(m)[1]
		return strings.TrimSpace(content) + "\n"
	})

	// <br>, <br/> → newline
	s = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(s, "\n")

	// <hr> → horizontal rule
	s = regexp.MustCompile(`<hr\s*/?>`).ReplaceAllString(s, "\n---\n")

	return s
}

// convertInlineElements handles strong, em, code, etc.
func convertInlineElements(s string) string {
	// Bold
	s = regexp.MustCompile(`<strong[^>]*>(.*?)</strong>`).ReplaceAllStringFunc(s, func(m string) string {
		return "**" + stripTags(regexp.MustCompile(`<strong[^>]*>(.*?)</strong>`).FindStringSubmatch(m)[1]) + "**"
	})
	s = regexp.MustCompile(`<b[^>]*>(.*?)</b>`).ReplaceAllStringFunc(s, func(m string) string {
		return "**" + stripTags(regexp.MustCompile(`<b[^>]*>(.*?)</b>`).FindStringSubmatch(m)[1]) + "**"
	})

	// Italic
	s = regexp.MustCompile(`<em[^>]*>(.*?)</em>`).ReplaceAllStringFunc(s, func(m string) string {
		return "*" + stripTags(regexp.MustCompile(`<em[^>]*>(.*?)</em>`).FindStringSubmatch(m)[1]) + "*"
	})
	s = regexp.MustCompile(`<i[^>]*>(.*?)</i>`).ReplaceAllStringFunc(s, func(m string) string {
		return "*" + stripTags(regexp.MustCompile(`<i[^>]*>(.*?)</i>`).FindStringSubmatch(m)[1]) + "*"
	})

	// Strikethrough
	s = regexp.MustCompile(`<s[^>]*>(.*?)</s>`).ReplaceAllStringFunc(s, func(m string) string {
		return "~~" + stripTags(regexp.MustCompile(`<s[^>]*>(.*?)</s>`).FindStringSubmatch(m)[1]) + "~~"
	})
	s = regexp.MustCompile(`<del[^>]*>(.*?)</del>`).ReplaceAllStringFunc(s, func(m string) string {
		return "~~" + stripTags(regexp.MustCompile(`<del[^>]*>(.*?)</del>`).FindStringSubmatch(m)[1]) + "~~"
	})

	// Inline code (not in pre)
	s = regexp.MustCompile(`<code[^>]*>(.*?)</code>`).ReplaceAllStringFunc(s, func(m string) string {
		content := regexp.MustCompile(`<code[^>]*>(.*?)</code>`).FindStringSubmatch(m)[1]
		// If it's already in a code block, skip
		if strings.Contains(s, "```"+content) {
			return content
		}
		return "`" + stripTags(content) + "`"
	})

	return s
}

// convertImages converts img tags.
func convertImages(s string) string {
	re := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*alt="([^"]*)"[^>]*>`)
	s = re.ReplaceAllStringFunc(s, func(m string) string {
		sub := re.FindStringSubmatch(m)
		src := sub[1]
		alt := sub[2]
		return "![" + alt + "](" + src + ")"
	})

	// img without alt
	re2 := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*>`)
	s = re2.ReplaceAllStringFunc(s, func(m string) string {
		sub := re2.FindStringSubmatch(m)
		src := sub[1]
		return "![image](" + src + ")"
	})

	return s
}

// convertLinks converts a tags.
func convertLinks(s string) string {
	re := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	s = re.ReplaceAllStringFunc(s, func(m string) string {
		sub := re.FindStringSubmatch(m)
		href := sub[1]
		text := stripTags(sub[2])
		return "[" + text + "](" + href + ")"
	})
	return s
}

// htmlDecode decodes common HTML entities.
func htmlDecode(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return s
}

// stripTags removes all HTML tags.
func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}

// stripRemainingTags removes any remaining HTML tags after conversion.
func stripRemainingTags(s string) string {
	return stripTags(s)
}

// cleanWhitespace normalizes whitespace.
func cleanWhitespace(s string) string {
	// Multiple newlines to max 2
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	// Trim each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
}

// ==================== Text Conversion ====================

// htmlToText strips HTML tags and returns plain text.
func htmlToText(html string) string {
	s := html
	s = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`</p>`).ReplaceAllString(s, "\n\n")
	s = regexp.MustCompile(`</div>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`</li>`).ReplaceAllString(s, "\n")
	s = stripTags(s)
	s = cleanWhitespace(s)
	return strings.TrimSpace(s)
}

// ==================== HTML Wrappers ====================

// wrapHTML creates a full HTML document.
func wrapHTML(title, body string) string {
	css := "body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;max-width:800px;margin:40px auto;padding:0 20px;line-height:1.6;color:#333}"
	css += "h1,h2,h3{margin-top:1.5em}code{background:#f4f4f4;padding:2px 6px;border-radius:3px}"
	css += "pre{background:#f4f4f4;padding:12px;border-radius:6px;overflow-x:auto}"
	css += "blockquote{border-left:4px solid #ddd;padding-left:1em;color:#666;margin-left:0}"
	css += "img{max-width:100%}table{border-collapse:collapse;width:100%}th,td{border:1px solid #ddd;padding:8px 12px}"
	return `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>` + title +
		`</title><style>` + css + `</style></head><body><h1>` + title +
		`</h1>` + body + `</body></html>`
}

func sanitizeFilename(name string) string {
	s := strings.ReplaceAll(name, " ", "_")
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, s)
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

// ==================== PDF Conversion ====================

// htmlToPDF converts HTML content to PDF using gofpdf.
// Note: gofpdf doesn't support Chinese fonts well.
// For documents with Chinese content, use frontend html2pdf.js instead.
func htmlToPDF(title, body string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 12, truncateUTF8(title, 60), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// Body text (strip HTML, add as plain text)
	text := htmlToText(body)
	lines := strings.Split(text, "\n")

	pdf.SetFont("Arial", "", 11)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			pdf.Ln(4)
			continue
		}
		// Check for heading-like lines (short + no period)
		if len(trimmed) < 40 && !strings.Contains(trimmed, "。") && !strings.Contains(trimmed, ",") {
			pdf.SetFont("Arial", "B", 13)
			pdf.MultiCell(0, 7, trimmed, "", "L", false)
			pdf.SetFont("Arial", "", 11)
		} else {
			pdf.MultiCell(0, 6, trimmed, "", "L", false)
		}
	}

	buf := &bytes.Buffer{}
	if err := pdf.Output(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func truncateUTF8(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// ==================== Word HTML ====================

// wrapWordHTML generates Word-compatible HTML that Microsoft Word and WPS can open natively
func wrapWordHTML(title, body string) string {
	css := "body{font-family:SimSun,Microsoft YaHei,sans-serif;font-size:12pt;line-height:1.8;color:#333}"
	css += "h1{font-size:22pt;text-align:center;margin:20pt 0}"
	css += "h2{font-size:16pt;margin-top:16pt;border-bottom:1pt solid #ccc}"
	css += "h3{font-size:14pt;margin-top:12pt}"
	css += "table{border-collapse:collapse;width:100%}th,td{border:1pt solid #999;padding:4pt 8pt}"
	css += "th{background:#f0f0f0;font-weight:bold}"
	css += "code{font-family:Consolas,monospace;background:#f4f4f4;padding:1pt 3pt}"
	css += "pre{background:#f4f4f4;padding:8pt}"
	css += "blockquote{border-left:3pt solid #ccc;padding-left:10pt;color:#666}"
	css += "img{max-width:100%}"
	return `<html><head><meta charset="UTF-8"><title>` + title +
		`</title><style>` + css + `</style></head><body><h1>` + title +
		`</h1>` + body + `</body></html>`
}