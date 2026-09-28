package handler

import (
	"database/sql"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
)

// AccessShare handles public access to a shared document.
// GET /s/:token
func AccessShare(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")

	var id, docID, sharePassword string
	var expiresAt sql.NullTime
	var docTitle, docType, deptID, teamID string
	var status int

	err := database.DB.QueryRow(
		"SELECT s.id, s.document_id, s.password, s.expires_at, s.status, d.title, d.type, d.department_id, d.team_id FROM md_shares s JOIN md_documents d ON s.document_id = d.id WHERE s.token = ?",
		token,
	).Scan(&id, &docID, &sharePassword, &expiresAt, &status, &docTitle, &docType, &deptID, &teamID)

	if err != nil || status != 1 {
		c.JSON(404, gin.H{"error": "分享链接不存在或已失效"})
		return
	}

	// Check expiry
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		database.DB.Exec("UPDATE md_shares SET status = 0 WHERE id = ?", id)
		c.JSON(410, gin.H{"error": "分享链接已过期"})
		return
	}

	// Check password
	if sharePassword != "" && sharePassword != password {
		c.JSON(403, gin.H{"error": "密码错误", "need_password": true})
		return
	}

	// Increment access count
	database.DB.Exec("UPDATE md_shares SET access_count = access_count + 1 WHERE id = ?", id)

	// Get document content
	var content []byte
	shareBucket := teamID
	if shareBucket == "" {
		shareBucket = deptID
	}
	if data, err := store.ReadCurrent(shareBucket, docID); err == nil {
		content = data
	}

	// Audit
	audit(c, "access_share", "document", docID, docTitle, "通过分享链接访问")

	c.JSON(200, gin.H{
		"title":   docTitle,
		"type":    docType,
		"content": string(content),
	})
}

// AccessShareInfo returns share info without full content (for password prompt).
// GET /s/:token/info
func AccessShareInfo(c *gin.Context) {
	token := c.Param("token")

	var docTitle string
	var sharePassword string
	var expiresAt sql.NullTime
	var status int

	err := database.DB.QueryRow(
		"SELECT d.title, s.password, s.expires_at, s.status FROM md_shares s JOIN md_documents d ON s.document_id = d.id WHERE s.token = ?",
		token,
	).Scan(&docTitle, &sharePassword, &expiresAt, &status)

	if err != nil || status != 1 {
		c.JSON(404, gin.H{"error": "分享链接不存在或已失效"})
		return
	}

	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		c.JSON(410, gin.H{"error": "分享链接已过期"})
		return
	}

	c.JSON(200, gin.H{
		"title":        docTitle,
		"has_password": sharePassword != "",
		"expires_at":   expiresAt.Time,
	})
}
