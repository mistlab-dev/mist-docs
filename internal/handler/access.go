package handler

import (
	"database/sql"
	"errors"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
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

func denyNotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{"error": msg})
}

func denyForbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
}

func requireTeamAdmin(c *gin.Context) bool {
	return requireRole(c, RoleAdmin)
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

// maxFolderDepth bounds the parent walk in isDescendantFolder.
const maxFolderDepth = 50

// isDescendantFolder reports whether candidateChildID is the same as or a descendant
// of folderID within the team. This prevents cyclic folder hierarchies. It fails
// closed: a query error or a chain deeper than maxFolderDepth counts as a cycle.
func isDescendantFolder(teamID, folderID, candidateChildID string) bool {
	if teamID == "" {
		return false
	}
	return folderChainReaches(func(id string) (string, error) {
		var parentID string
		err := database.DB.QueryRow(`SELECT IFNULL(parent_id, '') FROM md_team_folders WHERE id=? AND team_id=?`, id, teamID).Scan(&parentID)
		return parentID, err
	}, folderID, candidateChildID)
}

// folderChainReaches walks parent links from candidateChildID and reports whether
// folderID is reached. parentOf returns sql.ErrNoRows for a missing folder.
func folderChainReaches(parentOf func(id string) (string, error), folderID, candidateChildID string) bool {
	if folderID == "" || candidateChildID == "" {
		return false
	}
	if folderID == candidateChildID {
		return true
	}
	curr := candidateChildID
	for i := 0; i < maxFolderDepth; i++ {
		parentID, err := parentOf(curr)
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		if err != nil {
			return true
		}
		if parentID == "" {
			return false
		}
		if parentID == folderID {
			return true
		}
		curr = parentID
	}
	return true
}

// parseFolderIDField normalizes a JSON folder_id: strings are trimmed, integral
// numbers are formatted (0 means root). Any other type, or a non-integer
// number, is rejected.
func parseFolderIDField(v interface{}) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return strings.TrimSpace(x), true
	case float64:
		if x != math.Trunc(x) || math.IsInf(x, 0) || math.Abs(x) > 1<<53 {
			return "", false
		}
		if x == 0 {
			return "", true
		}
		return strconv.FormatInt(int64(x), 10), true
	default:
		return "", false
	}
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
	return requireOwnerOrAdmin(c, author)
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
