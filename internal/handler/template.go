package handler

import (
	"net/http"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListTemplates GET /docs/templates?type=doc
func ListTemplates(c *gin.Context) {
	userID := c.GetString("user_id")
	deptID := c.GetString("department_id")
	role := c.GetString("role")
	docType := c.DefaultQuery("type", "doc")

	// Show: user's own + public templates from same department + system built-in (front-end)
	query := `SELECT t.id, t.name, t.type, t.user_id, t.department_id, t.is_public, t.created_at, t.updated_at,
		IFNULL((SELECT display_name FROM users WHERE id COLLATE utf8mb4_unicode_ci = t.user_id), '未知') as user_name
		FROM md_templates t
		WHERE t.type = ? AND (t.user_id = ? OR t.is_public = 1`
	args := []interface{}{docType, userID}

	// Admin can see all department templates
	if role == "super_admin" {
		query += ` OR 1=1`
	} else if deptID != "" {
		query += ` OR t.department_id = ?`
		args = append(args, deptID)
	}
	query += `) ORDER BY t.is_public DESC, t.updated_at DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()

	var templates []*model.DocTemplate
	for rows.Next() {
		t := &model.DocTemplate{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.UserID, &t.DepartmentID, &t.IsPublic, &t.CreatedAt, &t.UpdatedAt, &t.UserName); err != nil {
			continue
		}
		templates = append(templates, t)
	}

	c.JSON(http.StatusOK, gin.H{"data": templates})
}

// GetTemplate GET /docs/templates/:id
func GetTemplate(c *gin.Context) {
	id := c.Param("id")
	t := &model.DocTemplate{}
	err := database.DB.QueryRow(
		`SELECT id, name, type, content, user_id, department_id, is_public, created_at, updated_at FROM md_templates WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Type, &t.Content, &t.UserID, &t.DepartmentID, &t.IsPublic, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": t})
}

// CreateTemplate POST /docs/templates
func CreateTemplate(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Type     string `json:"type"`
		Content  string `json:"content"`
		IsPublic bool   `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Type == "" {
		req.Type = "doc"
	}

	userID := c.GetString("user_id")
	deptID := c.GetString("department_id")
	id := uuid.New().String()
	now := time.Now()

	_, err := database.DB.Exec(
		`INSERT INTO md_templates (id, name, type, content, user_id, department_id, is_public, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, req.Type, req.Content, userID, deptID, req.IsPublic, now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": req.Name}})
}

// UpdateTemplate PUT /docs/templates/:id
func UpdateTemplate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name     string `json:"name"`
		Content  string `json:"content"`
		IsPublic *bool  `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	userID := c.GetString("user_id")
	role := c.GetString("role")

	// Check ownership (or admin)
	var ownerID string
	database.DB.QueryRow(`SELECT user_id FROM md_templates WHERE id = ?`, id).Scan(&ownerID)
	if ownerID != userID && role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	if req.Name != "" {
		database.DB.Exec(`UPDATE md_templates SET name = ? WHERE id = ?`, req.Name, id)
	}
	if req.Content != "" {
		database.DB.Exec(`UPDATE md_templates SET content = ? WHERE id = ?`, req.Content, id)
	}
	if req.IsPublic != nil {
		database.DB.Exec(`UPDATE md_templates SET is_public = ? WHERE id = ?`, *req.IsPublic, id)
	}

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// DeleteTemplate DELETE /docs/templates/:id
func DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	role := c.GetString("role")

	var ownerID string
	database.DB.QueryRow(`SELECT user_id FROM md_templates WHERE id = ?`, id).Scan(&ownerID)
	if ownerID != userID && role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	database.DB.Exec(`DELETE FROM md_templates WHERE id = ?`, id)
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}
