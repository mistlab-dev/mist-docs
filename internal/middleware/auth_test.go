package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "unit-test-secret"

func withJWTConfig(t *testing.T, secret, issuer string) {
	t.Helper()
	old := config.C.JWT
	config.C.JWT.Secret = secret
	config.C.JWT.Issuer = issuer
	t.Cleanup(func() { config.C.JWT = old })
}

// captureAuthLog swaps the throttled logger for one that records lines.
func captureAuthLog(t *testing.T) *[]string {
	t.Helper()
	var mu sync.Mutex
	lines := &[]string{}
	old := authLog
	authLog = &throttledLog{last: map[string]time.Time{}, interval: time.Hour, printf: func(f string, a ...any) {
		mu.Lock()
		defer mu.Unlock()
		*lines = append(*lines, fmt.Sprintf(f, a...))
	}}
	t.Cleanup(func() { authLog = old })
	return lines
}

func sign(t *testing.T, claims jwt.Claims, secret string) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func portalClaims(uid, iss string, exp time.Time) MistLabClaims {
	return MistLabClaims{UserID: uid, RegisteredClaims: jwt.RegisteredClaims{Issuer: iss, ExpiresAt: jwt.NewNumericDate(exp)}}
}

func TestParsePortalToken(t *testing.T) {
	withJWTConfig(t, testSecret, "mist-docs")
	logs := captureAuthLog(t)
	uid, err := ParseMistLabToken(sign(t, portalClaims("u_1", "mist-docs", time.Now().Add(time.Hour)), testSecret))
	if err != nil || uid != "u_1" {
		t.Fatalf("uid=%q err=%v", uid, err)
	}
	if len(*logs) != 0 {
		t.Fatalf("a matching portal token must not log: %v", *logs)
	}
}

func TestParseRejectsBadTokens(t *testing.T) {
	withJWTConfig(t, testSecret, "mist-docs")
	cases := map[string]string{
		"wrong secret": sign(t, portalClaims("u_1", "mist-docs", time.Now().Add(time.Hour)), "other-secret"),
		"expired":      sign(t, portalClaims("u_1", "mist-docs", time.Now().Add(-time.Minute)), testSecret),
		"no user id":   sign(t, portalClaims("", "mist-docs", time.Now().Add(time.Hour)), testSecret),
		"garbage":      "not.a.jwt",
		"empty":        "",
	}
	// alg=none must never be accepted.
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, portalClaims("u_1", "mist-docs", time.Now().Add(time.Hour))).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	cases["alg none"] = none
	for name, tok := range cases {
		if uid, err := ParseMistLabToken(tok); err == nil {
			t.Errorf("%s: accepted, uid=%q", name, uid)
		}
	}
}

func TestLegacyTokenAcceptedButLogged(t *testing.T) {
	withJWTConfig(t, testSecret, "mist-docs")
	logs := captureAuthLog(t)
	tok := sign(t, LegacyClaims{UserID: "legacy-1", Username: "old", RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}, testSecret)
	for i := 0; i < 3; i++ {
		uid, err := ParseMistLabToken(tok)
		if err != nil || uid != "legacy-1" {
			t.Fatalf("uid=%q err=%v", uid, err)
		}
	}
	if len(*logs) != 1 || !strings.Contains((*logs)[0], "legacy") || !strings.Contains((*logs)[0], "legacy-1") {
		t.Fatalf("want exactly one throttled legacy log line, got %v", *logs)
	}
}

func TestIssuerMismatchLoggedNotEnforced(t *testing.T) {
	withJWTConfig(t, testSecret, "mist-docs")
	logs := captureAuthLog(t)
	uid, err := ParseMistLabToken(sign(t, portalClaims("u_2", "someone-else", time.Now().Add(time.Hour)), testSecret))
	if err != nil || uid != "u_2" {
		t.Fatalf("issuer mismatch must still be accepted (D9): uid=%q err=%v", uid, err)
	}
	if len(*logs) != 1 || !strings.Contains((*logs)[0], "someone-else") {
		t.Fatalf("want one issuer log line, got %v", *logs)
	}

	// No issuer configured: nothing to compare against, nothing logged.
	withJWTConfig(t, testSecret, "")
	logs = captureAuthLog(t)
	if _, err := ParseMistLabToken(sign(t, portalClaims("u_2", "x", time.Now().Add(time.Hour)), testSecret)); err != nil {
		t.Fatal(err)
	}
	if len(*logs) != 0 {
		t.Fatalf("unexpected log: %v", *logs)
	}
}

func TestThrottledLogPerKey(t *testing.T) {
	n := 0
	l := &throttledLog{last: map[string]time.Time{}, interval: time.Hour, printf: func(string, ...any) { n++ }}
	l.note("a", "x")
	l.note("a", "x")
	l.note("b", "x")
	if n != 2 {
		t.Fatalf("printed %d times, want 2", n)
	}
	l.last["a"] = time.Now().Add(-2 * time.Hour)
	l.note("a", "x")
	if n != 3 {
		t.Fatalf("expired key should print again, printed %d", n)
	}
}

func TestExtractToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		header, query, want string
	}{
		{"Bearer abc", "", "abc"},
		{"", "q1", "q1"},
		{"Bearer hdr", "q1", "hdr"},
		{"Basic abc", "", ""},
		{"Bearer ", "", ""},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		url := "/x"
		if tc.query != "" {
			url += "?token=" + tc.query
		}
		c.Request = httptest.NewRequest(http.MethodGet, url, nil)
		if tc.header != "" {
			c.Request.Header.Set("Authorization", tc.header)
		}
		if got := extractToken(c); got != tc.want {
			t.Errorf("header=%q query=%q: got %q want %q", tc.header, tc.query, got, tc.want)
		}
	}
}

func TestJWTAuthRejectsMissingOrBadTokenWithoutDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withJWTConfig(t, testSecret, "mist-docs")
	r := gin.New()
	r.GET("/p", JWTAuth(), func(c *gin.Context) { c.String(200, "ok") })
	for name, hdr := range map[string]string{"missing": "", "bad": "Bearer nope"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/p", nil)
		if hdr != "" {
			req.Header.Set("Authorization", hdr)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, w.Code)
		}
	}
}

func TestTeamAuthNeedsTeamParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/t", TeamAuth(), func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/t", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestAcceptedIssuersNotLogged(t *testing.T) {
	withJWTConfig(t, testSecret, "mistlab")
	config.C.JWT.AcceptedIssuers = []string{"mist-team-server"}
	t.Cleanup(func() { config.C.JWT.AcceptedIssuers = nil })
	logs := captureAuthLog(t)
	for _, iss := range []string{"mistlab", "mist-team-server"} {
		if uid, err := ParseMistLabToken(sign(t, portalClaims("u_3", iss, time.Now().Add(time.Hour)), testSecret)); err != nil || uid != "u_3" {
			t.Fatalf("iss %q: uid=%q err=%v", iss, uid, err)
		}
	}
	if len(*logs) != 0 {
		t.Fatalf("accepted issuers must not be logged, got %v", *logs)
	}
	// Anything else is still logged (and still accepted, D9).
	if _, err := ParseMistLabToken(sign(t, portalClaims("u_3", "other", time.Now().Add(time.Hour)), testSecret)); err != nil {
		t.Fatal(err)
	}
	if len(*logs) != 1 || !strings.Contains((*logs)[0], "other") {
		t.Fatalf("want one log line for unknown issuer, got %v", *logs)
	}
}
