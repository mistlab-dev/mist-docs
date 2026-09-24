package handler

import "testing"

func TestParseWebhookEventsJSON(t *testing.T) {
	got := parseWebhookEvents(`["document.created","document.updated"]`)
	if len(got) != 2 || got[0] != "document.created" || got[1] != "document.updated" {
		t.Fatalf("json events: %#v", got)
	}
}

func TestParseWebhookEventsCSV(t *testing.T) {
	got := parseWebhookEvents("create_doc, edit_doc")
	if len(got) != 2 || got[0] != "create_doc" || got[1] != "edit_doc" {
		t.Fatalf("csv events: %#v", got)
	}
}

func TestWebhookDeliveryMatchesAdvertisedNames(t *testing.T) {
	subscribed := `["document.created","document.updated"]`
	if !webhookSubscribed(subscribed, canonicalWebhookEvent("create_doc")) {
		t.Fatal("create_doc should deliver to document.created")
	}
	if !webhookSubscribed(subscribed, canonicalWebhookEvent("edit_doc")) {
		t.Fatal("edit_doc should deliver to document.updated")
	}
	if webhookSubscribed(subscribed, canonicalWebhookEvent("delete_doc")) {
		t.Fatal("delete is not in the default subscription")
	}
}

func TestWebhookAliases(t *testing.T) {
	if !webhookSubscribed("create_doc,update_doc", "document.updated") {
		t.Fatal("update_doc alias should match document.updated")
	}
	if !webhookSubscribed("*", "document.created") {
		t.Fatal("wildcard should match")
	}
	if canonicalWebhookEvent("edit_doc") != "document.updated" {
		t.Fatal("canonical edit_doc")
	}
	if canonicalWebhookEvent("create_doc") != "document.created" {
		t.Fatal("canonical create_doc")
	}
}
