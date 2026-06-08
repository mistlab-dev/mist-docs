package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateShare creates a public share link for a document.
// POST /docs/documents/:id/share
func CreateShare(c *gin.Context) {
	if !service.RequirePlanFeature(c, "external_share") {
		return
	}
	docID := c.Param("id")
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")
	role, _ := c.Get("role")
	deptID, _ := c.Get("department_id")

	// Permission check: need at least 'read' to share
	if role != "super_admin" {
		perm, err := service.CheckPermission(c.Request.Context(), userID.(string), deptID.(string), "document", docID)
		if err != nil || perm == "none" {
			c.JSON(403, gin.H{"error": "无权限分享此文档"})
			return
		}
	}

	var req struct {
		Password  string `json:"password"`
		ExpiresIn int    `json:"expires_in"` // hours, 0 = never
	}
	c.ShouldBindJSON(&req)

	// Check document exists
	var exists int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE id = ? AND status = 1", docID).Scan(&exists)
	if exists == 0 {
		c.JSON(404, gin.H{"error": "文档不存在"})
		return
	}

	// Deactivate existing shares for this doc by this user
	database.DB.Exec("UPDATE md_shares SET status = 0 WHERE document_id = ? AND created_by = ? AND status = 1", docID, userID)

	id := uuid.New().String()
	token := generateShareToken()

	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &t
	}

	_, err := database.DB.Exec(
		"INSERT INTO md_shares (id, document_id, token, password, expires_at, created_by, status, created_at) VALUES (?, ?, ?, ?, ?, ?, 1, ?)",
		id, docID, token, req.Password, expiresAt, userID, time.Now(),
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Audit
	audit(c, "share", "document", docID, "", fmt.Sprintf("%s 分享了文档", userName))

	c.JSON(200, gin.H{
		"share_id":   id,
		"token":      token,
		"share_url":  fmt.Sprintf("/s/%s", token),
		"expires_at": expiresAt,
	})
}

// ListShares lists active shares for a document.
// GET /docs/documents/:id/shares
func ListShares(c *gin.Context) {
	docID := c.Param("id")

	rows, err := database.DB.Query(
		"SELECT id, token, password, expires_at, created_by, created_at, access_count FROM md_shares WHERE document_id = ? AND status = 1 ORDER BY created_at DESC",
		docID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	shares := []gin.H{}
	for rows.Next() {
		var id, token, createdBy string
		var createdAt time.Time
		var password sql.NullString
		var expiresAt sql.NullTime
		var accessCount int
		rows.Scan(&id, &token, &password, &expiresAt, &createdBy, &createdAt, &accessCount)
		item := gin.H{
			"id":           id,
			"token":        token,
			"has_password": password.Valid && password.String != "",
			"expires_at":   expiresAt.Time,
			"expired":      expiresAt.Valid && expiresAt.Time.Before(time.Now()),
			"created_by":   createdBy,
			"created_at":   createdAt,
			"access_count": accessCount,
		}
		shares = append(shares, item)
	}
	c.JSON(200, gin.H{"data": shares})
}

// DeleteShare deactivates a share link.
// DELETE /docs/shares/:id
func DeleteShare(c *gin.Context) {
	shareID := c.Param("id")
	userID, _ := c.Get("user_id")

	database.DB.Exec("UPDATE md_shares SET status = 0 WHERE id = ? AND created_by = ?", shareID, userID)
	c.JSON(200, gin.H{"message": "分享已取消"})
}

// AccessShare handles public access to a shared document.
// GET /s/:token
func AccessShare(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")

	var id, docID, sharePassword string
	var expiresAt sql.NullTime
	var docTitle, docType, deptID string
	var status int

	err := database.DB.QueryRow(
		"SELECT s.id, s.document_id, s.password, s.expires_at, s.status, d.title, d.type, d.department_id FROM md_shares s JOIN md_documents d ON s.document_id = d.id WHERE s.token = ?",
		token,
	).Scan(&id, &docID, &sharePassword, &expiresAt, &status, &docTitle, &docType, &deptID)

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
	if data, err := store.ReadCurrent(deptID, docID); err == nil {
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

func generateShareToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
