package webhook

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "127.8.9.1", "10.1.2.3", "172.16.0.1", "192.168.1.1",
		"169.254.169.254", "169.254.1.1", "100.64.0.1", "100.100.100.200", "0.0.0.0", "255.255.255.255",
		"224.0.0.1", "::1", "::", "fe80::1", "fc00::1", "fd00:ec2::254", "::ffff:127.0.0.1", "::ffff:10.0.0.1", "64:ff9b::a00:1"}
	for _, s := range blocked {
		if !BlockedIP(net.ParseIP(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "203.0.114.5", "2606:4700:4700::1111"} {
		if BlockedIP(net.ParseIP(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
}

func TestCheckURL(t *testing.T) {
	ctx := context.Background()
	for _, u := range []string{
		"http://127.0.0.1:8900/x", "http://localhost/hook", "http://169.254.169.254/latest/meta-data",
		"https://10.0.0.5/", "http://[::1]:80/", "http://[fd00:ec2::254]/", "http://0x7f000001/",
		"ftp://example.com/", "file:///etc/passwd", "gopher://example.com", "http://user:pw@example.com/", "not a url",
	} {
		if err := CheckURL(ctx, u); err == nil {
			t.Errorf("%q must be rejected", u)
		}
	}
	if err := CheckURL(ctx, "https://8.8.8.8/hook"); err != nil {
		t.Errorf("public IP rejected: %v", err)
	}
}

// At send time the dialer refuses internal addresses even if the URL got
// past save-time validation (DNS rebinding, redirects, old rows).
func TestClientRefusesLoopback(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()
	_, err := Client(2*time.Second).Post(srv.URL, "application/json", nil)
	if err == nil || !errors.Is(err, ErrBlockedTarget) || hit {
		t.Fatalf("loopback delivery must be refused: err=%v hit=%v", err, hit)
	}

	// The test-only switch lets the suite's 127.0.0.1 receivers work.
	AllowPrivateTargets = true
	resp, err := Client(2*time.Second).Post(srv.URL, "application/json", nil)
	AllowPrivateTargets = false
	if err != nil || !hit {
		t.Fatalf("with the test switch on, loopback must work: err=%v", err)
	}
	resp.Body.Close()
}
