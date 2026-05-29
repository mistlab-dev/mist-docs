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

// Roles
const (
	RoleSuperAdmin = "super_admin"
	RoleDeptAdmin  = "dept_admin"
	RoleMember     = "member"
)

// RequirePermission checks if the current user has the required permission on a resource
func RequirePermission(resourceType, requiredPerm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")

		// Super admin bypasses all checks
		if role == RoleSuperAdmin {
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
		deptID := c.GetString("department_id")

		if service.HasPermission(c.Request.Context(), userID, deptID, role, resourceType, resourceID, requiredPerm) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		c.Abort()
	}
}
