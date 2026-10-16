package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

func TestFolderChainReaches(t *testing.T) {
	tree := map[string]string{"a": "", "b": "a", "c": "b"}
	lookup := func(id string) (string, error) {
		p, ok := tree[id]
		if !ok {
			return "", sql.ErrNoRows
		}
		return p, nil
	}
	cases := []struct {
		folder, cand string
		want         bool
	}{
		{"a", "a", true}, {"a", "c", true}, {"b", "c", true},
		{"c", "a", false}, {"b", "a", false}, {"a", "missing", false},
	}
	for _, tc := range cases {
		if got := folderChainReaches(lookup, tc.folder, tc.cand); got != tc.want {
			t.Errorf("reaches(%s,%s)=%v want %v", tc.folder, tc.cand, got, tc.want)
		}
	}
	failing := func(string) (string, error) { return "", errors.New("db down") }
	if !folderChainReaches(failing, "a", "b") {
		t.Error("query error must be treated as cycle")
	}
	deep := func(id string) (string, error) {
		var n int
		fmt.Sscanf(id, "n%d", &n)
		return fmt.Sprintf("n%d", n+1), nil
	}
	if !folderChainReaches(deep, "root", "n0") {
		t.Error("exceeding depth limit must be treated as cycle")
	}
}

func TestParseFolderIDField(t *testing.T) {
	ok := map[string]struct {
		in   interface{}
		want string
	}{
		"nil": {nil, ""}, "str": {" abc ", "abc"}, "empty": {"", ""},
		"zero": {float64(0), ""}, "int": {float64(42), "42"},
	}
	for name, tc := range ok {
		got, valid := parseFolderIDField(tc.in)
		if !valid || got != tc.want {
			t.Errorf("%s: got %q,%v want %q,true", name, got, valid, tc.want)
		}
	}
	for name, in := range map[string]interface{}{
		"bool": true, "object": map[string]interface{}{}, "array": []interface{}{}, "frac": 1.5,
	} {
		if _, valid := parseFolderIDField(in); valid {
			t.Errorf("%s: expected rejection", name)
		}
	}
}
