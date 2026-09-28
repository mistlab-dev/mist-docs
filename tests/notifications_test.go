package tests

import (
	"encoding/json"
	"testing"
)

// The bell badge reads unread_count from the list response.
func TestNotificationListReportsUnreadCount(t *testing.T) {
	execSQL := func(q string, args ...any) { mustExec(t, q, args...) }
	execSQL(`DELETE FROM md_notifications WHERE user_id=? AND team_id=?`, viewerID, teamID)
	for i, read := range []int{0, 0, 1} {
		execSQL(`INSERT INTO md_notifications (id, user_id, team_id, type, title, is_read) VALUES (?, ?, ?, 'comment', '测试通知', ?)`,
			[]string{"test-unread-1", "test-unread-2", "test-unread-3"}[i], viewerID, teamID, read)
	}
	t.Cleanup(func() { mustExec(t, `DELETE FROM md_notifications WHERE id LIKE 'test-unread-%'`) })

	w := request("GET", teamPath("/notifications"), nil, viewerToken)
	if w.Code != 200 {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Data        []map[string]any `json:"data"`
		Total       int              `json:"total"`
		UnreadCount *int             `json:"unread_count"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Total != 3 || len(body.Data) != 3 {
		t.Fatalf("total=%d len=%d", body.Total, len(body.Data))
	}
	if body.UnreadCount == nil || *body.UnreadCount != 2 {
		t.Fatalf("unread_count = %v, want 2", body.UnreadCount)
	}
}
