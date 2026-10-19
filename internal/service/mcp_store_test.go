package service

import (
	"strings"
	"testing"
)

// mcpExcerpt must return plain text: HTML stripped, entities decoded, window
// centred on the first case-insensitive hit, never splitting the hit itself.
func TestMCPExcerptStripsHTMLAndCentresHit(t *testing.T) {
	// Long filler BEFORE the hit so the window must clip on the left too.
	body := `<h1>Redis 故障应急</h1>` + strings.Repeat(`<p>无关内容 padding padding padding padding。</p>`, 10) +
		`<p>检查主从延迟，如果延迟过高需要切换。</p>` +
		strings.Repeat(`<p>后续无关内容 padding padding padding。</p>`, 10)
	got := mcpExcerpt(body, "主从延迟", 60)
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Fatalf("excerpt contains HTML: %q", got)
	}
	if !strings.Contains(got, "主从延迟") {
		t.Fatalf("excerpt misses the hit: %q", got)
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis-clipped window, got %q", got)
	}
}

func TestMCPExcerptDropsScriptAndStyle(t *testing.T) {
	body := `<style>p{color:red}</style><p>正常正文包含关键词</p><script>alert('x')</script>`
	got := mcpExcerpt(body, "关键词", 100)
	if strings.Contains(got, "alert") || strings.Contains(got, "color") {
		t.Fatalf("script/style leaked into excerpt: %q", got)
	}
	if !strings.Contains(got, "正常正文包含关键词") {
		t.Fatalf("excerpt lost the visible text: %q", got)
	}
}

func TestMCPExcerptDecodesEntities(t *testing.T) {
	got := mcpExcerpt(`<p>a &amp; b &lt;tag&gt;</p>`, "b", 100)
	if strings.Contains(got, "&amp;") {
		t.Fatalf("entity not decoded: %q", got)
	}
	if !strings.Contains(got, "a & b") {
		t.Fatalf("decoded text missing: %q", got)
	}
}

func TestMCPExcerptCaseInsensitiveHit(t *testing.T) {
	got := mcpExcerpt(`<p>Connecting to REDIS primary</p>`, "redis", 60)
	if !strings.Contains(got, "REDIS") {
		t.Fatalf("case-insensitive hit failed: %q", got)
	}
}

func TestMCPExcerptNoHitTruncates(t *testing.T) {
	long := strings.Repeat("字", 300)
	got := mcpExcerpt(long, "不存在", 50)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected truncated fallback, got %q", got)
	}
	if r := []rune(strings.TrimSuffix(got, "…")); len(r) > 60 {
		t.Fatalf("truncation too long: %d runes", len(r))
	}
}

func TestMCPExcerptShortBodyReturnsWhole(t *testing.T) {
	got := mcpExcerpt(`<p>短正文</p>`, "不存在", 50)
	if got != "短正文" {
		t.Fatalf("got %q, want 短正文", got)
	}
}

// foldIndex maps byte offsets on the original string; a case change that
// alters UTF-8 byte length must not drift the offset.
func TestFoldIndexByteOffsetOnOriginal(t *testing.T) {
	body := "前缀 ȺȺȺ REDIS 后缀"
	idx := foldIndex(body, "redis")
	if idx < 0 {
		t.Fatal("hit not found")
	}
	if !strings.HasPrefix(body[idx:], "REDIS") {
		t.Fatalf("offset %d lands on %q", idx, body[idx:idx+5])
	}
}

func TestFoldIndexEmptyNeedle(t *testing.T) {
	if got := foldIndex("abc", ""); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}
