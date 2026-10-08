package tests

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/google/uuid"
)

// 1x1 PNG
var tinyPNG = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 0x49, 0x48, 0x44, 0x52, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89, 0, 0, 0, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0, 1, 0, 0, 5, 0, 1, 0x0d, 0x0a, 0x2d, 0xb4, 0, 0, 0, 0, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}

type folderFile struct {
	path string
	data []byte
}

func sampleFolder(root string) []folderFile {
	return []folderFile{
		{root + "/README.md", []byte("# 运维手册\n\n先看 [上线检查](部署/上线检查.md)。\n\n![总览](images/总览.png)")},
		{root + "/部署/上线检查.md", []byte("---\ntitle: 上线前检查\n---\n## 步骤\n\n- [ ] 备份\n- [x] 通知\n\n![流程](../images/总览.png)\n![截图](./img/a%20b.png)\n![丢了](missing.png)")},
		{root + "/部署/img/a b.png", tinyPNG},
		{root + "/部署/数据库/变更记录.md", []byte("# 变更\n\n回到 [首页](../../README.md)")},
		{root + "/images/总览.png", tinyPNG},
		{root + "/images/unused.png", tinyPNG},
		{root + "/.git/config", []byte("x")},
		{root + "/notes.pdf", []byte("%PDF")},
	}
}

func postFolder(t *testing.T, token string, files []folderFile, extra map[string]string) (int, map[string]interface{}) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for _, f := range files {
		part, _ := w.CreateFormFile("files", f.path[strings.LastIndex(f.path, "/")+1:])
		part.Write(f.data)
		w.WriteField("paths", f.path)
	}
	for k, v := range extra {
		w.WriteField(k, v)
	}
	w.Close()
	return sendMultipart(t, token, body, w.FormDataContentType())
}

func sendMultipart(t *testing.T, token string, body *bytes.Buffer, ct string) (int, map[string]interface{}) {
	req := httptest.NewRequest("POST", teamPath("/import/folder"), body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code, parseJSON(t, rec)
}

func folderChildren(t *testing.T, parent string) map[string]string {
	rows, err := database.DB.Query(`SELECT id, name FROM md_team_folders WHERE team_id=? AND parent_id=?`, teamID, parent)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		rows.Scan(&id, &name)
		out[name] = id
	}
	return out
}

func docsIn(t *testing.T, folder string) map[string]string {
	rows, err := database.DB.Query(`SELECT id, title FROM md_documents WHERE team_id=? AND folder_id=? AND status=1`, teamID, folder)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, title string
		rows.Scan(&id, &title)
		out[title] = id
	}
	return out
}

