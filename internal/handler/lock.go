package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// LockDocument POST /docs/documents/:id/lock
func LockDocument(c *gin.Context) {
	if !service.RequirePlanFeature(c, "doc_lock") {
		return
	}
	id := c.Param("id")
	userID := c.GetString("user_id")

	// Check if already locked by someone else
	var lockedBy string
	database.DB.QueryRowContext(c.Request.Context(),
		"SELECT locked_by FROM md_documents WHERE id = ? AND status = 1", id).Scan(&lockedBy)

	if lockedBy != "" && lockedBy != userID {
		var name string
		database.DB.QueryRowContext(c.Request.Context(),
			"SELECT display_name FROM users WHERE id COLLATE utf8mb4_unicode_ci = ?", lockedBy).Scan(&name)
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("文档已被 %s 锁定", name)})
		return
	}

	now := time.Now()
	_, err := database.DB.ExecContext(c.Request.Context(),
		"UPDATE md_documents SET locked_by = ?, locked_at = ? WHERE id = ?",
		userID, now, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "lock_doc", "document", id, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已锁定"})
}

// UnlockDocument POST /docs/documents/:id/unlock
func UnlockDocument(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	var lockedBy string
	database.DB.QueryRowContext(c.Request.Context(),
		"SELECT locked_by FROM md_documents WHERE id = ?", id).Scan(&lockedBy)

	if lockedBy != userID && role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能解锁自己锁定的文档"})
		return
	}

	_, err := database.DB.ExecContext(c.Request.Context(),
		"UPDATE md_documents SET locked_by = '', locked_at = NULL WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "unlock_doc", "document", id, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已解锁"})
}
