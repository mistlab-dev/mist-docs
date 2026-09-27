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

// A deadline reminder sent over the webhook channel must only reach the
// owning team's endpoints. Before the fix the scheduler selected every
// enabled row in md_webhooks, so another team's URL received our order
// numbers.
func TestReminderWebhookIsTeamScoped(t *testing.T) {
	var ownHits, otherHits int32
	own := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ownHits, 1)
	}))
	defer own.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&otherHits, 1)
	}))
	defer other.Close()

	otherTeam := "test-team-beta"
	db := database.DB
	cleanup := func() {
		db.Exec(`DELETE FROM md_reminder_log WHERE team_id IN (?, ?)`, teamID, otherTeam)
		db.Exec(`DELETE FROM md_reminder_rules WHERE id LIKE 'test-rw-%'`)
		db.Exec(`DELETE FROM md_deadlines WHERE id LIKE 'test-rw-%'`)
		db.Exec(`DELETE l FROM md_webhook_logs l JOIN md_webhooks w ON l.webhook_id = w.id WHERE w.id LIKE 'test-rw-%'`)
		db.Exec(`DELETE FROM md_webhooks WHERE id LIKE 'test-rw-%'`)
	}
	cleanup()
	defer cleanup()

	mustExec(t, `INSERT INTO md_webhooks (id, team_id, name, url, secret, events, created_by, enabled)
		VALUES ('test-rw-own', ?, 'own', ?, '', '["deadline.reminder"]', ?, 1)`, teamID, own.URL, adminID)
	mustExec(t, `INSERT INTO md_webhooks (id, team_id, name, url, secret, events, created_by, enabled)
		VALUES ('test-rw-other', ?, 'other', ?, '', '["deadline.reminder"]', ?, 1)`, otherTeam, other.URL, adminID)

	today := time.Now().Format("2006-01-02")
	mustExec(t, `INSERT INTO md_deadlines (id, team_id, order_no, title, due_date, status, priority, owner_id, created_by)
		VALUES ('test-rw-dl', ?, 'SO-LEAK-1', '跨团队泄露测试', ?, 'pending', 'normal', ?, ?)`, teamID, today, adminID, adminID)
	mustExec(t, `INSERT INTO md_reminder_rules (id, team_id, name, offset_days, channel, target, enabled, created_by)
		VALUES ('test-rw-rule', ?, '今天到期', 0, 'webhook', 'owner', 1, ?)`, teamID, adminID)

	scheduler.RunReminderScanOnce(time.Now())

	if got := atomic.LoadInt32(&ownHits); got != 1 {
		t.Fatalf("own team webhook hits = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&otherHits); got != 0 {
		t.Fatalf("another team's webhook received %d reminder(s); reminders must stay inside the team", got)
	}

	var result string
	db.QueryRow(`SELECT result FROM md_reminder_log WHERE rule_id = 'test-rw-rule'`).Scan(&result)
	if result != "ok" {
		t.Fatalf("reminder log result = %q, want ok", result)
	}
}

func TestWebhookLoadTargetsRequiresTeam(t *testing.T) {
	if got := webhook.LoadTargets(""); got != nil {
		t.Fatalf("LoadTargets(\"\") must not fall back to all teams, got %d target(s)", len(got))
	}
}

func mustExec(t *testing.T, q string, args ...interface{}) {
	t.Helper()
	if _, err := database.DB.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}
