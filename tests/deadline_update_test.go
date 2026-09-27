package tests

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
)

// A partial update must keep the fields it does not mention. Before the fix
// PUT {"due_date": ...} reset progress to 0 and cleared the start date.
func TestDeadlinePartialUpdateKeepsFields(t *testing.T) {
	day := func(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }
	w := request("POST", teamPath("/deadlines"), map[string]interface{}{
		"order_no": "SO-PART", "title": "部分更新", "due_date": day(5), "start_date": day(1), "progress": 40,
	}, editorToken)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	id := getString(parseJSON(t, w)["id"])
	defer database.DB.Exec(`DELETE FROM md_deadlines WHERE id=?`, id)
	defer database.DB.Exec(`DELETE FROM md_deadline_events WHERE deadline_id=?`, id)

	state := func() (int, string) {
		var p int
		var s sql.NullTime
		database.DB.QueryRow(`SELECT progress, start_date FROM md_deadlines WHERE id=?`, id).Scan(&p, &s)
		if !s.Valid {
			return p, ""
		}
		return p, s.Time.Format("2006-01-02")
	}

	if w := request("PUT", teamPath("/deadlines/"+id), map[string]interface{}{"due_date": day(7)}, editorToken); w.Code != http.StatusOK {
		t.Fatalf("update due: %d", w.Code)
	}
	if p, s := state(); p != 40 || s != day(1) {
		t.Fatalf("after due-date-only update: progress=%d start=%q, want 40 %q", p, s, day(1))
	}
	if w := request("PUT", teamPath("/deadlines/"+id), map[string]interface{}{"progress": 0}, editorToken); w.Code != http.StatusOK {
		t.Fatalf("update progress: %d", w.Code)
	}
	if p, _ := state(); p != 0 {
		t.Fatalf("explicit progress 0 not saved: %d", p)
	}
	if w := request("PUT", teamPath("/deadlines/"+id), map[string]interface{}{"start_date": ""}, editorToken); w.Code != http.StatusOK {
		t.Fatalf("clear start: %d", w.Code)
	}
	if _, s := state(); s != "" {
		t.Fatalf("empty start_date should clear it, got %q", s)
	}
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_deadline_events WHERE deadline_id=? AND field='progress'`, id).Scan(&n)
	if n != 1 {
		t.Fatalf("progress events = %d, want 1 (40 → 0)", n)
	}
}

// The board and list show the owner's name. The old lookup selected a
// "name" column the shared users table does not have, so every order
// looked unassigned.
func TestDeadlineOwnerNameShown(t *testing.T) {
	w := request("POST", teamPath("/deadlines"), map[string]interface{}{
		"order_no": "SO-OWNER", "title": "负责人显示", "due_date": time.Now().AddDate(0, 0, 2).Format("2006-01-02"),
	}, editorToken)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d", w.Code)
	}
	id := getString(parseJSON(t, w)["id"])
	defer database.DB.Exec(`DELETE FROM md_deadlines WHERE id=?`, id)
	defer database.DB.Exec(`DELETE FROM md_deadline_events WHERE deadline_id=?`, id)

	w = request("GET", teamPath("/deadlines?q=SO-OWNER"), nil, viewerToken)
	list, _ := parseJSON(t, w)["data"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("list = %v", list)
	}
	if got := getString(list[0].(map[string]interface{})["owner_name"]); got != "编辑者" {
		t.Fatalf("owner_name = %q, want 编辑者 (the creator is the default owner)", got)
	}
}
