package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// MistLabClaims matches the Portal (mist-team-server) JWT format
type MistLabClaims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

// LegacyClaims matches the old MistDocs JWT format (transition compat)
type LegacyClaims struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	DepartmentID string `json:"department_id"`
	jwt.RegisteredClaims
}

// JWTAuth validates the shared MistLab JWT token.
// It accepts both the new Portal token (uid) and the legacy MistDocs token (user_id).
// After validation, it looks up the user from the shared users table.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}

		userID, err := ParseMistLabToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token 无效"})
			c.Abort()
			return
		}

		// Look up user from shared users table
		var id, username, displayName, email string
		var isAdmin bool
		err = database.DB.QueryRow(
			`SELECT id, username, display_name, email, is_admin FROM users WHERE id = ?`, userID,
		).Scan(&id, &username, &displayName, &email, &isAdmin)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			c.Abort()
			return
		}

		// Get user's team memberships
		type teamInfo struct {
			TeamID string
			Role   string
		}
		rows, err := database.DB.Query(
			`SELECT tm.team_id, tm.role FROM team_members tm WHERE tm.user_id = ?`, userID,
		)
		if err == nil {
			defer rows.Close()
			var teams []teamInfo
			for rows.Next() {
				var t teamInfo
				if rows.Scan(&t.TeamID, &t.Role) == nil {
					teams = append(teams, t)
				}
			}
			// Store first team as default (frontend can switch)
			if len(teams) > 0 {
				c.Set("current_team_id", teams[0].TeamID)
				c.Set("current_team_role", teams[0].Role)
			}
			c.Set("teams", teams)
		}

		// If URL contains team_id param, validate membership
		if teamID := c.Param("team_id"); teamID != "" {
			c.Set("current_team_id", teamID)
			var role string
			err = database.DB.QueryRow(
				`SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID,
			).Scan(&role)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "不是该团队成员"})
				c.Abort()
				return
			}
			c.Set("current_team_role", role)
		}

		c.Set("user_id", id)
		c.Set("username", username)
		c.Set("display_name", displayName)
		c.Set("email", email)
		c.Set("is_admin", isAdmin)

		// Backward compat: set role/department_id for legacy handlers
		// role is derived from team membership
		if isAdmin {
			c.Set("role", "super_admin")
		} else {
			c.Set("role", "member")
		}
		c.Set("department_id", "") // deprecated

		c.Next()
	}
}

// ParseMistLabToken validates a token and returns the user ID.
// Tries MistLab (Portal) format first, then falls back to legacy MistDocs format.
func ParseMistLabToken(tokenStr string) (string, error) {
	secret := []byte(config.C.JWT.Secret)

	// Try MistLab (Portal) token: { uid: "u_xxx" }
	token, err := jwt.ParseWithClaims(tokenStr, &MistLabClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err == nil {
		if claims, ok := token.Claims.(*MistLabClaims); ok && token.Valid && claims.UserID != "" {
			return claims.UserID, nil
		}
	}

	// Fallback: try legacy MistDocs token: { user_id: "xxx", ... }
	token2, err := jwt.ParseWithClaims(tokenStr, &LegacyClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err == nil {
		if claims, ok := token2.Claims.(*LegacyClaims); ok && token2.Valid && claims.UserID != "" {
			return claims.UserID, nil
		}
	}

	return "", fmt.Errorf("invalid token")
}

func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return c.Query("token")
}

// TeamAuth ensures the user has access to the specified team.
// Requires JWTAuth to have already run (sets user_id).
// Validates team_id param and sets current_team_id + current_team_role.
func TeamAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID := c.Param("team_id")
		if teamID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing team_id"})
			c.Abort()
			return
		}

		userID := c.GetString("user_id")
		var role string
		err := database.DB.QueryRow(
			`SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID,
		).Scan(&role)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "不是该团队成员"})
			c.Abort()
			return
		}

		c.Set("current_team_id", teamID)
		c.Set("current_team_role", role)
		c.Next()
	}
}

// GenerateToken generates a MistLab-compatible JWT for testing.
// DEPRECATED: Only used by integration tests.
func GenerateToken(userID, username, role, departmentID string) (string, error) {
	claims := MistLabClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    config.C.JWT.Issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.C.JWT.Secret))
}
