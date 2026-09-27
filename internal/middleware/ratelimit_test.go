package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestIPLimiterBurstThenRefill(t *testing.T) {
	l := newIPLimiter(1, 3)
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < 3; i++ {
		if !l.allow("1.1.1.1", now) {
			t.Fatalf("request %d within burst was refused", i+1)
		}
	}
	if l.allow("1.1.1.1", now) {
		t.Fatal("4th request in the same instant should be refused")
	}
	if !l.allow("2.2.2.2", now) {
		t.Fatal("another IP has its own bucket")
	}
	if !l.allow("1.1.1.1", now.Add(1100*time.Millisecond)) {
		t.Fatal("one token should refill after a second")
	}
}

func TestIPLimiterSweep(t *testing.T) {
	l := newIPLimiter(1, 1)
	now := time.Unix(1_700_000_000, 0)
	l.allow("old", now)
	l.allow("new", now.Add(10*time.Minute))
	l.sweep(now.Add(10*time.Minute), 5*time.Minute)
	if l.size() != 1 {
		t.Fatalf("size %d, want 1 after sweeping the idle IP", l.size())
	}
}

func TestRateLimitInstancesAreIndependent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	strict := rateLimitHandler(newIPLimiter(0.001, 1))
	loose := rateLimitHandler(newIPLimiter(1000, 100))
	r := gin.New()
	r.GET("/strict", strict, func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/loose", loose, func(c *gin.Context) { c.String(200, "ok") })

	get := func(path string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "9.9.9.9:1234"
		r.ServeHTTP(w, req)
		return w.Code
	}
	if get("/strict") != 200 || get("/strict") != http.StatusTooManyRequests {
		t.Fatal("strict limiter should allow one then refuse")
	}
	for i := 0; i < 5; i++ {
		if code := get("/loose"); code != 200 {
			t.Fatalf("loose limiter refused request %d (%d): buckets leaked between instances", i+1, code)
		}
	}
}
