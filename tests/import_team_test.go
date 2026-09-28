package tests

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/c-wind/mist-docs/internal/database"
)

func importFiles(t *testing.T, token, field string, files map[string]string) (int, map[string]interface{}) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for name, content := range files {
		part, err := writer.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte(content))
	}
	writer.Close()
	req := httptest.NewRequest("POST", teamPath("/import"), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code, parseJSON(t, w)
}

func docContent(t *testing.T, docID string) string {
	t.Helper()
	w := request("GET", teamPath("/documents/"+docID+"/content"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("content: %d %s", w.Code, w.Body.String())
	}
	return getString(parseJSON(t, w)["data"].(map[string]interface{})["content"])
}

// The import dialog posts several files under "files". The handler used to
// read only "file" and answered 请选择文件, and stored Markdown unconverted.
func TestImportFilesFieldConvertsMarkdown(t *testing.T) {
	code, resp := importFiles(t, adminToken, "files", map[string]string{
		"plan.md":  "# 计划\n\n- 第一步\n- 第二步\n\n```\n<script>x</script>\n```",
		"memo.txt": "第一行\r\n第二行",
	})
	if code != 200 {
		t.Fatalf("import: %d %v", code, resp)
	}
	results := resp["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %v", results)
	}
	byTitle := map[string]string{}
	for _, r := range results {
		m := r.(map[string]interface{})
		if m["status"] != "created" {
			t.Fatalf("file not created: %v", m)
		}
		byTitle[getString(m["title"])] = getString(m["id"])
	}

	md := docContent(t, byTitle["plan"])
	for _, want := range []string{"<h1>计划</h1>", "<li>第一步</li>", "&lt;script&gt;"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown not converted, missing %q in %q", want, md)
		}
	}
	if strings.Contains(md, "# 计划") || strings.Contains(md, "<script>") {
		t.Fatalf("markdown stored raw or unescaped: %q", md)
	}
	if txt := docContent(t, byTitle["memo"]); txt != "<p>第一行</p><p>第二行</p>" {
		t.Fatalf("text import = %q", txt)
	}

	// Imported documents start with a history entry (version 1).
	if versionRows(t, byTitle["plan"]) != 1 {
		t.Fatalf("imported document should have version 1 recorded")
	}
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_audits WHERE action='import_doc' AND resource_id=?`, byTitle["plan"]).Scan(&n)
	if n != 1 {
		t.Fatalf("import should be audited once, got %d", n)
	}
}

func TestImportRejectsViewerAndUnsupported(t *testing.T) {
	if code, _ := importFiles(t, viewerToken, "files", map[string]string{"a.md": "# a"}); code != 403 {
		t.Fatalf("viewer import should be 403, got %d", code)
	}
	code, resp := importFiles(t, adminToken, "files", map[string]string{"virus.exe": "MZ"})
	if code != 400 {
		t.Fatalf("unsupported-only import should be 400, got %d %v", code, resp)
	}
}
