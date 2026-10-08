package tests

import (
	"context"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
)

// A member removed from the team loses access on the next request: the
// token stays valid, but the membership check runs per request.
func TestRemovedMemberLosesAccessImmediately(t *testing.T) {
	ctx := context.Background()
	uid := "test-user-removed"
	database.DB.ExecContext(ctx,
		`INSERT IGNORE INTO users (id, email, username, display_name, password_hash, is_admin, email_verified)
		 VALUES (?, 'removed@mistdocs.invalid', 'test-removed', '已移除成员', 'x', 0, 1)`, uid)
	t.Cleanup(func() {
		database.DB.ExecContext(ctx, `DELETE FROM team_members WHERE user_id=?`, uid)
		database.DB.ExecContext(ctx, `DELETE FROM users WHERE id=?`, uid)
	})
	if _, err := database.DB.ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'editor')`, teamID, uid); err != nil {
		t.Fatal(err)
	}
	tok, _ := middleware.GenerateToken(uid, "test-removed", "member", "")

	if w := request("GET", teamPath("/documents"), nil, tok); w.Code != 200 {
		t.Fatalf("member before removal: %d %s", w.Code, w.Body.String())
	}
	database.DB.ExecContext(ctx, `DELETE FROM team_members WHERE team_id=? AND user_id=?`, teamID, uid)
	if w := request("GET", teamPath("/documents"), nil, tok); w.Code != 403 {
		t.Fatalf("removed member should get 403, got %d", w.Code)
	}
}

// Legacy MistDocs tokens (user_id claim) still work during the transition (D8).
func TestLegacyTokenStillAccepted(t *testing.T) {
	claims := middleware.LegacyClaims{UserID: editorID, Username: "test-editor", RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.C.JWT.Secret))
	if err != nil {
		t.Fatal(err)
	}
	if w := request("GET", teamPath("/documents"), nil, tok); w.Code != 200 {
		t.Fatalf("legacy token: %d %s", w.Code, w.Body.String())
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	claims := middleware.MistLabClaims{UserID: editorID, RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.C.JWT.Secret))
	if w := request("GET", teamPath("/documents"), nil, tok); w.Code != 401 {
		t.Fatalf("expired token should get 401, got %d", w.Code)
	}
}

// A valid token for a user that no longer exists in the shared users table.
func TestTokenForDeletedUserRejected(t *testing.T) {
	tok, _ := middleware.GenerateToken("test-user-does-not-exist", "ghost", "member", "")
	if w := request("GET", teamPath("/documents"), nil, tok); w.Code != 401 {
		t.Fatalf("unknown user should get 401, got %d", w.Code)
	}
}

// One sign-in for mistlab.dev and docs.mistlab.dev (plan 1.22): a token from a
// browser sign-in carries its session id; once that sign-in is signed out on
// either site (auth_sessions.revoked_at set by mist-team-server), MistDocs
// rejects it too. Tokens without a session id are unaffected.
func TestSignedOutSessionRejected(t *testing.T) {
	ctx := context.Background()
	if _, err := database.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS auth_sessions (
		id VARCHAR(64) PRIMARY KEY, user_id VARCHAR(64) NOT NULL, secret_hash VARCHAR(64) NOT NULL,
		user_agent VARCHAR(255) NOT NULL DEFAULT '', created_at DATETIME NOT NULL, last_seen_at DATETIME NOT NULL, revoked_at DATETIME NULL)`); err != nil {
		t.Fatal(err)
	}
	sid := "ses_test_signout"
	database.DB.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id=?`, sid)
	database.DB.ExecContext(ctx, `INSERT INTO auth_sessions (id, user_id, secret_hash, created_at, last_seen_at) VALUES (?, ?, 'x', NOW(), NOW())`, sid, editorID)
	t.Cleanup(func() { database.DB.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id=?`, sid) })

	sign := func(sid string) string {
		claims := middleware.MistLabClaims{UserID: editorID, SessionID: sid, RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.C.JWT.Secret))
		return tok
	}
	tok := sign(sid)
	if w := request("GET", teamPath("/documents"), nil, tok); w.Code != 200 {
		t.Fatalf("live session: %d %s", w.Code, w.Body.String())
	}
	database.DB.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at=NOW() WHERE id=?`, sid)
	w := request("GET", teamPath("/documents"), nil, tok)
	if w.Code != 401 || getString(parseJSON(t, w)["code"]) != "signed_out" {
		t.Fatalf("signed-out session should get 401 signed_out, got %d %s", w.Code, w.Body.String())
	}
	if w := request("GET", teamPath("/documents"), nil, sign("ses_unknown")); w.Code != 401 {
		t.Fatalf("unknown session should get 401, got %d", w.Code)
	}
	if w := request("GET", teamPath("/documents"), nil, sign("")); w.Code != 200 {
		t.Fatalf("token without session id: %d", w.Code)
	}
}
