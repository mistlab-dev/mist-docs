package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/scheduler"
)

func pvDay(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }

func cleanProposals() {
	database.DB.Exec(`DELETE FROM md_proposals WHERE team_id=?`, teamID)
	database.DB.Exec(`DELETE FROM md_notifications WHERE team_id=? AND type='deadline'`, teamID)
}

// preview runs an insert preview and returns the response data.
func preview(t *testing.T, token string, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	w := request("POST", teamPath("/deadlines/preview-insert"), body, token)
	if w.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	return parseJSON(t, w)["data"].(map[string]interface{})
}

func dueOf(id string) string {
	var d time.Time
	database.DB.QueryRow(`SELECT due_date FROM md_deadlines WHERE id=?`, id).Scan(&d)
	return d.Format("2006-01-02")
}

func proposalStatus(id string) string {
	var s string
	database.DB.QueryRow(`SELECT status FROM md_proposals WHERE id=?`, id).Scan(&s)
	return s
}

// §9.3 scripts 2 and 3: confirm writes the ledger, the change history, a
// notification and a webhook; confirming again is refused.
func TestProposalApply(t *testing.T) {
	defer seedPreviewOrders(t)()
	cleanProposals()
	defer cleanProposals()

	var hits int32
	var lastBody atomic.Value
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(strings.Builder)
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		buf.Write(b[:n])
		lastBody.Store(buf.String())
		atomic.AddInt32(&hits, 1)
	}))
	defer hook.Close()
	mustExec(t, `INSERT INTO md_webhooks (id, team_id, name, url, secret, events, created_by, enabled)
		VALUES ('test-pp-hook', ?, 'pp', ?, '', '["deadline.proposal_applied"]', ?, 1)`, teamID, hook.URL, adminID)
	defer database.DB.Exec(`DELETE FROM md_webhook_logs WHERE webhook_id='test-pp-hook'`)
	defer database.DB.Exec(`DELETE FROM md_webhooks WHERE id='test-pp-hook'`)

	data := preview(t, editorToken, map[string]interface{}{"order_no": "SO-NEW", "title": "插单", "due_date": pvDay(1)})
	pid := getString(data["proposal_id"])
	changes, _ := data["changes"].([]interface{})
	if pid == "" || len(changes) != 2 {
		t.Fatalf("proposal_id=%q changes=%v, want an id and 2 changes (create + SO-T1 due date)", pid, changes)
	}
	if st := proposalStatus(pid); st != "pending" {
		t.Fatalf("status after preview = %q", st)
	}

	// Viewers can preview but not confirm (design R3: checked in the handler).
	if w := request("POST", teamPath("/proposals/"+pid+"/apply"), nil, viewerToken); w.Code != http.StatusForbidden {
		t.Fatalf("viewer apply: %d, want 403", w.Code)
	}

	w := request("POST", teamPath("/proposals/"+pid+"/apply"), map[string]interface{}{"reason": "客户同意"}, editorToken)
	if w.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	created := getString(parseJSON(t, w)["created_id"])

	var no, prio, owner string
	database.DB.QueryRow(`SELECT order_no, priority, owner_id FROM md_deadlines WHERE id=? AND team_id=?`, created, teamID).Scan(&no, &prio, &owner)
	if no != "SO-NEW" || prio != "inserted" || owner != editorID || dueOf(created) != pvDay(1) {
		t.Fatalf("created order = %s %s %s %s", no, prio, owner, dueOf(created))
	}
	if got := dueOf("test-pv-1"); got != pvDay(3) {
		t.Fatalf("SO-T1 due = %s, want %s (its new planned finish)", got, pvDay(3))
	}
	if got := dueOf("test-pv-2"); got != pvDay(2) {
		t.Fatalf("SO-T2 was late anyway; its due date must not move (got %s)", got)
	}

	var evOld, evNew, evReason, evActor string
	database.DB.QueryRow(`SELECT old_value, new_value, reason, actor_id FROM md_deadline_events
		WHERE deadline_id='test-pv-1' AND event_type='proposal_applied' AND field='due_date'`).Scan(&evOld, &evNew, &evReason, &evActor)
	if evOld != pvDay(1) || evNew != pvDay(3) || evActor != editorID || !strings.Contains(evReason, "插单 SO-NEW 影响面确认") || !strings.Contains(evReason, "客户同意") {
		t.Fatalf("event = %s→%s by %s, reason %q", evOld, evNew, evActor, evReason)
	}
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_deadline_events WHERE deadline_id=? AND event_type IN ('created','proposal_applied')`, created).Scan(&n)
	if n != 2 {
		t.Fatalf("events for the created order = %d, want 2", n)
	}
	var decidedBy string
	database.DB.QueryRow(`SELECT status, decided_by FROM md_proposals WHERE id=?`, pid).Scan(new(string), &decidedBy)
	if proposalStatus(pid) != "applied" || decidedBy != editorID {
		t.Fatalf("proposal = %s by %s", proposalStatus(pid), decidedBy)
	}

	// SO-T1's owner (the admin) is told; the person confirming is not.
	var title string
	database.DB.QueryRow(`SELECT title FROM md_notifications WHERE user_id=? AND team_id=? AND related_id='test-pv-1'`, adminID, teamID).Scan(&title)
	if !strings.Contains(title, "SO-T1") || !strings.Contains(title, pvDay(3)) {
		t.Fatalf("owner notification = %q", title)
	}
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_notifications WHERE user_id=? AND team_id=? AND type='deadline'`, editorID, teamID).Scan(&n)
	if n != 0 {
		t.Fatalf("the confirming user got %d notifications", n)
	}

	for i := 0; i < 40 && atomic.LoadInt32(&hits) == 0; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("webhook hits = %d, want 1", hits)
	}
	if b, _ := lastBody.Load().(string); !strings.Contains(b, "deadline.proposal_applied") || !strings.Contains(b, pid) {
		t.Fatalf("webhook body = %s", b)
	}

	if w := request("POST", teamPath("/proposals/"+pid+"/apply"), nil, editorToken); w.Code != http.StatusConflict {
		t.Fatalf("second apply: %d, want 409", w.Code)
	}
}

