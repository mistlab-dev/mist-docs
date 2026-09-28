package tests

import (
	"testing"

	"github.com/c-wind/mist-docs/internal/database"
)

// Documents created before folders existed have folder_id NULL. Scanning that
// into a string failed, so the detail endpoint answered 404 and the editor
// could not open them, although they were listed.
func TestLegacyDocWithNullFolderOpens(t *testing.T) {
	docID := createTestDoc(t, adminToken, "旧文档-无文件夹")
	if _, err := database.DB.Exec(`UPDATE md_documents SET folder_id = NULL, updated_by = NULL WHERE id = ?`, docID); err != nil {
		t.Fatal(err)
	}

	w := request("GET", teamPath("/documents/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("detail of a doc with NULL folder_id: %d %s", w.Code, w.Body.String())
	}
	data := parseJSON(t, w)["data"].(map[string]interface{})
	if data["title"] != "旧文档-无文件夹" || data["folder_id"] != "" {
		t.Fatalf("unexpected detail: %v", data)
	}

	w = request("GET", teamPath("/documents/recent"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("recent: %d", w.Code)
	}
	found := false
	for _, d := range parseJSON(t, w)["data"].([]interface{}) {
		if m := d.(map[string]interface{}); m["id"] == docID {
			found = m["title"] == "旧文档-无文件夹"
		}
	}
	if !found {
		t.Fatal("recent list lost the title of a doc with NULL folder_id")
	}
}