func checkImportedTree(t *testing.T, resp map[string]interface{}, rootName string) {
	t.Helper()
	data := resp["data"].(map[string]interface{})
	if getString(data["folder_name"]) != rootName {
		t.Fatalf("folder_name = %v", data["folder_name"])
	}
	if data["documents"].(float64) != 3 || data["images"].(float64) != 2 {
		t.Fatalf("counts = %v", data)
	}
	rootID := getString(data["folder_id"])

	// Structure: root/{README}, root/部署/{上线前检查}, root/部署/数据库/{变更记录}.
	// images/ and 部署/img/ hold only pictures, so they are not folders.
	sub := folderChildren(t, rootID)
	if len(sub) != 1 || sub["部署"] == "" {
		t.Fatalf("root children = %v", sub)
	}
	deploy := folderChildren(t, sub["部署"])
	if len(deploy) != 1 || deploy["数据库"] == "" {
		t.Fatalf("部署 children = %v", deploy)
	}
	rootDocs, deployDocs, dbDocs := docsIn(t, rootID), docsIn(t, sub["部署"]), docsIn(t, deploy["数据库"])
	if rootDocs["README"] == "" || deployDocs["上线前检查"] == "" || dbDocs["变更记录"] == "" {
		t.Fatalf("docs: root=%v deploy=%v db=%v", rootDocs, deployDocs, dbDocs)
	}

	readme := docContent(t, rootDocs["README"])
	if !strings.Contains(readme, `href="/docs/`+deployDocs["上线前检查"]+`"`) {
		t.Fatalf("link to sibling doc not rewritten: %s", readme)
	}
	if !strings.Contains(readme, `<img src="/api/media/`+teamID+`/`) {
		t.Fatalf("image not stored: %s", readme)
	}
	check := docContent(t, deployDocs["上线前检查"])
	if strings.Count(check, `<img src="/api/media/`) != 2 || !strings.Contains(check, `data-type="taskList"`) ||
		!strings.Contains(check, "图片没有导入：missing.png") {
		t.Fatalf("check doc: %s", check)
	}
	// The same picture used by two documents is stored once.
	src := func(html string) string {
		i := strings.Index(html, `<img src="`) + len(`<img src="`)
		return html[i : i+strings.Index(html[i:], `"`)]
	}
	if src(readme) != src(check) {
		t.Fatalf("shared image stored twice: %s vs %s", src(readme), src(check))
	}
	// The image URL works without login (signed), like editor uploads.
	w := request("GET", src(readme), nil, "")
	if w.Code != 200 {
		t.Fatalf("image fetch: %d", w.Code)
	}
	if dbc := docContent(t, dbDocs["变更记录"]); !strings.Contains(dbc, `href="/docs/`+rootDocs["README"]+`"`) {
		t.Fatalf("parent link not rewritten: %s", dbc)
	}

	warns := resp["warnings"].([]interface{})
	var joined []string
	for _, w := range warns {
		m := w.(map[string]interface{})
		joined = append(joined, getString(m["path"])+":"+getString(m["message"]))
	}
	all := strings.Join(joined, "\n")
	if !strings.Contains(all, "notes.pdf") || !strings.Contains(all, "missing.png") || strings.Contains(all, ".git") {
		t.Fatalf("warnings = %s", all)
	}
}

func TestImportFolderKeepsStructureAndImages(t *testing.T) {
	root := "运维手册-" + uuid.New().String()[:8] // the test database keeps folders between runs
	code, resp := postFolder(t, adminToken, sampleFolder(root), nil)
	if code != 200 {
		t.Fatalf("import folder: %d %v", code, resp)
	}
	checkImportedTree(t, resp, root)

	// Importing the same folder again does not merge into the first one.
	code, resp = postFolder(t, adminToken, sampleFolder(root), nil)
	if code != 200 {
		t.Fatalf("second import: %d %v", code, resp)
	}
	checkImportedTree(t, resp, root+" (2)")
}

func TestImportFolderZip(t *testing.T) {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	root := "手册zip-" + uuid.New().String()[:8]
	for _, f := range sampleFolder(root) {
		fw, _ := zw.Create(f.path)
		fw.Write(f.data)
	}
	zw.Close()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("archive", root+".zip")
	part.Write(buf.Bytes())
	w.Close()
	code, resp := sendMultipart(t, adminToken, body, w.FormDataContentType())
	if code != 200 {
		t.Fatalf("zip import: %d %v", code, resp)
	}
	checkImportedTree(t, resp, root)
}

func TestImportFolderRules(t *testing.T) {
	// Folder structure is an admin action (same as POST /folders).
	if code, _ := postFolder(t, editorToken, sampleFolder("编辑者"), nil); code != 403 {
		t.Fatalf("editor import folder = %d, want 403", code)
	}
	// Paths cannot escape the folder.
	code, resp := postFolder(t, adminToken, []folderFile{
		{"../../etc/passwd.md", []byte("# x")},
		{"/abs/../../y.md", []byte("# y")},
	}, nil)
	if code == 200 {
		data := resp["data"].(map[string]interface{})
		for name := range folderChildren(t, getString(data["folder_id"])) {
			if strings.Contains(name, "..") || name == "etc" {
				t.Fatalf("escaped path became folder %q", name)
			}
		}
	}
	// Nothing importable: no folders are left behind.
	before := len(folderChildren(t, ""))
	code, _ = postFolder(t, adminToken, []folderFile{{"空/a.pdf", []byte("x")}}, nil)
	if code != 400 {
		t.Fatalf("no docs = %d, want 400", code)
	}
	if after := len(folderChildren(t, "")); after != before {
		t.Fatalf("folders left behind: %d -> %d", before, after)
	}
}