// §9.3 script 3 / design R2: a preview computed from data that has changed
// since is refused and marked stale; nothing is written.
func TestProposalStaleAfterChange(t *testing.T) {
	defer seedPreviewOrders(t)()
	cleanProposals()
	defer cleanProposals()

	pid := getString(preview(t, editorToken, map[string]interface{}{"order_no": "SO-NEW", "due_date": pvDay(1)})["proposal_id"])
	// Someone moves an order meanwhile.
	if w := request("PUT", teamPath("/deadlines/test-pv-5"), map[string]interface{}{"due_date": pvDay(12)}, editorToken); w.Code != http.StatusOK {
		t.Fatalf("update: %d", w.Code)
	}
	w := request("POST", teamPath("/proposals/"+pid+"/apply"), nil, editorToken)
	if w.Code != http.StatusConflict || getString(parseJSON(t, w)["status"]) != "stale" {
		t.Fatalf("apply after change: %d %s, want 409 stale", w.Code, w.Body.String())
	}
	if st := proposalStatus(pid); st != "stale" {
		t.Fatalf("status = %q, want stale", st)
	}
	if got := dueOf("test-pv-1"); got != pvDay(1) {
		t.Fatalf("SO-T1 due changed to %s by a stale proposal", got)
	}
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_deadlines WHERE team_id=? AND order_no='SO-NEW'`, teamID).Scan(&n)
	if n != 0 {
		t.Fatalf("stale proposal created the order")
	}

	// Capacity changes invalidate too.
	pid = getString(preview(t, editorToken, map[string]interface{}{"order_no": "SO-NEW", "due_date": pvDay(1)})["proposal_id"])
	request("PUT", teamPath("/deadlines/capacity"), map[string]interface{}{"per_day": 3}, adminToken)
	if w := request("POST", teamPath("/proposals/"+pid+"/apply"), nil, editorToken); w.Code != http.StatusConflict {
		t.Fatalf("apply after capacity change: %d, want 409", w.Code)
	}
}

// State machine: pending → rejected; who may reject; rejected cannot be applied.
func TestProposalReject(t *testing.T) {
	defer seedPreviewOrders(t)()
	cleanProposals()
	defer cleanProposals()

	own := getString(preview(t, viewerToken, map[string]interface{}{"order_no": "SO-V", "due_date": pvDay(2)})["proposal_id"])
	if w := request("POST", teamPath("/proposals/"+own+"/reject"), map[string]interface{}{"reason": "不插了"}, viewerToken); w.Code != http.StatusOK {
		t.Fatalf("proposer reject: %d", w.Code)
	}
	if st := proposalStatus(own); st != "rejected" {
		t.Fatalf("status = %q", st)
	}
	if w := request("POST", teamPath("/proposals/"+own+"/apply"), nil, editorToken); w.Code != http.StatusConflict {
		t.Fatalf("apply rejected: %d, want 409", w.Code)
	}

	other := getString(preview(t, editorToken, map[string]interface{}{"order_no": "SO-E", "due_date": pvDay(2)})["proposal_id"])
	if w := request("POST", teamPath("/proposals/"+other+"/reject"), nil, viewerToken); w.Code != http.StatusForbidden {
		t.Fatalf("viewer rejecting someone else's proposal: %d, want 403", w.Code)
	}
	if w := request("POST", teamPath("/proposals/"+other+"/reject"), nil, adminToken); w.Code != http.StatusOK {
		t.Fatalf("admin reject: %d", w.Code)
	}
	if w := request("POST", teamPath("/proposals/"+other+"/reject"), nil, adminToken); w.Code != http.StatusConflict {
		t.Fatalf("reject twice: %d, want 409", w.Code)
	}
	if w := request("POST", "/api/teams/test-team-nope/proposals/"+other+"/reject", nil, adminToken); w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Fatalf("other team path: %d", w.Code)
	}
}

// History: every member sees it, including rejected and stale; pending
// proposals from earlier days turn stale.
func TestProposalHistory(t *testing.T) {
	defer seedPreviewOrders(t)()
	cleanProposals()
	defer cleanProposals()

	p1 := getString(preview(t, editorToken, map[string]interface{}{"order_no": "SO-H1", "due_date": pvDay(2)})["proposal_id"])
	request("POST", teamPath("/proposals/"+p1+"/reject"), nil, editorToken)
	p2 := getString(preview(t, editorToken, map[string]interface{}{"order_no": "SO-H2", "due_date": pvDay(2)})["proposal_id"])
	database.DB.Exec(`UPDATE md_proposals SET created_at = created_at - INTERVAL 2 DAY WHERE id=?`, p2)

	w := request("GET", teamPath("/proposals"), nil, viewerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	got := map[string]map[string]interface{}{}
	for _, v := range parseJSON(t, w)["data"].([]interface{}) {
		m := v.(map[string]interface{})
		got[getString(m["id"])] = m
	}
	if got[p1] == nil || getString(got[p1]["status"]) != "rejected" || getString(got[p1]["user_name"]) != "编辑者" || getString(got[p1]["title"]) != "插单 SO-H1" {
		t.Fatalf("p1 = %v", got[p1])
	}
	if got[p2] == nil || getString(got[p2]["status"]) != "stale" {
		t.Fatalf("yesterday's pending proposal = %v, want stale", got[p2])
	}
	if w := request("POST", teamPath("/proposals/"+p2+"/apply"), nil, editorToken); w.Code != http.StatusConflict {
		t.Fatalf("apply stale: %d, want 409", w.Code)
	}

	// A pending proposal whose data moved shows as stale in the history too.
	p3 := getString(preview(t, editorToken, map[string]interface{}{"order_no": "SO-H3", "due_date": pvDay(2)})["proposal_id"])
	request("GET", teamPath("/proposals"), nil, viewerToken)
	if st := proposalStatus(p3); st != "pending" {
		t.Fatalf("untouched proposal = %q, want pending", st)
	}
	request("PUT", teamPath("/deadlines/test-pv-3"), map[string]interface{}{"priority": "normal"}, editorToken)
	request("GET", teamPath("/proposals"), nil, viewerToken)
	if st := proposalStatus(p3); st != "stale" {
		t.Fatalf("proposal after data change = %q, want stale", st)
	}
}

// Design §9.1: confirming does not make an already-sent reminder fire again
// (uk_once is per rule and order, so a moved due date is not re-reminded,
// the same as editing the date by hand).
func TestProposalApplyKeepsReminderOnce(t *testing.T) {
	defer seedPreviewOrders(t)()
	cleanProposals()
	defer cleanProposals()
	defer database.DB.Exec(`DELETE FROM md_reminder_log WHERE rule_id='test-pp-rule'`)
	defer database.DB.Exec(`DELETE FROM md_reminder_rules WHERE id='test-pp-rule'`)
	mustExec(t, `INSERT INTO md_reminder_rules (id, team_id, name, offset_days, channel, target, enabled, created_by)
		VALUES ('test-pp-rule', ?, '提前2天', 2, 'inapp', 'owner', 1, ?)`, teamID, adminID)
	mustExec(t, `INSERT INTO md_reminder_log (id, rule_id, deadline_id, team_id, target_user_id, channel, result)
		VALUES ('test-pp-log', 'test-pp-rule', 'test-pv-1', ?, ?, 'inapp', 'ok')`, teamID, adminID)

	pid := getString(preview(t, editorToken, map[string]interface{}{"order_no": "SO-NEW", "due_date": pvDay(1)})["proposal_id"])
	if w := request("POST", teamPath("/proposals/"+pid+"/apply"), nil, editorToken); w.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	// SO-T1 is now due in 3 days; tomorrow is "2 days before" the new date.
	scheduler.RunReminderScanOnce(time.Now().AddDate(0, 0, 1))
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_reminder_log WHERE rule_id='test-pp-rule' AND deadline_id='test-pv-1'`).Scan(&n)
	if n != 1 {
		t.Fatalf("reminder log rows for SO-T1 = %d, want 1", n)
	}
}
