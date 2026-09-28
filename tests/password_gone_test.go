package tests

import "testing"

// D1: passwords live in the Portal.
func TestChangePasswordGone(t *testing.T) {
	w := request("PUT", "/api/auth/password", map[string]string{"old_password": "a", "new_password": "bbbbbb"}, editorToken)
	if w.Code != 410 {
		t.Fatalf("PUT /auth/password = %d, want 410", w.Code)
	}
	if getString(parseJSON(t, w)["portal_url"]) == "" {
		t.Fatalf("410 response should carry portal_url")
	}
}
