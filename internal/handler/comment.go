package handler

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListComments returns comments for a document.
// GET /docs/documents/:id/comments
func ListComments(c *gin.Context) {
	docID := c.Param("id")

	rows, err := database.DB.Query(
		"SELECT id, content, parent_id, user_id, user_name, created_at, updated_at FROM md_comments WHERE document_id = ? ORDER BY created_at ASC",
		docID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	comments := []gin.H{}
	for rows.Next() {
		var id, content, userID, userName string
		var createdAt, updatedAt time.Time
		var parentID sql.NullString
		rows.Scan(&id, &content, &parentID, &userID, &userName, &createdAt, &updatedAt)
		comments = append(comments, gin.H{
			"id":         id,
			"content":    content,
			"parent_id":  parentID.String,
			"user_id":    userID,
			"user_name":  userName,
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}
	c.JSON(200, gin.H{"data": comments})
}

// CreateComment adds a comment to a document.
// POST /docs/documents/:id/comments
func CreateComment(c *gin.Context) {
	docID := c.Param("id")
	userID, _ := c.Get("user_id")
	userName, _ := c.Get("username")

	var req struct {
		Content  string `json:"content" binding:"required"`
		ParentID string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "评论内容不能为空"})
		return
	}

	// Check document exists
	var exists int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE id = ? AND status = 1", docID).Scan(&exists)
	if exists == 0 {
		c.JSON(404, gin.H{"error": "文档不存在"})
		return
	}

	id := uuid.New().String()
	now := time.Now()
	parentID := sql.NullString{String: req.ParentID, Valid: req.ParentID != ""}

	_, err := database.DB.Exec(
		"INSERT INTO md_comments (id, document_id, content, parent_id, user_id, user_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, docID, req.Content, parentID, userID, userName, now, now,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Create notification for document owner if commenter is not the owner
	var createdBy string
	database.DB.QueryRow("SELECT created_by FROM md_documents WHERE id = ?", docID).Scan(&createdBy)
	if createdBy != userID && createdBy != "" {
		createNotification(createdBy, "comment", fmt.Sprintf("%s 评论了文档", userName), docID, id)
	}

	// Notify parent comment author (reply)
	if req.ParentID != "" {
		var parentAuthor string
		database.DB.QueryRow("SELECT user_id FROM md_comments WHERE id = ?", req.ParentID).Scan(&parentAuthor)
		if parentAuthor != userID && parentAuthor != "" {
			createNotification(parentAuthor, "reply", fmt.Sprintf("%s 回复了你的评论", userName), docID, id)
		}
	}

	// Audit
	audit(c, "comment", "document", docID, "", fmt.Sprintf("%s 添加了评论", userName))

	c.JSON(200, gin.H{
		"id":         id,
		"content":    req.Content,
		"parent_id":  req.ParentID,
		"user_id":    userID,
		"user_name":  userName,
		"created_at": now,
	})
}

// UpdateComment edits a comment.
// PUT /docs/comments/:id
func UpdateComment(c *gin.Context) {
	commentID := c.Param("id")
	userID, _ := c.Get("user_id")

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "评论内容不能为空"})
		return
	}

	result, err := database.DB.Exec(
		"UPDATE md_comments SET content = ?, updated_at = ? WHERE id = ? AND user_id = ?",
		req.Content, time.Now(), commentID, userID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(403, gin.H{"error": "无权编辑此评论"})
		return
	}

	c.JSON(200, gin.H{"message": "已更新"})
}

// DeleteComment deletes a comment.
// DELETE /docs/comments/:id
func DeleteComment(c *gin.Context) {
	commentID := c.Param("id")
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("role")

	query := "DELETE FROM md_comments WHERE id = ?"
	args := []interface{}{commentID}

	// Non-admin can only delete own comments
	if userRole != "super_admin" && userRole != "dept_admin" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}

	result, err := database.DB.Exec(query, args...)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(403, gin.H{"error": "无权删除此评论"})
		return
	}

	// Also delete child comments
	database.DB.Exec("DELETE FROM md_comments WHERE parent_id = ?", commentID)

	c.JSON(200, gin.H{"message": "已删除"})
}

// CommentCount returns comment count for a document.
func CommentCount(docID string) int {
	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_comments WHERE document_id = ?", docID).Scan(&count)
	return count
}
