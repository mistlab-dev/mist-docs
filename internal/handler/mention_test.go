package handler

import "testing"

// roster used across cases; ids are stable so assertions can reference them.
var (
	mAlice = mentionMember{id: "u-alice", dispName: "Alice", username: "alice"}
	mBob   = mentionMember{id: "u-bob", dispName: "Bob", username: "bob"}
	mJohn  = mentionMember{id: "u-john", dispName: "John Smith", username: "jsmith"}
	mWang  = mentionMember{id: "u-wang", dispName: "王五", username: "wang5"}
)

func matchedIDs(content string, members []mentionMember) map[string]bool {
	return matchMentions(content, members)
}

func TestMatchMentions(t *testing.T) {
	roster := []mentionMember{mAlice, mBob, mJohn, mWang}
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"none", "这是一段普通评论", nil},
		{"display name", "@王五 麻烦看下这段", []string{"u-wang"}},
		{"username case folded", "@Alice 看看", []string{"u-alice"}},
		{"display name case folded", "@alice 看看", []string{"u-alice"}},
		{"two members", "@alice @bob 请确认", []string{"u-alice", "u-bob"}},
		{"spaced display name", "请 @John Smith 复核", []string{"u-john"}},
		{"spaced name owns mention before shorter name", "@John Smith", []string{"u-john"}},
		{"email is not a mention", "联系 a@b.com 即可", nil},
		{"cjk punctuation terminates token", "@王五，同意", []string{"u-wang"}},
		{"prefix does not match", "@Aliceee 不是 Alice", nil},
		{"suffix does not match", "@Bobby 不是 Bob", nil},
		{"mention at start of line", "@bob 帮忙看看", []string{"u-bob"}},
		{"underscore before at blocks", "foo_@alice", nil},
		{"no members at all", "@谁都不认识", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchedIDs(tc.content, roster)
			want := map[string]bool{}
			for _, id := range tc.want {
				want[id] = true
			}
			if len(got) != len(want) {
				t.Fatalf("matchMentions(%q) = %v, want %v", tc.content, got, want)
			}
			for id := range want {
				if !got[id] {
					t.Fatalf("matchMentions(%q) missing %s (got %v)", tc.content, id, got)
				}
			}
		})
	}
}

// A longer display name must claim its own mention before a shorter name
// can steal the leading "@John" of "@John Smith".
func TestMatchMentionsLongestNameWins(t *testing.T) {
	members := []mentionMember{
		{id: "u-john", dispName: "John Smith", username: "jsmith"},
		{id: "u-johndoe", dispName: "John", username: "john"},
	}
	got := matchMentions("cc @John Smith dd", members)
	if !got["u-john"] {
		t.Fatalf("John Smith not matched: %v", got)
	}
	if got["u-johndoe"] {
		t.Fatalf("shorter 'John' must not claim '@John Smith': %v", got)
	}
	// Bare "@John" still notifies John.
	got = matchMentions("cc @John dd", members)
	if !got["u-johndoe"] {
		t.Fatalf("bare @John not matched: %v", got)
	}
	if got["u-john"] {
		t.Fatalf("'John Smith' must not match bare @John: %v", got)
	}
}

func TestCommentNotifyKind(t *testing.T) {
	cases := []struct {
		mentioned, replyTo bool
		wantType           string
		wantMention        bool
	}{
		{true, true, "comment", true},
		{true, false, "comment", true},
		{false, true, "reply", false},
		{false, false, "", false},
	}
	for _, tc := range cases {
		nType, mention := commentNotifyKind(tc.mentioned, tc.replyTo)
		if nType != tc.wantType || mention != tc.wantMention {
			t.Fatalf("commentNotifyKind(%v,%v) = (%q,%v), want (%q,%v)",
				tc.mentioned, tc.replyTo, nType, mention, tc.wantType, tc.wantMention)
		}
	}
}
