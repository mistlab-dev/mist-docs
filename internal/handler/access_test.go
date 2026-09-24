package handler

import "testing"

func TestSafeBaseName(t *testing.T) {
	okName, ok := safeBaseName("a1b2.png")
	if !ok || okName != "a1b2.png" {
		t.Fatalf("plain name: %q %v", okName, ok)
	}
	for _, bad := range []string{"", ".", "..", "../etc/passwd", `..\win`, "a/b", "a\\b", "..hidden"} {
		if _, ok := safeBaseName(bad); ok {
			t.Errorf("should reject %q", bad)
		}
	}
}

func TestValidWebhookURL(t *testing.T) {
	if !validWebhookURL("https://example.com/hook") {
		t.Error("https should pass")
	}
	if !validWebhookURL("http://hooks.internal:8080/docs") {
		t.Error("http with port should pass")
	}
	for _, bad := range []string{"", "javascript:alert(1)", "file:///etc/passwd", "https://user:pass@example.com/hook", "notaurl", "ftp://example.com/x"} {
		if validWebhookURL(bad) {
			t.Errorf("should reject %q", bad)
		}
	}
}

func TestResolveCollabPermission(t *testing.T) {
	if p, ok := resolveCollabPermission("", "viewer"); !ok || p != "read" {
		t.Fatalf("role viewer: %q %v", p, ok)
	}
	if p, ok := resolveCollabPermission("write", "viewer"); !ok || p != "write" {
		t.Fatalf("permission wins when both set: %q %v", p, ok)
	}
	if _, ok := resolveCollabPermission("", "owner"); ok {
		t.Error("owner is not a document role")
	}
}
