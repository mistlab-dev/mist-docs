package middleware

// Unified actor authentication (#87, design finalized in issue #87):
// one bearer-parsing layer for both principals.
//
//   - "Bearer agt_..." -> agent path: resolve via service.AuthenticateAgent
//     (md_agents, expiry/revocation enforced there), inject actor context
//     (actor_type=agent, agent_id/agent_name/agent_team_id/agent_creator,
//     user_id="agent:<id>" for backward compat).
//   - anything else    -> human path: existing JWTAuth verbatim (Portal
//     token or legacy token), injecting actor_type=user.
//
// Identity failures are 401 in both branches; team-level checks stay in
// TeamAuth / route-specific middleware per the #87 decision table.
//
// Migration notes (#87 落点):
//   - /mcp keeps MCPAuth semantics; MCPAuth now delegates to ActorAuth so
//     the two token parsers cannot drift.
//   - Cross-product write endpoints (search side effects, compliance
//     writes) default to ActorAuth + TeamAuth; "JWT-only" special cases
//     are forbidden unless the product issue says so.

import (
	"net/http"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

const bearerAgentPrefix = "Bearer agt_"

// setAgentContext injects the agent principal into the gin context. Keys
// are fixed by #87 (semantic contract; key names may be adjusted only with
// an issue-level decision).
func setAgentContext(c *gin.Context, a service.Agent) {
	c.Set("user_id", "agent:"+a.ID)
	c.Set("username", a.Name)
	c.Set("actor_type", "agent")
	c.Set("agent_id", a.ID)
	c.Set("agent_team_id", a.TeamID)
	c.Set("agent_creator", a.CreatorUserID)
}

// ActorAuth parses the bearer once and branches on the agt_ prefix.
func ActorAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, bearerAgentPrefix) {
			res, err := service.AuthenticateAgent(c.Request.Context(), auth)
			if err != nil {
				// Identity failure => 401 (issue #87 error table).
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			setAgentContext(c, res.Agent)
			c.Next()
			return
		}
		human := JWTAuth()
		human(c)
		if c.IsAborted() {
			return
		}
		// JWTAuth authenticated a human; tag the actor type explicitly so
		// downstream gates can branch without sniffing context keys.
		if c.GetString("actor_type") == "" {
			c.Set("actor_type", "user")
		}
	}
}

// ActorTeamAuth guards team-scoped routes for both principals, replacing
// per-route ad-hoc branches (issue #87 permission model §2):
//
//	user  -> existing rule: team_members contains (user_id, team_id)
//	agent -> agent_team_id == :team_id; empty binding => 403 (no silent skip)
//
// Must run AFTER ActorAuth. A bare JWTAuth chain in front of a team route
// would 401 agents before this middleware ever runs — the exact root cause
// of #83's first CI failure.
func ActorTeamAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID := c.Param("team_id")
		if teamID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "team_id required"})
			return
		}
		if c.GetString("actor_type") == "agent" {
			agentTeam := c.GetString("agent_team_id")
			if agentTeam == "" || teamID != agentTeam {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "agent team mismatch"})
				return
			}
			c.Set("current_team_id", teamID)
			c.Next()
			return
		}
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		var role string
		err := database.DB.QueryRow(
			`SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID,
		).Scan(&role)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "不是该团队成员"})
			return
		}
		c.Set("current_team_id", teamID)
		c.Set("current_team_role", role)
		c.Next()
	}
}
