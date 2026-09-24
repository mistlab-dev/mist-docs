package handler

import "testing"

func TestBuildAuditFilter(t *testing.T) {
	where, args := buildAuditFilter("edit_doc", "林", "2026-01-02", "2026-01-31")
	if where == "" || len(args) != 5 {
		t.Fatalf("filter = %q args=%v", where, args)
	}
	if args[0] != "edit_doc" || args[3] != "2026-01-02 00:00:00" || args[4] != "2026-01-31 23:59:59" {
		t.Fatalf("args = %#v", args)
	}

	where, args = buildAuditFilter("bad action", "100%", "yesterday", "")
	if where == "" || len(args) != 2 {
		t.Fatalf("unsafe action/date should be dropped, where=%q args=%v", where, args)
	}
	if args[0] != `%100\%%` || args[1] != `%100\%%` {
		t.Fatalf("like pattern = %#v", args)
	}
}

func TestValidAuditAction(t *testing.T) {
	if !validAuditAction("set_permission") || validAuditAction("Edit") || validAuditAction("") {
		t.Fatal("action whitelist mismatch")
	}
}
