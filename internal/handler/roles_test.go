package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func ctxWithRole(role, uid string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("current_team_role", role)
	c.Set("user_id", uid)
	return c, w
}

func TestRoleAtLeast(t *testing.T) {
	cases := []struct {
		role, need string
		want       bool
	}{
		{"owner", RoleAdmin, true}, // D4: owner has admin rights
		{"admin", RoleAdmin, true},
		{"editor", RoleAdmin, false},
		{"editor", RoleEditor, true},
		{"member", RoleEditor, true}, // legacy name for editor
		{"viewer", RoleEditor, false},
		{"viewer", RoleViewer, true},
		{"", RoleViewer, false},
		{"guest", RoleViewer, false}, // unknown roles get nothing
	}
	for _, tc := range cases {
		c, _ := ctxWithRole(tc.role, "u")
		if got := roleAtLeast(c, tc.need); got != tc.want {
			t.Errorf("roleAtLeast(%q, %q) = %v, want %v", tc.role, tc.need, got, tc.want)
		}
	}
}

func TestRequireOwnerOrAdmin(t *testing.T) {
	c, _ := ctxWithRole("editor", "u1")
	if !requireOwnerOrAdmin(c, "", "u1") {
		t.Error("record owner should pass")
	}
	c, w := ctxWithRole("editor", "u2")
	if requireOwnerOrAdmin(c, "u1") || w.Code != 403 {
		t.Errorf("other editor should get 403, got %d", w.Code)
	}
	c, _ = ctxWithRole("owner", "u3")
	if !requireOwnerOrAdmin(c, "u1") {
		t.Error("team owner counts as admin")
	}
	c, _ = ctxWithRole("editor", "")
	if requireOwnerOrAdmin(c, "") {
		t.Error("empty user id must not match an empty owner")
	}
}
