package handler

// Agent management REST API (#76): team-scoped create / list / revoke.
// Team admins only (role admin/owner). Tokens are shown exactly once at
// creation (design #68 §1); never listed afterwards.

import (
	"net/http"
	"time"

	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

func requireTeamAdminAgent(c *gin.Context) bool {
	switch getTeamRole(c) {
	case "admin", "owner":
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "team admin required"})
	return false
}

// TeamCreateAgent POST /teams/:team_id/agents
func TeamCreateAgent(c *gin.Context) {
	if !requireTeamAdminAgent(c) {
		return
	}
	var req struct {
		Name      string  `json:"name"`
		Tools     []string `json:"tools"`
		Docs      []string `json:"docs"`
		ExpiresAt string  `json:"expires_at"` // RFC3339; empty = 30 days
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	var expires *time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at must be RFC3339"})
			return
		}
		expires = &t
	}
	scopes := service.AgentScopes{Tools: req.Tools, Docs: req.Docs}
	a, token, err := service.CreateAgent(c.Request.Context(), getTeamID(c), req.Name, c.GetString("user_id"), scopes, expires)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit(c, "agent.created", "agent", a.ID, a.Name, `{"tools":`+jsonStringList(a.Scopes.Tools)+"}")
	c.JSON(http.StatusOK, gin.H{"agent": a, "token": token})
}

// TeamListAgents GET /teams/:team_id/agents
func TeamListAgents(c *gin.Context) {
	if !requireTeamAdminAgent(c) {
		return
	}
	agents, err := service.ListAgents(c.Request.Context(), getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// TeamRevokeAgent DELETE /teams/:team_id/agents/:id
func TeamRevokeAgent(c *gin.Context) {
	if !requireTeamAdminAgent(c) {
		return
	}
	ok, err := service.RevokeAgent(c.Request.Context(), getTeamID(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found or already revoked"})
		return
	}
	audit(c, "agent.revoked", "agent", c.Param("id"), "", "")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func jsonStringList(list []string) string {
	out := ""
	for i, s := range list {
		if i > 0 {
			out += ","
		}
		out += `"` + s + `"`
	}
	return "[" + out + "]"
}
