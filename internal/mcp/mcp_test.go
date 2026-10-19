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
	locked map[string]string       // docID -> locker user id
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
		locked: map[string]string{},
	}
}

func (f *fakeStore) IsMember(teamID, userID string) (bool, error) {
	return f.member[teamID+":"+userID], nil
}

func (f *fakeStore) SearchDocuments(teamID, userID, keyword string, limit int) ([]DocHit, error) {
	var out []DocHit
	for _, d := range f.docs[teamID] {
		can, _ := f.CanRead(teamID, d.ID, userID)
		if !can {
			continue
		}
		if strings.Contains(d.Title, keyword) || strings.Contains(d.Content, keyword) {
			out = append(out, DocHit{ID: d.ID, Title: d.Title, Excerpt: d.Content})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeStore) RecentDocuments(teamID, userID string, limit int) ([]DocMeta, error) {
	var out []DocMeta
	for _, d := range f.docs[teamID] {
		can, _ := f.CanRead(teamID, d.ID, userID)
		if !can {
			continue
		}
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

func (f *fakeStore) CanCreate(teamID, userID string) (bool, string, error) {
	ok, _ := f.IsMember(teamID, userID)
	if !ok {
		return false, "not a member", nil
	}
	if userID == "viewer_user" {
		return false, "editor or admin role required to create documents", nil
	}
	return true, "", nil
}

func (f *fakeStore) CanRead(teamID, docID, userID string) (bool, error) {
	ok, _ := f.IsMember(teamID, userID)
	if !ok {
		return false, nil
	}
	if docID == "secret_doc" && userID != "admin_user" {
		return false, nil
	}
	_, found, _ := f.ReadDocument(teamID, docID)
	return found, nil
}

func (f *fakeStore) CanWrite(teamID, docID, userID string) (bool, string, error) {
	if userID == "viewer_user" {
		return false, "write permission required (owner/admin/editor role, or an explicit write grant)", nil
	}
	ok, err := f.CanRead(teamID, docID, userID)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "document not found or not writable", nil
	}
	return true, "", nil
}

func (f *fakeStore) UpdateDocument(teamID, docID, title, content string, hasTitle, hasContent bool, userID string) (int, error) {
	if by, ok := f.locked[docID]; ok && by != userID {
		return 0, errors.New("document is locked by another user")
	}
	for i := range f.docs[teamID] {
		d := &f.docs[teamID][i]
		if d.ID != docID {
			continue
		}
		if hasTitle {
			d.Title = title
		}
		if hasContent {
			d.Content = content
			d.Version++
		}
		return d.Version, nil
	}
	return 0, errors.New("document not found")
}

func (f *fakeStore) IsLockConflict(err error) bool {
	return err != nil && err.Error() == "document is locked by another user"
}

func (f *fakeStore) ListDocuments(teamID, userID, folderID, docType, tag string, limit, offset int) ([]DocMeta, int, error) {
	var out []DocMeta
	total := 0
	for _, d := range f.docs[teamID] {
		can, _ := f.CanRead(teamID, d.ID, userID)
		if !can {
			continue
		}
		total++
		if folderID != "" || docType != "" || tag != "" {
			continue // filters are exercised in service-level tests
		}
		if offset > 0 {
			offset--
			continue
		}
		if len(out) < limit {
			out = append(out, DocMeta{ID: d.ID, Title: d.Title, Type: d.Type, UpdatedAt: d.UpdatedAt})
		}
	}
	return out, total, nil
}

func (f *fakeStore) SearchDocContent(teamID, userID, keyword string, limit int) ([]DocHit, error) {
	var out []DocHit
	for _, d := range f.docs[teamID] {
		can, _ := f.CanRead(teamID, d.ID, userID)
		if !can {
			continue
		}
		if strings.Contains(d.Title, keyword) || strings.Contains(d.Content, keyword) {
			out = append(out, DocHit{ID: d.ID, Title: d.Title, Excerpt: d.Content})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
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
	if len(toolsList) != 6 {
		t.Fatalf("tools = %d, want 6", len(toolsList))
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
	for _, want := range []string{"search_docs", "read_doc", "create_doc", "update_doc", "list_docs", "search_doc_content"} {
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

func TestCreateDocRejectsViewer(t *testing.T) {
	f := newFake()
	f.member["t1:viewer_user"] = true
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "viewer_user") })
	r.POST("/api/mcp", New(f).Serve)

	_, resp := post(t, r, `{"jsonrpc":"2.0","id":101,"method":"tools/call","params":{"name":"create_doc","arguments":{"team_id":"t1","title":"未授权创建"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError for viewer creating doc, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "editor or admin role required") {
		t.Fatalf("expected role error, got text = %q", text)
	}
}

func TestSearchDocsFiltersUnreadable(t *testing.T) {
	f := newFake()
	f.docs["t1"] = append(f.docs["t1"], DocContent{
		ID:      "secret_doc",
		Title:   "机密运维指南",
		Content: "超级密码",
	})
	r := newServer(f) // user_id is u1, cannot read secret_doc

	_, resp := post(t, r, `{"jsonrpc":"2.0","id":102,"method":"tools/call","params":{"name":"search_docs","arguments":{"team_id":"t1","query":"机密"}}}`)
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if strings.Contains(text, "secret_doc") || strings.Contains(text, "机密运维指南") {
		t.Fatalf("search_docs leaked secret_doc: %q", text)
	}
	if !strings.Contains(text, "No documents matched") {
		t.Fatalf("expected no matched documents, got %q", text)
	}
}

// --- update_doc ---

func TestToolUpdateContent(t *testing.T) {
	f := newFake()
	r := newServer(f)
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":201,"method":"tools/call","params":{"name":"update_doc","arguments":{"team_id":"t1","doc_id":"d1","content":"# 新正文"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("unexpected error: %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "content") || !strings.Contains(text, "d1") {
		t.Fatalf("update text = %q", text)
	}
	doc, _, _ := f.ReadDocument("t1", "d1")
	if doc.Content != "# 新正文" {
		t.Fatalf("content = %q, want %q", doc.Content, "# 新正文")
	}
	if doc.Version != 4 {
		t.Fatalf("version = %d, want 4", doc.Version)
	}
}

func TestToolUpdateTitleOnly(t *testing.T) {
	f := newFake()
	r := newServer(f)
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":202,"method":"tools/call","params":{"name":"update_doc","arguments":{"team_id":"t1","doc_id":"d1","title":"新标题"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("unexpected error: %v", result)
	}
	doc, _, _ := f.ReadDocument("t1", "d1")
	if doc.Title != "新标题" {
		t.Fatalf("title = %q", doc.Title)
	}
	if doc.Content == "# 新正文" {
		t.Fatal("title-only update must not touch content")
	}
	if doc.Version != 3 {
		t.Fatalf("version = %d, want unchanged 3", doc.Version)
	}
}

func TestToolUpdateRequiresSomething(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":203,"method":"tools/call","params":{"name":"update_doc","arguments":{"team_id":"t1","doc_id":"d1"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError without title/content, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "at least one of title or content") {
		t.Fatalf("text = %q", text)
	}
}

func TestToolUpdateRejectsViewer(t *testing.T) {
	f := newFake()
	f.member["t1:viewer_user"] = true
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "viewer_user") })
	r.POST("/api/mcp", New(f).Serve)

	_, resp := post(t, r, `{"jsonrpc":"2.0","id":204,"method":"tools/call","params":{"name":"update_doc","arguments":{"team_id":"t1","doc_id":"d1","content":"越权"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError for viewer updating doc, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "write permission required") {
		t.Fatalf("expected write permission error, got text = %q", text)
	}
	// Viewer must not have changed anything.
	doc, _, _ := f.ReadDocument("t1", "d1")
	if doc.Content == "越权" {
		t.Fatal("viewer mutated document content")
	}
}

func TestToolUpdateMissingDoc(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":205,"method":"tools/call","params":{"name":"update_doc","arguments":{"team_id":"t1","doc_id":"nope","content":"x"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError for missing doc, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "not found") {
		t.Fatalf("text = %q", text)
	}
}

func TestToolUpdateLockedDoc(t *testing.T) {
	f := newFake()
	// Simulate the store reporting a lock held by someone else.
	f.locked = map[string]string{"d1": "someone_else"}
	r := newServer(f)
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":206,"method":"tools/call","params":{"name":"update_doc","arguments":{"team_id":"t1","doc_id":"d1","content":"x"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError for locked doc, got %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "locked by another user") {
		t.Fatalf("text = %q", text)
	}
}

// --- list_docs ---

func TestToolList(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":211,"method":"tools/call","params":{"name":"list_docs","arguments":{"team_id":"t1"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("unexpected error: %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "Redis 故障应急") || !strings.Contains(text, "d1") {
		t.Fatalf("list text = %q", text)
	}
	if !strings.Contains(text, "2 of 2") {
		t.Fatalf("expected total count, got %q", text)
	}
}

func TestToolListFiltersUnreadable(t *testing.T) {
	f := newFake()
	f.docs["t1"] = append(f.docs["t1"], DocContent{
		ID: "secret_doc", Title: "机密运维指南", Content: "超级密码",
	})
	r := newServer(f) // u1 cannot read secret_doc
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":212,"method":"tools/call","params":{"name":"list_docs","arguments":{"team_id":"t1"}}}`)
	result, _ := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if strings.Contains(text, "secret_doc") || strings.Contains(text, "机密运维指南") {
		t.Fatalf("list_docs leaked secret_doc: %q", text)
	}
}

func TestToolListPaging(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":213,"method":"tools/call","params":{"name":"list_docs","arguments":{"team_id":"t1","limit":1,"page":2}}}`)
	result, _ := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "page 2") {
		t.Fatalf("list text = %q", text)
	}
	if !strings.Contains(text, "发布回滚流程") {
		t.Fatalf("expected second doc on page 2, got %q", text)
	}
}

// --- search_doc_content ---

func TestToolSearchContent(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":221,"method":"tools/call","params":{"name":"search_doc_content","arguments":{"team_id":"t1","query":"rollback"}}}`)
	result, _ := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("unexpected error: %v", result)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "发布回滚流程") || !strings.Contains(text, "d2") {
		t.Fatalf("search_doc_content text = %q", text)
	}
}

func TestToolSearchContentNoMatch(t *testing.T) {
	r := newServer(newFake())
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":222,"method":"tools/call","params":{"name":"search_doc_content","arguments":{"team_id":"t1","query":"不存在的关键词"}}}`)
	result, _ := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "No paragraphs matched") {
		t.Fatalf("text = %q", text)
	}
}

func TestToolSearchContentFiltersUnreadable(t *testing.T) {
	f := newFake()
	f.docs["t1"] = append(f.docs["t1"], DocContent{
		ID: "secret_doc", Title: "机密运维指南", Content: "超级密码",
	})
	r := newServer(f)
	_, resp := post(t, r, `{"jsonrpc":"2.0","id":223,"method":"tools/call","params":{"name":"search_doc_content","arguments":{"team_id":"t1","query":"机密"}}}`)
	result, _ := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if strings.Contains(text, "secret_doc") || strings.Contains(text, "机密运维指南") {
		t.Fatalf("search_doc_content leaked secret_doc: %q", text)
	}
}
