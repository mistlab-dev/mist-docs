package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/webhook"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==================== Webhook CRUD ====================

// WebhookConfig stored in DB
type WebhookConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Secret    string `json:"secret,omitempty"`
	Events    string `json:"events"` // comma-separated: create,update,delete,share,comment
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	Enabled   bool   `json:"enabled"`
}

// ListWebhooks GET /admin/webhooks
func ListWebhooks(c *gin.Context) {
	rows, err := database.DB.QueryContext(c.Request.Context(),
		"SELECT id, name, url, secret, events, created_by, created_at, IF(enabled,1,0) FROM md_webhooks ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var webhooks []WebhookConfig
	for rows.Next() {
		var w WebhookConfig
		rows.Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &w.Events, &w.CreatedBy, &w.CreatedAt, &w.Enabled)
		w.Secret = "***" // hide secret
		webhooks = append(webhooks, w)
	}
	if webhooks == nil {
		webhooks = []WebhookConfig{}
	}
	c.JSON(http.StatusOK, gin.H{"data": webhooks})
}

// CreateWebhook POST /admin/webhooks
func CreateWebhook(c *gin.Context) {
	var req struct {
		Name   string `json:"name" binding:"required"`
		URL    string `json:"url" binding:"required"`
		Secret string `json:"secret"`
		Events string `json:"events"` // default: document.created,document.updated
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Events == "" {
		req.Events = `["document.created","document.updated"]`
	}

	id := uuid.New().String()
	userID := c.GetString("user_id")
	_, err := database.DB.ExecContext(c.Request.Context(),
		"INSERT INTO md_webhooks (id, name, url, secret, events, created_by, enabled) VALUES (?,?,?,?,?,?,1)",
		id, req.Name, req.URL, req.Secret, req.Events, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// DeleteWebhook DELETE /admin/webhooks/:id
func DeleteWebhook(c *gin.Context) {
	_, err := database.DB.ExecContext(c.Request.Context(), "DELETE FROM md_webhooks WHERE id = ?", c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ToggleWebhook PUT /admin/webhooks/:id/toggle
func ToggleWebhook(c *gin.Context) {
	_, err := database.DB.ExecContext(c.Request.Context(),
		"UPDATE md_webhooks SET enabled = NOT enabled WHERE id = ?", c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已切换"})
}

// ==================== Webhook Dispatcher ====================

// webhookPayload is sent to webhook URLs
type webhookPayload struct {
	Event     string `json:"event"`
	Resource  string `json:"resource"`
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name,omitempty"`
	Timestamp string `json:"timestamp"`
	Detail    string `json:"detail,omitempty"`
}

// fireWebhooks posts event to the team's enabled webhooks that subscribe to it.
// event should be the name the API advertises (document.created / document.updated).
// Subscriptions may use that name or an audit alias such as create_doc / edit_doc.
func fireWebhooks(teamID, event, resourceType, resourceID, title, detail string) {
	go func() {
		targets := webhook.LoadTargets(teamID)
		if len(targets) == 0 {
			return
		}

		payload := webhookPayload{
			Event:     event,
			Resource:  resourceType,
			ID:        resourceID,
			Title:     title,
			Timestamp: time.Now().Format(time.RFC3339),
			Detail:    detail,
		}
		body, _ := json.Marshal(payload)
		client := &http.Client{Timeout: 5 * time.Second}

		for _, t := range targets {
			if !webhook.Subscribed(t.Events, event) {
				continue
			}

			req, err := http.NewRequest("POST", t.URL, bytes.NewReader(body))
			if err != nil {
				webhook.LogDelivery(t.ID, event, "error:"+err.Error())
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			if t.Secret != "" {
				req.Header.Set("X-Webhook-Secret", t.Secret)
			}
			req.Header.Set("X-Webhook-Event", event)

			resp, err := client.Do(req)
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}

			status := "ok"
			if err != nil {
				status = "error:" + err.Error()
			}
			webhook.LogDelivery(t.ID, event, status)
		}
	}()
}

// Thin aliases kept so existing call sites and tests read the same.
func parseWebhookEvents(s string) []string             { return webhook.ParseEvents(s) }
func canonicalWebhookEvent(action string) string       { return webhook.Canonical(action) }
func webhookSubscribed(eventsField, fired string) bool { return webhook.Subscribed(eventsField, fired) }

// ListWebhookLogs GET /admin/webhooks/:id/logs
func ListWebhookLogs(c *gin.Context) {
	id := c.Param("id")
	rows, err := database.DB.QueryContext(c.Request.Context(),
		"SELECT id, event, status, created_at FROM md_webhook_logs WHERE webhook_id = ? ORDER BY created_at DESC LIMIT 50", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type logEntry struct {
		ID        string `json:"id"`
		Event     string `json:"event"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	var logs []logEntry
	for rows.Next() {
		var l logEntry
		var createdAt sql.NullString
		rows.Scan(&l.ID, &l.Event, &l.Status, &createdAt)
		l.CreatedAt = createdAt.String
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []logEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}
