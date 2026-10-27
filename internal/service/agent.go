package service

// Agent 身份与最小权限（#76，落地 #68 设计）。
//
// agent 是独立 principal（agent:<id>），挂在团队下，不引入登录：
// 只以 `Authorization: Bearer <agent token>` 调 MCP 单一入口。
// token = 随机 32 字节，SHA-256 哈希入库，可轮换（旧行 revoked + 新行）。

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/google/uuid"
)

// Agent is one row of md_agents.
type Agent struct {
	ID            string     `json:"id"`
	TeamID        string     `json:"team_id"`
	Name          string     `json:"name"`
	CreatorUserID string     `json:"creator_user_id"`
	Scopes        AgentScopes `json:"scopes"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// AgentScopes is the whitelist model from #68 §2.
type AgentScopes struct {
	// Tools: whitelist; unlisted tools are denied. create_doc/update_doc are
	// never grantable in T6 (design §5: default deny; approval flow later).
	Tools []string `json:"tools"`
	// Docs: empty = team semantics via HasTeamPermission; non-empty = only
	// the listed doc IDs (checked before HasTeamPermission).
	Docs []string `json:"docs,omitempty"`
}

// AgentScopesJSON marshals scopes for the md_agents.scopes column.
func AgentScopesJSON(s AgentScopes) (string, error) {
	if len(s.Tools) == 0 {
		return "", fmt.Errorf("agent scopes: tools whitelist must not be empty")
	}
	for _, t := range s.Tools {
		if !agentToolGrantable(t) {
			return "", fmt.Errorf("agent scopes: tool %q is not grantable to agents", t)
		}
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// agentToolGrantable reports whether a tool may appear in an agent scope.
// Write tools stay deny-by-default in T6 (design §5).
func agentToolGrantable(tool string) bool {
	switch tool {
	case "search_docs", "read_doc", "list_docs", "search_doc_content":
		return true
	default:
		return false
	}
}

// clampAgentExpiry enforces design §4: default 30 days out, a requested
// expiry may only shorten, never extend; past dates are invalid.
func clampAgentExpiry(expiresAt *time.Time) *time.Time {
	def := time.Now().AddDate(0, 0, 30).UTC().Truncate(time.Second)
	if expiresAt == nil || expiresAt.After(def) || expiresAt.IsZero() {
		return &def
	}
	return expiresAt
}

// agentTokenGenerate returns (plaintext, sha256hex). Plaintext is shown once.
func agentTokenGenerate() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	plain := "agt_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	return plain, hex.EncodeToString(sum[:]), nil
}

// agentTokenHash is the stored form of a token.
func agentTokenHash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// CreateAgent issues a new agent + token. expiresAt empty = 30 days out
// (design §4: may shorten, never extend).
func CreateAgent(ctx context.Context, teamID, name, creatorUserID string, scopes AgentScopes, expiresAt *time.Time) (*Agent, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return nil, "", fmt.Errorf("agent name must be 1..64 chars")
	}
	if _, err := AgentScopesJSON(scopes); err != nil {
		return nil, "", err
	}
	expiresAt = clampAgentExpiry(expiresAt)
	if expiresAt.Before(time.Now()) {
		return nil, "", fmt.Errorf("agent expires_at is in the past")
	}
	plain, hash, err := agentTokenGenerate()
	if err != nil {
		return nil, "", err
	}
	id := uuid.New().String()
	scopesJSON, _ := json.Marshal(scopes)
	_, err = database.DB.ExecContext(ctx,
		`INSERT INTO md_agents (id, team_id, name, creator_user_id, token_hash, scopes, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`,
		id, teamID, name, creatorUserID, hash, string(scopesJSON), expiresAt)
	if err != nil {
		return nil, "", fmt.Errorf("create agent: %w", err)
	}
	a := &Agent{ID: id, TeamID: teamID, Name: name, CreatorUserID: creatorUserID, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: time.Now()}
	return a, plain, nil
}

// AgentAuthResult is what AgentAuthMiddleware needs per request.
type AgentAuthResult struct {
	Agent  Agent
	Scopes AgentScopes
}

// AuthenticateAgent resolves a bearer token to a live agent row.
// revoked_at non-null or expires_at past => invalid (design §4).
func AuthenticateAgent(ctx context.Context, bearer string) (*AgentAuthResult, error) {
	bearer = strings.TrimSpace(strings.TrimPrefix(bearer, "Bearer "))
	if !strings.HasPrefix(bearer, "agt_") {
		return nil, fmt.Errorf("not an agent token")
	}
	var a Agent
	var scopesJSON string
	err := database.DB.QueryRowContext(ctx,
		`SELECT id, team_id, name, creator_user_id, scopes, expires_at, revoked_at
		 FROM md_agents WHERE token_hash = ?`, agentTokenHash(bearer),
	).Scan(&a.ID, &a.TeamID, &a.Name, &a.CreatorUserID, &scopesJSON, &a.ExpiresAt, &a.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid agent token")
	}
	if a.RevokedAt != nil {
		return nil, fmt.Errorf("agent %s is revoked", a.ID)
	}
	if a.ExpiresAt != nil && a.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("agent %s is expired", a.ID)
	}
	if err := json.Unmarshal([]byte(scopesJSON), &a.Scopes); err != nil {
		return nil, fmt.Errorf("agent %s has corrupt scopes", a.ID)
	}
	return &AgentAuthResult{Agent: a, Scopes: a.Scopes}, nil
}

// ListAgents returns the agents of a team (admin view).
func ListAgents(ctx context.Context, teamID string) ([]Agent, error) {
	rows, err := database.DB.QueryContext(ctx,
		`SELECT id, team_id, name, creator_user_id, scopes, expires_at, revoked_at, created_at
		 FROM md_agents WHERE team_id = ? ORDER BY created_at DESC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		var scopesJSON string
		if err := rows.Scan(&a.ID, &a.TeamID, &a.Name, &a.CreatorUserID, &scopesJSON, &a.ExpiresAt, &a.RevokedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(scopesJSON), &a.Scopes)
		out = append(out, a)
	}
	return out, rows.Err()
}

