package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

type BatchImportResult struct {
	Title  string `json:"title"`
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// BatchImport POST /docs/import
// Accepts .txt, .md, .html, .docx, .xlsx
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

	var results []BatchImportResult

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		supported := map[string]bool{".txt": true, ".md": true, ".html": true, ".htm": true, ".docx": true, ".xlsx": true}
		if !supported[ext] {
			results = append(results, BatchImportResult{Title: file.Filename, Status: "skipped", Error: "不支持的格式"})
			continue
		}
		if file.Size > 10*1024*1024 {
			results = append(results, BatchImportResult{Title: file.Filename, Status: "skipped", Error: "文件超过10MB"})
			continue
		}

		src, err := file.Open()
		if err != nil {
			results = append(results, BatchImportResult{Title: file.Filename, Status: "error", Error: err.Error()})
			continue
		}
		data, _ := io.ReadAll(src)
		src.Close()

		title := strings.TrimSuffix(filepath.Base(file.Filename), ext)
		deptID := resolveDeptID(c, folderID, userDeptID)
		if role == "dept_admin" && deptID != userDeptID {
			results = append(results, BatchImportResult{Title: title, Status: "error", Error: "只能导入到本部门"})
			continue
		}

		switch ext {
		case ".xlsx":
			sheetJSON, err := xlsxToSheet(data)
			if err != nil {
				results = append(results, BatchImportResult{Title: title, Status: "error", Error: "Excel解析: " + err.Error()})
				continue
			}
			doc := &model.Document{Title: title, Type: "sheet", FolderID: folderID, DepartmentID: deptID}
			if err := service.CreateDocument(c.Request.Context(), doc, []byte(sheetJSON), userID); err != nil {
				results = append(results, BatchImportResult{Title: title, Status: "error", Error: err.Error()})
				continue
			}
			audit(c, "import_doc", "document", doc.ID, title, "")
			results = append(results, BatchImportResult{Title: title, ID: doc.ID, Type: "sheet", Status: "created"})

		case ".docx":
			html, err := docxToHTML(data)
			if err != nil {
				results = append(results, BatchImportResult{Title: title, Status: "error", Error: "Word解析: " + err.Error()})
				continue
			}
			createDocFromImport(c, title, "doc", html, folderID, deptID, userID, &results)

		case ".html", ".htm":
			createDocFromImport(c, title, "doc", string(data), folderID, deptID, userID, &results)

		case ".md":
			html := markdownToHTML(string(data))
			createDocFromImport(c, title, "doc", html, folderID, deptID, userID, &results)

		case ".txt":
			html := textToHTML(string(data))
			createDocFromImport(c, title, "doc", html, folderID, deptID, userID, &results)
		}
	}

	created := 0
	for _, r := range results {
		if r.Status == "created" {
			created++
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": results, "message": fmt.Sprintf("成功导入 %d/%d 个文件", created, len(files))})
}

func createDocFromImport(c *gin.Context, title, docType, html, folderID, deptID, userID string, results *[]BatchImportResult) {
	doc := &model.Document{Title: title, Type: docType, FolderID: folderID, DepartmentID: deptID}
	if err := service.CreateDocument(c.Request.Context(), doc, []byte(html), userID); err != nil {
		*results = append(*results, BatchImportResult{Title: title, Status: "error", Error: err.Error()})
		return
	}
	audit(c, "import_doc", "document", doc.ID, title, "")
	*results = append(*results, BatchImportResult{Title: title, ID: doc.ID, Type: docType, Status: "created"})
}

// ─── Word (.docx) → HTML ───

type wDocument struct {
	XMLName xml.Name    `xml:"document"`
	Body    wBody       `xml:"body"`
}
type wBody struct {
	Paragraphs []wParagraph `xml:"p"`
}
type wParagraph struct {
	Runs []wRun `xml:"r"`
	PPr  *wPPr  `xml:"pPr"`
}
type wPPr struct {
	PStyle wVal `xml:"pStyle"`
}
type wRun struct {
	Text wText `xml:"t"`
}
type wText struct {
	Text string `xml:",chardata"`
}
type wVal struct {
	Val string `xml:"val,attr"`
}

func docxToHTML(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("未找到document.xml")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	docXML, _ := io.ReadAll(rc)

	var doc wDocument
	if err := xml.Unmarshal(docXML, &doc); err != nil {
		return "", err
	}

	var html strings.Builder
	for _, p := range doc.Body.Paragraphs {
		var text strings.Builder
		for _, run := range p.Runs {
			text.WriteString(run.Text.Text)
		}
		line := strings.TrimSpace(text.String())
		if line == "" {
			html.WriteString("<p><br></p>")
			continue
		}

		style := ""
		if p.PPr != nil {
			style = p.PPr.PStyle.Val
		}
		escaped := escapeHTML(line)
		switch {
		case strings.Contains(style, "Heading1") || strings.HasSuffix(style, "1"):
			html.WriteString("<h1>" + escaped + "</h1>")
		case strings.Contains(style, "Heading2") || strings.HasSuffix(style, "2"):
			html.WriteString("<h2>" + escaped + "</h2>")
		case strings.Contains(style, "Heading3") || strings.HasSuffix(style, "3") || strings.HasSuffix(style, "4"):
			html.WriteString("<h3>" + escaped + "</h3>")
		default:
			html.WriteString("<p>" + escaped + "</p>")
		}
	}

	result := html.String()
	if result == "" {
		result = "<p>（空文档）</p>"
	}
	return result, nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ─── Excel (.xlsx) → Sheet JSON ───

func xlsxToSheet(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	// Parse shared strings
	sharedStrings := []string{}
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			if ss, err := parseXlsxSharedStrings(f); err == nil {
				sharedStrings = ss
			}
			break
		}
	}

	// Parse first worksheet
	var sheetFile *zip.File
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			sheetFile = f
			break
		}
	}
	if sheetFile == nil {
		return "", fmt.Errorf("未找到工作表")
	}

	rc, err := sheetFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	sheetXML, _ := io.ReadAll(rc)

	type xCell struct {
		Ref   string `xml:"r,attr"`
		Type  string `xml:"t,attr"`
		Value string `xml:"v"`
	}
	type xRow struct {
		Cells []xCell `xml:"c"`
	}
	type xSheetData struct {
		Rows []xRow `xml:"row"`
	}
	type xWorksheet struct {
		SheetData xSheetData `xml:"sheetData"`
	}

	var ws xWorksheet
	if err := xml.Unmarshal(sheetXML, &ws); err != nil {
		return "", err
	}

	// Build grid
	maxCol, maxRow := 0, 0
	type coord struct{ r, c int }
	cellMap := make(map[coord]string)

	for ri, row := range ws.SheetData.Rows {
		rowIdx := ri
		for _, cell := range row.Cells {
			colStr, rowStr := splitCellRef(cell.Ref)
			colIdx := colLetterToIdx(colStr)
			if n, err := parseSimpleInt(rowStr); err == nil {
				rowIdx = n - 1
			}
			if rowIdx >= maxRow {
				maxRow = rowIdx + 1
			}
			if colIdx >= maxCol {
				maxCol = colIdx + 1
			}

			var value string
			if cell.Type == "s" {
				if idx, err := parseSimpleInt(cell.Value); err == nil && idx < len(sharedStrings) {
					value = sharedStrings[idx]
				}
			} else {
				value = cell.Value
			}
			cellMap[coord{rowIdx, colIdx}] = value
		}
	}

	if maxRow < 1 {
		maxRow = 1
	}
	if maxCol < 1 {
		maxCol = 1
	}
	if maxRow > 200 {
		maxRow = 200
	}
	if maxCol > 26 {
		maxCol = 26
	}

	rows := make([][]string, maxRow)
	for i := range rows {
		rows[i] = make([]string, maxCol)
	}
	for k, v := range cellMap {
		if k.r < maxRow && k.c < maxCol {
			rows[k.r][k.c] = v
		}
	}

	out, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parseXlsxSharedStrings(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)

	type xSI struct {
		Text string `xml:"t"`
	}
	type xSST struct {
		Items []xSI `xml:"si"`
	}
	var sst xSST
	if err := xml.Unmarshal(data, &sst); err != nil {
		return nil, err
	}
	result := make([]string, len(sst.Items))
	for i, item := range sst.Items {
		result[i] = item.Text
	}
	return result, nil
}

