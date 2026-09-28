package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
)

// auditRows returns action -> resource_name for a document's audit rows.
func auditRows(t *testing.T, docID string) map[string]string {
	t.Helper()
	rows, err := database.DB.Query(`SELECT action, IFNULL(resource_name,'') FROM md_audits WHERE resource_id=?`, docID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var a, n string
		rows.Scan(&a, &n)
		out[a] = n
	}
	return out
}

// Phase 2.1: sharing, comments, collaborators, locks and deletions leave an
// audit trail, and every row carries the document title (extra bug 5: edit
// rows showed "—").
func TestAuditTrailForDocumentActions(t *testing.T) {
	doc := createTestDoc(t, adminToken, "审计轨迹文档")
	request("PUT", teamPath("/documents/"+doc+"/content"), map[string]string{"content": "<p>改过</p>"}, adminToken)

	w := request("POST", teamPath("/documents/"+doc+"/share"), map[string]interface{}{"password": "s3cret", "expiresIn": 24}, adminToken)
	shareID := getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])
	request("DELETE", teamPath("/shares/"+shareID), nil, adminToken)

	w = request("POST", teamPath("/documents/"+doc+"/comments"), map[string]string{"content": "请补充机柜编号"}, adminToken)
	commentID := getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])
	request("DELETE", teamPath("/comments/"+commentID), nil, adminToken)

	request("POST", teamPath("/documents/"+doc+"/collaborators"), map[string]string{"target_id": editorID, "role": "viewer"}, adminToken)
	w = request("GET", teamPath("/documents/"+doc+"/collaborators"), nil, adminToken)
	var collabID string
	for _, it := range parseJSON(t, w)["data"].([]interface{}) {
		if m := it.(map[string]interface{}); m["target_id"] == editorID {
			collabID = getString(m["id"])
		}
	}
	request("PUT", teamPath("/collaborators/"+collabID), map[string]string{"role": "editor"}, adminToken)
	request("DELETE", teamPath("/collaborators/"+collabID), nil, adminToken)

	request("POST", teamPath("/documents/"+doc+"/lock"), nil, adminToken)
	request("POST", teamPath("/documents/"+doc+"/unlock"), nil, adminToken)
	request("POST", teamPath("/documents/"+doc+"/restore"), map[string]int{"version": 1}, adminToken)

	got := auditRows(t, doc)
	for _, action := range []string{
		"create_doc", "edit_doc", "create_share", "delete_share", "create_comment", "delete_comment",
		"add_collaborator", "update_collaborator", "remove_collaborator", "lock_doc", "unlock_doc", "restore_doc",
	} {
		name, ok := got[action]
		if !ok {
			t.Errorf("no %s audit row (have %v)", action, got)
			continue
		}
		if name != "审计轨迹文档" {
			t.Errorf("%s row has resource_name %q, want the document title", action, name)
		}
	}
	if _, legacy := got["restore"]; legacy {
		t.Error("version restore should be written as restore_doc (D10)")
	}

	// The share password never lands in the audit detail.
	var detail string
	database.DB.QueryRow(`SELECT detail FROM md_audits WHERE resource_id=? AND action='create_share'`, doc).Scan(&detail)
	if detail == "" || strings.Contains(detail, "s3cret") || !json.Valid([]byte(detail)) {
		t.Errorf("create_share detail = %q", detail)
	}
}

// Rows written before this change have no name; the list fills it in.
func TestAuditListFillsMissingNames(t *testing.T) {
	doc := createTestDoc(t, adminToken, "旧审计行文档")
	mustExec(t, `INSERT INTO md_audits (id, user_id, user_name, team_id, action, resource_type, resource_id, resource_name, detail)
		VALUES ('test-audit-legacy', ?, 'admin', ?, 'edit_doc', 'document', ?, '', '')`, adminID, teamID, doc)
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM md_audits WHERE id='test-audit-legacy'`) })

	w := request("GET", teamPath("/audits?action=edit_doc&page_size=100"), nil, adminToken)
	for _, it := range parseJSON(t, w)["data"].([]interface{}) {
		m := it.(map[string]interface{})
		if m["id"] == "test-audit-legacy" {
			if m["resource_name"] != "旧审计行文档" {
				t.Fatalf("legacy row name = %v", m["resource_name"])
			}
			return
		}
	}
	t.Fatal("legacy row not listed")
}

// D10: filtering by restore_doc also finds historical "restore" rows.
func TestAuditFilterRestoreIncludesLegacy(t *testing.T) {
	doc := createTestDoc(t, adminToken, "恢复筛选")
	mustExec(t, `INSERT INTO md_audits (id, user_id, user_name, team_id, action, resource_type, resource_id, resource_name, detail)
		VALUES ('test-audit-restore', ?, 'admin', ?, 'restore', 'document', ?, '恢复筛选', '')`, adminID, teamID, doc)
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM md_audits WHERE id='test-audit-restore'`) })
	w := request("GET", teamPath("/audits?action=restore_doc&page_size=100"), nil, adminToken)
	for _, it := range parseJSON(t, w)["data"].([]interface{}) {
		if it.(map[string]interface{})["id"] == "test-audit-restore" {
			return
		}
	}
	t.Fatal("restore_doc filter did not include the legacy restore row")
}

