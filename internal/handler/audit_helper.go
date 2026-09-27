package handler

import (
	"encoding/json"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/c-wind/mist-docs/internal/webhook"
	"github.com/gin-gonic/gin"
)

func audit(c *gin.Context, action, resourceType, resourceID, resourceName, detail string) {
	// Rows without a name showed "—" in the audit log (e.g. every edit).
	// Fill in the current title so the log stays readable after renames
	// and deletes.
	if resourceName == "" && resourceID != "" {
		resourceName = resourceTitle(c.GetString("current_team_id"), resourceType, resourceID)
	}
	service.CreateAudit(
		c.Request.Context(),
		c.GetString("user_id"),
		c.GetString("username"),
		c.GetString("department_id"),
		c.GetString("current_team_id"),
		action,
		resourceType,
		resourceID,
		resourceName,
		detail,
		c.ClientIP(),
	)

	// Every audited action with a webhook event name fires it (document.*,
	// comment.created). Actions without one (views, exports, permission
	// changes) are audit-only.
	if ev := webhook.Canonical(action); ev != "deadline.reminder" && webhook.Valid(ev) && ev != "*" {
		fireWebhooks(c.GetString("current_team_id"), ev, resourceType, resourceID, resourceName, detail)
	}
}

// auditDetail encodes structured detail for md_audits.detail.
func auditDetail(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// excerpt returns at most n runes of s, for audit/webhook details.
func excerpt(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

// userDisplayName: display name, else username, else "".
func userDisplayName(id string) string {
	var name string
	if id != "" {
		database.DB.QueryRow(
			`SELECT COALESCE(NULLIF(display_name,''), NULLIF(username,''), '') FROM users WHERE id COLLATE utf8mb4_unicode_ci = ?`, id,
		).Scan(&name)
	}
	return name
}

// permissionTarget returns the user/department an ACL row grants access to.
func permissionTarget(id string) string {
	var target string
	database.DB.QueryRow(`SELECT target_id FROM md_permissions WHERE id=?`, id).Scan(&target)
	return target
}
