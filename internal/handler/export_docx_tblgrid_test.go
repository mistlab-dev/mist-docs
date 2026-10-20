package handler

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

func docxDocumentXMLForTest(t *testing.T, html string) string {
	t.Helper()
	data, err := htmlToDocx(html)
	if err != nil {
		t.Fatalf("htmlToDocx: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return string(b)
		}
	}
	t.Fatal("word/document.xml missing")
	return ""
}

// TestHtmlToDocx_TableGrid guards #43: CT_Tbl requires <w:tblGrid> after
// tblPr and before the first row, with one gridCol per (widest-row) column.
func TestHtmlToDocx_TableGrid(t *testing.T) {
	cases := []struct {
		name string
		html string
		cols int
	}{
		{"header+body", `<table><thead><tr><th>A</th><th>B</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>`, 2},
		{"ragged rows use widest", `<table><tr><td>a</td></tr><tr><td>b</td><td>c</td><td>d</td></tr></table>`, 3},
		{"empty table still has a grid", `<table></table>`, 1},
	}
	tblRe := regexp.MustCompile(`(?s)<w:tbl>.*?</w:tbl>`)
	colRe := regexp.MustCompile(`<w:gridCol w:w="(\d+)"/>`)
	for _, tc := range cases {
		doc := docxDocumentXMLForTest(t, tc.html)
		tbl := tblRe.FindString(doc)
		if tbl == "" {
			t.Fatalf("%s: no table", tc.name)
		}
		pr := strings.Index(tbl, "</w:tblPr>")
		grid := strings.Index(tbl, "<w:tblGrid>")
		tr := strings.Index(tbl, "<w:tr>")
		if pr < 0 || grid < 0 || grid < pr || (tr >= 0 && grid > tr) {
			t.Fatalf("%s: tblGrid must sit between tblPr and first tr: %s", tc.name, tbl)
		}
		if strings.Count(tbl, "<w:tblGrid>") != 1 {
			t.Fatalf("%s: want exactly one tblGrid", tc.name)
		}
		if got := len(colRe.FindAllString(tbl, -1)); got != tc.cols {
			t.Fatalf("%s: gridCol=%d want %d", tc.name, got, tc.cols)
		}
	}
}