type hookSink struct {
	srv    *httptest.Server
	mu     sync.Mutex
	events []string
}

func newHookSink(t *testing.T) *hookSink {
	s := &hookSink{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Event string `json:"event"`
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &p)
		s.mu.Lock()
		s.events = append(s.events, p.Event)
		s.mu.Unlock()
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *hookSink) got() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.events...)
}

// Phase 2.4: webhooks deliver only the events they subscribe to, using the
// dotted names; PUT /webhooks/:id changes the subscription.
func TestWebhookDeliversSubscribedEventsOnly(t *testing.T) {
	shares, comments := newHookSink(t), newHookSink(t)
	var ids []string
	for _, h := range []struct {
		url    string
		events []string
	}{{shares.srv.URL, []string{"document.shared"}}, {comments.srv.URL, []string{"comment.created"}}} {
		w := request("POST", teamPath("/webhooks"), map[string]interface{}{"name": "t", "url": h.url, "events": h.events}, adminToken)
		if w.Code != 200 {
			t.Fatalf("create webhook: %d %s", w.Code, w.Body.String())
		}
		ids = append(ids, getString(parseJSON(t, w)["data"].(map[string]interface{})["id"]))
	}
	t.Cleanup(func() {
		for _, id := range ids {
			database.DB.Exec(`DELETE FROM md_webhook_logs WHERE webhook_id=?`, id)
			database.DB.Exec(`DELETE FROM md_webhooks WHERE id=?`, id)
		}
	})

	doc := createTestDoc(t, adminToken, "Webhook 订阅")
	request("POST", teamPath("/documents/"+doc+"/share"), map[string]interface{}{}, adminToken)
	waitFor(t, func() bool { return len(shares.got()) == 1 }, "share hook got document.shared")
	time.Sleep(200 * time.Millisecond)
	if g := shares.got(); g[0] != "document.shared" {
		t.Fatalf("share hook events = %v", g)
	}
	if g := comments.got(); len(g) != 0 {
		t.Fatalf("comment hook received %v for a share", g)
	}

	// Switch the comment hook to shares too.
	w := request("PUT", teamPath("/webhooks/"+ids[1]), map[string]interface{}{"events": []string{"document.shared", "comment.created"}}, adminToken)
	if w.Code != 200 {
		t.Fatalf("update webhook: %d %s", w.Code, w.Body.String())
	}
	request("POST", teamPath("/documents/"+doc+"/share"), map[string]interface{}{}, adminToken)
	waitFor(t, func() bool { return len(comments.got()) == 1 }, "updated hook got document.shared")

	// Unknown events and editors are refused.
	if w := request("PUT", teamPath("/webhooks/"+ids[1]), map[string]interface{}{"events": []string{"document.exploded"}}, adminToken); w.Code != 400 {
		t.Fatalf("unknown event accepted: %d", w.Code)
	}
	if w := request("PUT", teamPath("/webhooks/"+ids[1]), map[string]interface{}{"name": "x"}, editorToken); w.Code != 403 {
		t.Fatalf("editor updated a webhook: %d", w.Code)
	}

	// The list shows the catalogue and each hook's canonical events.
	w = request("GET", teamPath("/webhooks"), nil, adminToken)
	body := parseJSON(t, w)
	if len(body["available_events"].([]interface{})) < 10 {
		t.Fatalf("available_events = %v", body["available_events"])
	}
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting: %s", what)
}
