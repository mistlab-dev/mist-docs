package middleware

// MCP auth for the single /mcp entry (#76, design #68), replacing the
// JWTAuth+AgentAuth chain: JWTAuth would 401 an agt_ bearer before the
// agent path ever ran (review blocker 1).
//
// MCPAuth branches on the bearer prefix:
//   - "Bearer agt_..."  -> agent path: resolve the agent, inject
//     user_id="agent:<id>" / username / actor_type=agent / agent_id /
//     agent_creator; permission subject resolution happens in the store
//     (review blocker 2: team_members has no agent rows, so store helpers
//     map agent: users to their creator for role checks).
//   - anything else     -> human path: JWTAuth verbatim.
//
// No user and no agt_ token => 401 here (MCPServe would 401 anyway, but an
// explicit message keeps logs clear).

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// MCPAuth is the /mcp auth entry: agent token or human JWT.
func MCPAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer agt_") {
			res, err := service.AuthenticateAgent(c.Request.Context(), auth)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			a := res.Agent
			c.Set("user_id", "agent:"+a.ID)
			c.Set("username", a.Name)
			c.Set("actor_type", "agent")
			c.Set("agent_id", a.ID)
			c.Set("agent_team_id", a.TeamID)
			c.Set("agent_creator", a.CreatorUserID)
			c.Next()
			return
		}
		JWTAuth()(c)
	}
}

// agentReadTools maps MCP tool names to their doc-ID argument key (empty =
// doc-less tool). Only these are grantable in T6; write tools are denied
// before dispatch (design §5).
var agentReadTools = map[string]string{
	"search_docs":        "",
	"list_docs":          "",
	"search_doc_content": "",
	"read_doc":           "doc_id",
}

// AgentScopeGate enforces the tool/docs whitelist for agent callers.
// Runs after AgentAuth, before MCPServe. Human callers pass through
// untouched. The request body is read and re-injected so the MCP handler
// sees it unchanged. Denied calls are audited (mcp.denied, design §3).
func AgentScopeGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("actor_type") != "agent" {
			c.Next()
			return
		}
		agentID := c.GetString("agent_id")
		agentTeam := c.GetString("agent_team_id")

		body, err := c.GetRawData()
		if err != nil {
			abortAgentDenied(c, agentID, "", "", "unreadable body")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			// Let the MCP handler produce its own parse error response.
			c.Next()
			return
		}

		// Agent must stay inside its own team.
		teamID := ""
		var args struct {
			Arguments struct {
				TeamID string `json:"team_id"`
			} `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &args)
		teamID = strings.TrimSpace(args.Arguments.TeamID)
		if teamID != "" && teamID != agentTeam {
			abortAgentDenied(c, agentID, teamID, "", "team mismatch")
			return
		}

		if req.Method == "tools/call" {
			// req.Params IS the {name, arguments} object (JSON-RPC request
			// shape); do not wrap another "params" layer (review blocker A).
			var call struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &call); err != nil {
				c.Next()
				return
			}
			docKey, known := agentReadTools[call.Name]
			if !known {
				abortAgentDenied(c, agentID, teamID, call.Name, "tool not grantable to agents")
				return
			}
			var callArgs map[string]any
			_ = json.Unmarshal(call.Arguments, &callArgs)
			tool := service.ToolGrant{
				Name: call.Name,
			}
			if docKey != "" {
				if docID, _ := callArgs[docKey].(string); docID != "" {
					tool.DocID = docID
				}
			}
			scopes, err := service.ScopesForAgent(c.Request.Context(), agentID)
			if err != nil || !service.AgentAllowed(scopes, tool.Name, tool.DocID) {
				abortAgentDenied(c, agentID, teamID, call.Name, "outside agent scopes")
				return
			}
		}
		c.Next()
	}
}

func abortAgentDenied(c *gin.Context, agentID, teamID, tool, reason string) {
	_ = service.CreateAgentDeniedAudit(c.Request.Context(), agentID, teamID, tool, reason, c.ClientIP())
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"jsonrpc": "2.0",
		"id":      nil,
		"error":   gin.H{"code": -32002, "message": "agent scope denied: " + reason},
	})
}
