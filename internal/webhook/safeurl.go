package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// AllowPrivateTargets turns off the internal-address block. Only the test
// suite sets it (its receivers listen on 127.0.0.1); production never does.
var AllowPrivateTargets bool

// ErrBlockedTarget is returned when a webhook points at an internal address.
var ErrBlockedTarget = errors.New("webhook target is an internal address")

var blockedNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{
		"0.0.0.0/8",          // "this network"
		"100.64.0.0/10",      // carrier-grade NAT
		"192.0.0.0/24",       // IETF protocol assignments
		"198.18.0.0/15",      // benchmarking
		"240.0.0.0/4",        // reserved, incl. broadcast
		"64:ff9b::/96",       // NAT64 (embeds an IPv4 address)
		"64:ff9b:1::/48",     // local-use NAT64
		"2001:db8::/32",      // documentation
		"fd00:ec2::/32",      // AWS IPv6 metadata (also inside fc00::/7)
		"169.254.0.0/16",     // link-local incl. 169.254.169.254 metadata
		"100.100.100.200/32", // Alibaba Cloud metadata
	} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}()

// BlockedIP reports whether ip is loopback, private, link-local, multicast,
// unspecified, CGNAT, a cloud metadata address or otherwise not a public
// unicast address.
func BlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// CheckURL validates a webhook URL when it is saved: http or https, a host,
// no credentials, and every address the host resolves to must be public.
// The returned error text is shown to the user.
func CheckURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil {
		return errors.New("Webhook 地址格式不正确")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("Webhook 地址必须是 http 或 https")
	}
	if AllowPrivateTargets {
		return nil
	}
	host := u.Hostname()
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return fmt.Errorf("Webhook 地址无法解析：%s", host)
		}
		for _, a := range addrs {
			ips = append(ips, a.IP)
		}
	}
	for _, ip := range ips {
		if BlockedIP(ip) {
			return errors.New("Webhook 地址不能指向内网、本机或云元数据地址")
		}
	}
	return nil
}

// dialControl runs after DNS resolution, on the exact address being
// dialled, so a hostname that resolves (or re-resolves, or redirects) to an
// internal address is refused at send time too.
func dialControl(network, address string, _ syscall.RawConn) error {
	if AllowPrivateTargets {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if BlockedIP(net.ParseIP(host)) {
		return fmt.Errorf("%w: %s", ErrBlockedTarget, host)
	}
	return nil
}

// Client returns the HTTP client every webhook delivery must use. It never
// uses an environment proxy (the proxy would do the dial), refuses internal
// addresses at connect time and only follows http/https redirects.
func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, Control: dialControl}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: timeout,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect to non-http scheme")
			}
			return nil
		},
	}
}
