package service

import "testing"

func TestNormalizePermission(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"viewer", "read", true},
		{"editor", "write", true},
		{"admin", "admin", true},
		{"read", "read", true},
		{"WRITE", "write", true},
		{"commenter", "comment", true},
		{"owner", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizePermission(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("NormalizePermission(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPermAtLeast(t *testing.T) {
	if !PermAtLeast("write", "read") {
		t.Error("write should satisfy read")
	}
	if PermAtLeast("read", "write") {
		t.Error("read should not satisfy write")
	}
	if PermAtLeast("", "read") || PermAtLeast("none", "read") {
		t.Error("empty permission should not satisfy read")
	}
	if !PermAtLeast("admin", "admin") {
		t.Error("admin should satisfy admin")
	}
}

func TestDefaultTeamPerm(t *testing.T) {
	if defaultTeamPerm("viewer") != "read" {
		t.Error("viewer defaults to read")
	}
	if defaultTeamPerm("editor") != "write" {
		t.Error("editor defaults to write")
	}
	if defaultTeamPerm("member") != "write" {
		t.Error("member keeps write, matching previous non-viewer behavior")
	}
	if defaultTeamPerm("") != "none" {
		t.Error("missing role is none")
	}
	if defaultTeamPerm("owner") != "admin" {
		t.Error("owner is admin")
	}
}

func TestFrontendRole(t *testing.T) {
	if FrontendRole("read") != "viewer" || FrontendRole("editor") != "editor" || FrontendRole("write") != "editor" {
		t.Fatalf("frontend roles: %s %s %s", FrontendRole("read"), FrontendRole("editor"), FrontendRole("write"))
	}
}
