package tests

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/middleware"
)

// Extra members for the D2 matrix: a team owner and a second editor, so
// "own record" rules can be told apart from "any editor".
const (
	ownerID   = "test-user-owner"
	editor2ID = "test-user-editor2"
)

func roleFixtures(t *testing.T) (ownerTok, editor2Tok string) {
	t.Helper()
	for _, u := range []struct{ id, role string }{{ownerID, "owner"}, {editor2ID, "editor"}} {
		database.DB.Exec(`INSERT IGNORE INTO users (id, email, username, display_name, password_hash, is_admin, email_verified)
			VALUES (?, ?, ?, ?, 'x', 0, 1)`, u.id, u.id+"@mistdocs.invalid", u.id, u.id)
		database.DB.Exec(`INSERT IGNORE INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)`, teamID, u.id, u.role)
	}
	ownerTok, _ = middleware.GenerateToken(ownerID, "owner", "member", "")
	editor2Tok, _ = middleware.GenerateToken(editor2ID, "editor2", "member", "")
	return
}

type roleCase struct {
	name   string
	tokens map[string]string // role label -> token
}

func tokensByRole(t *testing.T) map[string]string {
	ownerTok, _ := roleFixtures(t)
	return map[string]string{"viewer": viewerToken, "editor": editorToken, "admin": adminToken, "owner": ownerTok}
}

// allowed lists the roles that must succeed; the others must get 403.
type matrixRow struct {
	action  string
	allowed []string
	// run performs the call as token and returns the HTTP status. setup-free
	// rows create whatever they need with the admin token.
	run func(t *testing.T, token string) int
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func runMatrix(t *testing.T, rows []matrixRow) {
	tokens := tokensByRole(t)
	for _, row := range rows {
		for _, role := range []string{"viewer", "editor", "admin", "owner"} {
			row, role := row, role
			t.Run(fmt.Sprintf("%s/%s", row.action, role), func(t *testing.T) {
				code := row.run(t, tokens[role])
				ok := code >= 200 && code < 300
				if contains(row.allowed, role) && !ok {
					t.Fatalf("%s as %s should be allowed, got %d", row.action, role, code)
				}
				if !contains(row.allowed, role) && code != 403 {
					t.Fatalf("%s as %s should be 403, got %d", row.action, role, code)
				}
			})
		}
	}
}

func newFolder(t *testing.T) string {
	w := request("POST", teamPath("/folders"), map[string]string{"name": "矩阵文件夹"}, adminToken)
	if w.Code != 200 {
		t.Fatalf("admin create folder: %d %s", w.Code, w.Body.String())
	}
	return getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])
}

func newTemplate(t *testing.T, token string) string {
	w := request("POST", teamPath("/templates"), map[string]string{"name": "矩阵模板", "type": "doc", "content": "<p>x</p>"}, token)
	if w.Code != 200 {
		t.Fatalf("create template: %d %s", w.Code, w.Body.String())
	}
	return getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])
}

func newDeadline(t *testing.T, token, owner string) string {
	w := request("POST", teamPath("/deadlines"), map[string]interface{}{
		"order_no": "SO-MATRIX", "title": "矩阵交期", "due_date": "2026-12-01", "owner_id": owner,
	}, token)
	if w.Code != 200 {
		t.Fatalf("create deadline: %d %s", w.Code, w.Body.String())
	}
	return getString(parseJSON(t, w)["id"])
}

func newRule(t *testing.T) string {
	w := request("POST", teamPath("/reminder-rules"), map[string]interface{}{"name": "矩阵规则", "offset_days": 2, "channel": "inapp", "target": "owner"}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create rule: %d %s", w.Code, w.Body.String())
	}
	return getString(parseJSON(t, w)["id"])
}

func upload(t *testing.T, token, name string) (int, string) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", name)
	part.Write([]byte("PNGDATA"))
	mw.Close()
	req := httptest.NewRequest("POST", teamPath("/upload"), body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		return w.Code, ""
	}
	return w.Code, getString(parseJSON(t, w)["data"].(map[string]interface{})["filename"])
}

