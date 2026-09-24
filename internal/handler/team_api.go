package handler

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// getTeamID extracts team_id from context (set by TeamAuth middleware)
func getTeamID(c *gin.Context) string {
	return c.GetString("current_team_id")
}

// getTeamRole extracts team role from context
func getTeamRole(c *gin.Context) string {
	return c.GetString("current_team_role")
}

// ==================== 团队文件夹树 ====================

// TeamFolderTree GET /teams/:team_id/folders/tree
func TeamFolderTree(c *gin.Context) {
	teamID := getTeamID(c)

	rows, err := database.DB.Query(
		`SELECT id, parent_id, name, sort_order, created_by, created_at, updated_at
		 FROM md_team_folders WHERE team_id = ? ORDER BY sort_order, created_at`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Folder struct {
		ID        string    `json:"id"`
		ParentID  string    `json:"parent_id"`
		Name      string    `json:"name"`
		SortOrder int       `json:"sort_order"`
		CreatedBy string    `json:"created_by"`
		CreatedAt string    `json:"created_at"`
		UpdatedAt string    `json:"updated_at"`
		Children  []*Folder `json:"children"`
	}
	folderMap := make(map[string]*Folder)
	var folders []*Folder
	for rows.Next() {
		var f Folder
		var parentID string
		var createdBy, createdAt, updatedAt string
		if rows.Scan(&f.ID, &parentID, &f.Name, &f.SortOrder, &createdBy, &createdAt, &updatedAt) != nil {
			continue
		}
		f.ParentID = parentID
		f.CreatedBy = createdBy
		f.CreatedAt = createdAt
		f.UpdatedAt = updatedAt
		f.Children = []*Folder{}
		folderMap[f.ID] = &f
		folders = append(folders, &f)
	}

	// Build tree
	var tree []*Folder
	for _, f := range folders {
		if f.ParentID == "" {
			tree = append(tree, f)
		} else if parent, ok := folderMap[f.ParentID]; ok {
			parent.Children = append(parent.Children, f)
		} else {
			// orphan → root
			tree = append(tree, f)
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": tree})
}

// CreateTeamFolder POST /teams/:team_id/folders
func CreateTeamFolder(c *gin.Context) {
	teamID := getTeamID(c)
	role := getTeamRole(c)
	if role != "admin" && role != "editor" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要编辑者或管理员权限"})
		return
	}

	var req struct {
		Name     string `json:"name" binding:"required"`
		ParentID string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	id := uuid.New().String()
	userID := c.GetString("user_id")
	_, err := database.DB.Exec(
		`INSERT INTO md_team_folders (id, team_id, parent_id, name, sort_order, created_by) VALUES (?, ?, ?, ?, 0, ?)`,
		id, teamID, req.ParentID, req.Name, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "create_folder", "folder", id, req.Name, "")
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": id, "team_id": teamID, "parent_id": req.ParentID, "name": req.Name,
	}})
}

