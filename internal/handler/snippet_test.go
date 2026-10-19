package handler

import "testing"

func TestSnippetAround(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		keyword string
		want    string
	}{
		{"empty body", "", "kw", ""},
		{"no hit short body", "短正文", "不存在", "短正文"},
		// width=40 字节 ≈ 13 个汉字（39B），第 14 字放不下，退到 rune 边界。
		{"no hit long body", "一二三四五六七八九十一二三四五六七八九十", "不存在", "一二三四五六七八九十一二三…"},
		{"hit near start", "关键词在开头 rest", "关键词", "关键词在开头 rest"},
		// 命中距开头 27B > 半窗 20B，start 归 6（rune 边界）；窗口右端触底无后缀省略号。
		{"hit near end", "这是前面的内容然后关键词", "关键词", "…前面的内容然后关键词"},
		{"cjk centering keeps runes", "前置内容关键词后置内容", "关键词", "前置内容关键词后置内容"},
		{"whitespace collapsed", "a\n\n  b\t关键词  c\n", "关键词", "a b 关键词 c"},
		{"case insensitive hit", "Hello WORLD again", "world", "Hello WORLD again"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := snippetAround(tc.body, tc.keyword, 40)
			if got != tc.want {
				t.Fatalf("snippetAround(%q,%q) = %q, want %q", tc.body, tc.keyword, got, tc.want)
			}
		})
	}
}
