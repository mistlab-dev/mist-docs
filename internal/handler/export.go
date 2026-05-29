package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
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

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的格式，可选: markdown, html, txt"})
	}

	// Audit
	userName, _ := c.Get("username")
	audit(c, "export", "document", docID, title, fmt.Sprintf("%s 导出文档 (%s)", userName, format))
}

// htmlToMarkdown converts basic HTML to Markdown.
func htmlToMarkdown(html string) string {
	s := html
	// Headings
	s = replaceAll(s, "<h1>", "</h1>", "# ", "")
	s = replaceAll(s, "<h2>", "</h2>", "## ", "")
	s = replaceAll(s, "<h3>", "</h3>", "### ", "")
	s = replaceAll(s, "<h4>", "</h4>", "#### ", "")
	s = replaceAll(s, "<h5>", "</h5>", "##### ", "")
	// Inline
	s = replaceAll(s, "<strong>", "</strong>", "**", "**")
	s = replaceAll(s, "<b>", "</b>", "**", "**")
	s = replaceAll(s, "<em>", "</em>", "*", "*")
	s = replaceAll(s, "<i>", "</i>", "*", "*")
	s = replaceAll(s, "<s>", "</s>", "~~", "~~")
	s = replaceAll(s, "<code>", "</code>", "`", "`")
	// Links
	s = replaceLinks(s)
	// Images
	s = replaceImages(s)
	// Lists
	s = replaceAll(s, "<li>", "</li>", "- ", "")
	s = replaceAllBy(s, "<ul>", "", "\n")
	s = replaceAllBy(s, "</ul>", "", "\n")
	s = replaceAllBy(s, "<ol>", "", "\n")
	s = replaceAllBy(s, "</ol>", "", "\n")
	// Blockquote
	s = replaceAll(s, "<blockquote>", "</blockquote>", "> ", "")
	// Paragraph
	s = replaceAllBy(s, "<p>", "", "")
	s = replaceAllBy(s, "</p>", "", "\n\n")
	// Line breaks
	s = replaceAllBy(s, "<br>", "", "\n")
	s = replaceAllBy(s, "<br/>", "", "\n")
	s = replaceAllBy(s, "<hr>", "", "\n---\n")
	// Strip remaining tags
	s = stripTags(s)
	// Clean up multiple newlines
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// htmlToText strips HTML tags and returns plain text.
func htmlToText(html string) string {
	s := html
	s = replaceAllBy(s, "<br>", "", "\n")
	s = replaceAllBy(s, "<br/>", "", "\n")
	s = replaceAllBy(s, "</p>", "", "\n\n")
	s = replaceAllBy(s, "</div>", "", "\n")
	s = replaceAllBy(s, "</li>", "", "\n")
	s = stripTags(s)
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// wrapHTML creates a full HTML document.
func wrapHTML(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>%s</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; line-height: 1.6; color: #333; }
h1, h2, h3 { margin-top: 1.5em; }
code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
pre { background: #f4f4f4; padding: 12px; border-radius: 6px; overflow-x: auto; }
blockquote { border-left: 4px solid #ddd; padding-left: 1em; color: #666; margin-left: 0; }
img { max-width: 100%; }
table { border-collapse: collapse; width: 100%%; }
th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
</style>
</head>
<body>
%s
</body>
</html>`, title, body)
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

func replaceAll(s, openTag, closeTag, openReplace, closeReplace string) string {
	s = strings.ReplaceAll(s, openTag, openReplace)
	s = strings.ReplaceAll(s, closeTag, closeReplace)
	return s
}

func replaceAllBy(s, tag, _, replacement string) string {
	return strings.ReplaceAll(s, tag, replacement)
}

func replaceLinks(s string) string {
	// Simple <a href="url">text</a> → [text](url)
	result := s
	i := 0
	for {
		start := strings.Index(result[i:], "<a ")
		if start == -1 {
			break
		}
		start += i
		hrefIdx := strings.Index(result[start:], "href=\"")
		if hrefIdx == -1 {
			i = start + 3
			continue
		}
		hrefIdx += start + 6
		hrefEnd := strings.Index(result[hrefIdx:], "\"")
		if hrefEnd == -1 {
			i = start + 3
			continue
		}
		href := result[hrefIdx : hrefIdx+hrefEnd]
		closeTag := strings.Index(result[hrefIdx+hrefEnd:], ">")
		if closeTag == -1 {
			i = start + 3
			continue
		}
		textStart := hrefIdx + hrefEnd + closeTag + 1
		textEnd := strings.Index(result[textStart:], "</a>")
		if textEnd == -1 {
			i = start + 3
			continue
		}
		text := result[textStart : textStart+textEnd]
		replacement := fmt.Sprintf("[%s](%s)", text, href)
		result = result[:start] + replacement + result[textStart+textEnd+4:]
		i = start + len(replacement)
	}
	return result
}

func replaceImages(s string) string {
	result := s
	i := 0
	for {
		start := strings.Index(result[i:], "<img ")
		if start == -1 {
			break
		}
		start += i
		srcIdx := strings.Index(result[start:], "src=\"")
		if srcIdx == -1 {
			i = start + 5
			continue
		}
		srcIdx += start + 5
		srcEnd := strings.Index(result[srcIdx:], "\"")
		if srcEnd == -1 {
			i = start + 5
			continue
		}
		src := result[srcIdx : srcIdx+srcEnd]
		tagEnd := strings.Index(result[srcIdx+srcEnd:], ">")
		if tagEnd == -1 {
			i = start + 5
			continue
		}
		fullEnd := srcIdx + srcEnd + tagEnd + 1
		replacement := fmt.Sprintf("![image](%s)", src)
		result = result[:start] + replacement + result[fullEnd:]
		i = start + len(replacement)
	}
	return result
}

func stripTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}