// UpdateTeamFolder PUT /teams/:team_id/folders/:id
func UpdateTeamFolder(c *gin.Context) {
	role := getTeamRole(c)
	if role != "admin" && role != "editor" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	id := c.Param("id")
	teamID := getTeamID(c)
	if !folderInTeam(teamID, id) {
		denyNotFound(c, "文件夹不存在")
		return
	}
	var req struct {
		Name      string `json:"name"`
		ParentID  string `json:"parent_id"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.ParentID != "" && !folderInTeam(teamID, req.ParentID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "上级文件夹不存在"})
		return
	}

	_, err := database.DB.Exec(
		`UPDATE md_team_folders SET name=?, parent_id=?, sort_order=? WHERE id=? AND team_id=?`,
		req.Name, req.ParentID, req.SortOrder, id, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// DeleteTeamFolder DELETE /teams/:team_id/folders/:id
func DeleteTeamFolder(c *gin.Context) {
	role := getTeamRole(c)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可删除文件夹"})
		return
	}

	id := c.Param("id")
	teamID := getTeamID(c)
	if !folderInTeam(teamID, id) {
		denyNotFound(c, "文件夹不存在")
		return
	}
	// Move documents in this folder to team root
	database.DB.Exec(`UPDATE md_documents SET folder_id='' WHERE folder_id=? AND team_id=?`, id, teamID)
	// Move sub-folders to root
	database.DB.Exec(`UPDATE md_team_folders SET parent_id='' WHERE parent_id=? AND team_id=?`, id, teamID)
	_, err := database.DB.Exec(`DELETE FROM md_team_folders WHERE id=? AND team_id=?`, id, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "delete_folder", "folder", id, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== 团队文档 ====================

// TeamListDocuments GET /teams/:team_id/documents
func TeamListDocuments(c *gin.Context) {
	teamID := getTeamID(c)
	folderID := c.Query("folder_id")
	docType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	where := "WHERE d.team_id = ? AND d.status = 1"
	args := []interface{}{teamID}
	if folderID != "" {
		where += " AND d.folder_id = ?"
		args = append(args, folderID)
	}
	if docType != "" {
		where += " AND d.type = ?"
		args = append(args, docType)
	}

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents d "+where, args...).Scan(&total)

	query := `SELECT d.id, d.team_id, d.folder_id, d.title, d.type, d.file_size, d.version,
		d.locked_by, d.locked_at, d.status, d.created_by, d.updated_by, d.created_at, d.updated_at,
		COALESCE(NULLIF(u1.display_name, ''), NULLIF(u1.username, ''), '') as creator_name,
		COALESCE(NULLIF(u2.display_name, ''), NULLIF(u2.username, ''), '') as updater_name,
		IFNULL(d.content_text, '')
		FROM md_documents d
		LEFT JOIN users u1 ON d.created_by COLLATE utf8mb4_unicode_ci = u1.id
		LEFT JOIN users u2 ON d.updated_by COLLATE utf8mb4_unicode_ci = u2.id ` + where +
		" ORDER BY d.updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Doc struct {
		ID            string       `json:"id"`
		TeamID        string       `json:"team_id"`
		FolderID      string       `json:"folder_id"`
		Title         string       `json:"title"`
		Type          string       `json:"type"`
		FileSize      int64        `json:"file_size"`
		Version       int          `json:"version"`
		LockedBy      string       `json:"locked_by"`
		LockedAt      string       `json:"locked_at,omitempty"`
		Status        int          `json:"status"`
		CreatedBy     string       `json:"created_by"`
		UpdatedBy     string       `json:"updated_by"`
		CreatedAt     string       `json:"created_at"`
		UpdatedAt     string       `json:"updated_at"`
		CreatedByName string       `json:"created_by_name"`
		CreatorName   string       `json:"creator_name"`
		UpdatedByName string       `json:"updated_by_name"`
		Excerpt       string       `json:"excerpt"`
		Tags          []docTagJSON `json:"tags"`
	}
	var docs []Doc
	var ids []string
	for rows.Next() {
		var d Doc
		var folderID, lockedBy, lockedAt sql.NullString
		var contentText string
		rows.Scan(&d.ID, &d.TeamID, &folderID, &d.Title, &d.Type, &d.FileSize, &d.Version,
			&lockedBy, &lockedAt, &d.Status, &d.CreatedBy, &d.UpdatedBy, &d.CreatedAt, &d.UpdatedAt,
			&d.CreatedByName, &d.UpdatedByName, &contentText)
		d.CreatorName = d.CreatedByName
		d.FolderID = folderID.String
		d.LockedBy = lockedBy.String
		d.LockedAt = lockedAt.String
		if d.Type != "sheet" {
			d.Excerpt = previewExcerpt(contentText)
		}
		d.Tags = []docTagJSON{}
		docs = append(docs, d)
		ids = append(ids, d.ID)
	}
	if docs == nil {
		docs = []Doc{}
	}
	tagMap := tagsByDocument(ids)
	for i := range docs {
		if tags, ok := tagMap[docs[i].ID]; ok {
			docs[i].Tags = tags
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": docs, "total": total, "page": page, "page_size": pageSize})
}

// TeamDocParagraphSearch GET /teams/:team_id/docs/search
// 满足 MistTerm 客户端文档检索契约 (docs/tech/TEAM-DOCS.md)
func TeamDocParagraphSearch(c *gin.Context) {
	teamID := getTeamID(c)
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		c.JSON(http.StatusOK, gin.H{"items": []interface{}{}})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 {
		limit = 1
	} else if limit > 20 {
		limit = 20
	}

	pattern := "%" + keyword + "%"
	query := `SELECT d.id, d.title, IFNULL(d.content_text, '') as content_text
		FROM md_documents d
		WHERE d.team_id = ? AND d.status = 1 AND (d.title LIKE ? OR d.content_text LIKE ?)
		ORDER BY d.updated_at DESC LIMIT ?`

	rows, err := database.DB.Query(query, teamID, pattern, pattern, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type SearchHitItem struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Excerpt string `json:"excerpt"`
		Slug    string `json:"slug"`
		Score   int    `json:"score"`
	}

	var items []SearchHitItem
	for rows.Next() {
		var id, title, contentText string
		if err := rows.Scan(&id, &title, &contentText); err != nil {
			continue
		}

		// 从正文中截取包含关键词的段落片段作为 excerpt
		excerpt := ""
		lowerContent := strings.ToLower(contentText)
		lowerKw := strings.ToLower(keyword)
		idx := strings.Index(lowerContent, lowerKw)
		if idx >= 0 {
			start := idx - 40
			if start < 0 {
				start = 0
			}
			end := idx + len(keyword) + 60
			if end > len(contentText) {
				end = len(contentText)
			}
			snippet := contentText[start:end]
			snippet = strings.ReplaceAll(snippet, "\n", " ")
			snippet = strings.TrimSpace(snippet)
			if start > 0 {
				snippet = "…" + snippet
			}
			if end < len(contentText) {
				snippet = snippet + "…"
			}
			excerpt = snippet
		} else {
			// 如果命中的是标题，截取开头一段
			r := []rune(strings.ReplaceAll(contentText, "\n", " "))
			if len(r) > 100 {
				excerpt = string(r[:100]) + "…"
			} else {
				excerpt = string(r)
			}
		}

		items = append(items, SearchHitItem{
			ID:      id,
			Title:   title,
			Excerpt: excerpt,
			Slug:    "",
			Score:   10,
		})
	}

	if items == nil {
		items = []SearchHitItem{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// TeamSearchDocuments GET /teams/:team_id/documents/search
func TeamSearchDocuments(c *gin.Context) {
	teamID := getTeamID(c)
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入搜索关键词"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	pattern := "%" + keyword + "%"
	where := `WHERE d.team_id = ? AND d.status = 1 AND (d.title LIKE ? OR d.content_text LIKE ?)`
	args := []interface{}{teamID, pattern, pattern}

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents d "+where, args...).Scan(&total)

	query := `SELECT d.id, d.team_id, d.folder_id, d.title, d.type, d.file_size, d.version,
		d.locked_by, d.status, d.created_by, d.updated_by, d.created_at, d.updated_at,
		COALESCE(NULLIF(u1.display_name, ''), NULLIF(u1.username, ''), '') as creator_name
		FROM md_documents d
		LEFT JOIN users u1 ON d.created_by COLLATE utf8mb4_unicode_ci = u1.id ` + where +
		" ORDER BY d.updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type SearchResult struct {
		ID            string `json:"id"`
		TeamID        string `json:"team_id"`
		FolderID      string `json:"folder_id"`
		Title         string `json:"title"`
		Type          string `json:"type"`
		FileSize      int64  `json:"file_size"`
		Version       int    `json:"version"`
		CreatedBy     string `json:"created_by"`
		UpdatedBy     string `json:"updated_by"`
		CreatedAt     string `json:"created_at"`
		UpdatedAt     string `json:"updated_at"`
		CreatorName   string `json:"creator_name"`
		CreatedByName string `json:"created_by_name"`
		Snippet       string `json:"snippet"`
	}
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var lockedBy sql.NullString
		var status int
		rows.Scan(&r.ID, &r.TeamID, &r.FolderID, &r.Title, &r.Type, &r.FileSize, &r.Version,
			&lockedBy, &status, &r.CreatedBy, &r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt,
			&r.CreatorName)
		r.CreatedByName = r.CreatorName
		results = append(results, r)
	}
	if results == nil {
		results = []SearchResult{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results, "total": total, "page": page, "page_size": pageSize})
}

// TeamRecentDocuments GET /teams/:team_id/documents/recent
func TeamRecentDocuments(c *gin.Context) {
	teamID := getTeamID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	rows, err := database.DB.Query(
		`SELECT id, team_id, folder_id, title, type, version, updated_at, updated_by
		 FROM md_documents WHERE team_id = ? AND status = 1
		 ORDER BY updated_at DESC LIMIT ?`, teamID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var docs []map[string]interface{}
	for rows.Next() {
		var id, teamID2, folderID, title, docType, updatedBy, updatedAt string
		var version int
		rows.Scan(&id, &teamID2, &folderID, &title, &docType, &version, &updatedAt, &updatedBy)
		docs = append(docs, map[string]interface{}{
			"id": id, "team_id": teamID2, "folder_id": folderID,
			"title": title, "type": docType, "version": version,
			"updated_at": updatedAt, "updated_by": updatedBy,
		})
	}
	if docs == nil {
		docs = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": docs})
}

// TeamCreateDocument POST /teams/:team_id/documents
func TeamCreateDocument(c *gin.Context) {
	teamID := getTeamID(c)
	role := getTeamRole(c)
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	// Check document limit for this team
	var docCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE status = 1 AND team_id = ?", teamID).Scan(&docCount)
	if !service.CheckDocumentLimit(c, docCount) {
		return
	}

	var req struct {
		Title    string `json:"title" binding:"required"`
		Type     string `json:"type"`
		FolderID string `json:"folder_id"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if req.Type == "" {
		req.Type = "doc"
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}
	if len([]rune(req.Title)) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题过长"})
		return
	}
	if req.FolderID != "" && !folderInTeam(teamID, req.FolderID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件夹不存在"})
		return
	}
	userID := c.GetString("user_id")
	docID := uuid.New().String()

	_, err := database.DB.Exec(
		`INSERT INTO md_documents (id, team_id, folder_id, department_id, title, type, status, created_by, updated_by)
		 VALUES (?, ?, ?, '', ?, ?, 1, ?, ?)`,
		docID, teamID, req.FolderID, req.Title, req.Type, userID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save initial content
	if req.Content != "" {
		database.DB.Exec(`UPDATE md_documents SET content_text=? WHERE id=?`, req.Content, docID)
		// Write file
		writeDocContent(docID, []byte(req.Content))
	}

	audit(c, "create_doc", "document", docID, req.Title, fmt.Sprintf(`{"type":"%s"}`, req.Type))
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": docID, "team_id": teamID, "folder_id": req.FolderID,
		"title": req.Title, "type": req.Type,
	}})
}

// TeamGetDocument GET /teams/:team_id/documents/:id
func TeamGetDocument(c *gin.Context) {
	teamID := getTeamID(c)
	docID := c.Param("id")

	var id, teamID2, folderID, title, docType, createdBy, updatedBy, createdAt, updatedAt string
	var version int
	var fileSize int64
	err := database.DB.QueryRow(
		`SELECT id, team_id, folder_id, title, type, file_size, version,
		 created_by, updated_by, created_at, updated_at
		 FROM md_documents WHERE id = ? AND team_id = ? AND status = 1`,
		docID, teamID).Scan(&id, &teamID2, &folderID, &title, &docType, &fileSize, &version,
		&createdBy, &updatedBy, &createdAt, &updatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": id, "team_id": teamID2, "folder_id": folderID,
		"title": title, "type": docType, "file_size": fileSize,
		"version": version, "created_by": createdBy, "updated_by": updatedBy,
		"created_at": createdAt, "updated_at": updatedAt,
		"permission": docPermission(c, docID),
	}})
}

