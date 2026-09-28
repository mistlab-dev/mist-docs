package webhook

import "testing"

func TestParseWebhookEventsJSON(t *testing.T) {
	got := ParseEvents(`["document.created","document.updated"]`)
	if len(got) != 2 || got[0] != "document.created" || got[1] != "document.updated" {
		t.Fatalf("json events: %#v", got)
	}
}

func TestParseWebhookEventsCSV(t *testing.T) {
	got := ParseEvents("create_doc, edit_doc")
	if len(got) != 2 || got[0] != "create_doc" || got[1] != "edit_doc" {
		t.Fatalf("csv events: %#v", got)
	}
}

func TestWebhookDeliveryMatchesAdvertisedNames(t *testing.T) {
	subscribed := `["document.created","document.updated"]`
	if !Subscribed(subscribed, Canonical("create_doc")) {
		t.Fatal("create_doc should deliver to document.created")
	}
	if !Subscribed(subscribed, Canonical("edit_doc")) {
		t.Fatal("edit_doc should deliver to document.updated")
	}
	if Subscribed(subscribed, Canonical("delete_doc")) {
		t.Fatal("delete is not in the default subscription")
	}
}

func TestWebhookAliases(t *testing.T) {
	if !Subscribed("create_doc,update_doc", "document.updated") {
		t.Fatal("update_doc alias should match document.updated")
	}
	if !Subscribed("*", "document.created") {
		t.Fatal("wildcard should match")
	}
	if Canonical("edit_doc") != "document.updated" {
		t.Fatal("canonical edit_doc")
	}
	if Canonical("create_doc") != "document.created" {
		t.Fatal("canonical create_doc")
	}
}
