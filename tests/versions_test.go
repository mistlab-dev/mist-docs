package tests

import (
	"fmt"
	"testing"

	"github.com/c-wind/mist-docs/internal/database"
)

func saveContent(t *testing.T, docID, content string) map[string]interface{} {
	t.Helper()
	w := request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{"content": content}, adminToken)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	return parseJSON(t, w)
}

func versionContent(t *testing.T, docID string, ver int) string {
	t.Helper()
	w := request("GET", teamPath(fmt.Sprintf("/documents/%s/versions/%d/content", docID, ver)), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("version %d content: %d %s", ver, w.Code, w.Body.String())
	}
	return w.Body.String()
}

func versionRows(t *testing.T, docID string) int {
	t.Helper()
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_versions WHERE document_id=?`, docID).Scan(&n)
	return n
}

// Restoring an old version must add a new version with the old content and
// leave every existing version file untouched. It used to overwrite the file
// of the newest version, so the latest edit was gone for good.
func TestRestoreVersionCreatesNewVersion(t *testing.T) {
	docID := createTestDoc(t, adminToken, "restore-keeps-history")
	v1 := int(getFloat(saveContent(t, docID, "<p>first</p>")["version"]))
	v2 := int(getFloat(saveContent(t, docID, "<p>second</p>")["version"]))
	rowsBefore := versionRows(t, docID)

	w := request("POST", teamPath("/documents/"+docID+"/restore"), map[string]interface{}{"version": v1}, adminToken)
	if w.Code != 200 {
		t.Fatalf("restore: %d %s", w.Code, w.Body.String())
	}
	v3 := int(getFloat(parseJSON(t, w)["version"]))
	if v3 != v2+1 {
		t.Fatalf("restore should create version %d, got %d", v2+1, v3)
	}
	if got := versionContent(t, docID, v2); got != "<p>second</p>" {
		t.Fatalf("newest version before restore was overwritten: %q", got)
	}
	if got := versionContent(t, docID, v3); got != "<p>first</p>" {
		t.Fatalf("restored version content = %q", got)
	}
	if got := versionRows(t, docID); got != rowsBefore+1 {
		t.Fatalf("restore should add one version row: before=%d after=%d", rowsBefore, got)
	}

	w = request("GET", teamPath("/documents/"+docID+"/content"), nil, adminToken)
	data := parseJSON(t, w)["data"].(map[string]interface{})
	if getString(data["content"]) != "<p>first</p>" || int(getFloat(data["version"])) != v3 {
		t.Fatalf("current content after restore = %v", data)
	}
}

// Saving the same content again (what the editor does right after opening a
// document) must not create a version.
func TestSaveUnchangedContentKeepsVersion(t *testing.T) {
	docID := createTestDoc(t, adminToken, "no-empty-versions")
	v := int(getFloat(saveContent(t, docID, "<p>same</p>")["version"]))
	rows := versionRows(t, docID)

	resp := saveContent(t, docID, "<p>same</p>")
	if int(getFloat(resp["version"])) != v || resp["unchanged"] != true {
		t.Fatalf("unchanged save should keep version %d, got %v", v, resp)
	}
	if got := versionRows(t, docID); got != rows {
		t.Fatalf("unchanged save added a version row: %d -> %d", rows, got)
	}

	// Restoring the version that is already current is a no-op too.
	w := request("POST", teamPath("/documents/"+docID+"/restore"), map[string]interface{}{"version": v}, adminToken)
	if w.Code != 200 || parseJSON(t, w)["unchanged"] != true {
		t.Fatalf("restoring the current version should be a no-op: %d %s", w.Code, w.Body.String())
	}
	if got := versionRows(t, docID); got != rows {
		t.Fatalf("no-op restore added a version row: %d -> %d", rows, got)
	}
}
