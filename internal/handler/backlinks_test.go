package handler

import (
	"testing"
	"unicode/utf8"
)

func TestFormatBacklinkExcerpt(t *testing.T) {
	targetID := "11111111-2222-3333-4444-555555555555"

	// 真实 TipTap 双链渲染出来的 HTML 片段
	tipTapHTML := `<p>在日常发布前，必须先查看 <a target="_blank" rel="noopener" class="editor-link" href="https://docs.mistlab.dev/docs/11111111-2222-3333-4444-555555555555">上线检查清单</a> 后再执行生产灰度发布流程。</p>`

	excerpt := formatBacklinkExcerpt(tipTapHTML, targetID)

	// 1. 验证不包含未闭合或残缺的 HTML 属性（如 noopener、class="editor-link" 等）
	if !utf8.ValidString(excerpt) {
		t.Fatalf("excerpt is not valid utf-8: %q", excerpt)
	}
	for _, bad := range []string{"noopener", "editor-link", "https://", "<a", "</a>", "<p>", "</p>"} {
		if containsSubstring(excerpt, bad) {
			t.Errorf("excerpt should not contain HTML tags/attributes %q, got: %q", bad, excerpt)
		}
	}

	// 2. 验证锚文本「上线检查清单」被保留在摘要中
	if !containsSubstring(excerpt, "上线检查清单") {
		t.Errorf("excerpt should contain anchor text '上线检查清单', got: %q", excerpt)
	}

	// 3. 验证上下文文字被正确保留
	if !containsSubstring(excerpt, "发布前") || !containsSubstring(excerpt, "灰度发布") {
		t.Errorf("excerpt should retain context around the link, got: %q", excerpt)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && len(sub) > 0 && searchSub(s, sub)))
}

func searchSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
