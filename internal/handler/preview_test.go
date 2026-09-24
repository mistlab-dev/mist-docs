package handler

import "testing"

func TestPreviewExcerpt(t *testing.T) {
	got := previewExcerpt("<h1>机房巡检</h1><p>先在门禁登记。</p>")
	if got != "机房巡检 先在门禁登记。" {
		t.Fatalf("excerpt = %q", got)
	}
	if previewExcerpt(`{"cells":[]}`) != "" {
		t.Fatal("sheet json should not become card text")
	}
	long := stringsRepeat("甲", 120)
	if n := len([]rune(previewExcerpt(long))); n != 96 {
		t.Fatalf("rune len = %d", n)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("  ", "林晓", "lin") != "林晓" {
		t.Fatal("display name should win")
	}
	if firstNonEmpty("", "  lin ") != "lin" {
		t.Fatal("username fallback")
	}
	if firstNonEmpty("", " ") != "" {
		t.Fatal("empty")
	}
}

func stringsRepeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