// TeamUpdateDocument PUT /teams/:team_id/documents/:id
// Only fields present in the JSON are written. A move that sends folder_id
// alone must not blank the title, and a rename must not pull the doc to root.
func TeamUpdateDocument(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	var req struct {
		Title    *string `json:"title"`
		FolderID *string `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Title == nil && req.FolderID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	userID := c.GetString("user_id")
	teamID := getTeamID(c)
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
			return
		}
		if len([]rune(title)) > 200 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "标题过长"})
			return
		}
		if _, err := database.DB.Exec(
			`UPDATE md_documents SET title=?, updated_by=? WHERE id=? AND team_id=?`,
			title, userID, docID, teamID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if req.FolderID != nil {
		if *req.FolderID != "" && !folderInTeam(teamID, *req.FolderID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件夹不存在"})
			return
		}
		if _, err := database.DB.Exec(
			`UPDATE md_documents SET folder_id=?, updated_by=? WHERE id=? AND team_id=?`,
			*req.FolderID, userID, docID, teamID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var title string
		database.DB.QueryRow(`SELECT title FROM md_documents WHERE id=? AND team_id=?`, docID, teamID).Scan(&title)
		audit(c, "move", "document", docID, title, fmt.Sprintf(`{"folder_id":"%s"}`, *req.FolderID))
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// TeamDeleteDocument DELETE /teams/:team_id/documents/:id
func TeamDeleteDocument(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "admin", true) {
		return
	}
	teamID := getTeamID(c)
	_, err := database.DB.Exec(`UPDATE md_documents SET status=0, deleted_at=NOW() WHERE id=? AND team_id=?`, docID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "delete_doc", "document", docID, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已移入回收站"})
}

// TeamGetDocumentContent GET /teams/:team_id/documents/:id/content
func TeamGetDocumentContent(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	var title, docType string
	var version int
	var updatedAt string

	err := database.DB.QueryRow(
		`SELECT title, type, version, updated_at FROM md_documents WHERE id=? AND status=1`,
		docID).Scan(&title, &docType, &version, &updatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	content := readDocContent(docID)
	audit(c, "view", "document", docID, title, "")
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"content": string(content), "version": version,
		"title": title, "type": docType, "updated_at": updatedAt,
		"permission": docPermission(c, docID),
	}})
}

// TeamSaveDocumentContent PUT /teams/:team_id/documents/:id/content
func TeamSaveDocumentContent(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	userID := c.GetString("user_id")

	// Resolve storage bucket
	var teamID, deptID string
	database.DB.QueryRow("SELECT team_id, department_id FROM md_documents WHERE id=?", docID).Scan(&teamID, &deptID)
	bucket := teamID
	if bucket == "" {
		bucket = deptID
	}

	// Check lock
	var lockedBy string
	database.DB.QueryRow("SELECT locked_by FROM md_documents WHERE id=?", docID).Scan(&lockedBy)
	role := getTeamRole(c)
	if lockedBy != "" && lockedBy != userID && role != "admin" && role != "owner" {
		c.JSON(http.StatusConflict, gin.H{"error": "文档已被锁定"})
		return
	}

	body, err := readBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取内容失败"})
		return
	}

	// Try to parse JSON {"content":"..."} wrapper
	var contentReq struct {
		Content string `json:"content"`
	}
	contentBody := body
	if json.Unmarshal(body, &contentReq) == nil && contentReq.Content != "" {
		contentBody = []byte(contentReq.Content)
	}

	// Increment version
	var version int
	database.DB.QueryRow("SELECT version FROM md_documents WHERE id=?", docID).Scan(&version)
	version++

	_, err = database.DB.Exec(
		`UPDATE md_documents SET content_text=?, version=?, updated_by=?, updated_at=NOW() WHERE id=? AND team_id=?`,
		string(contentBody), version, userID, docID, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	writeDocContent(docID, contentBody)

	// Save version record
	versionPath := store.VersionPath(bucket, docID, version)
	database.DB.Exec(`INSERT INTO md_versions (id, document_id, version, file_path, file_size, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), docID, version, versionPath, int64(len(contentBody)), userID)
	service.PruneDocumentVersions(docID)

	audit(c, "edit_doc", "document", docID, "", fmt.Sprintf(`{"version":%d}`, version))
	c.JSON(http.StatusOK, gin.H{"message": "已保存", "version": version})
}

// ==================== 回收站 ====================

