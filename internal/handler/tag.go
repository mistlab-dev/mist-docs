package handler

import (
	"net/http"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==================== Tag CRUD ====================

// ListTags GET /docs/tags
func ListTags(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := database.DB.QueryContext(c.Request.Context(),
		`SELECT t.id, t.name, t.color, t.user_id, t.created_at,
		 (SELECT COUNT(*) FROM md_doc_tags dt WHERE dt.tag_id = t.id) AS doc_count
		 FROM md_tags t WHERE t.user_id = ? ORDER BY t.name`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type tagItem struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Color     string `json:"color"`
		UserID    string `json:"user_id"`
		DocCount  int    `json:"doc_count"`
		CreatedAt string `json:"created_at"`
	}
	var tags []tagItem
	for rows.Next() {
		var t tagItem
		rows.Scan(&t.ID, &t.Name, &t.Color, &t.UserID, &t.CreatedAt, &t.DocCount)
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []tagItem{}
	}
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

// CreateTag POST /docs/tags
func CreateTag(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name  string `json:"name" binding:"required"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Color == "" {
		req.Color = "#409eff"
	}
	id := uuid.New().String()
	_, err := database.DB.ExecContext(c.Request.Context(),
		`INSERT INTO md_tags (id, name, color, user_id) VALUES (?, ?, ?, ?)`,
		id, strings.TrimSpace(req.Name), req.Color, userID)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": "标签已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": req.Name, "color": req.Color}})
}

// DeleteTag DELETE /docs/tags/:id
func DeleteTag(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	// Remove all doc-tag relations first
	database.DB.ExecContext(c.Request.Context(), "DELETE FROM md_doc_tags WHERE tag_id = ?", id)
	_, err := database.DB.ExecContext(c.Request.Context(), "DELETE FROM md_tags WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== Document-Tag Relations ====================

// GetDocTags GET /docs/documents/:id/tags
func GetDocTags(c *gin.Context) {
	docID := c.Param("id")
	rows, err := database.DB.QueryContext(c.Request.Context(),
		`SELECT t.id, t.name, t.color FROM md_tags t
		 JOIN md_doc_tags dt ON dt.tag_id = t.id
		 WHERE dt.document_id = ? ORDER BY t.name`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type tagInfo struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	var tags []tagInfo
	for rows.Next() {
		var t tagInfo
		rows.Scan(&t.ID, &t.Name, &t.Color)
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []tagInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

// SetDocTags PUT /docs/documents/:id/tags
// Replaces all tags for a document with the given tag IDs
func SetDocTags(c *gin.Context) {
	docID := c.Param("id")
	var req struct {
		TagIDs []string `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	tx, err := database.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	// Clear existing
	tx.ExecContext(c.Request.Context(), "DELETE FROM md_doc_tags WHERE document_id = ?", docID)

	// Insert new
	for _, tagID := range req.TagIDs {
		id := uuid.New().String()
		_, err := tx.ExecContext(c.Request.Context(),
			"INSERT INTO md_doc_tags (id, document_id, tag_id) VALUES (?, ?, ?)",
			id, docID, tagID)
		if err != nil {
			continue // skip invalid tag IDs
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "标签已更新"})
}

// GetDocsByTag GET /docs/tags/:id/documents
func GetDocsByTag(c *gin.Context) {
	tagID := c.Param("id")
	userID := c.GetString("user_id")
	role := c.GetString("role")
	deptID := c.GetString("department_id")

	rows, err := database.DB.QueryContext(c.Request.Context(),
		`SELECT d.id, IFNULL(d.folder_id,''), d.department_id, d.title, d.type, d.file_size, d.version,
		        IFNULL(d.created_by,''), IFNULL(d.updated_by,''), d.created_at, d.updated_at
		 FROM md_documents d
		 JOIN md_doc_tags dt ON dt.document_id = d.id
		 WHERE dt.tag_id = ? AND d.status = 1`, tagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type docItem struct {
		ID           string `json:"id"`
		FolderID     string `json:"folder_id"`
		DepartmentID string `json:"department_id"`
		Title        string `json:"title"`
		Type         string `json:"type"`
		FileSize     int64  `json:"file_size"`
		Version      int    `json:"version"`
		CreatedBy    string `json:"created_by"`
		UpdatedBy    string `json:"updated_by"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
	}
	var docs []docItem
	for rows.Next() {
		var d docItem
		rows.Scan(&d.ID, &d.FolderID, &d.DepartmentID, &d.Title, &d.Type, &d.FileSize,
			&d.Version, &d.CreatedBy, &d.UpdatedBy, &d.CreatedAt, &d.UpdatedAt)
		// Permission check: super_admin sees all, others see own dept or public
		if role == "super_admin" || d.DepartmentID == deptID || d.CreatedBy == userID || d.DepartmentID == "" {
			docs = append(docs, d)
		}
	}
	if docs == nil {
		docs = []docItem{}
	}
	c.JSON(http.StatusOK, gin.H{"data": docs})
}
