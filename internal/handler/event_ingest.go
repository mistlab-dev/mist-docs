package handler

// Cross-product event ingest (#77 T2): MistTerm terminal events and audit
// alerts land in the same inbox (md_notifications) and webhook fan-out.
//
// Auth: agent tokens only (agt_ bearer via MCPAuth's agent path). A team
// admin creates the agent with read-only scopes — the agent cannot call
// write tools, but it IS allowed to push attention/alert events, which is
// the one-directional "terminal tells docs" channel the Epic needs.
//
// Payload: { team_id, event: "term.attention"|"audit.alert", title,
//            source_session?, document_id?, related_id? }
// Recipients: every member of the team (inbox rows), then one webhook
// fire for subscribers of the event.

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/webhook"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var ingestableEvents = map[string]bool{
	"term.attention": true,
	"audit.alert":    true,
}

// TeamIngestEvent POST /teams/:team_id/events/ingest (agent token only).
func TeamIngestEvent(c *gin.Context) {
	// Only the agent path may ingest: humans have no reason to synthesize
	// terminal events, and this keeps the endpoint unusable with a stolen
	// user JWT.
	if c.GetString("actor_type") != "agent" {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent token required"})
		return
	}
	agentTeam := c.GetString("agent_team_id")
	teamID := getTeamID(c)
	if agentTeam != "" && teamID != agentTeam {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent team mismatch"})
		return
	}

	var req struct {
		Event     string `json:"event"`
		Title     string `json:"title"`
		SessionID string `json:"source_session"`
		DocID     string `json:"document_id"`
		RelatedID string `json:"related_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	req.Event = strings.TrimSpace(req.Event)
	req.Title = strings.TrimSpace(req.Title)
	if !ingestableEvents[req.Event] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event must be one of term.attention / audit.alert"})
		return
	}
	if req.Title == "" || len(req.Title) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title must be 1..200 chars"})
		return
	}
	if len(req.SessionID) > 128 || len(req.DocID) > 64 || len(req.RelatedID) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id fields too long"})
		return
	}

	agentID := c.GetString("agent_id")
	agentName := c.GetString("username")
	detail := map[string]string{"source": "mistterm", "agent": agentID}
	if req.SessionID != "" {
		detail["session"] = req.SessionID
	}
	title := req.Title
	if agentName != "" && !strings.Contains(title, agentName) {
		title = "[" + agentName + "] " + title
	}

	// Inbox: one row per team member. Failure of a single insert is logged
	// and skipped; the webhook fan-out must still happen.
	memberIDs := []string{}
	rows, err := database.DB.Query(
		`SELECT user_id FROM team_members WHERE team_id = ?`, teamID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil && id != "" {
				memberIDs = append(memberIDs, id)
			}
		}
	}
	inserted := 0
	for _, uid := range memberIDs {
		_, err := database.DB.Exec(
			`INSERT INTO md_notifications (id, user_id, team_id, type, title, document_id, related_id, is_read, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 0, NOW())`,
			uuid.New().String(), uid, teamID, req.Event, title, req.DocID, req.RelatedID)
		if err == nil {
			inserted++
		}
	}

	// Webhook fan-out for subscribers of this event.
	fireWebhooks(teamID, req.Event, "event", req.RelatedID, title, mustJSON(detail))

	c.JSON(http.StatusOK, gin.H{
		"ok": true, "event": req.Event, "recipients": inserted,
	})
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// webhook.Valid is referenced to keep the import meaningful even if the
// catalogue changes shape later.
var _ = webhook.Valid
