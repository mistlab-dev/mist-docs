package tests

// Unified actor auth (#87): ActorAuth branches on the agt_ prefix, and
// ActorTeamAuth enforces the per-actor team rule:
//   user  -> team_members contains (user_id, team_id)
//   agent -> agent_team_id == :team_id, empty binding => 403
// Identity failures are 401 in both branches (issue #87 error table).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/gin-gonic/gin"
)

// probeRouter mounts ActorAuth + ActorTeamAuth + a handler that reports the
// resolved principal — the minimal contract surface of #87, independent of
// any business route.
func probeRouter() *gin.Engine {
	r := gin.New()
	g := r.Group("/api/teams/:team_id")
	g.Use(middleware.ActorAuth(), middleware.ActorTeamAuth())
	g.GET("/probe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"actor_type": c.GetString("actor_type"),
			"user_id":    c.GetString("user_id"),
			"agent_id":   c.GetString("agent_id"),
			"role":       c.GetString("current_team_role"),
		})
	})
	return r
}

func probeRequest(token, teamID string) *httptest.ResponseRecorder {
	r := probeRouter()
	req := httptest.NewRequest("GET", "/api/teams/"+teamID+"/probe", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 人 JWT 走 user 分支，团队校验走 team_members。
func TestActorAuthUserPath(t *testing.T) {
	w := probeRequest(adminToken, teamID)
	if w.Code != http.StatusOK {
		t.Fatalf("admin probe: %d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["actor_type"] != "user" {
		t.Fatalf("actor_type = %v, want user", out["actor_type"])
	}
	if out["role"] != "admin" {
		t.Fatalf("role = %v, want admin", out["role"])
	}
}

// 非团队成员的人 JWT：身份有效但团队不符 → 403。
func TestActorAuthUserWrongTeam(t *testing.T) {
	w := probeRequest(viewerToken, "team-no-such")
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer on foreign team: %d %s", w.Code, w.Body.String())
	}
}

// agt_ token 走 agent 分支，绑定团队内放行。
func TestActorAuthAgentPath(t *testing.T) {
	_, token := createTestAgent(t, []string{"list_docs"}, nil)
	w := probeRequest(token, teamID)
	if w.Code != http.StatusOK {
		t.Fatalf("agent probe: %d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["actor_type"] != "agent" {
		t.Fatalf("actor_type = %v, want agent", out["actor_type"])
	}
	if out["agent_id"] == "" || out["user_id"] == "" {
		t.Fatalf("agent context incomplete: %v", out)
	}
}

// agent 打非绑定团队 → 403（#87：勿静默跳过）。
func TestActorAuthAgentWrongTeam(t *testing.T) {
	_, token := createTestAgent(t, []string{"list_docs"}, nil)
	w := probeRequest(token, "team-other")
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent on foreign team: %d %s", w.Code, w.Body.String())
	}
}

// 无 token → 401；乱 token → 401（身份失败统一 401）。
func TestActorAuthIdentityFailures(t *testing.T) {
	if w := probeRequest("", teamID); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", w.Code)
	}
	if w := probeRequest("garbage-token", teamID); w.Code != http.StatusUnauthorized {
		t.Fatalf("garbage token: %d", w.Code)
	}
}