func TeamListTrash(c *gin.Context) {
	teamID := getTeamID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=0", teamID).Scan(&total)

	rows, err := database.DB.Query(
		`SELECT d.id, d.title, d.type, d.file_size, IFNULL(d.deleted_at, d.updated_at), d.created_by,
		 COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), '')
		 FROM md_documents d
		 LEFT JOIN users u ON d.created_by COLLATE utf8mb4_unicode_ci = u.id
		 WHERE d.team_id=? AND d.status=0
		 ORDER BY IFNULL(d.deleted_at, d.updated_at) DESC LIMIT ? OFFSET ?`,
		teamID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var docs []map[string]interface{}
	for rows.Next() {
		var id, title, docType, createdBy, createdByName string
		var fileSize int64
		var deletedAt string
		rows.Scan(&id, &title, &docType, &fileSize, &deletedAt, &createdBy, &createdByName)
		docs = append(docs, map[string]interface{}{
			"id": id, "title": title, "type": docType,
			"file_size": fileSize, "deleted_at": deletedAt, "updated_at": deletedAt,
			"created_by": createdBy, "created_by_name": createdByName,
		})
	}
	if docs == nil {
		docs = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": docs, "total": total})
}

func TeamRestoreFromTrash(c *gin.Context) {
	docID := c.Param("id")
	if !requireTrashDoc(c, docID) {
		return
	}
	_, err := database.DB.Exec(`UPDATE md_documents SET status=1, deleted_at=NULL WHERE id=? AND team_id=? AND status=0`, docID, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "restore_doc", "document", docID, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

func TeamPurgeFromTrash(c *gin.Context) {
	docID := c.Param("id")
	if !requireTrashDoc(c, docID) {
		return
	}
	_, err := database.DB.Exec(`DELETE FROM md_documents WHERE id=? AND team_id=? AND status=0`, docID, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "purge_doc", "document", docID, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已永久删除"})
}

func TeamEmptyTrash(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	res, err := database.DB.Exec(`DELETE FROM md_documents WHERE team_id=? AND status=0`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	count, _ := res.RowsAffected()
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("已永久删除 %d 个文档", count), "count": count})
}

// ==================== 收藏 ====================

func TeamListFavorites(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := getTeamID(c)
	rows, err := database.DB.Query(
		`SELECT d.id, d.title, d.type, d.updated_at
		 FROM md_favorites f
		 JOIN md_documents d ON f.document_id = d.id
		 WHERE f.user_id = ? AND d.team_id = ? AND d.status = 1`,
		userID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var docs []map[string]interface{}
	for rows.Next() {
		var id, title, docType, updatedAt string
		rows.Scan(&id, &title, &docType, &updatedAt)
		docs = append(docs, map[string]interface{}{
			"id": id, "title": title, "type": docType, "updated_at": updatedAt,
		})
	}
	if docs == nil {
		docs = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": docs})
}

func TeamAddFavorite(c *gin.Context) {
	userID := c.GetString("user_id")
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	_, err := database.DB.Exec(`INSERT IGNORE INTO md_favorites (user_id, document_id) VALUES (?, ?)`, userID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已收藏"})
}

func TeamRemoveFavorite(c *gin.Context) {
	userID := c.GetString("user_id")
	docID := c.Param("id")
	_, err := database.DB.Exec(`DELETE FROM md_favorites WHERE user_id=? AND document_id=?`, userID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已取消收藏"})
}

// ==================== 标签 ====================

func TeamListTags(c *gin.Context) {
	teamID := getTeamID(c)
	rows, err := database.DB.Query(`SELECT id, name, color FROM md_tags WHERE team_id=?`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var tags []map[string]interface{}
	for rows.Next() {
		var id, name, color string
		rows.Scan(&id, &name, &color)
		tags = append(tags, map[string]interface{}{"id": id, "name": name, "color": color})
	}
	if tags == nil {
		tags = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

func TeamCreateTag(c *gin.Context) {
	role := getTeamRole(c)
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	teamID := getTeamID(c)
	var req struct {
		Name  string `json:"name" binding:"required"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	id := uuid.New().String()
	_, err := database.DB.Exec(`INSERT INTO md_tags (id, team_id, user_id, name, color) VALUES (?, ?, ?, ?, ?)`,
		id, teamID, c.GetString("user_id"), req.Name, req.Color)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": req.Name, "color": req.Color}})
}

func TeamDeleteTag(c *gin.Context) {
	role := getTeamRole(c)
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	id := c.Param("id")
	teamID := getTeamID(c)
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_tags WHERE id=? AND team_id=?`, id, teamID).Scan(&n)
	if n == 0 {
		denyNotFound(c, "标签不存在")
		return
	}
	database.DB.Exec(`DELETE FROM md_doc_tags WHERE tag_id=?`, id)
	_, err := database.DB.Exec(`DELETE FROM md_tags WHERE id=? AND team_id=?`, id, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func TeamGetDocTags(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	rows, err := database.DB.Query(
		`SELECT t.id, t.name, t.color FROM md_doc_tags dt JOIN md_tags t ON dt.tag_id=t.id WHERE dt.document_id=?`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var tags []map[string]interface{}
	for rows.Next() {
		var id, name, color string
		rows.Scan(&id, &name, &color)
		tags = append(tags, map[string]interface{}{"id": id, "name": name, "color": color})
	}
	if tags == nil {
		tags = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

func TeamSetDocTags(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	var req struct {
		TagIDs []string `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	database.DB.Exec(`DELETE FROM md_doc_tags WHERE document_id=?`, docID)
	for _, tagID := range req.TagIDs {
		id := uuid.New().String()
		database.DB.Exec(`INSERT IGNORE INTO md_doc_tags (id, document_id, tag_id) VALUES (?, ?, ?)`, id, docID, tagID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// ==================== 版本 ====================

func TeamListVersions(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	rows, err := database.DB.Query(
		`SELECT v.id, v.version, v.file_size, v.created_by, v.created_at,
		 COALESCE(NULLIF(u.display_name,''), NULLIF(u.username,''), '') as user_name
		 FROM md_versions v LEFT JOIN users u ON v.created_by COLLATE utf8mb4_unicode_ci = u.id
		 WHERE v.document_id=? ORDER BY v.version DESC`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var versions []map[string]interface{}
	for rows.Next() {
		var id, createdBy, createdAt, userName string
		var version int
		var fileSize int64
		rows.Scan(&id, &version, &fileSize, &createdBy, &createdAt, &userName)
		versions = append(versions, map[string]interface{}{
			"id": id, "version": version, "file_size": fileSize,
			"created_by": createdBy, "created_at": createdAt,
			"user_name": userName, "created_by_name": userName,
		})
	}
	if versions == nil {
		versions = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": versions})
}

func TeamGetVersionContent(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	ver := c.Param("ver")
	verNum, err := strconv.Atoi(ver)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "版本号无效"})
		return
	}

	var teamID, deptID string
	database.DB.QueryRow("SELECT team_id, department_id FROM md_documents WHERE id=?", docID).Scan(&teamID, &deptID)
	bucket := teamID
	if bucket == "" {
		bucket = deptID
	}

	content, err := store.ReadVersion(bucket, docID, verNum)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}

func TeamRestoreVersion(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	var req struct {
		Version int `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定版本号"})
		return
	}
	userID := c.GetString("user_id")

	// Read version from file store
	var teamID, deptID string
	database.DB.QueryRow("SELECT team_id, department_id FROM md_documents WHERE id=?", docID).Scan(&teamID, &deptID)
	bucket := teamID
	if bucket == "" {
		bucket = deptID
	}
	content, err := store.ReadVersion(bucket, docID, req.Version)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}

	writeDocContent(docID, content)
	var version int
	database.DB.QueryRow("SELECT version FROM md_documents WHERE id=?", docID).Scan(&version)
	version++
	database.DB.Exec(`UPDATE md_documents SET content_text=?, version=?, updated_by=?, updated_at=NOW() WHERE id=? AND team_id=?`,
		string(content), version, userID, docID, getTeamID(c))
	audit(c, "restore", "document", docID, "", fmt.Sprintf(`{"version":%d}`, req.Version))
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

// ==================== 锁定 ====================

func TeamLockDocument(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	userID := c.GetString("user_id")
	_, err := database.DB.Exec(`UPDATE md_documents SET locked_by=?, locked_at=NOW() WHERE id=? AND team_id=?`, userID, docID, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已锁定"})
}

func TeamUnlockDocument(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	_, err := database.DB.Exec(`UPDATE md_documents SET locked_by='', locked_at=NULL WHERE id=? AND team_id=?`, docID, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已解锁"})
}

// ==================== 分享 ====================

func TeamCreateShare(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	userID := c.GetString("user_id")
	teamID := getTeamID(c)

	var req struct {
		Permission string `json:"permission"`
		Expires    string `json:"expires"`
		ExpiresIn  int    `json:"expiresIn"`  // hours; the editor sends this
		ExpiresIn2 int    `json:"expires_in"` // same meaning, snake_case
		Password   string `json:"password"`
	}
	c.ShouldBindJSON(&req)
	if req.Permission == "" {
		req.Permission = "read"
	}

	hours := req.ExpiresIn
	if hours == 0 {
		hours = req.ExpiresIn2
	}

	token := uuid.New().String()
	id := uuid.New().String()
	shareURL := "/s/" + token
	var err error
	switch {
	case req.Expires != "":
		_, err = database.DB.Exec(
			`INSERT INTO md_shares (id, document_id, team_id, token, permission, password, expires_at, created_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, docID, teamID, token, req.Permission, req.Password, req.Expires, userID)
	case hours > 0:
		expiresAt := time.Now().Add(time.Duration(hours) * time.Hour)
		_, err = database.DB.Exec(
			`INSERT INTO md_shares (id, document_id, team_id, token, permission, password, expires_at, created_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, docID, teamID, token, req.Permission, req.Password, expiresAt, userID)
	default:
		_, err = database.DB.Exec(
			`INSERT INTO md_shares (id, document_id, team_id, token, permission, password, expires_at, created_by)
			 VALUES (?, ?, ?, ?, ?, ?, NULL, ?)`,
			id, docID, teamID, token, req.Permission, req.Password, userID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": id, "token": token, "permission": req.Permission, "share_url": shareURL,
	}})
}

func TeamListShares(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	rows, err := database.DB.Query(
		`SELECT id, token, password, IFNULL(permission,''), expires_at, created_by, created_at, access_count
		 FROM md_shares WHERE document_id=? AND status=1 ORDER BY created_at DESC`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var shares []map[string]interface{}
	for rows.Next() {
		var id, token, perm, createdBy string
		var createdAt time.Time
		var password sql.NullString
		var expiresAt sql.NullTime
		var accessCount int
		if rows.Scan(&id, &token, &password, &perm, &expiresAt, &createdBy, &createdAt, &accessCount) != nil {
			continue
		}
		shares = append(shares, map[string]interface{}{
			"id": id, "token": token, "permission": perm,
			"has_password": password.Valid && password.String != "",
			"expires_at":   expiresAt.Time,
			"expired":      expiresAt.Valid && expiresAt.Time.Before(time.Now()),
			"created_by":   createdBy, "created_at": createdAt,
			"access_count": accessCount,
		})
	}
	if shares == nil {
		shares = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": shares})
}

// ==================== 协作者 ====================

func TeamListCollaborators(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	rows, err := database.DB.Query(
		`SELECT p.id, p.target_type, p.target_id, p.permission, p.created_by,
		 COALESCE(NULLIF(u.display_name,''), NULLIF(u.username,''), '') as user_name
		 FROM md_permissions p
		 LEFT JOIN users u ON p.target_type='user' AND p.target_id COLLATE utf8mb4_unicode_ci = u.id
		 WHERE p.resource_type='document' AND p.resource_id=?`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var collabs []map[string]interface{}
	for rows.Next() {
		var id, targetType, targetID, perm, createdBy, userName string
		if rows.Scan(&id, &targetType, &targetID, &perm, &createdBy, &userName) != nil {
			continue
		}
		collabs = append(collabs, map[string]interface{}{
			"id": id, "target_type": targetType, "target_id": targetID,
			"permission": perm, "role": service.FrontendRole(perm),
			"target_name": userName, "user_name": userName,
			"created_by": createdBy,
		})
	}
	if collabs == nil {
		collabs = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": collabs})
}

func TeamAddCollaborator(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	userID := c.GetString("user_id")
	var req struct {
		TargetID string `json:"target_id"`
		Perm     string `json:"permission"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.TargetID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	perm, ok := resolveCollabPermission(req.Perm, req.Role)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的角色"})
		return
	}
	var exists int
	database.DB.QueryRow(`SELECT 1 FROM users WHERE id COLLATE utf8mb4_unicode_ci=?`, req.TargetID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户不存在"})
		return
	}
	var member int
	database.DB.QueryRow(`SELECT 1 FROM team_members WHERE team_id=? AND user_id=?`, getTeamID(c), req.TargetID).Scan(&member)
	if member == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能添加本团队成员"})
		return
	}
	id := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_permissions (id, resource_type, resource_id, target_type, target_id, permission, inherit, created_by)
		 VALUES (?, 'document', ?, 'user', ?, ?, 0, ?)
		 ON DUPLICATE KEY UPDATE permission=VALUES(permission)`,
		id, docID, req.TargetID, perm, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已添加", "permission": perm, "role": service.FrontendRole(perm)})
}

// ==================== 评论 ====================

func TeamListComments(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	rows, err := database.DB.Query(
		`SELECT c.id, c.content, c.user_id, IFNULL(c.parent_id,''), c.created_at, c.updated_at,
		 COALESCE(NULLIF(u.display_name,''), NULLIF(u.username,''), NULLIF(c.user_name,''), '') as user_name
		 FROM md_comments c LEFT JOIN users u ON c.user_id COLLATE utf8mb4_unicode_ci = u.id
		 WHERE c.document_id=? ORDER BY c.created_at`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var comments []map[string]interface{}
	for rows.Next() {
		var id, content, userID, parentID, createdAt, updatedAt, userName string
		rows.Scan(&id, &content, &userID, &parentID, &createdAt, &updatedAt, &userName)
		comments = append(comments, map[string]interface{}{
			"id": id, "content": content, "user_id": userID,
			"parent_id": parentID, "created_at": createdAt,
			"updated_at": updatedAt, "user_name": userName,
		})
	}
	if comments == nil {
		comments = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": comments})
}

func TeamCreateComment(c *gin.Context) {
	docID := c.Param("id")
	// comment 及以上可以留言；只读成员不能。
	if !requireDoc(c, docID, "comment", true) {
		return
	}
	userID := c.GetString("user_id")
	teamID := getTeamID(c)
	var req struct {
		Content  string `json:"content" binding:"required"`
		ParentID string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	userName := firstNonEmpty(c.GetString("display_name"), c.GetString("username"))
	id := uuid.New().String()
	now := time.Now()
	_, err := database.DB.Exec(
		`INSERT INTO md_comments (id, team_id, document_id, user_id, user_name, content, parent_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, teamID, docID, userID, userName, req.Content, req.ParentID, now, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": id, "content": req.Content, "user_id": userID, "user_name": userName,
	}})
}

// ==================== 导出 ====================

func TeamExportDocument(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	format := c.DefaultQuery("format", "html")
	var title, docType string
	err := database.DB.QueryRow(`SELECT title, type FROM md_documents WHERE id=? AND team_id=? AND status=1`, docID, getTeamID(c)).Scan(&title, &docType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}
	content := readDocContent(docID)
	if !serveDocumentExport(c, title, string(content), format) {
		return
	}
	userName, _ := c.Get("username")
	audit(c, "export", "document", docID, title, fmt.Sprintf("%s 导出文档 (%s, %s)", userName, format, docType))
}

// ==================== 审计 ====================

// buildAuditFilter turns the audit page query into a SQL fragment.
// Dates must be YYYY-MM-DD; anything else is ignored so a bad filter cannot error the query.
func buildAuditFilter(action, userName, startDate, endDate string) (string, []interface{}) {
	where := ""
	args := []interface{}{}
	if validAuditAction(action) {
		where += " AND a.action=?"
		args = append(args, action)
	}
	if name := strings.TrimSpace(userName); name != "" {
		runes := []rune(name)
		if len(runes) > 50 {
			name = string(runes[:50])
		}
		name = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(name)
		where += " AND (IFNULL(u.display_name,'') LIKE ? OR IFNULL(a.user_name,'') LIKE ?)"
		pat := "%" + name + "%"
		args = append(args, pat, pat)
	}
	if _, err := time.Parse("2006-01-02", startDate); err == nil {
		where += " AND a.created_at>=?"
		args = append(args, startDate+" 00:00:00")
	}
	if _, err := time.Parse("2006-01-02", endDate); err == nil {
		where += " AND a.created_at<=?"
		args = append(args, endDate+" 23:59:59")
	}
	return where, args
}

func validAuditAction(action string) bool {
	if action == "" || len(action) > 30 {
		return false
	}
	for _, r := range action {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func auditFromClause() string {
	return ` FROM md_audits a LEFT JOIN users u ON a.user_id COLLATE utf8mb4_unicode_ci = u.id WHERE a.team_id=?`
}

func TeamListAudits(c *gin.Context) {
	if !service.RequirePlanFeature(c, "audit") {
		return
	}
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	filter, filterArgs := buildAuditFilter(c.Query("action"), c.Query("user_name"), c.Query("start_date"), c.Query("end_date"))

	var total int
	countArgs := append([]interface{}{teamID}, filterArgs...)
	database.DB.QueryRow("SELECT COUNT(*)"+auditFromClause()+filter, countArgs...).Scan(&total)

	listArgs := append([]interface{}{teamID}, filterArgs...)
	listArgs = append(listArgs, pageSize, offset)
	rows, err := database.DB.Query(
		`SELECT a.id, a.user_id, a.action, a.resource_type, a.resource_id, a.resource_name, a.detail, IFNULL(a.ip,''), a.created_at,
		 COALESCE(NULLIF(u.display_name,''), NULLIF(a.user_name,''), '') as user_name`+
			auditFromClause()+filter+` ORDER BY a.created_at DESC LIMIT ? OFFSET ?`,
		listArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var audits []map[string]interface{}
	for rows.Next() {
		var id, userID, action, resType, resID, resName, detail, ip, createdAt, userName string
		rows.Scan(&id, &userID, &action, &resType, &resID, &resName, &detail, &ip, &createdAt, &userName)
		audits = append(audits, map[string]interface{}{
			"id": id, "user_id": userID, "action": action,
			"resource_type": resType, "resource_id": resID,
			"resource_name": resName, "detail": detail, "ip": ip,
			"created_at": createdAt, "user_name": userName,
		})
	}
	if audits == nil {
		audits = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": audits, "total": total, "page": page, "page_size": pageSize})
}

func TeamExportAudits(c *gin.Context) {
	if !service.RequirePlanFeature(c, "audit") {
		return
	}
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	filter, filterArgs := buildAuditFilter(c.Query("action"), c.Query("user_name"), c.Query("start_date"), c.Query("end_date"))
	args := append([]interface{}{teamID}, filterArgs...)
	rows, err := database.DB.Query(
		`SELECT a.created_at, COALESCE(NULLIF(u.display_name,''), NULLIF(a.user_name,''), ''),
		 a.action, a.resource_type, IFNULL(a.resource_name,''), IFNULL(a.detail,''), IFNULL(a.ip,'')`+
			auditFromClause()+filter+` ORDER BY a.created_at DESC LIMIT 10000`, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=audits.csv")
	c.Writer.Write([]byte("\xEF\xBB\xBF"))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"时间", "用户", "操作", "资源类型", "资源名称", "详情", "IP"})
	for rows.Next() {
		var createdAt, userName, action, resType, resName, detail, ip string
		if rows.Scan(&createdAt, &userName, &action, &resType, &resName, &detail, &ip) != nil {
			continue
		}
		_ = w.Write([]string{createdAt, userName, action, resType, resName, detail, ip})
	}
	w.Flush()
}

func TeamAuditStats(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_audits WHERE team_id=?", teamID).Scan(&total)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"total": total}})
}

// ==================== 权限 ====================

func TeamListPermissions(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	resType := c.Query("resource_type")
	resID := c.Query("resource_id")
	if resType != "document" && resType != "folder" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 resource_type"})
		return
	}
	if resID != "" && !resourceInTeam(teamID, resType, resID) {
		denyNotFound(c, "资源不存在")
		return
	}
	q := `SELECT p.id, p.resource_type, p.resource_id, p.target_type, p.target_id, p.permission, p.inherit, IFNULL(p.created_by,''),
		 IFNULL(u.display_name,'') as user_name,
		 CASE WHEN p.resource_type='document' THEN IFNULL(d.title,'') ELSE IFNULL(f.name,'') END as resource_name
		 FROM md_permissions p
		 LEFT JOIN users u ON p.target_type='user' AND p.target_id COLLATE utf8mb4_unicode_ci = u.id
		 LEFT JOIN md_documents d ON p.resource_type='document' AND p.resource_id = d.id AND d.team_id=?
		 LEFT JOIN md_team_folders f ON p.resource_type='folder' AND p.resource_id = f.id AND f.team_id=?
		 WHERE p.resource_type=?`
	args := []interface{}{teamID, teamID, resType}
	if resID != "" {
		q += ` AND p.resource_id=?`
		args = append(args, resID)
	} else if resType == "document" {
		q += ` AND d.id IS NOT NULL`
	} else {
		q += ` AND f.id IS NOT NULL`
	}
	q += ` ORDER BY p.created_at DESC LIMIT 200`
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var perms []map[string]interface{}
	for rows.Next() {
		var id, rType, rID, tType, tID, perm, createdBy, userName, resourceName string
		var inherit bool
		rows.Scan(&id, &rType, &rID, &tType, &tID, &perm, &inherit, &createdBy, &userName, &resourceName)
		perms = append(perms, map[string]interface{}{
			"id": id, "resource_type": rType, "resource_id": rID,
			"target_type": tType, "target_id": tID,
			"permission": perm, "inherit": inherit,
			"created_by": createdBy, "user_name": userName,
			"resource_name": resourceName,
		})
	}
	if perms == nil {
		perms = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": perms})
}

func TeamSetPermission(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	var req struct {
		ResourceType string `json:"resource_type" binding:"required"`
		ResourceID   string `json:"resource_id" binding:"required"`
		TargetType   string `json:"target_type" binding:"required"`
		TargetID     string `json:"target_id" binding:"required"`
		Permission   string `json:"permission" binding:"required"`
		Inherit      bool   `json:"inherit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.TargetType != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目前只支持按成员授权"})
		return
	}
	perm, ok := service.NormalizePermission(req.Permission)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的权限"})
		return
	}
	teamID := getTeamID(c)
	if !resourceInTeam(teamID, req.ResourceType, req.ResourceID) {
		denyNotFound(c, "资源不存在")
		return
	}
	var member int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM team_members WHERE team_id=? AND user_id=?`, teamID, req.TargetID).Scan(&member); err != nil || member == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户不在团队中"})
		return
	}
	req.Permission = perm
	id := uuid.New().String()
	userID := c.GetString("user_id")
	_, err := database.DB.Exec(
		`INSERT INTO md_permissions (id, resource_type, resource_id, target_type, target_id, permission, inherit, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE permission=VALUES(permission), inherit=VALUES(inherit)`,
		id, req.ResourceType, req.ResourceID, req.TargetType, req.TargetID, req.Permission, req.Inherit, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "set_permission", req.ResourceType, req.ResourceID, resourceTitle(teamID, req.ResourceType, req.ResourceID),
		fmt.Sprintf(`{"target_id":"%s","permission":"%s"}`, req.TargetID, req.Permission))
	c.JSON(http.StatusOK, gin.H{"data": req})
}

func resourceTitle(teamID, resType, resID string) string {
	var name string
	switch resType {
	case "document":
		database.DB.QueryRow(`SELECT title FROM md_documents WHERE id=? AND team_id=?`, resID, teamID).Scan(&name)
	case "folder":
		database.DB.QueryRow(`SELECT name FROM md_team_folders WHERE id=? AND team_id=?`, resID, teamID).Scan(&name)
	}
	return name
}

func TeamRemovePermission(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	id := c.Param("id")
	resType, resID, ok := permissionResource(id)
	if !ok || !resourceInTeam(getTeamID(c), resType, resID) {
		denyNotFound(c, "权限不存在")
		return
	}
	name := resourceTitle(getTeamID(c), resType, resID)
	_, err := database.DB.Exec(`DELETE FROM md_permissions WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "remove_permission", resType, resID, name, "")
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func TeamCheckPermission(c *gin.Context) {
	userID := c.GetString("user_id")
	resType := c.DefaultQuery("resource_type", "document")
	resID := c.Query("resource_id")
	if resID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 resource_id"})
		return
	}

	if !resourceInTeam(getTeamID(c), resType, resID) {
		denyNotFound(c, "资源不存在")
		return
	}
	perm := service.CheckTeamPermission(c.Request.Context(), userID, getTeamID(c), getTeamRole(c), resType, resID)
	c.JSON(http.StatusOK, gin.H{"permission": perm})
}

// ==================== 模板 ====================

func TeamListTemplates(c *gin.Context) {
	teamID := getTeamID(c)
	rows, err := database.DB.Query(
		`SELECT id, name, type, is_public, created_at, updated_at
		 FROM md_templates WHERE team_id=?`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var templates []map[string]interface{}
	for rows.Next() {
		var id, name, docType, createdAt, updatedAt string
		var isPublic bool
		rows.Scan(&id, &name, &docType, &isPublic, &createdAt, &updatedAt)
		templates = append(templates, map[string]interface{}{
			"id": id, "name": name, "type": docType,
			"is_public": isPublic, "created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if templates == nil {
		templates = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

func TeamGetTemplate(c *gin.Context) {
	id := c.Param("id")
	var name, docType, content string
	err := database.DB.QueryRow(`SELECT name, type, content FROM md_templates WHERE id=? AND team_id=?`, id, getTeamID(c)).Scan(&name, &docType, &content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": name, "type": docType, "content": content}})
}

func TeamCreateTemplate(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	var req struct {
		Name    string `json:"name" binding:"required"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	id := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_templates (id, team_id, name, type, content, user_id) VALUES (?, ?, ?, ?, ?, ?)`,
		id, teamID, req.Name, req.Type, req.Content, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": req.Name}})
}

func TeamUpdateTemplate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	_, err := database.DB.Exec(`UPDATE md_templates SET name=?, content=? WHERE id=? AND team_id=?`, req.Name, req.Content, id, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func TeamDeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(`DELETE FROM md_templates WHERE id=? AND team_id=?`, id, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== 存储 ====================

// TeamStorageStatus 返回团队存储用量总览（活跃文档、回收站、版本历史、媒体附件）
// 以及配额占用情况（配额取当前用户所属套餐，与创建时的拦截口径一致）。
func TeamStorageStatus(c *gin.Context) {
	teamID := getTeamID(c)

	// 活跃文档占用（status=1）
	var activeSize int64
	var activeCount int
	database.DB.QueryRow(
		`SELECT COALESCE(SUM(file_size), 0) FROM md_documents WHERE team_id=? AND status=1`,
		teamID).Scan(&activeSize)
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=1`,
		teamID).Scan(&activeCount)

	// 回收站占用（status=0，文件仍在磁盘）
	var trashSize int64
	var trashCount int
	database.DB.QueryRow(
		`SELECT COALESCE(SUM(file_size), 0) FROM md_documents WHERE team_id=? AND status=0`,
		teamID).Scan(&trashSize)
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=0`,
		teamID).Scan(&trashCount)

	// 文档类型分布（活跃）
	byType := map[string]int64{}
	rows, err := database.DB.Query(
		`SELECT type, COALESCE(SUM(file_size), 0) FROM md_documents WHERE team_id=? AND status=1 GROUP BY type`,
		teamID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var s int64
			if rows.Scan(&t, &s) == nil {
				byType[t] = s
			}
		}
	}

	// 版本历史额外占用 = 团队全部版本文件 - 当前文档文件
	// （回收到归档态的文档仍保留版本文件，故版本统计覆盖 status IN (0,1)）
	var versionAll int64
	database.DB.QueryRow(
		`SELECT COALESCE(SUM(v.file_size), 0) FROM md_versions v
		 JOIN md_documents d ON v.document_id = d.id WHERE d.team_id=? AND d.status IN (0,1)`,
		teamID).Scan(&versionAll)
	versionExtra := versionAll - activeSize
	if versionExtra < 0 {
		versionExtra = 0
	}

	// 媒体附件占用（<root>/<teamID>/media/）
	var mediaSize int64
	mediaDir := fmt.Sprintf("%s/%s/media", store.RootPath(), teamID)
	if entries, err := os.ReadDir(mediaDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if info, err := e.Info(); err == nil {
				mediaSize += info.Size()
			}
		}
	}

	// 总占用（不含回收站）
	totalBytes := activeSize + versionExtra + mediaSize

	// 配额：取当前用户套餐（0 = 不限）
	userID := c.GetString("user_id")
	limits := service.GetPlanLimits(userID)
	maxBytes := int64(limits.MaxStorageMB) * 1024 * 1024

	// 组装配额信息
	quota := gin.H{
		"max_mb":        limits.MaxStorageMB,
		"max_bytes":     maxBytes,
		"unlimited":     limits.MaxStorageMB == 0,
		"usage_percent": nil,
		"used_bytes":    totalBytes,
		"used_human":    formatBytes(totalBytes),
	}
	if limits.MaxStorageMB != 0 {
		usagePercent := float64(totalBytes) / float64(maxBytes) * 100
		quota["usage_percent"] = usagePercent
		quota["max_human"] = formatBytes(maxBytes)
		quota["remaining_bytes"] = maxBytes - totalBytes
		quota["remaining_human"] = formatBytes(maxBytes - totalBytes)
	}

	// 请求者是否团队管理员（仅管理员可写，前端据此展示）
	role := getTeamRole(c)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"team_id":   teamID,
		"doc_count": activeCount,
		"usage": gin.H{
			"total_bytes": totalBytes,
			"total_human": formatBytes(totalBytes),
			"documents":   gin.H{"bytes": activeSize, "count": activeCount, "human": formatBytes(activeSize)},
			"versions":    gin.H{"bytes": versionExtra, "human": formatBytes(versionExtra)},
			"media":       gin.H{"bytes": mediaSize, "human": formatBytes(mediaSize)},
			"trash":       gin.H{"bytes": trashSize, "count": trashCount, "human": formatBytes(trashSize)},
			"by_type":     byType,
		},
		// 向后兼容字段
		"quota":      quota,
		"role":       role,
		"is_admin":   role == "admin" || role == "owner",
		"total_size": activeSize,
	}})
}

// ==================== Webhooks ====================

func TeamListWebhooks(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	rows, err := database.DB.Query(
		`SELECT id, name, url, events, enabled, created_at, updated_at
		 FROM md_webhooks WHERE team_id=? ORDER BY created_at DESC`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var webhooks []map[string]interface{}
	for rows.Next() {
		var id, name, url, events, createdAt, updatedAt string
		var enabled bool
		rows.Scan(&id, &name, &url, &events, &enabled, &createdAt, &updatedAt)
		webhooks = append(webhooks, map[string]interface{}{
			"id": id, "name": name, "url": url, "events": events,
			"enabled": enabled, "created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if webhooks == nil {
		webhooks = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": webhooks})
}

func TeamCreateWebhook(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	var req struct {
		Name   string `json:"name" binding:"required"`
		URL    string `json:"url" binding:"required"`
		Events string `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if !validWebhookURL(req.URL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Webhook 地址必须是 http 或 https"})
		return
	}
	if req.Events == "" {
		req.Events = `["document.created","document.updated"]`
	}
	id := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_webhooks (id, team_id, name, url, events, enabled, created_by) VALUES (?, ?, ?, ?, ?, 1, ?)`,
		id, teamID, req.Name, req.URL, req.Events, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": req.Name, "url": req.URL}})
}

func TeamDeleteWebhook(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	id := c.Param("id")
	res, err := database.DB.Exec(`DELETE FROM md_webhooks WHERE id=? AND team_id=?`, id, getTeamID(c))
	if err == nil {
		if n, _ := res.RowsAffected(); n == 0 {
			denyNotFound(c, "Webhook 不存在")
			return
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func TeamToggleWebhook(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	id := c.Param("id")
	var enabled bool
	err := database.DB.QueryRow("SELECT enabled FROM md_webhooks WHERE id=? AND team_id=?", id, getTeamID(c)).Scan(&enabled)
	if err != nil {
		denyNotFound(c, "Webhook 不存在")
		return
	}
	_, err = database.DB.Exec(`UPDATE md_webhooks SET enabled=? WHERE id=? AND team_id=?`, !enabled, id, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已切换", "enabled": !enabled})
}

func TeamListWebhookLogs(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	webhookID := c.Param("id")
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_webhooks WHERE id=? AND team_id=?`, webhookID, getTeamID(c)).Scan(&n)
	if n == 0 {
		denyNotFound(c, "Webhook 不存在")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_webhook_logs WHERE webhook_id=?", webhookID).Scan(&total)

	rows, err := database.DB.Query(
		`SELECT id, event, status, created_at
		 FROM md_webhook_logs WHERE webhook_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		webhookID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"data": []interface{}{}, "total": 0})
		return
	}
	defer rows.Close()
	var logs []map[string]interface{}
	for rows.Next() {
		var id, event, status, createdAt string
		rows.Scan(&id, &event, &status, &createdAt)
		logs = append(logs, map[string]interface{}{
			"id": id, "event": event, "status": status, "created_at": createdAt,
		})
	}
	if logs == nil {
		logs = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": logs, "total": total})
}

// ==================== 文档统计 ====================

func TeamDocStats(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	var version int
	var fileSize int64
	database.DB.QueryRow(`SELECT version, file_size FROM md_documents WHERE id=? AND team_id=?`, docID, getTeamID(c)).Scan(&version, &fileSize)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"version": version, "file_size": fileSize}})
}

// ==================== Helpers ====================

func writeDocContent(docID string, content []byte) {
	var teamID, deptID string
	database.DB.QueryRow("SELECT team_id, department_id FROM md_documents WHERE id=?", docID).Scan(&teamID, &deptID)
	bucket := teamID
	if bucket == "" {
		bucket = deptID // fallback for legacy docs
	}

	var version int
	database.DB.QueryRow("SELECT version FROM md_documents WHERE id=?", docID).Scan(&version)
	if version < 1 {
		version = 1
	}

	_, _, err := store.WriteVersion(bucket, docID, version, content)
	if err != nil {
		log.Printf("writeDocContent error: %v", err)
	}
}

// teamStorageBytes matches the storage page: active documents, extra versions, and media files.
// Trash is excluded, same as TeamStorageStatus.
func teamStorageBytes(teamID string) int64 {
	var activeSize, versionAll, mediaSize int64
	database.DB.QueryRow(
		`SELECT COALESCE(SUM(file_size), 0) FROM md_documents WHERE team_id=? AND status=1`,
		teamID).Scan(&activeSize)
	database.DB.QueryRow(
		`SELECT COALESCE(SUM(v.file_size), 0) FROM md_versions v
		 JOIN md_documents d ON v.document_id = d.id WHERE d.team_id=? AND d.status IN (0,1)`,
		teamID).Scan(&versionAll)
	versionExtra := versionAll - activeSize
	if versionExtra < 0 {
		versionExtra = 0
	}
	mediaDir := fmt.Sprintf("%s/%s/media", store.RootPath(), teamID)
	if entries, err := os.ReadDir(mediaDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if info, err := e.Info(); err == nil {
				mediaSize += info.Size()
			}
		}
	}
	return activeSize + versionExtra + mediaSize
}

func readDocContent(docID string) []byte {
	var teamID, deptID string
	database.DB.QueryRow("SELECT team_id, department_id FROM md_documents WHERE id=?", docID).Scan(&teamID, &deptID)
	bucket := teamID
	if bucket == "" {
		bucket = deptID // fallback for legacy docs
	}
	data, err := store.ReadCurrent(bucket, docID)
	if err != nil {
		log.Printf("readDocContent error (bucket=%s): %v", bucket, err)
		return []byte{}
	}
	return data
}

func readBody(c *gin.Context) ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

// Ensure model types are referenced (avoid unused import errors during dev)
var _ = model.Document{}

// ==================== Media Upload (Team-scoped) ====================

func TeamUploadFile(c *gin.Context) {
	role := getTeamRole(c)
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	teamID := getTeamID(c)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
		return
	}
	defer file.Close()

	if !service.CheckStorageLimit(c, teamStorageBytes(teamID), header.Size) {
		return
	}

	// Generate unique filename
	ext := ""
	if idx := strings.LastIndex(header.Filename, "."); idx >= 0 {
		ext = header.Filename[idx:]
	}
	filename := uuid.New().String() + ext

	// Save to team storage
	dir := fmt.Sprintf("%s/%s/media", store.RootPath(), teamID)
	os.MkdirAll(dir, 0755)
	path := dir + "/" + filename

	f, err := os.Create(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	io.Copy(f, file)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"filename": filename,
		"original": header.Filename,
		"size":     header.Size,
		"url":      "/api/teams/" + teamID + "/media/" + filename,
	}})
}

func TeamListMedia(c *gin.Context) {
	teamID := getTeamID(c)
	dir := fmt.Sprintf("%s/%s/media", store.RootPath(), teamID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	var files []map[string]interface{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		files = append(files, map[string]interface{}{
			"filename": e.Name(),
			"size":     info.Size(),
			"modified": info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	if files == nil {
		files = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": files})
}

func TeamDeleteMedia(c *gin.Context) {
	teamID := getTeamID(c)
	filename, ok := safeBaseName(c.Param("filename"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件名无效"})
		return
	}
	path := filepath.Join(store.RootPath(), teamID, "media", filename)
	os.Remove(path)
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func TeamGetMedia(c *gin.Context) {
	teamID := getTeamID(c)
	filename, ok := safeBaseName(c.Param("filename"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件名无效"})
		return
	}
	path := filepath.Join(store.RootPath(), teamID, "media", filename)
	c.File(path)
}

// ==================== Notifications (Team-scoped) ====================

func TeamListNotifications(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := getTeamID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_notifications WHERE user_id=? AND team_id=?", userID, teamID).Scan(&total)

	rows, err := database.DB.Query(
		`SELECT id, type, title, is_read, created_at
		 FROM md_notifications WHERE user_id=? AND team_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		userID, teamID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var notifications []map[string]interface{}
	for rows.Next() {
		var id, nType, title, createdAt string
		var isRead bool
		rows.Scan(&id, &nType, &title, &isRead, &createdAt)
		notifications = append(notifications, map[string]interface{}{
			"id": id, "type": nType, "title": title,
			"is_read": isRead, "created_at": createdAt,
		})
	}
	if notifications == nil {
		notifications = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": notifications, "total": total})
}

func TeamMarkNotificationRead(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	teamID := getTeamID(c)
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_notifications WHERE id=? AND user_id=? AND team_id=?`, id, userID, teamID).Scan(&n)
	if n == 0 {
		denyNotFound(c, "通知不存在")
		return
	}
	if _, err := database.DB.Exec(`UPDATE md_notifications SET is_read=1 WHERE id=? AND user_id=? AND team_id=?`, id, userID, teamID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已标记已读"})
}

func TeamMarkAllNotificationsRead(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := getTeamID(c)
	database.DB.Exec(`UPDATE md_notifications SET is_read=1 WHERE user_id=? AND team_id=?`, userID, teamID)
	c.JSON(http.StatusOK, gin.H{"message": "已全部标记已读"})
}

func TeamDeleteNotification(c *gin.Context) {
	id := c.Param("id")
	res, err := database.DB.Exec(`DELETE FROM md_notifications WHERE id=? AND user_id=? AND team_id=?`, id, c.GetString("user_id"), getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		denyNotFound(c, "通知不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func TeamUnreadCount(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := getTeamID(c)
	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_notifications WHERE user_id=? AND team_id=? AND is_read=0", userID, teamID).Scan(&count)
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// ==================== Shares (delete) ====================

func TeamDeleteShare(c *gin.Context) {
	id := c.Param("id")
	var docID string
	err := database.DB.QueryRow(
		`SELECT s.document_id FROM md_shares s JOIN md_documents d ON s.document_id=d.id WHERE s.id=? AND d.team_id=?`,
		id, getTeamID(c)).Scan(&docID)
	if err != nil {
		denyNotFound(c, "分享不存在")
		return
	}
	if !requireDoc(c, docID, "write", true) {
		return
	}
	_, err = database.DB.Exec(`DELETE FROM md_shares WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== Collaborators (update/delete) ====================

func TeamUpdateCollaborator(c *gin.Context) {
	id := c.Param("id")
	resType, resID, ok := permissionResource(id)
	if !ok || resType != "document" || !documentInTeam(getTeamID(c), resID, true) {
		denyNotFound(c, "协作者不存在")
		return
	}
	if !requireDoc(c, resID, "write", true) {
		return
	}
	var req struct {
		Permission string `json:"permission"`
		Role       string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	perm, ok := resolveCollabPermission(req.Permission, req.Role)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的角色"})
		return
	}
	_, err := database.DB.Exec(`UPDATE md_permissions SET permission=? WHERE id=?`, perm, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func TeamRemoveCollaborator(c *gin.Context) {
	id := c.Param("id")
	resType, resID, ok := permissionResource(id)
	if !ok || resType != "document" || !documentInTeam(getTeamID(c), resID, true) {
		denyNotFound(c, "协作者不存在")
		return
	}
	if !requireDoc(c, resID, "write", true) {
		return
	}
	_, err := database.DB.Exec(`DELETE FROM md_permissions WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已移除"})
}

// ==================== Comments (update/delete) ====================

func TeamUpdateComment(c *gin.Context) {
	id := c.Param("id")
	if !commentEditable(c, id) {
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	_, err := database.DB.Exec(`UPDATE md_comments SET content=?, updated_at=NOW() WHERE id=?`, req.Content, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func TeamDeleteComment(c *gin.Context) {
	id := c.Param("id")
	if !commentEditable(c, id) {
		return
	}
	_, err := database.DB.Exec(`DELETE FROM md_comments WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== Team members ====================

func TeamListMembers(c *gin.Context) {
	rows, err := database.DB.Query(
		`SELECT u.id, IFNULL(u.username,''), IFNULL(u.display_name,''), IFNULL(u.email,''), tm.role
		 FROM team_members tm
		 JOIN users u ON tm.user_id COLLATE utf8mb4_unicode_ci = u.id
		 WHERE tm.team_id=?
		 ORDER BY u.display_name
		 LIMIT 200`, getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var members []map[string]interface{}
	for rows.Next() {
		var id, username, name, email, role string
		rows.Scan(&id, &username, &name, &email, &role)
		members = append(members, map[string]interface{}{
			"id": id, "username": username, "display_name": name, "email": email, "role": role,
		})
	}
	if members == nil {
		members = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": members})
}

// ==================== Search Targets ====================

func TeamSearchTargets(c *gin.Context) {
	teamID := getTeamID(c)
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	pattern := "%" + q + "%"
	rows, err := database.DB.Query(
		`SELECT id, display_name, email FROM users
		 WHERE (display_name LIKE ? OR email LIKE ? OR username LIKE ?)
		 AND id IN (SELECT user_id FROM team_members WHERE team_id=?)
		 LIMIT 20`, pattern, pattern, pattern, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var targets []map[string]interface{}
	for rows.Next() {
		var id, name, email string
		rows.Scan(&id, &name, &email)
		targets = append(targets, map[string]interface{}{
			"id": id, "display_name": name, "email": email,
		})
	}
	if targets == nil {
		targets = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": targets})
}

// ==================== Tags (documents by tag) ====================

func TeamGetDocsByTag(c *gin.Context) {
	teamID := getTeamID(c)
	tagID := c.Param("id")
	rows, err := database.DB.Query(
		`SELECT d.id, d.title, d.type, d.updated_at
		 FROM md_doc_tags dt
		 JOIN md_documents d ON dt.document_id = d.id
		 WHERE dt.tag_id = ? AND d.team_id = ? AND d.status = 1
		 ORDER BY d.updated_at DESC`, tagID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var docs []map[string]interface{}
	for rows.Next() {
		var id, title, docType, updatedAt string
		rows.Scan(&id, &title, &docType, &updatedAt)
		docs = append(docs, map[string]interface{}{
			"id": id, "title": title, "type": docType, "updated_at": updatedAt,
		})
	}
	if docs == nil {
		docs = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": docs})
}

// ==================== Dashboard ====================

func TeamDashboardStats(c *gin.Context) {
	teamID := getTeamID(c)
	var docCount, sheetCount, trashCount, userCount, shareCount, commentCount, weekNew, deptCount int

	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=1", teamID).Scan(&docCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=1 AND type='sheet'", teamID).Scan(&sheetCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=0", teamID).Scan(&trashCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM team_members WHERE team_id=?", teamID).Scan(&userCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM md_shares s JOIN md_documents d ON s.document_id=d.id WHERE d.team_id=?", teamID).Scan(&shareCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM md_comments c JOIN md_documents d ON c.document_id=d.id WHERE d.team_id=?", teamID).Scan(&commentCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=1 AND created_at > DATE_SUB(NOW(), INTERVAL 7 DAY)", teamID).Scan(&weekNew)
	database.DB.QueryRow("SELECT COUNT(*) FROM md_team_folders WHERE team_id=?", teamID).Scan(&deptCount)

	// Daily new docs (last 7 days)
	rows, _ := database.DB.Query(
		`SELECT DATE(created_at) as date, COUNT(*) as count FROM md_documents
		 WHERE team_id=? AND status=1 AND created_at > DATE_SUB(NOW(), INTERVAL 7 DAY)
		 GROUP BY DATE(created_at) ORDER BY date`, teamID)
	var dailyNew []map[string]interface{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var date string
			var count int
			rows.Scan(&date, &count)
			dailyNew = append(dailyNew, map[string]interface{}{"date": date, "count": count})
		}
	}
	if dailyNew == nil {
		dailyNew = []map[string]interface{}{}
	}

	// Recent activities
	auditRows, _ := database.DB.Query(
		`SELECT a.action, a.resource_name, a.created_at,
		 COALESCE(NULLIF(u.display_name,''), NULLIF(u.username,''), NULLIF(a.user_name,''), '') as user_name
		 FROM md_audits a
		 LEFT JOIN users u ON a.user_id COLLATE utf8mb4_unicode_ci = u.id
		 WHERE a.team_id=? ORDER BY a.created_at DESC LIMIT 10`, teamID)
	var recentActivities []map[string]interface{}
	if auditRows != nil {
		defer auditRows.Close()
		for auditRows.Next() {
			var action, resourceName, createdAt, userName string
			auditRows.Scan(&action, &resourceName, &createdAt, &userName)
			recentActivities = append(recentActivities, map[string]interface{}{
				"action": action, "resource_name": resourceName,
				"created_at": createdAt, "user_name": userName,
			})
		}
	}
	if recentActivities == nil {
		recentActivities = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"users":             gin.H{"total": userCount},
		"documents":         gin.H{"total": docCount, "sheets": sheetCount},
		"trash":             trashCount,
		"departments":       deptCount,
		"shares":            shareCount,
		"comments":          gin.H{"total": commentCount},
		"week_new":          weekNew,
		"daily_new":         dailyNew,
		"recent_activities": recentActivities,
	}})
}

func TeamSystemInfo(c *gin.Context) {
	role := getTeamRole(c)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可查看"})
		return
	}
	var dbSize int64
	database.DB.QueryRow("SELECT COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&dbSize)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"version": "1.0.0",
		"db_size": dbSize,
	}})
}

// ==================== Import ====================

func TeamImportDocument(c *gin.Context) {
	if getTeamRole(c) == "viewer" {
		denyForbidden(c)
		return
	}
	teamID := getTeamID(c)
	userID := c.GetString("user_id")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
		return
	}
	defer file.Close()

	var content []byte
	var title string
	var docType string = "doc"

	// Determine type from extension
	name := header.Filename
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		switch strings.ToLower(name[idx:]) {
		case ".md", ".txt", ".html":
			body, _ := io.ReadAll(file)
			content = body
			title = name[:idx]
		case ".xlsx":
			// For xlsx, store raw bytes
			body, _ := io.ReadAll(file)
			content = body
			title = name[:idx]
			docType = "sheet"
		default:
			body, _ := io.ReadAll(file)
			content = body
			title = name[:idx]
		}
	} else {
		body, _ := io.ReadAll(file)
		content = body
		title = name
	}

	folderID := c.PostForm("folder_id")
	docID := uuid.New().String()
	_, err = database.DB.Exec(
		`INSERT INTO md_documents (id, team_id, folder_id, department_id, title, type, content_text, status, created_by, updated_by)
		 VALUES (?, ?, ?, '', ?, ?, ?, 1, ?, ?)`,
		docID, teamID, folderID, title, docType, string(content), userID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	writeDocContent(docID, content)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": docID, "title": title, "type": docType}})
}
