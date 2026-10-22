package handler

import (
	"strings"
	"testing"

	"github.com/c-wind/mist-docs/internal/config"
)

func TestParagraphSlugFromHeadingText(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Safe Cleanup", "safe-cleanup"},
		{"  磁盘满了  ", "磁盘满了"},
		{"Step 1: Check / Disk?", "step-1-check-disk"},
		{"Redis 超时", "redis-超时"},
		{"a/b?c#d&e\\f", "a-b-c-d-e-f"},
		{"<strong>Safe</strong> Cleanup", "safe-cleanup"},
		{"磁盘&amp;满", "磁盘-满"},
		{"...", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := paragraphSlug(tc.in); got != tc.want {
			t.Errorf("paragraphSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParagraphSlugCapsAt128Runes(t *testing.T) {
	got := paragraphSlug(strings.Repeat("节", 200))
	if n := len([]rune(got)); n != 128 {
		t.Fatalf("len = %d, want 128 (%q…)", n, string([]rune(got)[:8]))
	}
	if strings.ContainsAny(got, " \t#?&/\\") {
		t.Fatalf("slug has characters MistTerm rejects: %q", got)
	}
}

func TestParagraphAnchorUsesNearestHeading(t *testing.T) {
	body := `<h2>先看这里</h2><p>无关</p><h2>磁盘满了</h2><p>执行 df -h</p><h2>后面</h2>`
	if got := paragraphAnchor(body, "df"); got != "磁盘满了" {
		t.Fatalf("got %q", got)
	}
	if got := paragraphAnchor(body, "DF"); got != "磁盘满了" {
		t.Fatalf("case fold: got %q", got)
	}
	// Keyword before every heading, or only in the title, has no paragraph slug.
	if got := paragraphAnchor(`<p>先看到 KEYWORD</p><h2>后面的标题</h2>`, "KEYWORD"); got != "" {
		t.Fatalf("heading after the hit leaked: %q", got)
	}
	if got := paragraphAnchor(`<p>没有这个词</p>`, "KEYWORD"); got != "" {
		t.Fatalf("missing keyword got %q", got)
	}
	// The heading that contains the hit wins over the previous section.
	inner := `<h2>Safe Cleanup</h2><p>禁止 rm -rf</p>`
	if got := paragraphAnchor(inner, "cleanup"); got != "safe-cleanup" {
		t.Fatalf("hit inside heading: got %q", got)
	}
	// A nearer heading with no text does not keep the previous section's slug.
	empty := `<h2>保留</h2><h2><img src="x.png"></h2><p>KEYWORD</p>`
	if got := paragraphAnchor(empty, "KEYWORD"); got != "" {
		t.Fatalf("empty nearest heading should not reuse the previous slug, got %q", got)
	}
}

func TestParagraphAnchorIgnoresScriptAndStyle(t *testing.T) {
	onlyScript := `<script><h2>假标题</h2>KEYWORD</script><h2>真标题</h2><p>正文</p>`
	if got := paragraphAnchor(onlyScript, "KEYWORD"); got != "" {
		t.Fatalf("script hit should not anchor, got %q", got)
	}
	both := `<style>h2{color:red}</style><script>KEYWORD</script><h2>真标题</h2><p>正文 KEYWORD</p>`
	if got := paragraphAnchor(both, "KEYWORD"); got != "真标题" {
		t.Fatalf("got %q", got)
	}
}

func TestParagraphDocURL(t *testing.T) {
	prev := config.C.Server.BaseURL
	t.Cleanup(func() { config.C.Server.BaseURL = prev })
	config.C.Server.BaseURL = ""

	if got := paragraphDocURL("abc-1", "safe-cleanup"); got != "https://docs.mistlab.dev/docs/abc-1#safe-cleanup" {
		t.Fatalf("default url: %q", got)
	}
	if got := paragraphDocURL("abc-1", ""); got != "https://docs.mistlab.dev/docs/abc-1" {
		t.Fatalf("no slug: %q", got)
	}
	if got := paragraphDocURL("../etc", "x"); got != "" {
		t.Fatalf("unsafe id should not produce a url, got %q", got)
	}
	if got := paragraphDocURL("has space", ""); got != "" {
		t.Fatalf("space in id: %q", got)
	}

	config.C.Server.BaseURL = "https://docs.example.com/"
	if got := paragraphDocURL("abc-1", "disk"); got != "https://docs.example.com/docs/abc-1#disk" {
		t.Fatalf("base_url: %q", got)
	}
	for _, bad := range []string{
		"javascript:alert(1)",
		"https://user:pw@docs.example.com",
		"https://docs.example.com/extra",
		"https://docs.example.com/docs?q=1",
		"notes://docs.example.com",
	} {
		config.C.Server.BaseURL = bad
		if got := paragraphDocURL("abc-1", ""); got != "https://docs.mistlab.dev/docs/abc-1" {
			t.Fatalf("base %q fell through to %q", bad, got)
		}
	}
}
