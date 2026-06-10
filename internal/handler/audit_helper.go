package handler

import (
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

func audit(c *gin.Context, action, resourceType, resourceID, resourceName, detail string) {
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

	// Fire webhook for important events
	webhookEvents := map[string]bool{
		"create_doc": true, "update_doc": true, "delete_doc": true,
		"create_share": true, "create_comment": true,
		"import_doc": true, "lock_doc": true, "unlock_doc": true,
		"restore": true,
	}
	if webhookEvents[action] {
		fireWebhooks(action, resourceType, resourceID, resourceName, detail)
	}
}
