// Package webhook holds the team-scoped webhook lookup and event matching that
// both the HTTP handlers and the reminder scheduler use.
//
// Every lookup is keyed on team_id. A caller without a team gets nothing back:
// an unscoped query would deliver one team's events to every other team's
// endpoints.
package webhook

import (
	"encoding/json"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/google/uuid"
)

// Target is one enabled webhook endpoint of a team.
type Target struct {
	ID, URL, Secret, Events string
}

// LoadTargets returns the enabled webhooks of teamID. An empty teamID returns
// nil rather than falling back to every team's hooks.
//
// Rows are fully read and closed before returning so callers can make slow
// HTTP calls without holding a pooled connection.
func LoadTargets(teamID string) []Target {
	if strings.TrimSpace(teamID) == "" {
		return nil
	}
	rows, err := database.DB.Query(
		`SELECT id, url, secret, events FROM md_webhooks WHERE enabled = 1 AND team_id = ?`, teamID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.URL, &t.Secret, &t.Events); err != nil {
			continue
		}
		targets = append(targets, t)
	}
	return targets
}

// LogDelivery records one delivery attempt in md_webhook_logs.
func LogDelivery(webhookID, event, status string) {
	const maxStatus = 200
	if len(status) > maxStatus {
		status = status[:maxStatus]
	}
	database.DB.Exec(
		"INSERT INTO md_webhook_logs (id, webhook_id, event, status, created_at) VALUES (?,?,?,?,NOW())",
		uuid.New().String(), webhookID, event, status)
}

// ParseEvents accepts a JSON array or a comma-separated list.
func ParseEvents(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, e := range arr {
				e = strings.TrimSpace(e)
				if e != "" {
					out = append(out, e)
				}
			}
			return out
		}
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		e := strings.TrimSpace(part)
		e = strings.Trim(e, `[]"'`)
		e = strings.TrimSpace(e)
		if e != "" {
			result = append(result, e)
		}
	}
	return result
}

// Canonical maps audit actions onto the event names the API advertises.
func Canonical(action string) string {
	switch action {
	case "create_doc", "create", "document.created":
		return "document.created"
	case "edit_doc", "update_doc", "update", "document.updated":
		return "document.updated"
	case "delete_doc", "delete", "document.deleted":
		return "document.deleted"
	default:
		return action
	}
}

var aliasGroups = [][]string{
	{"document.created", "create_doc", "create"},
	{"document.updated", "edit_doc", "update_doc", "update"},
	{"document.deleted", "delete_doc", "delete"},
	{"create_share", "document.shared", "share"},
	{"create_comment", "comment.created", "comment"},
	{"import_doc", "document.imported", "import"},
	{"lock_doc", "document.locked", "lock"},
	{"unlock_doc", "document.unlocked", "unlock"},
	{"restore", "restore_doc", "document.restored"},
}

// Aliases returns every name that should match event.
func Aliases(event string) map[string]struct{} {
	for _, g := range aliasGroups {
		for _, e := range g {
			if e == event {
				m := make(map[string]struct{}, len(g))
				for _, x := range g {
					m[x] = struct{}{}
				}
				return m
			}
		}
	}
	return map[string]struct{}{event: {}}
}

// Subscribed reports whether a webhook's events field includes fired.
func Subscribed(eventsField, fired string) bool {
	aliases := Aliases(fired)
	for _, e := range ParseEvents(eventsField) {
		if e == "*" {
			return true
		}
		if _, ok := aliases[e]; ok {
			return true
		}
	}
	return false
}
