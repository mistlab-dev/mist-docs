package handler

import "testing"

func TestParseMentionTokens(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"no mention", "这是一段普通评论", nil},
		{"single mention", "@张三 麻烦看下这段", []string{"张三"}},
		{"two mentions", "@alice @bob 请确认", []string{"alice", "bob"}},
		{"case folded", "@Alice 看看", []string{"alice"}},
		{"mention at end no space", "结论是@a", []string{"a"}},
		{"trailing space consumed", "@李四 ", []string{"李四"}},
		{"double at not matched", "email a@b.com", nil},
		{"cjk before at ok", "联系a@b.com 之外 @王五，同意", []string{"王五"}},
		{"underscore blocks", "foo_@bar", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMentionTokens(tc.in)
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("want no tokens, got %v", got)
				}
				return
			}
			for _, w := range tc.want {
				if !got[w] {
					t.Fatalf("want token %q in %v", w, got)
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("want %d tokens, got %d: %v", len(tc.want), len(got), got)
			}
		})
	}
}
