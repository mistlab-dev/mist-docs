package mcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeStore is an in-memory Store for protocol-level tests.
type fakeStore struct {
	docs   map[string][]DocContent // teamID -> docs
	member map[string]bool         // "team:user"
}

func newFake() *fakeStore {
	return &fakeStore{
		docs: map[string][]DocContent{
			"t1": {
				{ID: "d1", Title: "Redis 故障应急", Content: "# Redis 故障应急\n1. 检查主从延迟", Version: 3, UpdatedAt: "2026-10-16 10:00:00"},
				{ID: "d2", Title: "发布回滚流程", Content: "# 发布回滚\n helm rollback", Version: 1, UpdatedAt: "2026-10-15 09:00:00"},
			},
		},
		member: map[string]bool{"t1:u1": true},
	}
}

func (f *fakeStore) IsMember(teamID, userID string) (bool, error) {
	return f.member[teamID+":"+userID], nil
}

func (f *fakeStore) SearchDocuments(teamID, keyword string, limit int) ([]DocHit, error) {
	var out []DocHit
	for _, d := range f.docs[teamID] {
		if strings.Contains(d.Title, keyword) || strings.Contains(d.Content, keyword) {
			out = append(out, DocHit{ID: d.ID, Title: d.Title, Excerpt: d.Content})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeStore) RecentDocuments(teamID string, limit int) ([]DocMeta, error) {
	var out []DocMeta
	for _, d := range f.docs[teamID] {
		out = append(out, DocMeta{ID: d.ID, Title: d.Title, UpdatedAt: d.UpdatedAt})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) ReadDocument(teamID, docID string) (*DocContent, bool, error) {
	for _, d := range f.docs[teamID] {
		if d.ID == docID {
			cp := d
			return &cp, true, nil
		}
	}
	return nil, false, nil
}

func (f *fakeStore) CreateDocument(teamID, title, folderID, content, userID string) (string, error) {
	id := "new-" + title
	f.docs[teamID] = append(f.docs[teamID], DocContent{ID: id, Title: title, Content: content})
	return id, nil
}

func (f *fakeStore) CanRead(teamID, docID, userID string) (bool, error) {
	ok, _ := f.IsMember(teamID, userID)
	if !ok {
		return false, nil
	}
	_, found, _ := f.ReadDocument(teamID, docID)
	return found, nil
}

// errStore fails membership checks to exercise the error path.
type errStore struct{ fakeStore }

func (errStore) IsMember(teamID, userID string) (bool, error) { return false, errors.New("db down") }

func newServer(s Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Stand-in for middleware.JWTAuth: inject a known user.
	r.Use(func(c *gin.Context) { c.Set("user_id", "u1") })
	r.POST("/api/mcp", New(s).Serve)
	return r
}

func post(t *testing.T, r *gin.Engine, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, w.Body.String())
	}
	return w.Code, resp
}

func TestInitialize(t *testing.T) {
	r := newServer(newFake())
	code, resp := post(t, r, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	result, _ := resp["result"].(map[string]any)
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("protocolVersion = %v", result["protocolVersion"])
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != ServerName {
		t.Fatalf("serverInfo.name = %v", info["name"])
	}
}

func TestToolsList(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	result, _ := resp["result"].(map[string]any)
	toolsList, _ := result["tools"].([]any)
	if len(toolsList) != 3 {
		t.Fatalf("tools = %d, want 3", len(toolsList))
	}
	names := map[string]bool{}
	for _, raw := range toolsList {
		tool := raw.(map[string]any)
		names[tool["name"].(string)] = true
		// Every tool must require team_id — that is the isolation contract.
		schema := tool["inputSchema"].(map[string]any)
		req, _ := schema["required"].([]any)
		found := false
		for _, r := range req {
			if r == "team_id" {
				found = true
			}
		}
		if !found {
			t.Errorf("tool %v does not require team_id", tool["name"])
		}
	}
	for _, want := range []string{"search_docs", "read_doc", "create_doc"} {
		if !names[want] {
			t.Errorf("missing tool %s", want)
		}
	}
}

func TestToolSearch(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_docs","arguments":{"team_id":"t1","query":"Redis"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("unexpected error: %v", result)
	}
	content, _ := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "d1") || !strings.Contains(text, "Redis 故障应急") {
		t.Fatalf("search text = %q", text)
	}
}

func TestToolSearchRejectsNonMember(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search_docs","arguments":{"team_id":"t9","query":"Redis"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError for non-member team, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "not a member") {
		t.Fatalf("error text = %q", text)
	}
}

func TestToolRead(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_doc","arguments":{"team_id":"t1","doc_id":"d1"}}}`)
	result, _ := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "# Redis 故障应急") || !strings.Contains(text, "主从延迟") {
		t.Fatalf("read text = %q", text)
	}
}

func TestToolReadMissingDoc(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"read_doc","arguments":{"team_id":"t1","doc_id":"nope"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError for missing doc, got %v", result)
	}
}

func TestToolCreate(t *testing.T) {
	f := newFake()
	r := newServer(f)
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"create_doc","arguments":{"team_id":"t1","title":"故障复盘","content":"## 结论\n无"}}}`)
	result, _ := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "new-故障复盘") {
		t.Fatalf("create text = %q", text)
	}
	if len(f.docs["t1"]) != 3 {
		t.Fatalf("docs = %d, want 3", len(f.docs["t1"]))
	}
}

func TestCreateRequiresTitle(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"create_doc","arguments":{"team_id":"t1"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError without title, got %v", result)
	}
}

func TestUnknownMethod(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":9,"method":"resources/subscribe","params":{}}`)
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("expected error, got %v", resp)
	}
	if int(errObj["code"].(float64)) != -32601 {
		t.Fatalf("code = %v, want -32601", errObj["code"])
	}
}

func TestUnknownTool(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"drop_all","arguments":{"team_id":"t1"}}}`)
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("expected error, got %v", resp)
	}
}

func TestMissingTeamID(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"search_docs","arguments":{"query":"x"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError without team_id, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "team_id is required") {
		t.Fatalf("text = %q", text)
	}
}

func TestMembershipErrorPropagates(t *testing.T) {
	r := newServer(&errStore{})
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"search_docs","arguments":{"team_id":"t1","query":"x"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError when membership check fails, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "membership check failed") {
		t.Fatalf("text = %q", text)
	}
}

func TestNotificationGetsNoBody(t *testing.T) {
	r := newServer(newFake())
	req := httptest.NewRequest(http.MethodPost, "/api/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.Len() != 0 {
		t.Fatalf("notification body = %q, want empty", w.Body.String())
	}
}

func TestResourcesReadRecent(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":13,"method":"resources/read","params":{"uri":"docs://t1/recent"}}`)
	result, _ := resp["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents = %v", contents)
	}
	text := contents[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "Redis 故障应急") {
		t.Fatalf("resource text = %q", text)
	}
}

func TestResourcesReadRejectsNonMember(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":14,"method":"resources/read","params":{"uri":"docs://t9/recent"}}`)
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("expected error for non-member, got %v", resp)
	}
}

func TestInvalidJSONRPCVersion(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"1.0","id":15,"method":"ping"}`)
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("expected error for jsonrpc 1.0, got %v", resp)
	}
	if int(errObj["code"].(float64)) != -32600 {
		t.Fatalf("code = %v, want -32600", errObj["code"])
	}
}
