package middleware

import (
	"net/http"

	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// Permission levels
const (
	PermRead  = "read"
	PermWrite = "write"
	PermAdmin = "admin"
)

// RequireTeamPermission checks if the current user has the required permission on a resource.
// Uses team role + folder ACL + doc share (three-layer model).
func RequireTeamPermission(resourceType, requiredPerm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Team admin bypasses all checks
		teamRole := c.GetString("current_team_role")
		if teamRole == "admin" {
			c.Next()
			return
		}

		resourceID := c.Param("id")
		if resourceID == "" {
			resourceID = c.Query("resource_id")
		}
		if resourceID == "" {
			c.Next()
			return
		}

		userID := c.GetString("user_id")
		teamID := c.GetString("current_team_id")

		if service.HasTeamPermission(c.Request.Context(), userID, teamID, teamRole, resourceType, resourceID, requiredPerm) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		c.Abort()
	}
}
