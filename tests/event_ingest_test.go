package tests

// #77 T2: cross-product event ingest (agent token only).

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func createIngestAgent(t *testing.T) string {
	t.Helper()
	// Ingest is keyed on actor_type=agent, not scopes; give read tools too
	// so the same agent could also search docs.
	w := request("POST", teamPath("/agents"),
		map[string]any{"name": "ingest-bot", "tools": []string{"search_docs", "read_doc"}}, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("create agent: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Agent struct {
			ID string `json:"id"`
		} `json:"agent"`
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	t.Cleanup(func() {
		request("DELETE", teamPath("/agents/"+out.Agent.ID), nil, adminToken)
	})
	return out.Token
}

// 人类 JWT 不允许伪造终端事件。
func TestIngestRejectsUserJWT(t *testing.T) {
	w := request("POST", teamPath("/events/ingest"),
		map[string]any{"event": "term.attention", "title": "fake"}, adminToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("user JWT ingest must be 403, got %d", w.Code)
	}
}

// agent 汇入合法事件 → 200 且事件名被接受。
func TestIngestAcceptsAgentEvent(t *testing.T) {
	token := createIngestAgent(t)
	w := request("POST", teamPath("/events/ingest"),
		map[string]any{"event": "term.attention", "title": "长任务完成: backup.sh", "source_session": "sess-1"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("agent ingest: %d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("ingest not ok: %s", w.Body.String())
	}
	if rec, _ := out["recipients"].(float64); rec < 3 {
		t.Fatalf("expected inbox rows for >=3 team members, got %v", out)
	}
}

// 非法事件名 / 空 title / 超长字段全部 400。
func TestIngestValidatesInput(t *testing.T) {
	token := createIngestAgent(t)
	cases := []map[string]any{
		{"event": "document.deleted", "title": "not ingestable"},
		{"event": "term.attention", "title": ""},
		{"event": "term.attention", "title": strings.Repeat("x", 201)},
	}
	for i, body := range cases {
		w := request("POST", teamPath("/events/ingest"), body, token)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("case %d: expected 400, got %d %s", i, w.Code, w.Body.String())
		}
	}
}

// agent 不能给别的团队注入事件。
func TestIngestTeamMismatch(t *testing.T) {
	token := createIngestAgent(t)
	w := request("POST", "/api/teams/other-team/events/ingest",
		map[string]any{"event": "term.attention", "title": "x"}, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-team ingest must be 403, got %d", w.Code)
	}
}
