package tests

// #78 T3: unified cross-product search (docs + term always, audits gated).

import (
	"encoding/json"
	"net/http"
	"testing"
)

// createSearchAgent: a dedicated agent for the search tests (#78). Ingest
// tests keep their own helper (createIngestAgent in event_ingest_test.go,
// landed with #83); the two differ only in name, but separate helpers keep
// each suite's fixture intent explicit.
func createSearchAgent(t *testing.T) string {
	t.Helper()
	w := request("POST", teamPath("/agents"),
		map[string]any{"name": "search-bot", "tools": []string{"search_docs", "read_doc"}}, adminToken)
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

func unifiedSearch(t *testing.T, token, q string) (int, map[string]any) {
	t.Helper()
	w := request("GET", teamPath("/search/all?q="+q), nil, token)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func sectionIDs(t *testing.T, res map[string]any, key string) []any {
	t.Helper()
	sec, ok := res[key].(map[string]any)
	if !ok {
		return nil
	}
	items, _ := sec["items"].([]any)
	return items
}

// docs + term 两个分区恒在；关键词同时命中文档与汇入事件（验收：两种来源）。
func TestUnifiedSearchHitsDocsAndTerm(t *testing.T) {
	// seed: one doc + one ingested term event sharing a unique keyword.
	seedAgentDoc(t, "Unified needle doc", "<p>unifiedneedle content</p>")
	token := createSearchAgent(t)
	w := request("POST", teamPath("/events/ingest"),
		map[string]any{"event": "term.attention", "title": "unifiedneedle in terminal", "source_session": "s-9"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}

	code, res := unifiedSearch(t, adminToken, "unifiedneedle")
	if code != http.StatusOK {
		t.Fatalf("search: %d", code)
	}
	docs := sectionIDs(t, res, "docs")
	term := sectionIDs(t, res, "term")
	if len(docs) == 0 {
		t.Fatalf("docs section empty: %v", res)
	}
	if len(term) == 0 {
		t.Fatalf("term section empty: %v", res)
	}
}

// 空查询返回空分区而不是错误。
func TestUnifiedSearchEmptyQuery(t *testing.T) {
	code, res := unifiedSearch(t, adminToken, "")
	if code != http.StatusOK {
		t.Fatalf("empty q: %d", code)
	}
	if _, has := res["error"]; has {
		t.Fatalf("empty q must not error: %v", res)
	}
}

// viewer 看不到 audits 分区（无权限的源不返回，而不是空数组）。
func TestUnifiedSearchAuditsOmittedForViewer(t *testing.T) {
	code, res := unifiedSearch(t, viewerToken, "anything")
	if code != http.StatusOK {
		t.Fatalf("viewer search: %d", code)
	}
	if _, has := res["audits"]; has {
		t.Fatalf("audits section must be omitted for viewer: %v", res["audits"])
	}
	// docs/term 仍可用
	if _, has := res["docs"]; !has {
		t.Fatal("docs section must exist for viewer")
	}
}