// RevokeAgent sets revoked_at (idempotent). Ownership check is caller's job.
func RevokeAgent(ctx context.Context, teamID, agentID string) (bool, error) {
	res, err := database.DB.ExecContext(ctx,
		`UPDATE md_agents SET revoked_at = NOW() WHERE id = ? AND team_id = ? AND revoked_at IS NULL`,
		agentID, teamID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// AgentAllowed reports whether agent a may call tool on docID ("" for
// doc-less tools). Docs whitelist applies to read-like tools (design §2).
func AgentAllowed(a *Agent, tool, docID string) bool {
	if a == nil || !agentToolGrantable(tool) {
		return false
	}
	ok := false
	for _, t := range a.Scopes.Tools {
		if t == tool {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	if len(a.Scopes.Docs) == 0 || docID == "" {
		return true
	}
	for _, d := range a.Scopes.Docs {
		if subtle.ConstantTimeCompare([]byte(d), []byte(docID)) == 1 {
			return true
		}
	}
	return false
}

// ToolGrant is the per-call scope check input.
type ToolGrant struct {
	Name  string
	DocID string
}

// AgentAllowedAt re-exports the whitelist check with the ToolGrant shape
// used by the middleware (nil-safe).
func AgentAllowedAt(a *Agent, grant ToolGrant) bool {
	return AgentAllowed(a, grant.Name, grant.DocID)
}

// ScopesForAgent loads a live agent's scopes by id (used per-request by the
// gate so a mid-flight revoke takes effect immediately).
func ScopesForAgent(ctx context.Context, agentID string) (*Agent, error) {
	agentID = strings.TrimPrefix(agentID, "agent:")
	var a Agent
	var scopesJSON string
	err := database.DB.QueryRowContext(ctx,
		`SELECT id, team_id, name, creator_user_id, scopes, expires_at, revoked_at
		 FROM md_agents WHERE id = ?`, agentID,
	).Scan(&a.ID, &a.TeamID, &a.Name, &a.CreatorUserID, &scopesJSON, &a.ExpiresAt, &a.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}
	if a.RevokedAt != nil || (a.ExpiresAt != nil && a.ExpiresAt.Before(time.Now())) {
		return nil, fmt.Errorf("agent %s is disabled", a.ID)
	}
	if err := json.Unmarshal([]byte(scopesJSON), &a.Scopes); err != nil {
		return nil, fmt.Errorf("agent %s has corrupt scopes", a.ID)
	}
	return &a, nil
}

// CreateAgentDeniedAudit records a denied agent tool call (design §3:
// every tool call is audited, including refusals).
func CreateAgentDeniedAudit(ctx context.Context, agentID, teamID, tool, reason, ip string) error {
	detail := map[string]string{"reason": reason}
	if tool != "" {
		detail["tool"] = tool
	}
	b, _ := json.Marshal(detail)
	_, err := database.DB.ExecContext(ctx,
		`INSERT INTO md_audits (id, team_id, user_id, user_name, actor_type, action, resource_type, resource_id, resource_name, detail, ip)
		 VALUES (?, ?, ?, ?, 'agent', 'mcp.denied', 'mcp', '', '', ?, ?)`,
		uuid.New().String(), teamID, "agent:"+agentID, "agent", string(b), ip)
	return err
}
