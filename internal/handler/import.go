package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// BatchImport POST /docs/import
// Accepts multipart files (.txt, .md, .html) and creates documents
func BatchImport(c *gin.Context) {
	userID := c.GetString("user_id")
	userDeptID := c.GetString("department_id")
	role := c.GetString("role")
	folderID := c.DefaultPostForm("folder_id", "")

	if role != "super_admin" && role != "dept_admin" && role != "member" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到文件"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
		return
	}

	if len(files) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "最多同时导入20个文件"})
		return
	}

	type importResult struct {
		Title  string `json:"title"`
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	var results []importResult

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".txt" && ext != ".md" && ext != ".html" && ext != ".htm" {
			results = append(results, importResult{
				Title:  file.Filename,
				Status: "skipped",
				Error:  "不支持的格式（仅 .txt/.md/.html）",
			})
			continue
		}

		if file.Size > 2*1024*1024 {
			results = append(results, importResult{
				Title:  file.Filename,
				Status: "skipped",
				Error:  "文件超过2MB",
			})
			continue
		}

		// Read file content
		src, err := file.Open()
		if err != nil {
			results = append(results, importResult{Title: file.Filename, Status: "error", Error: err.Error()})
			continue
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			results = append(results, importResult{Title: file.Filename, Status: "error", Error: err.Error()})
			continue
		}

		content := string(data)
		// Convert to HTML based on source format
		var htmlContent string
		title := strings.TrimSuffix(filepath.Base(file.Filename), ext)

		switch ext {
		case ".html", ".htm":
			htmlContent = content
		case ".md":
			htmlContent = markdownToHTML(content)
		case ".txt":
			htmlContent = textToHTML(content)
		}

		// Resolve department
		deptID := resolveDeptID(c, folderID, userDeptID)
		if role == "dept_admin" && deptID != userDeptID {
			results = append(results, importResult{Title: title, Status: "error", Error: "只能导入到本部门"})
			continue
		}

		doc := &model.Document{
			Title:        title,
			Type:         "doc",
			FolderID:     folderID,
			DepartmentID: deptID,
		}

		if err := service.CreateDocument(c.Request.Context(), doc, []byte(htmlContent), userID); err != nil {
			results = append(results, importResult{Title: title, Status: "error", Error: err.Error()})
			continue
		}

		audit(c, "import_doc", "document", doc.ID, title, fmt.Sprintf(`{"file":"%s"}`, file.Filename))
		results = append(results, importResult{Title: title, ID: doc.ID, Status: "created"})
	}

	created := 0
	for _, r := range results {
		if r.Status == "created" {
			created++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    results,
		"message": fmt.Sprintf("成功导入 %d/%d 个文件", created, len(files)),
	})
}

// markdownToHTML converts basic markdown to HTML
func markdownToHTML(md string) string {
	lines := strings.Split(md, "\n")
	var html strings.Builder
	inCode := false
	inList := false

	for _, line := range lines {
		// Code blocks
		if strings.HasPrefix(line, "```") {
			if inCode {
				html.WriteString("</code></pre>")
				inCode = false
			} else {
				lang := strings.TrimPrefix(line, "```")
				html.WriteString(fmt.Sprintf("<pre><code%s>", func() string {
					if lang != "" {
						return ` class="language-` + lang + `"`
					}
					return ""
				}()))
				inCode = true
			}
			continue
		}
		if inCode {
			html.WriteString(line + "\n")
			continue
		}

		// Close list if needed
		if inList && !strings.HasPrefix(strings.TrimSpace(line), "- ") && !strings.HasPrefix(strings.TrimSpace(line), "* ") {
			html.WriteString("</ul>")
			inList = false
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Headings
		if strings.HasPrefix(trimmed, "### ") {
			html.WriteString("<h3>" + inlineMD(trimmed[4:]) + "</h3>")
		} else if strings.HasPrefix(trimmed, "## ") {
			html.WriteString("<h2>" + inlineMD(trimmed[3:]) + "</h2>")
		} else if strings.HasPrefix(trimmed, "# ") {
			html.WriteString("<h1>" + inlineMD(trimmed[2:]) + "</h1>")
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				html.WriteString("<ul>")
				inList = true
			}
			html.WriteString("<li>" + inlineMD(trimmed[2:]) + "</li>")
		} else {
			html.WriteString("<p>" + inlineMD(trimmed) + "</p>")
		}
	}

	if inCode {
		html.WriteString("</code></pre>")
	}
	if inList {
		html.WriteString("</ul>")
	}

	return html.String()
}

func inlineMD(s string) string {
	// Bold
	s = strings.ReplaceAll(s, "**", "</strong>")
	// Simple approach: odd occurrences get <strong>, even get </strong>
	parts := strings.Split(s, "</strong>")
	for i, p := range parts {
		if i%2 == 0 && i < len(parts)-1 {
			parts[i] = p + "<strong>"
		} else if i < len(parts)-1 {
			parts[i] = p + "</strong>"
		}
	}
	s = strings.Join(parts, "")

	// Italic
	s = strings.ReplaceAll(s, "*", "<em>")
	parts2 := strings.Split(s, "<em>")
	for i, p := range parts2 {
		if i%2 == 0 && i < len(parts2)-1 {
			parts2[i] = p + "<em>"
		} else if i < len(parts2)-1 {
			parts2[i] = p + "</em>"
		}
	}
	return strings.Join(parts2, "")
}

func textToHTML(txt string) string {
	lines := strings.Split(txt, "\n")
	var html strings.Builder
	for _, line := range lines {
		escaped := strings.TrimSpace(line)
		if escaped == "" {
			html.WriteString("<br>")
		} else {
			html.WriteString("<p>" + escaped + "</p>")
		}
	}
	return html.String()
}
