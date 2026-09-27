package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
)

const previewBeta = "test-team-pv-beta"

func seedPreviewOrders(t *testing.T) func() {
	t.Helper()
	db := database.DB
	cleanup := func() {
		// The whole test team's orders: other tests create some via the API.
		db.Exec(`DELETE FROM md_deadline_events WHERE team_id IN (?, ?)`, teamID, previewBeta)
		db.Exec(`DELETE FROM md_deadlines WHERE team_id IN (?, ?)`, teamID, previewBeta)
		db.Exec(`DELETE FROM md_team_capacity WHERE team_id IN (?, ?)`, teamID, previewBeta)
	}
	cleanup()
	day := func(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }
	// §9.3 script 1: five open orders, one 50% done, one urgent.
	rows := []struct {
		id, no, due, prio, status string
		progress                  int
		remark, team              string
	}{
		{"test-pv-1", "SO-T1", day(1), "normal", "pending", 0, "", teamID},
		{"test-pv-2", "SO-T2", day(2), "normal", "pending", 0, "逾期违约金 3%/天", teamID},
		{"test-pv-3", "SO-T3", day(5), "urgent", "pending", 0, "", teamID},
		{"test-pv-4", "SO-T4", day(2), "normal", "running", 50, "", teamID},
		{"test-pv-5", "SO-T5", day(10), "normal", "pending", 0, "", teamID},
		{"test-pv-6", "SO-T6", day(0), "urgent", "done", 100, "", teamID},         // done: ignored
		{"test-pv-b", "SO-BETA", day(0), "urgent", "pending", 0, "", previewBeta}, // other team: ignored
	}
	for _, r := range rows {
		mustExec(t, `INSERT INTO md_deadlines (id, team_id, order_no, title, customer, due_date, status, progress, priority, owner_id, remark, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, r.id, r.team, r.no, r.no, "客户"+r.no, r.due, r.status, r.progress, r.prio, adminID, r.remark, adminID)
	}
	return cleanup
}

func deadlineSnapshot(t *testing.T) string {
	t.Helper()
	var s1, s2 string
	if err := database.DB.QueryRow(`SELECT CONCAT(COUNT(*), '|', COALESCE(MAX(updated_at),''), '|',
		COALESCE(GROUP_CONCAT(CONCAT(id, due_date, priority, status, progress) ORDER BY id), ''))
		FROM md_deadlines WHERE team_id IN (?, ?)`, teamID, previewBeta).Scan(&s1); err != nil {
		t.Fatal(err)
	}
	database.DB.QueryRow(`SELECT CONCAT(COUNT(*), '|', COALESCE(MAX(created_at),'')) FROM md_deadline_events WHERE team_id IN (?, ?)`, teamID, previewBeta).Scan(&s2)
	return s1 + "#" + s2
}

func previewItems(t *testing.T, data map[string]interface{}, key string) []string {
	t.Helper()
	out := []string{}
	list, _ := data[key].([]interface{})
	for _, v := range list {
		out = append(out, getString(v.(map[string]interface{})["order_no"]))
	}
	return out
}

func sameList(a []string, b ...string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The preview is read-only, open to viewers, and only sees the team's own
// open orders.
func TestPreviewInsertReadOnlyAndTeamScoped(t *testing.T) {
	defer seedPreviewOrders(t)()
	before := deadlineSnapshot(t)

	due := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	w := request("POST", teamPath("/deadlines/preview-insert"),
		map[string]interface{}{"order_no": "SO-NEW", "title": "插单", "due_date": due}, viewerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("viewer preview: %d %s", w.Code, w.Body.String())
	}
	data := parseJSON(t, w)["data"].(map[string]interface{})

	if got := getString(data["conclusion"]); got != "breach" {
		t.Errorf("conclusion = %q, want breach", got)
	}
	if got := previewItems(t, data, "new_breaches"); !sameList(got, "SO-T1") {
		t.Errorf("new_breaches = %v, want [SO-T1]", got)
	}
	if got := previewItems(t, data, "delayed"); !sameList(got, "SO-T2", "SO-T5") {
		t.Errorf("delayed = %v, want [SO-T2 SO-T5]", got)
	}
	if got := previewItems(t, data, "started"); !sameList(got, "SO-T4") {
		t.Errorf("started = %v, want [SO-T4]", got)
	}
	if got := previewItems(t, data, "unaffected"); !sameList(got, "SO-T3") {
		t.Errorf("unaffected = %v (SO-T6 is done, SO-BETA is another team's)", got)
	}
	ins := data["insert"].(map[string]interface{})
	if getString(ins["new_finish"]) != due {
		t.Errorf("insert finish = %v, want %s", ins["new_finish"], due)
	}
	t2 := data["delayed"].([]interface{})[0].(map[string]interface{})
	flags, _ := t2["flags"].([]interface{})
	if len(flags) != 2 || flags[0] != "penalty" || flags[1] != "already_late" {
		t.Errorf("SO-T2 flags = %v, want [penalty already_late]", flags)
	}

	if after := deadlineSnapshot(t); after != before {
		t.Fatalf("preview changed deadline data:\nbefore %s\nafter  %s", before, after)
	}

	// Another team's order cannot be used as the insert.
	w = request("POST", teamPath("/deadlines/preview-insert"),
		map[string]interface{}{"deadline_id": "test-pv-b", "due_date": due}, viewerToken)
	if w.Code != http.StatusNotFound {
		t.Errorf("other team's order as insert: %d, want 404", w.Code)
	}
	w = request("POST", teamPath("/deadlines/preview-insert"), map[string]interface{}{"order_no": "X", "due_date": "tomorrow"}, viewerToken)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad date: %d, want 400", w.Code)
	}
}

// Capacity: anyone reads, only admins change it, and the preview uses it.
func TestTeamCapacity(t *testing.T) {
	defer seedPreviewOrders(t)()

	w := request("GET", teamPath("/deadlines/capacity"), nil, viewerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("get capacity: %d", w.Code)
	}
	data := parseJSON(t, w)["data"].(map[string]interface{})
	if getFloat(data["per_day"]) != 1 || data["is_default"] != true {
		t.Fatalf("default capacity = %v", data)
	}

	body := map[string]interface{}{"per_day": 2, "key_customers": []string{"客户SO-T5", " 客户SO-T5 ", ""}}
	for _, tok := range []string{viewerToken, editorToken} {
		if w := request("PUT", teamPath("/deadlines/capacity"), body, tok); w.Code != http.StatusForbidden {
			t.Fatalf("non-admin PUT capacity: %d, want 403", w.Code)
		}
	}
	if w := request("PUT", teamPath("/deadlines/capacity"), map[string]interface{}{"per_day": 0}, adminToken); w.Code != http.StatusBadRequest {
		t.Fatalf("per_day 0: %d, want 400", w.Code)
	}
	w = request("PUT", teamPath("/deadlines/capacity"), body, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("admin PUT capacity: %d %s", w.Code, w.Body.String())
	}
	data = parseJSON(t, w)["data"].(map[string]interface{})
	keys, _ := data["key_customers"].([]interface{})
	if getFloat(data["per_day"]) != 2 || len(keys) != 1 {
		t.Fatalf("after PUT = %v", data)
	}

	// Two a day: the preview must use the new capacity and flag key customers.
	due := time.Now().Format("2006-01-02")
	w = request("POST", teamPath("/deadlines/preview-insert"),
		map[string]interface{}{"order_no": "SO-NEW", "due_date": due, "priority": "urgent"}, viewerToken)
	res := parseJSON(t, w)["data"].(map[string]interface{})
	if getFloat(res["per_day"]) != 2 {
		t.Errorf("preview per_day = %v, want 2", res["per_day"])
	}
	for _, it := range append(res["delayed"].([]interface{}), res["new_breaches"].([]interface{})...) {
		m := it.(map[string]interface{})
		if getString(m["order_no"]) == "SO-T5" {
			f, _ := m["flags"].([]interface{})
			if len(f) == 0 || f[0] != "key_customer" {
				t.Errorf("SO-T5 flags = %v, want key_customer", f)
			}
		}
	}
}
