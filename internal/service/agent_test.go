package service

import (
	"context"
	"testing"
	"time"
)

func TestAgentToolGrantableDeniesWrites(t *testing.T) {
	// #76 验收：create_doc / update_doc 默认禁止（#68 §5）
	for _, tool := range []string{"create_doc", "update_doc", "", "drop_db"} {
		if agentToolGrantable(tool) {
			t.Errorf("tool %q must not be grantable to agents", tool)
		}
	}
	for _, tool := range []string{"search_docs", "read_doc", "list_docs", "search_doc_content"} {
		if !agentToolGrantable(tool) {
			t.Errorf("read-only tool %q must be grantable", tool)
		}
	}
}

func TestAgentScopesJSONRejectsWriteTools(t *testing.T) {
	if _, err := AgentScopesJSON(AgentScopes{Tools: []string{"read_doc", "update_doc"}}); err == nil {
		t.Fatal("scopes with update_doc must be rejected at creation")
	}
	if _, err := AgentScopesJSON(AgentScopes{Tools: nil}); err == nil {
		t.Fatal("empty tools whitelist must be rejected")
	}
	if _, err := AgentScopesJSON(AgentScopes{Tools: []string{"read_doc"}}); err != nil {
		t.Fatalf("valid scopes rejected: %v", err)
	}
}

func TestAgentAllowedToolAndDocsWhitelist(t *testing.T) {
	a := &Agent{Scopes: AgentScopes{
		Tools: []string{"read_doc", "search_docs"},
		Docs:  []string{"doc-1"},
	}}
	if !AgentAllowed(a, "read_doc", "doc-1") {
		t.Fatal("read_doc on whitelisted doc must pass")
	}
	if AgentAllowed(a, "read_doc", "doc-2") {
		t.Fatal("read_doc outside docs whitelist must be denied")
	}
	if AgentAllowed(a, "update_doc", "doc-1") {
		t.Fatal("update_doc must be denied even on whitelisted doc")
	}
	// 空文档白名单 = 团队语义（交由 HasTeamPermission 判定）
	wide := &Agent{Scopes: AgentScopes{Tools: []string{"read_doc"}}}
	if !AgentAllowed(wide, "read_doc", "any-doc") {
		t.Fatal("empty docs whitelist = team semantics")
	}
}

func TestAgentExpiryDefaultsTo30DaysAndNeverExtends(t *testing.T) {
	// 设计 §4：默认 30 天，创建时可缩短不可延长（纯函数钳制，不碰库）。
	// nil => 默认 30 天
	got := clampAgentExpiry(nil)
	if got == nil || got.Before(time.Now().AddDate(0, 0, 29)) || got.After(time.Now().AddDate(0, 0, 31)) {
		t.Fatalf("nil expiry must clamp to ~30 days, got %v", got)
	}
	// 90 天请求 => 钳回 30 天
	far := time.Now().AddDate(0, 0, 90)
	got = clampAgentExpiry(&far)
	if got.After(time.Now().AddDate(0, 0, 31)) {
		t.Fatalf("90-day expiry must be clamped to ~30 days, got %v", got)
	}
	// 7 天请求 => 保留（只能缩短）
	soon := time.Now().AddDate(0, 0, 7)
	got = clampAgentExpiry(&soon)
	if got.Sub(soon) > time.Minute {
		t.Fatalf("shorter expiry must be kept, got %v want %v", got, soon)
	}
}

func TestAgentTokenPrefixAndHash(t *testing.T) {
	plain, hash, err := agentTokenGenerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 {
		t.Fatalf("sha256 hex must be 64 chars, got %d", len(hash))
	}
	if agentTokenHash(plain) != hash {
		t.Fatal("hash(plain) mismatch")
	}
}

// CreateAgentForTest wraps CreateAgent with test cleanup; uses the shared
// test DB handle (service tests run against the dev DSN like mcp_store_test).
func CreateAgentForTest(t *testing.T, teamID, name, creator string, scopes AgentScopes, expires *time.Time) (*Agent, string, error) {
	t.Helper()
	return CreateAgent(context.Background(), teamID, name, creator, scopes, expires)
}
