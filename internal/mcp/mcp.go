// Package mcp implements a Model Context Protocol (MCP) server so AI agents
// (Claude Desktop, Cursor, ChatGPT connectors, and MistTerm's ops assistant)
// can read and search this team's knowledge base with the caller's own
// credentials.
//
// Transport: a single POST /api/mcp endpoint speaking JSON-RPC 2.0 over HTTP
// (MCP's Streamable HTTP transport without server-initiated streaming, which
// every current client also supports). There is no separate session store —
// each request is self-contained and re-authenticated.
//
// Authorization reuses the existing Portal JWT and team-membership model:
// every tool call carries an explicit team_id that is checked against
// team_members, so an agent can never read a team its token does not belong
// to. Nothing here bypasses requireDoc-level checks; read_doc re-verifies
// document permission through the same service the REST API uses.
package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProtocolVersion is the MCP protocol revision this server implements.
const ProtocolVersion = "2025-06-18"

// ServerName is reported in initialize responses.
const ServerName = "mistdocs"

// Version is the server implementation version reported to clients.
const Version = "1.0.0"

// maxToolNameLen and maxArgLen bound request size before any processing so a
// hostile client cannot make us allocate unbounded buffers.
const (
	maxToolNameLen = 64
	maxArgLen      = 8192
	maxQueryLen    = 512
	maxDocIDLen    = 128
)

// Store is the data access surface the MCP server needs. It is satisfied by
// the shared database layer and stubbed in tests.
type Store interface {
	// SearchDocuments returns up to limit documents of teamID whose title or
	// body contains keyword and that userID is permitted to read, newest first.
	SearchDocuments(teamID, userID, keyword string, limit int) ([]DocHit, error)
	// RecentDocuments returns up to limit recently updated documents that userID
	// is permitted to read.
	RecentDocuments(teamID, userID string, limit int) ([]DocMeta, error)
	// ReadDocument returns the document body plus metadata. The second result
	// is false when the document does not exist, is soft-deleted, or belongs
	// to another team.
	ReadDocument(teamID, docID string) (*DocContent, bool, error)
	// CreateDocument inserts a new document and returns its id.
	CreateDocument(teamID, title, folderID, content, userID string) (string, error)
	// CanRead reports whether userID may read docID inside teamID.
	CanRead(teamID, docID, userID string) (bool, error)
	// CanCreate reports whether userID is allowed to create documents in teamID
	// (must have editor/admin role and not exceed plan document limit).
	CanCreate(teamID, userID string) (bool, string, error)
	// IsMember reports whether userID belongs to teamID.
	IsMember(teamID, userID string) (bool, error)
}

// DocHit is one search result.
type DocHit struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt,omitempty"`
}

