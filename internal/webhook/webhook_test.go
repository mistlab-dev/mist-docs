package webhook

import "testing"

func TestCanonicalMapsEveryAlias(t *testing.T) {
	cases := map[string]string{
		"create_doc": "document.created", "edit_doc": "document.updated", "delete_doc": "document.deleted",
		"create_share": "document.shared", "create_comment": "comment.created", "import_doc": "document.imported",
		"lock_doc": "document.locked", "unlock_doc": "document.unlocked",
		"restore": "document.restored", "restore_doc": "document.restored",
		"deadline.reminder": "deadline.reminder", "view": "view",
	}
	for in, want := range cases {
		if got := Canonical(in); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
	for _, e := range Events {
		if Canonical(e) != e || !Valid(e) {
			t.Errorf("catalogue event %q is not canonical/valid", e)
		}
	}
	if Valid("view") || Valid("") {
		t.Error("audit-only actions are not webhook events")
	}
}

func TestSubscribedAcceptsLegacyNames(t *testing.T) {
	if !Subscribed("create_share", "document.shared") {
		t.Error("legacy action name should match the dotted event")
	}
	if !Subscribed(`["document.restored"]`, Canonical("restore")) {
		t.Error("restore should reach document.restored subscribers")
	}
	if Subscribed(`["document.created"]`, "deadline.reminder") {
		t.Error("reminder must not reach a hook that did not subscribe")
	}
}

func TestReminderTargets(t *testing.T) {
	a := Target{ID: "a", Events: `["document.created"]`}
	b := Target{ID: "b", Events: `["deadline.reminder"]`}
	c := Target{ID: "c", Events: `*`}
	got := ReminderTargets([]Target{a, b, c})
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "c" {
		t.Fatalf("subscribed hooks only, got %+v", got)
	}
	// Teams whose hooks predate the event picker keep getting reminders.
	legacy := ReminderTargets([]Target{a})
	if len(legacy) != 1 || legacy[0].ID != "a" {
		t.Fatalf("legacy fallback, got %+v", legacy)
	}
}
