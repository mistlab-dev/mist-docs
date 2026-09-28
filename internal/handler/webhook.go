package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/c-wind/mist-docs/internal/webhook"
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
		client := webhook.Client(5 * time.Second) // refuses internal addresses

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
