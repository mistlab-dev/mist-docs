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
		action,
		resourceType,
		resourceID,
		resourceName,
		detail,
		c.ClientIP(),
	)
}
