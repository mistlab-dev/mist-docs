package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Team roles come from the Portal (team_members.role). owner is the team's
// creator and has every admin right (D4). Unknown roles get nothing from
// the role-gated endpoints.
const (
	RoleViewer = "viewer"
	RoleEditor = "editor"
	RoleAdmin  = "admin"
)

func roleRank(role string) int {
	switch role {
	case "owner", "admin":
		return 3
	case "editor", "member":
		return 2
	case "viewer":
		return 1
	default:
		return 0
	}
}

// roleAtLeast reports whether the caller's team role is need or higher.
func roleAtLeast(c *gin.Context, need string) bool {
	return roleRank(getTeamRole(c)) >= roleRank(need)
}

// isTeamAdmin: admin or owner.
func isTeamAdmin(c *gin.Context) bool { return roleAtLeast(c, RoleAdmin) }

// requireRole answers 403 unless the caller is need or higher.
func requireRole(c *gin.Context, need string) bool {
	if roleAtLeast(c, need) {
		return true
	}
	msg := "需要编辑者或管理员权限"
	if need == RoleAdmin {
		msg = "仅管理员可操作"
	}
	c.JSON(http.StatusForbidden, gin.H{"error": msg})
	return false
}

// requireOwnerOrAdmin allows the record's creator(s) or a team admin.
func requireOwnerOrAdmin(c *gin.Context, owners ...string) bool {
	uid := c.GetString("user_id")
	for _, o := range owners {
		if o != "" && o == uid {
			return true
		}
	}
	if isTeamAdmin(c) {
		return true
	}
	denyForbidden(c)
	return false
}
