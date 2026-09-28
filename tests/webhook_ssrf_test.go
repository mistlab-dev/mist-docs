package tests

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/scheduler"
	"github.com/c-wind/mist-docs/internal/webhook"
)

// With the production setting, webhooks cannot be saved with internal
// targets, and a row that already points at one is not delivered to.
func TestWebhookInternalTargetsBlocked(t *testing.T) {
	webhook.AllowPrivateTargets = false
	defer func() { webhook.AllowPrivateTargets = true }()

	for _, u := range []string{
		"http://127.0.0.1:8900/api/files/x", "http://localhost/", "http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/", "http://[::1]/", "http://192.168.1.10:8080/", "ftp://example.com/",
	} {
		w := request("POST", teamPath("/webhooks"), map[string]interface{}{"name": "ssrf", "url": u, "events": []string{"document.created"}}, adminToken)
		if w.Code != http.StatusBadRequest {
			t.Errorf("create %s: %d %s, want 400", u, w.Code, w.Body.String())
		}
	}

	// A legacy row pointing at loopback: the reminder must not reach it.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	defer srv.Close()
	db := database.DB
	cleanup := func() {
		db.Exec(`DELETE FROM md_webhooks WHERE id='test-ssrf-hook'`)
		db.Exec(`DELETE FROM md_reminder_log WHERE rule_id='test-ssrf-rule'`)
		db.Exec(`DELETE FROM md_reminder_rules WHERE id='test-ssrf-rule'`)
		db.Exec(`DELETE FROM md_deadlines WHERE id='test-ssrf-dl'`)
	}
	cleanup()
	defer cleanup()
	mustExec(t, `INSERT INTO md_webhooks (id, team_id, name, url, events, enabled, created_by) VALUES ('test-ssrf-hook', ?, 'old', ?, '["deadline.reminder"]', 1, ?)`, teamID, srv.URL, adminID)
	mustExec(t, `INSERT INTO md_reminder_rules (id, team_id, name, offset_days, channel, target, enabled, created_by)
		VALUES ('test-ssrf-rule', ?, '当天', 0, 'webhook', 'owner', 1, ?)`, teamID, adminID)
	mustExec(t, `INSERT INTO md_deadlines (id, team_id, order_no, title, due_date, status, owner_id, created_by)
		VALUES ('test-ssrf-dl', ?, 'SO-SSRF', 'ssrf', ?, 'pending', ?, ?)`, teamID, time.Now().Format("2006-01-02"), adminID, adminID)
	scheduler.RunReminderScanOnce(time.Now())
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("internal webhook received %d deliveries", n)
	}
	var result string
	db.QueryRow(`SELECT result FROM md_reminder_log WHERE rule_id='test-ssrf-rule'`).Scan(&result)
	if result == "ok" || result == "" {
		t.Fatalf("reminder log result = %q, want a failure", result)
	}
}
