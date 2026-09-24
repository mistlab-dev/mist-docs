package handler

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// safeBaseName accepts a single path segment and rejects traversal.
func safeBaseName(name string) (string, bool) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", false
	}
	base := filepath.Base(name)
	if base != name || base == "." || base == ".." {
		return "", false
	}
	return base, true
}

func validWebhookURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func denyNotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{"error": msg})
}

func denyForbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
}

func requireTeamAdmin(c *gin.Context) bool {
	role := getTeamRole(c)
	if role == "admin" || role == "owner" {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可操作"})
	return false
}

func documentInTeam(teamID, docID string, activeOnly bool) bool {
	if docID == "" || teamID == "" {
		return false
	}
	q := `SELECT COUNT(*) FROM md_documents WHERE id=? AND team_id=?`
	if activeOnly {
		q += ` AND status=1`
	}
	var n int
	if err := database.DB.QueryRow(q, docID, teamID).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

func folderInTeam(teamID, folderID string) bool {
	if folderID == "" || teamID == "" {
		return false
	}
	var n int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM md_team_folders WHERE id=? AND team_id=?`, folderID, teamID).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// requireDoc ensures the document belongs to the caller's team and the caller
// has at least need (read/write/admin). Missing documents are 404 so another
// team's id cannot be distinguished from a typo.
func requireDoc(c *gin.Context, docID, need string, activeOnly bool) bool {
	teamID := getTeamID(c)
	if !documentInTeam(teamID, docID, activeOnly) {
		denyNotFound(c, "文档不存在")
		return false
	}
	if !service.HasTeamPermission(c.Request.Context(), c.GetString("user_id"), teamID, getTeamRole(c), "document", docID, need) {
		denyForbidden(c)
		return false
	}
	return true
}

func docPermission(c *gin.Context, docID string) string {
	return service.CheckTeamPermission(c.Request.Context(), c.GetString("user_id"), getTeamID(c), getTeamRole(c), "document", docID)
}

func requireTrashDoc(c *gin.Context, docID string) bool {
	if !requireTeamAdmin(c) {
		return false
	}
	teamID := getTeamID(c)
	var status int
	err := database.DB.QueryRow(`SELECT status FROM md_documents WHERE id=? AND team_id=?`, docID, teamID).Scan(&status)
	if err != nil || status != 0 {
		denyNotFound(c, "文档不存在")
		return false
	}
	return true
}

func resourceInTeam(teamID, resType, resID string) bool {
	switch resType {
	case "document":
		return documentInTeam(teamID, resID, false)
	case "folder":
		return folderInTeam(teamID, resID)
	default:
		return false
	}
}

func commentEditable(c *gin.Context, id string) bool {
	var docID, author string
	err := database.DB.QueryRow(`SELECT document_id, user_id FROM md_comments WHERE id=?`, id).Scan(&docID, &author)
	if err != nil || !documentInTeam(getTeamID(c), docID, false) {
		denyNotFound(c, "评论不存在")
		return false
	}
	role := getTeamRole(c)
	if author == c.GetString("user_id") || role == "admin" || role == "owner" {
		return true
	}
	denyForbidden(c)
	return false
}

func permissionResource(id string) (string, string, bool) {
	var resType, resID string
	err := database.DB.QueryRow(`SELECT resource_type, resource_id FROM md_permissions WHERE id=?`, id).Scan(&resType, &resID)
	if err != nil {
		return "", "", false
	}
	return resType, resID, true
}

// resolveCollabPermission accepts either the stored ACL value (permission)
// or the share-dialog role (viewer/editor/admin).
func resolveCollabPermission(permission, role string) (string, bool) {
	if permission != "" {
		if canon, ok := service.NormalizePermission(permission); ok {
			return canon, true
		}
	}
	if role != "" {
		return service.NormalizePermission(role)
	}
	return "", false
}
