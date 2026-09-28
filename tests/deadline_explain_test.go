package tests

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
)

func explain(t *testing.T, id, token string) map[string]interface{} {
	t.Helper()
	w := request("GET", teamPath("/deadlines/"+id+"/explain"), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("explain %s: %d %s", id, w.Code, w.Body.String())
	}
	return parseJSON(t, w)["data"].(map[string]interface{})
}

func createTestDeadline(t *testing.T, body map[string]interface{}) string {
	t.Helper()
	w := request("POST", teamPath("/deadlines"), body, editorToken)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	id := getString(parseJSON(t, w)["id"])
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM md_deadline_events WHERE deadline_id=?`, id)
		database.DB.Exec(`DELETE FROM md_deadlines WHERE id=?`, id)
	})
	return id
}

// An order with recorded changes gets at least two evidence items, and the
// event evidence points at real md_deadline_events rows.
func TestExplainDeadlineWithHistory(t *testing.T) {
	day := func(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }
	id := createTestDeadline(t, map[string]interface{}{"order_no": "SO-EXP1", "title": "解释器", "due_date": day(3)})
	if w := request("PUT", teamPath("/deadlines/"+id), map[string]interface{}{
		"due_date": day(6), "reason": "铝材到货延迟",
	}, editorToken); w.Code != http.StatusOK {
		t.Fatalf("update: %d", w.Code)
	}
	if w := request("PUT", teamPath("/deadlines/"+id), map[string]interface{}{
		"priority": "urgent", "start_date": day(1), "reason": "客户催单",
	}, editorToken); w.Code != http.StatusOK {
		t.Fatalf("update priority: %d", w.Code)
	}

	// priority and start date changes are now part of the history
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_deadline_events WHERE deadline_id=? AND field IN ('priority','start_date')`, id).Scan(&n)
	if n != 2 {
		t.Fatalf("priority/start_date events = %d, want 2", n)
	}

	ex := explain(t, id, viewerToken) // read-only: viewers may ask too
	ev, _ := ex["evidence"].([]interface{})
	if len(ev) < 2 {
		t.Fatalf("want >= 2 evidence items, got %v", ex["evidence"])
	}
	foundEvent := false
	for _, raw := range ev {
		e := raw.(map[string]interface{})
		if e["type"] == "event" {
			foundEvent = true
			if !strings.Contains(getString(e["text"]), "铝材到货延迟") {
				t.Errorf("event evidence should quote the reason: %v", e["text"])
			}
			var c int
			database.DB.QueryRow(`SELECT COUNT(*) FROM md_deadline_events WHERE id=? AND deadline_id=?`, e["ref"], id).Scan(&c)
			if c != 1 {
				t.Errorf("evidence ref %v is not an event of this order", e["ref"])
			}
		}
	}
	if !foundEvent {
		t.Errorf("no event evidence in %v", ev)
	}
	if strings.HasPrefix(getString(ex["conclusion"]), "数据不足") {
		t.Errorf("has history, conclusion = %v", ex["conclusion"])
	}
	for _, k := range []string{"verdict", "confidence", "suggestion", "due_date"} {
		if getString(ex[k]) == "" {
			t.Errorf("%s is empty: %v", k, ex)
		}
	}
}

// An order nobody has touched since creation: say so instead of guessing.
func TestExplainDeadlineNoEvents(t *testing.T) {
	due := time.Now().AddDate(0, 0, 20).Format("2006-01-02")
	id := createTestDeadline(t, map[string]interface{}{"order_no": "SO-EXP2", "title": "没有记录", "due_date": due})
	ex := explain(t, id, editorToken)
	if !strings.HasPrefix(getString(ex["conclusion"]), "数据不足") {
		t.Fatalf("conclusion = %v", ex["conclusion"])
	}
	if ex["confidence"] != "low" {
		t.Errorf("confidence = %v", ex["confidence"])
	}
	missing, _ := ex["missing"].([]interface{})
	if len(missing) == 0 {
		t.Errorf("missing should explain the gap: %v", ex)
	}
}

func TestExplainDeadlineNotFound(t *testing.T) {
	if w := request("GET", teamPath("/deadlines/test-no-such-deadline/explain"), nil, editorToken); w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", w.Code)
	}
}
