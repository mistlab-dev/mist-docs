package handler

import "testing"

func TestParseBacklinkTargets(t *testing.T) {
	docA := "11111111-2222-3333-4444-555555555555"
	docB := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"empty", "", nil},
		{"no links", "<p>普通段落没有链接</p>", nil},
		{"anchor html", "<p>参见 <a href=\"/docs/" + docA + "\">方案文档</a> 一节</p>", []string{docA}},
		{"markdown link", "详见 [方案文档](/docs/" + docA + ")", []string{docA}},
		{"absolute url", "https://mistlab.dev/docs/" + docB, []string{docB}},
		{"two links", "<a href=\"/docs/" + docA + "\">a</a> 和 <a href=\"/docs/" + docB + "\">b</a>", []string{docA, docB}},
		{"duplicate dedup", "<a href=\"/docs/" + docA + "\">a</a><a href=\"/docs/" + docA + "\">a</a>", []string{docA}},
		{"not a uuid", "<a href=\"/docs/hello-world-123\">x</a>", nil},
		{"self not special-cased", "自己链接 /docs/" + docA, []string{docA}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseBacklinkTargets(tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("parseBacklinkTargets(%q) = %v, want %v", tc.body, got, tc.want)
			}
			for _, id := range tc.want {
				if !got[id] {
					t.Fatalf("parseBacklinkTargets(%q) missing %s (got %v)", tc.body, id, got)
				}
			}
		})
	}
}
