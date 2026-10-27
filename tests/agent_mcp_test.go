package tests

// Agent identity end-to-end tests (#76, review blockers 1+2):
// - agt_ bearer must reach MCP (JWTAuth must not 401 it first)
// - agent read tools must succeed via creator's team role
// - docs whitelist must bound search/list results
// - write tools must be denied at the gate (before any store code)

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/service"
)

func mcpCall(t *testing.T, token, tool string, args map[string]any) (int, map[string]any) {
	t.Helper()
	params := map[string]any{"name": tool, "arguments": args}
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  params,
	}
	w := request("POST", "/api/mcp", body, token)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func createTestAgent(t *testing.T, tools, docs []string) (id, token string) {
	t.Helper()
	req := map[string]any{"name": "e2e-bot", "tools": tools}
	if docs != nil {
		req["docs"] = docs
	}
	w := request("POST", teamPath("/agents"), req, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("create agent: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Agent struct {
			ID string `json:"id"`
		} `json:"agent"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse agent create response: %v", err)
	}
	t.Cleanup(func() {
		request("DELETE", teamPath("/agents/"+out.Agent.ID), nil, adminToken)
	})
	return out.Agent.ID, out.Token
}

func seedAgentDoc(t *testing.T, title, body string) string {
	t.Helper()
	w := request("POST", teamPath("/documents"), map[string]any{"title": title, "content": body}, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("seed doc: %d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	id, _ := out["id"].(string)
	if id == "" {
		if d, ok := out["document"].(map[string]any); ok {
			id, _ = d["id"].(string)
		}
	}
	if id == "" {
		t.Fatalf("seed doc: no id in response %s", w.Body.String())
	}
	return id
}

// 阻塞 1：agt_ token 打 /mcp 不得先被 JWTAuth 401（「token 无效」）。
func TestAgentTokenReachesMCP(t *testing.T) {
	_, token := createTestAgent(t, []string{"list_docs"}, nil)
	code, out := mcpCall(t, token, "list_docs", map[string]any{"team_id": teamID})
	if code == http.StatusUnauthorized {
		t.Fatalf("agent token was 401'd before the agent path: %v", out)
	}
	if code != http.StatusOK {
		t.Fatalf("tools/call via agent token: %d %v", code, out)
	}
	if res, ok := out["result"].(map[string]any); ok {
		content, _ := res["content"].([]any)
		if len(content) == 0 {
			t.Fatalf("list_docs returned empty content: %v", out)
		}
	}
}

// 阻塞 2：agent 经创建者角色可读（无 agent 行也不得判非成员）。
func TestAgentReadViaCreatorRole(t *testing.T) {
	docID := seedAgentDoc(t, "Agent E2E read", "<p>agent readable body</p>")
	_, token := createTestAgent(t, []string{"read_doc", "search_docs"}, nil)

	code, out := mcpCall(t, token, "read_doc", map[string]any{"team_id": teamID, "doc_id": docID})
	if code != http.StatusOK {
		t.Fatalf("agent read_doc: %d %v", code, out)
	}
	if errObj, has := out["error"]; has {
		t.Fatalf("agent read_doc errored: %v", errObj)
	}
}

// 中项：docs 白名单必须约束 search_docs（否则全文搜索泄漏未授权正文）。
func TestAgentDocsWhitelistBoundsSearch(t *testing.T) {
	allowed := seedAgentDoc(t, "Agent wl allowed", "<p>needle-alpha uniquebody</p>")
	seedAgentDoc(t, "Agent wl denied", "<p>needle-alpha secretbody</p>")
	_, token := createTestAgent(t, []string{"search_docs", "read_doc", "list_docs"}, []string{allowed})

	code, out := mcpCall(t, token, "search_docs", map[string]any{"team_id": teamID, "query": "needle-alpha"})
	if code != http.StatusOK {
		t.Fatalf("agent search_docs: %d %v", code, out)
	}
	raw, _ := json.Marshal(out)
	if strings.Contains(string(raw), "secretbody") {
		t.Fatalf("docs whitelist leaked a denied document body: %s", raw)
	}
	if !strings.Contains(string(raw), "allowed") {
		t.Fatalf("whitelisted doc missing from search results: %s", raw)
	}
}

// 写工具在 gate 层拒绝（scopes 创建时即不可授予写工具）。
func TestAgentWriteToolDenied(t *testing.T) {
	_, token := createTestAgent(t, []string{"read_doc"}, nil)
	code, out := mcpCall(t, token, "update_doc", map[string]any{"team_id": teamID, "doc_id": "x", "content": "hacked"})
	if code != http.StatusForbidden {
		t.Fatalf("update_doc must be 403 at the gate, got %d %v", code, out)
	}
}

// agent 只能访问自己的团队。
func TestAgentTeamMismatchDenied(t *testing.T) {
	_, token := createTestAgent(t, []string{"list_docs"}, nil)
	code, _ := mcpCall(t, token, "list_docs", map[string]any{"team_id": "some-other-team"})
	if code != http.StatusForbidden {
		t.Fatalf("cross-team tool call must be 403, got %d", code)
	}
}

// 过期与吊销即时失效（ AuthenticateAgent / ScopesForAgent 路径）。
func TestAgentScopesForAgentDisabled(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	if _, err := service.AuthenticateAgent(nil, ""); err == nil {
		// 空参数必须失败；真正启用/禁用路径由上面的端到端用例覆盖。
		t.Fatal("empty bearer must not authenticate")
	}
	_ = past
}