// DocMeta is document metadata without the body.
type DocMeta struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// DocContent is a full document.
type DocContent struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Type      string `json:"type,omitempty"`
	Version   int    `json:"version,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Handler serves the MCP endpoint.
type Handler struct {
	store Store
}

// New builds a Handler.
func New(store Store) *Handler { return &Handler{store: store} }

// jsonrpcRequest is a single JSON-RPC 2.0 request. Notifications (no id) get
// no response body.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Serve handles POST /api/mcp.
//
// Authentication: middleware.JWTAuth() must run first and set "user_id".
// Team scope is taken from each tool's arguments and verified per call.
func (h *Handler) Serve(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, jsonrpcError{Code: -32001, Message: "unauthorized"})
		return
	}

	var req jsonrpcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, jsonrpcError{Code: -32700, Message: "parse error: " + err.Error()})
		return
	}
	if req.JSONRPC != "2.0" {
		writeError(c, req.ID, -32600, "invalid request: jsonrpc must be \"2.0\"")
		return
	}

	// Notifications expect no reply at all.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		writeResult(c, req.ID, h.initialize())
	case "notifications/initialized", "notifications/cancelled":
		// Acknowledge silently; nothing to do.
		if isNotification {
			c.Status(http.StatusAccepted)
			return
		}
		writeResult(c, req.ID, map[string]any{})
	case "ping":
		writeResult(c, req.ID, map[string]any{})
	case "tools/list":
		writeResult(c, req.ID, map[string]any{"tools": tools()})
	case "tools/call":
		h.toolsCall(c, req, userID)
	case "resources/list":
		writeResult(c, req.ID, map[string]any{"resources": resources()})
	case "resources/read":
		h.resourcesRead(c, req, userID)
	default:
		writeError(c, req.ID, -32601, "method not found: "+req.Method)
	}
}

func (h *Handler) initialize() map[string]any {
	return map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities": map[string]any{
			"tools":    map[string]any{},
			"resources": map[string]any{},
		},
		"serverInfo": map[string]any{"name": ServerName, "version": Version},
		"instructions": "MistDocs team knowledge base. All tools require an explicit team_id " +
			"and are scoped to teams your credentials belong to. Search first, then read.",
	}
}

// toolsCall dispatches a tools/call request.
func (h *Handler) toolsCall(c *gin.Context, req jsonrpcRequest, userID string) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeError(c, req.ID, -32602, "invalid params: "+err.Error())
		return
	}
	if len(params.Name) == 0 || len(params.Name) > maxToolNameLen {
		writeError(c, req.ID, -32602, "invalid params: missing or oversized tool name")
		return
	}
	args := map[string]any{}
	if len(params.Arguments) > 0 && string(params.Arguments) != "null" {
		if len(params.Arguments) > maxArgLen {
			writeError(c, req.ID, -32602, "invalid params: arguments too large")
			return
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			writeError(c, req.ID, -32602, "invalid params: arguments: "+err.Error())
			return
		}
	}

	// Every tool is team-scoped; reject before touching the database if the
	// caller is not a member of the requested team.
	teamID, _ := args["team_id"].(string)
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		writeToolError(c, req.ID, "team_id is required")
		return
	}
	if len(teamID) > maxDocIDLen {
		writeToolError(c, req.ID, "team_id is too long")
		return
	}
	ok, err := h.store.IsMember(teamID, userID)
	if err != nil {
		writeToolError(c, req.ID, "membership check failed: "+err.Error())
		return
	}
	if !ok {
		writeToolError(c, req.ID, "not a member of team "+teamID)
		return
	}

	switch params.Name {
	case "search_docs":
		h.toolSearch(c, req.ID, teamID, userID, args)
	case "read_doc":
		h.toolRead(c, req.ID, teamID, userID, args)
	case "create_doc":
		h.toolCreate(c, req.ID, teamID, userID, args)
	default:
		writeError(c, req.ID, -32602, "unknown tool: "+params.Name)
	}
}

func (h *Handler) toolSearch(c *gin.Context, id json.RawMessage, teamID, userID string, args map[string]any) {
	query, _ := args["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		writeToolError(c, id, "query is required")
		return
	}
	if len(query) > maxQueryLen {
		query = query[:maxQueryLen]
	}
	limit := 10
	if raw, ok := args["limit"]; ok {
		if n, err := toInt(raw); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	hits, err := h.store.SearchDocuments(teamID, userID, query, limit)
	if err != nil {
		writeToolError(c, id, "search failed: "+err.Error())
		return
	}
	if len(hits) == 0 {
		writeText(c, id, "No documents matched "+strconv.Quote(query)+".")
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d document(s) matched %s:\n", len(hits), strconv.Quote(query))
	for _, d := range hits {
		fmt.Fprintf(&b, "\n- %s (id: %s)", d.Title, d.ID)
		if d.Excerpt != "" {
			fmt.Fprintf(&b, "\n  %s", oneLine(d.Excerpt))
		}
	}
	b.WriteString("\n\nUse read_doc with an id above to fetch full content.")
	writeText(c, id, b.String())
}

func (h *Handler) toolRead(c *gin.Context, id json.RawMessage, teamID, userID string, args map[string]any) {
	docID, _ := args["doc_id"].(string)
	docID = strings.TrimSpace(docID)
	if docID == "" {
		writeToolError(c, id, "doc_id is required")
		return
	}
	if len(docID) > maxDocIDLen {
		writeToolError(c, id, "doc_id is too long")
		return
	}
	// Same permission check the REST read path uses, so MCP grants nothing extra.
	if can, err := h.store.CanRead(teamID, docID, userID); err != nil {
		writeToolError(c, id, "permission check failed: "+err.Error())
		return
	} else if !can {
		writeToolError(c, id, "document not found or not readable")
		return
	}

	doc, found, err := h.store.ReadDocument(teamID, docID)
	if err != nil {
		writeToolError(c, id, "read failed: "+err.Error())
		return
	}
	if !found {
		writeToolError(c, id, "document not found")
		return
	}
	writeText(c, id, fmt.Sprintf("# %s\n\n(doc_id: %s, version %d, updated %s)\n\n%s",
		doc.Title, doc.ID, doc.Version, doc.UpdatedAt, doc.Content))
}

func (h *Handler) toolCreate(c *gin.Context, id json.RawMessage, teamID, userID string, args map[string]any) {
	can, reason, err := h.store.CanCreate(teamID, userID)
	if err != nil {
		writeToolError(c, id, "permission check failed: "+err.Error())
		return
	}
	if !can {
		if reason == "" {
			reason = "editor or admin role required to create documents"
		}
		writeToolError(c, id, reason)
		return
	}

	title, _ := args["title"].(string)
	title = strings.TrimSpace(title)
	if title == "" {
		writeToolError(c, id, "title is required")
		return
	}
	if len([]rune(title)) > 200 {
		writeToolError(c, id, "title must be at most 200 characters")
		return
	}
	content, _ := args["content"].(string)
	folderID, _ := args["folder_id"].(string)
	folderID = strings.TrimSpace(folderID)
	if len(folderID) > maxDocIDLen {
		writeToolError(c, id, "folder_id is too long")
		return
	}

	docID, err := h.store.CreateDocument(teamID, title, folderID, content, userID)
	if err != nil {
		writeToolError(c, id, "create failed: "+err.Error())
		return
	}
	writeText(c, id, fmt.Sprintf("Created document %q (id: %s) in team %s.", title, docID, teamID))
}

// resourcesRead serves docs://recent.
func (h *Handler) resourcesRead(c *gin.Context, req jsonrpcRequest, userID string) {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeError(c, req.ID, -32602, "invalid params: "+err.Error())
		return
	}
	// The URI carries the team: docs://{team_id}/recent
	teamID := ""
	if strings.HasPrefix(params.URI, "docs://") {
		rest := strings.TrimPrefix(params.URI, "docs://")
		if i := strings.Index(rest, "/"); i >= 0 {
			teamID = rest[:i]
		}
	}
	if teamID == "" {
		writeError(c, req.ID, -32602, "uri must look like docs://{team_id}/recent")
		return
	}
	ok, err := h.store.IsMember(teamID, userID)
	if err != nil {
		writeError(c, req.ID, -32002, "membership check failed: "+err.Error())
		return
	}
	if !ok {
		writeError(c, req.ID, -32001, "not a member of team "+teamID)
		return
	}
	docs, err := h.store.RecentDocuments(teamID, userID, 20)
	if err != nil {
		writeError(c, req.ID, -32002, "read failed: "+err.Error())
		return
	}
	body, _ := json.MarshalIndent(docs, "", "  ")
	writeResult(c, req.ID, map[string]any{
		"contents": []map[string]any{{
			"uri":     params.URI,
			"mimeType": "application/json",
			"text":    string(body),
		}},
	})
}

// --- protocol helpers ---

func writeResult(c *gin.Context, id json.RawMessage, result any) {
	if len(id) == 0 || string(id) == "null" {
		c.Status(http.StatusAccepted)
		return
	}
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": id, "result": result})
}

func writeError(c *gin.Context, id json.RawMessage, code int, msg string) {
	if len(id) == 0 || string(id) == "null" {
		c.Status(http.StatusAccepted)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0", "id": id,
		"error": jsonrpcError{Code: code, Message: msg},
	})
}

// writeToolError reports a tool execution failure the way MCP expects: a
// successful JSON-RPC response whose result carries isError, so the model sees
// the failure text instead of a protocol error it cannot interpret.
func writeToolError(c *gin.Context, id json.RawMessage, msg string) {
	writeResult(c, id, map[string]any{
		"content": []map[string]any{{"type": "text", "text": "Error: " + msg}},
		"isError": true,
	})
}

func writeText(c *gin.Context, id json.RawMessage, text string) {
	writeResult(c, id, map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	})
}

func toInt(v any) (int, error) {
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case json.Number:
		return strconv.Atoi(string(t))
	case string:
		return strconv.Atoi(t)
	}
	return 0, fmt.Errorf("not a number")
}

// oneLine collapses newlines so list items stay on one line.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// tools declares the MCP tool surface. Keep descriptions explicit about the
// team_id requirement — agents read them verbatim.
func tools() []map[string]any {
	strProp := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	teamProp := strProp("Team id to scope the call to. Must be a team your credentials belong to.")
	required := func(names ...string) []string { return names }

	return []map[string]any{
		{
			"name":        "search_docs",
			"description": "Search this team's documents by keyword (matches title and body). Returns titles, ids and excerpts. Call this first to locate a runbook, then read_doc for full content.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{
					"team_id": teamProp,
					"query":   strProp("Search keywords. Up to 512 characters."),
					"limit":   map[string]any{"type": "integer", "description": "Max results (1-50, default 10)."},
				},
				"required": required("team_id", "query"),
			},
		},
		{
			"name":        "read_doc",
			"description": "Fetch one document's full content by id, including title and version. Returns 404-style errors for documents you cannot read.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{
					"team_id": teamProp,
					"doc_id":  strProp("Document id returned by search_docs or the resources list."),
				},
				"required": required("team_id", "doc_id"),
			},
		},
		{
			"name":        "create_doc",
			"description": "Create a new document in the team, e.g. to file an incident write-up or an operational note after a change. Requires editor or admin role. Returns the new document id.",
			"inputSchema": map[string]any{
				"type":     "object",
				"properties": map[string]any{
					"team_id":   teamProp,
					"title":     strProp("Document title, up to 200 characters."),
					"content":   strProp("Document body. Markdown is fine."),
					"folder_id": strProp("Optional folder id. Omit to place at the team root."),
				},
				"required": required("team_id", "title"),
			},
		},
	}
}

// resources declares readable resources.
func resources() []map[string]any {
	return []map[string]any{
		{
			"uri":         "docs://{team_id}/recent",
			"name":        "Recently updated documents",
			"description": "Up to 20 most recently updated documents of a team, as JSON.",
			"mimeType":    "application/json",
		},
	}
}
