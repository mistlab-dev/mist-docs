package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipLimiter keeps one token bucket per client IP. Each RateLimit call gets
// its own set, so two limits with different rates cannot share buckets.
type ipLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

func newIPLimiter(rps float64, burst int) *ipLimiter {
	return &ipLimiter{visitors: make(map[string]*visitor), rps: rate.Limit(rps), burst: burst}
}

func (l *ipLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = now
	return v.limiter.AllowN(now, 1)
}

// sweep forgets IPs not seen for idle, so the map does not grow forever.
func (l *ipLimiter) sweep(now time.Time, idle time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, v := range l.visitors {
		if now.Sub(v.lastSeen) > idle {
			delete(l.visitors, ip)
		}
	}
}

func (l *ipLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.visitors)
}

// RateLimit returns a middleware that limits requests per IP.
// rps = requests per second, burst = max burst size.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	l := newIPLimiter(rps, burst)
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			l.sweep(time.Now(), 5*time.Minute)
		}
	}()
	return rateLimitHandler(l)
}

func rateLimitHandler(l *ipLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c.ClientIP(), time.Now()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
