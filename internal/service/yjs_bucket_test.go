package service

import "testing"

func TestYjsBucket(t *testing.T) {
	if got := yjsBucket("dept-1", "team-1"); got != "dept-1" {
		t.Fatalf("existing department bucket changed: %s", got)
	}
	if got := yjsBucket("", "team-1"); got != "team-1" {
		t.Fatalf("team docs should use team id, got %s", got)
	}
}
