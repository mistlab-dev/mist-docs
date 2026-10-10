package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRedactQuery(t *testing.T) {
	cases := map[string]string{
		"/ws/teams/t/docs/d?token=abc123":                                 "/ws/teams/t/docs/d?token=***",
		"/api/media/t/a.png?sig=xyz":                                      "/api/media/t/a.png?sig=***",
		"/api/s/abc?password=pw":                                          "/api/s/abc?password=***",
		"/dashboard?access_token=a.b.c&refresh_token=d&oauth_redirect=/x": "/dashboard?access_token=***&refresh_token=***&oauth_redirect=/x",
		"/v1/vault/setup/script?token=st_1":                               "/v1/vault/setup/script?token=***",
		"/v1/teams?page=2&keyword=a":                                      "/v1/teams?page=2&keyword=a",
		"/health":                                                         "/health",
	}
	for in, want := range cases {
		if got := RedactQuery(in); got != want {
			t.Errorf("RedactQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRequestLoggerHidesSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = nil }()
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/ws/teams/:id", func(c *gin.Context) {
		if c.Query("token") != "s3cr3t" {
			t.Errorf("handler must still see the real secret")
		}
		c.Status(200)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ws/teams/team_x?token=s3cr3t", nil))
	out := buf.String()
	if strings.Contains(out, "s3cr3t") || !strings.Contains(out, `"/ws/teams/team_x?token=***"`) {
		t.Fatalf("log line not redacted: %q", out)
	}
}
