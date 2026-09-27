package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type BatchImportResult struct {
	Title  string `json:"title"`
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ─── Word (.docx) → HTML ───

type wDocument struct {
	XMLName xml.Name `xml:"document"`
	Body    wBody    `xml:"body"`
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
	RPr  *wRPr `xml:"rPr"`
}
type wRPr struct {
	Bold      *struct{} `xml:"b"`
	Italic    *struct{} `xml:"i"`
	Underline *struct{} `xml:"u"`
	Strike    *struct{} `xml:"strike"`
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
		// Build runs with inline formatting
		var runsHTML strings.Builder
		hasContent := false
		for _, run := range p.Runs {
			t := run.Text.Text
			if strings.TrimSpace(t) == "" {
				// Preserve whitespace-only runs as a single space
				if t != "" {
					runsHTML.WriteString(" ")
				}
				continue
			}
			hasContent = true
			escaped := escapeHTML(t)
			// Apply inline formatting based on rPr
			if run.RPr != nil {
				if run.RPr.Strike != nil {
					escaped = "<s>" + escaped + "</s>"
				}
				if run.RPr.Underline != nil {
					escaped = "<u>" + escaped + "</u>"
				}
				if run.RPr.Italic != nil {
					escaped = "<em>" + escaped + "</em>"
				}
				if run.RPr.Bold != nil {
					escaped = "<strong>" + escaped + "</strong>"
				}
			}
			runsHTML.WriteString(escaped)
		}
		if !hasContent {
			html.WriteString("<p><br></p>")
			continue
		}

		style := ""
		if p.PPr != nil {
			style = p.PPr.PStyle.Val
		}
		content := runsHTML.String()
		switch {
		case strings.Contains(style, "Heading1") || strings.HasSuffix(style, "1"):
			html.WriteString("<h1>" + content + "</h1>")
		case strings.Contains(style, "Heading2") || strings.HasSuffix(style, "2"):
			html.WriteString("<h2>" + content + "</h2>")
		case strings.Contains(style, "Heading3") || strings.HasSuffix(style, "3") || strings.HasSuffix(style, "4"):
			html.WriteString("<h3>" + content + "</h3>")
		default:
			html.WriteString("<p>" + content + "</p>")
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
			} else if cell.Type == "b" {
				switch cell.Value {
				case "1":
					value = "true"
				default:
					value = "false"
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
	if maxCol > 702 {
		maxCol = 702
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

	// 支持 simple text (<si><t>text</t></si>) 和 rich text (<si><r><t>text</t></r>...</si>)
	type xSI struct {
		XMLName xml.Name `xml:"si"`
		Text    string   `xml:"t"` // simple text
		Runs    []struct {
			Text string `xml:"t"`
		} `xml:"r"` // rich text runs
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
		if item.Text != "" {
			result[i] = item.Text
		} else {
			// rich text: concatenate all runs
			var sb strings.Builder
			for _, run := range item.Runs {
				sb.WriteString(run.Text)
			}
			result[i] = sb.String()
		}
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
		idx = idx*26 + int(ch-'A') + 1
	}
	return idx - 1
}

func parseSimpleInt(s string) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("not a number")
	}
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
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
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
			html.WriteString(escapeHTML(line) + "\n")
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
			html.WriteString("<h3>" + escapeHTML(trimmed[4:]) + "</h3>")
		} else if strings.HasPrefix(trimmed, "## ") {
			html.WriteString("<h2>" + escapeHTML(trimmed[3:]) + "</h2>")
		} else if strings.HasPrefix(trimmed, "# ") {
			html.WriteString("<h1>" + escapeHTML(trimmed[2:]) + "</h1>")
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				html.WriteString("<ul>")
				inList = true
			}
			html.WriteString("<li>" + escapeHTML(trimmed[2:]) + "</li>")
		} else {
			html.WriteString("<p>" + escapeHTML(trimmed) + "</p>")
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
	lines := strings.Split(strings.ReplaceAll(txt, "\r\n", "\n"), "\n")
	var html strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			html.WriteString("<br>")
		} else {
			html.WriteString("<p>" + escapeHTML(strings.TrimSpace(line)) + "</p>")
		}
	}
	return html.String()
}
