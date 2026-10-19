package handler

import (
	"testing"
	"unicode/utf8"
)

func TestFormatBacklinkExcerpt(t *testing.T) {
	targetID := "11111111-2222-3333-4444-555555555555"

	// 1. 真实 TipTap 双链渲染出来的 HTML 片段
	tipTapHTML := `<p>在日常发布前，必须先查看 <a target="_blank" rel="noopener" class="editor-link" href="https://docs.mistlab.dev/docs/11111111-2222-3333-4444-555555555555">上线检查清单</a> 后再执行生产灰度发布流程。</p>`

	excerpt := formatBacklinkExcerpt(tipTapHTML, targetID)

	if !utf8.ValidString(excerpt) {
		t.Fatalf("excerpt is not valid utf-8: %q", excerpt)
	}
	for _, bad := range []string{"noopener", "editor-link", "https://", "<a", "</a>", "<p>", "</p>"} {
		if containsSubstring(excerpt, bad) {
			t.Errorf("excerpt should not contain HTML tags/attributes %q, got: %q", bad, excerpt)
		}
	}
	if !containsSubstring(excerpt, "上线检查清单") {
		t.Errorf("excerpt should contain anchor text '上线检查清单', got: %q", excerpt)
	}
	if !containsSubstring(excerpt, "发布前") || !containsSubstring(excerpt, "灰度发布") {
		t.Errorf("excerpt should retain context around the link, got: %q", excerpt)
	}

	// 2. 模拟切片起点落在前一个标签属性中间，且目标链接前面有多条双链
	multiLinkBrokenPrefix := `002">相关2</a> 和 <a target="_blank" rel="noopener" class="editor-link" href="https://docs.mistlab.dev/docs/22222222-2222-3333-4444-555555555555">相关3</a> 参见 <a target="_blank" rel="noopener" class="editor-link" href="https://docs.mistlab.dev/docs/11111111-2222-3333-4444-555555555555">上线检查清单</a> 确认无误`
	excerpt2 := formatBacklinkExcerpt(multiLinkBrokenPrefix, targetID)

	if containsSubstring(excerpt2, "002\"") {
		t.Errorf("broken attribute prefix should be cleaned, got: %q", excerpt2)
	}
	if !containsSubstring(excerpt2, "上线检查清单") {
		t.Errorf("anchor text must be accurately centered and visible, got: %q", excerpt2)
	}

	// 3. 模拟 Markdown 链接形式 [标题](/docs/{id})
	mdInput := `前置内容详见 [部署架构说明](/docs/11111111-2222-3333-4444-555555555555) 以及后续配置`
	excerpt3 := formatBacklinkExcerpt(mdInput, targetID)
	if containsSubstring(excerpt3, "/docs/11111111") {
		t.Errorf("markdown link url should be replaced by title, got: %q", excerpt3)
	}
	if !containsSubstring(excerpt3, "部署架构说明") {
		t.Errorf("markdown anchor title should be present, got: %q", excerpt3)
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
