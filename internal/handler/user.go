package handler

import (
	"net/http"
	"strings"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
)

// ==================== 认证 ====================

func Login(c *gin.Context) {
	// SSO deprecated: use Portal login instead
	c.JSON(http.StatusGone, gin.H{
		"error":      "此登录接口已废弃，请通过 mistlab.dev 登录",
		"portal_url": "https://mistlab.dev/login",
	})
}

func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

func Me(c *gin.Context) {
	userID := c.GetString("user_id")

	// Query from shared users table
	var username, displayName, email string
	var isAdmin bool
	err := database.DB.QueryRow(
		`SELECT username, display_name, email, is_admin FROM users WHERE id = ?`, userID,
	).Scan(&username, &displayName, &email, &isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// Get team memberships
	type teamEntry struct {
		TeamID   string `json:"team_id"`
		TeamName string `json:"team_name"`
		Role     string `json:"role"`
	}
	rows, err := database.DB.Query(
		`SELECT tm.team_id, t.name, tm.role FROM team_members tm JOIN teams t ON t.id = tm.team_id WHERE tm.user_id = ?`, userID,
	)
	var teams []teamEntry
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t teamEntry
			if rows.Scan(&t.TeamID, &t.TeamName, &t.Role) == nil {
				teams = append(teams, t)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":           userID,
		"username":     username,
		"name":         displayName,
		"display_name": displayName,
		"email":        email,
		"is_admin":     isAdmin,
		"role":         c.GetString("role"),
		"teams":        teams,
	}})
}

// ChangePassword PUT /api/auth/password
//
// Accounts live in the Portal (shared users table, SSO; GitHub/Google users
// have no password at all). The old handler read the legacy md_users table,
// so it could never succeed for a Portal account. Answer 410 Gone with the
// place to manage the account instead (D1).
func ChangePassword(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":      "MistDocs 不再管理密码，请在 Portal 的账号设置中修改",
		"portal_url": portalURL(),
	})
}

func portalURL() string {
	if u := strings.TrimRight(config.C.Portal.URL, "/"); u != "" {
		return u
	}
	return "https://mistlab.dev"
}