func ensureFragment(t *testing.T) string {
	database.DB.Exec(`INSERT IGNORE INTO fragments (id, team_id, title, command, category, status, deleted)
		VALUES ('test-frag-matrix', ?, '矩阵片段', 'echo ok', 'test', 'published', 0)`, teamID)
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM fragments WHERE id='test-frag-matrix'`) })
	return "test-frag-matrix"
}

func TestRoleMatrix(t *testing.T) {
	admins := []string{"admin", "owner"}
	editors := []string{"editor", "admin", "owner"}
	frag := ensureFragment(t)

	runMatrix(t, []matrixRow{
		// Folders: admin only (D2 follows the design, not the old behaviour).
		{"folder.create", admins, func(t *testing.T, tok string) int {
			return request("POST", teamPath("/folders"), map[string]string{"name": "f"}, tok).Code
		}},
		{"folder.rename", admins, func(t *testing.T, tok string) int {
			return request("PUT", teamPath("/folders/"+newFolder(t)), map[string]string{"name": "改名"}, tok).Code
		}},
		{"folder.delete", admins, func(t *testing.T, tok string) int {
			return request("DELETE", teamPath("/folders/"+newFolder(t)), nil, tok).Code
		}},
		// Templates: editors create; only admins touch someone else's.
		{"template.create", editors, func(t *testing.T, tok string) int {
			return request("POST", teamPath("/templates"), map[string]string{"name": "t", "content": "x"}, tok).Code
		}},
		{"template.update-others", admins, func(t *testing.T, tok string) int {
			id := newTemplate(t, editorToken)
			if tok == editorToken { // the author is allowed; test a different editor instead
				_, tok = roleFixtures(t)
			}
			return request("PUT", teamPath("/templates/"+id), map[string]string{"name": "改", "content": "y"}, tok).Code
		}},
		{"template.delete-others", admins, func(t *testing.T, tok string) int {
			id := newTemplate(t, editorToken)
			if tok == editorToken {
				_, tok = roleFixtures(t)
			}
			return request("DELETE", teamPath("/templates/"+id), nil, tok).Code
		}},
		// Media: editors upload; deleting someone else's upload is admin-only.
		{"media.upload", editors, func(t *testing.T, tok string) int {
			code, _ := upload(t, tok, "a.png")
			return code
		}},
		{"media.delete-others", admins, func(t *testing.T, tok string) int {
			_, fn := upload(t, adminToken, "b.png")
			if tok == adminToken { // uploader == admin here; use a file from another editor
				_, fn = upload(t, editorToken, "c.png")
			}
			return request("DELETE", teamPath("/media/"+fn), nil, tok).Code
		}},
		// Snippets on a document need write access to that document.
		{"fragment.attach", editors, func(t *testing.T, tok string) int {
			doc := createTestDoc(t, adminToken, "片段矩阵")
			return request("POST", teamPath("/documents/"+doc+"/fragments"), map[string]string{"fragment_id": frag}, tok).Code
		}},
		{"fragment.detach", editors, func(t *testing.T, tok string) int {
			doc := createTestDoc(t, adminToken, "片段矩阵")
			request("POST", teamPath("/documents/"+doc+"/fragments"), map[string]string{"fragment_id": frag}, adminToken)
			return request("DELETE", teamPath("/documents/"+doc+"/fragments/"+frag), nil, tok).Code
		}},
		// Deadlines: editors create/update; delete is creator, 负责人 or admin.
		{"deadline.create", editors, func(t *testing.T, tok string) int {
			return request("POST", teamPath("/deadlines"), map[string]interface{}{"order_no": "SO-M", "title": "t", "due_date": "2026-12-01"}, tok).Code
		}},
		{"deadline.update", editors, func(t *testing.T, tok string) int {
			id := newDeadline(t, adminToken, adminID)
			return request("PUT", teamPath("/deadlines/"+id), map[string]interface{}{"order_no": "SO-M", "title": "改", "due_date": "2026-12-02"}, tok).Code
		}},
		{"deadline.delete-others", admins, func(t *testing.T, tok string) int {
			id := newDeadline(t, adminToken, adminID)
			return request("DELETE", teamPath("/deadlines/"+id), nil, tok).Code
		}},
		// Reminder rules: admin only.
		{"rule.create", admins, func(t *testing.T, tok string) int {
			return request("POST", teamPath("/reminder-rules"), map[string]interface{}{"name": "r", "offset_days": 1, "channel": "inapp", "target": "owner"}, tok).Code
		}},
		{"rule.update", admins, func(t *testing.T, tok string) int {
			return request("PUT", teamPath("/reminder-rules/"+newRule(t)), map[string]interface{}{"enabled": false}, tok).Code
		}},
		{"rule.delete", admins, func(t *testing.T, tok string) int {
			return request("DELETE", teamPath("/reminder-rules/"+newRule(t)), nil, tok).Code
		}},
		{"rule.seed", admins, func(t *testing.T, tok string) int {
			return request("POST", teamPath("/reminder-rules/seed"), nil, tok).Code
		}},
		// owner counts as admin (D4).
		{"system-info", admins, func(t *testing.T, tok string) int {
			return request("GET", teamPath("/system-info"), nil, tok).Code
		}},
	})
}

// "Own record" rules: authors may manage their own templates, uploads and
// deadlines even though they are not admins.
func TestRoleOwnRecords(t *testing.T) {
	_, editor2Tok := roleFixtures(t)

	tpl := newTemplate(t, editorToken)
	if w := request("PUT", teamPath("/templates/"+tpl), map[string]string{"name": "自己改", "content": "z"}, editorToken); w.Code != 200 {
		t.Fatalf("author should edit own template: %d", w.Code)
	}
	if w := request("DELETE", teamPath("/templates/"+tpl), nil, editorToken); w.Code != 200 {
		t.Fatalf("author should delete own template: %d", w.Code)
	}

	_, fn := upload(t, editorToken, "mine.png")
	if w := request("DELETE", teamPath("/media/"+fn), nil, editor2Tok); w.Code != 403 {
		t.Fatalf("other editor must not delete my upload: %d", w.Code)
	}
	if w := request("DELETE", teamPath("/media/"+fn), nil, editorToken); w.Code != 200 {
		t.Fatalf("uploader should delete own file: %d %s", w.Code, w.Body.String())
	}

	// Legacy file without an md_media row: admin only.
	_, legacy := upload(t, editorToken, "legacy.png")
	database.DB.Exec(`DELETE FROM md_media WHERE filename=?`, legacy)
	if w := request("DELETE", teamPath("/media/"+legacy), nil, editorToken); w.Code != 403 {
		t.Fatalf("untracked file must be admin-only, editor got %d", w.Code)
	}
	if w := request("DELETE", teamPath("/media/"+legacy), nil, adminToken); w.Code != 200 {
		t.Fatalf("admin should delete untracked file: %d", w.Code)
	}

	created := newDeadline(t, editorToken, adminID)
	if w := request("DELETE", teamPath("/deadlines/"+created), nil, editorToken); w.Code != 200 {
		t.Fatalf("creator should delete own deadline: %d %s", w.Code, w.Body.String())
	}
	owned := newDeadline(t, adminToken, editor2ID)
	if w := request("DELETE", teamPath("/deadlines/"+owned), nil, editor2Tok); w.Code != 200 {
		t.Fatalf("负责人 should delete the deadline: %d %s", w.Code, w.Body.String())
	}

	// A document shared read-only with an editor blocks snippet changes.
	frag := ensureFragment(t)
	doc := createTestDoc(t, adminToken, "只读片段")
	request("POST", teamPath("/permissions"), map[string]interface{}{
		"resource_type": "document", "resource_id": doc, "target_type": "user", "target_id": editorID, "permission": "read",
	}, adminToken)
	if w := request("POST", teamPath("/documents/"+doc+"/fragments"), map[string]string{"fragment_id": frag}, editorToken); w.Code != 403 {
		t.Fatalf("read-only editor attached a snippet: %d", w.Code)
	}
}

// Notifications and favorites only ever touch the caller's own rows.
func TestNotificationsAndFavoritesAreOwn(t *testing.T) {
	mustExec(t, `INSERT INTO md_notifications (id, user_id, team_id, type, title, is_read) VALUES ('test-notif-admin', ?, ?, 'comment', '给管理员的', 0)`, adminID, teamID)
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM md_notifications WHERE id='test-notif-admin'`) })

	if w := request("PUT", teamPath("/notifications/test-notif-admin/read"), nil, editorToken); w.Code == 200 {
		t.Fatalf("editor marked someone else's notification read")
	}
	if w := request("DELETE", teamPath("/notifications/test-notif-admin"), nil, editorToken); w.Code == 200 {
		var n int
		database.DB.QueryRow(`SELECT COUNT(*) FROM md_notifications WHERE id='test-notif-admin'`).Scan(&n)
		if n == 0 {
			t.Fatalf("editor deleted someone else's notification")
		}
	}
	var read int
	database.DB.QueryRow(`SELECT is_read FROM md_notifications WHERE id='test-notif-admin'`).Scan(&read)
	if read != 0 {
		t.Fatalf("notification of another user changed")
	}

	doc := createTestDoc(t, adminToken, "收藏隔离")
	request("POST", teamPath("/favorites/"+doc), nil, adminToken)
	request("DELETE", teamPath("/favorites/"+doc), nil, editorToken)
	var fav int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_favorites WHERE user_id=? AND document_id=?`, adminID, doc).Scan(&fav)
	if fav != 1 {
		t.Fatalf("editor removed the admin's favorite")
	}
}

// Regression: favorites were inserted without an id, so only the very first
// favorite in the database was ever stored.
func TestFavoritesSeveralDocuments(t *testing.T) {
	a := createTestDoc(t, editorToken, "收藏一")
	b := createTestDoc(t, editorToken, "收藏二")
	for _, d := range []string{a, b} {
		if w := request("POST", teamPath("/favorites/"+d), nil, editorToken); w.Code != 200 {
			t.Fatalf("add favorite: %d", w.Code)
		}
	}
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_favorites WHERE user_id=? AND document_id IN (?, ?)`, editorID, a, b).Scan(&n)
	if n != 2 {
		t.Fatalf("stored %d favorites, want 2", n)
	}
}