func splitCellRef(ref string) (string, string) {
	var col, row string
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			col += string(ch)
		} else {
			row += string(ch)
		}
	}
	return col, row
}

func colLetterToIdx(s string) int {
	idx := 0
	for _, ch := range s {
		idx = idx*26 + int(ch-'A')
	}
	return idx
}

func parseSimpleInt(s string) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}

// ─── Text converters ───

func markdownToHTML(md string) string {
	lines := strings.Split(md, "\n")
	var html strings.Builder
	inCode, inList := false, false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCode {
				html.WriteString("</code></pre>")
				inCode = false
			} else {
				html.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			html.WriteString(line + "\n")
			continue
		}
		if inList && !strings.HasPrefix(strings.TrimSpace(line), "- ") && !strings.HasPrefix(strings.TrimSpace(line), "* ") {
			html.WriteString("</ul>")
			inList = false
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			html.WriteString("<h3>" + trimmed[4:] + "</h3>")
		} else if strings.HasPrefix(trimmed, "## ") {
			html.WriteString("<h2>" + trimmed[3:] + "</h2>")
		} else if strings.HasPrefix(trimmed, "# ") {
			html.WriteString("<h1>" + trimmed[2:] + "</h1>")
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				html.WriteString("<ul>")
				inList = true
			}
			html.WriteString("<li>" + trimmed[2:] + "</li>")
		} else {
			html.WriteString("<p>" + trimmed + "</p>")
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

func textToHTML(txt string) string {
	lines := strings.Split(txt, "\n")
	var html strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			html.WriteString("<br>")
		} else {
			html.WriteString("<p>" + strings.TrimSpace(line) + "</p>")
		}
	}
	return html.String()
}
