package handler

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

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
	var req struct {
		Name      string `json:"name"`
		ParentID  string `json:"parent_id"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	_, err := database.DB.Exec(
		`UPDATE md_team_folders SET name=?, parent_id=?, sort_order=? WHERE id=?`,
		req.Name, req.ParentID, req.SortOrder, id)
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
	// Move documents in this folder to team root
	database.DB.Exec(`UPDATE md_documents SET folder_id='' WHERE folder_id=?`, id)
	// Move sub-folders to root
	database.DB.Exec(`UPDATE md_team_folders SET parent_id='' WHERE parent_id=?`, id)
	_, err := database.DB.Exec(`DELETE FROM md_team_folders WHERE id=?`, id)
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
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }

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
		IFNULL(u1.display_name, '') as creator_name,
		IFNULL(u2.display_name, '') as updater_name
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
		ID          string `json:"id"`
		TeamID      string `json:"team_id"`
		FolderID    string `json:"folder_id"`
		Title       string `json:"title"`
		Type        string `json:"type"`
		FileSize    int64  `json:"file_size"`
		Version     int    `json:"version"`
		LockedBy    string `json:"locked_by"`
		LockedAt    string `json:"locked_at,omitempty"`
		Status      int    `json:"status"`
		CreatedBy   string `json:"created_by"`
		UpdatedBy   string `json:"updated_by"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		CreatorName string `json:"creator_name"`
		UpdaterName string `json:"updater_name"`
	}
	var docs []Doc
	for rows.Next() {
		var d Doc
		var folderID, lockedBy, lockedAt sql.NullString
		rows.Scan(&d.ID, &d.TeamID, &folderID, &d.Title, &d.Type, &d.FileSize, &d.Version,
			&lockedBy, &lockedAt, &d.Status, &d.CreatedBy, &d.UpdatedBy, &d.CreatedAt, &d.UpdatedAt,
			&d.CreatorName, &d.UpdaterName)
		d.FolderID = folderID.String
		d.LockedBy = lockedBy.String
		d.LockedAt = lockedAt.String
		docs = append(docs, d)
	}
	if docs == nil {
		docs = []Doc{}
	}
	c.JSON(http.StatusOK, gin.H{"data": docs, "total": total, "page": page, "page_size": pageSize})
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
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	offset := (page - 1) * pageSize

	pattern := "%" + keyword + "%"
	where := `WHERE d.team_id = ? AND d.status = 1 AND (d.title LIKE ? OR d.content_text LIKE ?)`
	args := []interface{}{teamID, pattern, pattern}

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents d "+where, args...).Scan(&total)

	query := `SELECT d.id, d.team_id, d.folder_id, d.title, d.type, d.file_size, d.version,
		d.locked_by, d.status, d.created_by, d.updated_by, d.created_at, d.updated_at,
		IFNULL(u1.display_name, '') as creator_name
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
		ID          string `json:"id"`
		TeamID      string `json:"team_id"`
		FolderID    string `json:"folder_id"`
		Title       string `json:"title"`
		Type        string `json:"type"`
		FileSize    int64  `json:"file_size"`
		Version     int    `json:"version"`
		CreatedBy   string `json:"created_by"`
		UpdatedBy   string `json:"updated_by"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		CreatorName string `json:"creator_name"`
		Snippet     string `json:"snippet"`
	}
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		rows.Scan(&r.ID, &r.TeamID, &r.FolderID, &r.Title, &r.Type, &r.FileSize, &r.Version,
			&r.CreatedBy, &r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt,
			&r.CreatorName)
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
	if role == "" {
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
	}})
}

// TeamUpdateDocument PUT /teams/:team_id/documents/:id
func TeamUpdateDocument(c *gin.Context) {
	docID := c.Param("id")
	var req struct {
		Title    string `json:"title"`
		FolderID string `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	userID := c.GetString("user_id")
	_, err := database.DB.Exec(
		`UPDATE md_documents SET title=?, folder_id=?, updated_by=? WHERE id=?`,
		req.Title, req.FolderID, userID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// TeamDeleteDocument DELETE /teams/:team_id/documents/:id
func TeamDeleteDocument(c *gin.Context) {
	docID := c.Param("id")
	_, err := database.DB.Exec(`UPDATE md_documents SET status=0 WHERE id=?`, docID)
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
	}})
}

// TeamSaveDocumentContent PUT /teams/:team_id/documents/:id/content
func TeamSaveDocumentContent(c *gin.Context) {
	docID := c.Param("id")
	userID := c.GetString("user_id")

	// Check lock
	var lockedBy string
	database.DB.QueryRow("SELECT locked_by FROM md_documents WHERE id=?", docID).Scan(&lockedBy)
	role := getTeamRole(c)
	if lockedBy != "" && lockedBy != userID && role != "admin" {
		c.JSON(http.StatusConflict, gin.H{"error": "文档已被锁定"})
		return
	}

	body, err := readBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取内容失败"})
		return
	}

	// Increment version
	var version int
	database.DB.QueryRow("SELECT version FROM md_documents WHERE id=?", docID).Scan(&version)
	version++

	_, err = database.DB.Exec(
		`UPDATE md_documents SET content_text=?, version=?, updated_by=?, updated_at=NOW() WHERE id=?`,
		string(body), version, userID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	writeDocContent(docID, body)
	audit(c, "edit_doc", "document", docID, "", fmt.Sprintf(`{"version":%d}`, version))
	c.JSON(http.StatusOK, gin.H{"message": "已保存", "version": version})
}

// ==================== 回收站 ====================

func TeamListTrash(c *gin.Context) {
	teamID := getTeamID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	offset := (page - 1) * pageSize

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=0", teamID).Scan(&total)

	rows, err := database.DB.Query(
		`SELECT id, title, type, file_size, deleted_at, created_by
		 FROM md_documents WHERE team_id=? AND status=0 ORDER BY updated_at DESC LIMIT ? OFFSET ?`,
		teamID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var docs []map[string]interface{}
	for rows.Next() {
		var id, title, docType, createdBy string
		var fileSize int64
		var deletedAt string
		rows.Scan(&id, &title, &docType, &fileSize, &deletedAt, &createdBy)
		docs = append(docs, map[string]interface{}{
			"id": id, "title": title, "type": docType,
			"file_size": fileSize, "deleted_at": deletedAt, "created_by": createdBy,
		})
	}
	if docs == nil { docs = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": docs, "total": total})
}

func TeamRestoreFromTrash(c *gin.Context) {
	docID := c.Param("id")
	_, err := database.DB.Exec(`UPDATE md_documents SET status=1 WHERE id=?`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "restore_doc", "document", docID, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

func TeamPurgeFromTrash(c *gin.Context) {
	docID := c.Param("id")
	_, err := database.DB.Exec(`DELETE FROM md_documents WHERE id=? AND status=0`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "purge_doc", "document", docID, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已永久删除"})
}

func TeamEmptyTrash(c *gin.Context) {
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
	if docs == nil { docs = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": docs})
}

func TeamAddFavorite(c *gin.Context) {
	userID := c.GetString("user_id")
	docID := c.Param("id")
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
	if tags == nil { tags = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

func TeamCreateTag(c *gin.Context) {
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
	_, err := database.DB.Exec(`INSERT INTO md_tags (id, team_id, name, color) VALUES (?, ?, ?, ?)`,
		id, teamID, req.Name, req.Color)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "name": req.Name, "color": req.Color}})
}

func TeamDeleteTag(c *gin.Context) {
	id := c.Param("id")
	database.DB.Exec(`DELETE FROM md_doc_tags WHERE tag_id=?`, id)
	_, err := database.DB.Exec(`DELETE FROM md_tags WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func TeamGetDocTags(c *gin.Context) {
	docID := c.Param("id")
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
	if tags == nil { tags = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": tags})
}

func TeamSetDocTags(c *gin.Context) {
	docID := c.Param("id")
	var req struct {
		TagIDs []string `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	database.DB.Exec(`DELETE FROM md_doc_tags WHERE document_id=?`, docID)
	for _, tagID := range req.TagIDs {
		database.DB.Exec(`INSERT IGNORE INTO md_doc_tags (document_id, tag_id) VALUES (?, ?)`, docID, tagID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// ==================== 版本 ====================

func TeamListVersions(c *gin.Context) {
	docID := c.Param("id")
	rows, err := database.DB.Query(
		`SELECT v.id, v.version, v.file_size, v.created_by, v.created_at,
		 IFNULL(u.display_name,'') as user_name
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
			"created_by": createdBy, "created_at": createdAt, "user_name": userName,
		})
	}
	if versions == nil { versions = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": versions})
}

func TeamGetVersionContent(c *gin.Context) {
	docID := c.Param("id")
	ver := c.Param("ver")
	var content []byte
	err := database.DB.QueryRow(
		`SELECT content FROM md_versions WHERE document_id=? AND version=?`, docID, ver).Scan(&content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}

func TeamRestoreVersion(c *gin.Context) {
	docID := c.Param("id")
	var req struct {
		Version int `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定版本号"})
		return
	}
	userID := c.GetString("user_id")
	var content []byte
	err := database.DB.QueryRow(`SELECT content FROM md_versions WHERE document_id=? AND version=?`, docID, req.Version).Scan(&content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	writeDocContent(docID, content)
	var version int
	database.DB.QueryRow("SELECT version FROM md_documents WHERE id=?", docID).Scan(&version)
	version++
	database.DB.Exec(`UPDATE md_documents SET content_text=?, version=?, updated_by=?, updated_at=NOW() WHERE id=?`,
		string(content), version, userID, docID)
	audit(c, "restore", "document", docID, "", fmt.Sprintf(`{"version":%d}`, req.Version))
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

// ==================== 锁定 ====================

func TeamLockDocument(c *gin.Context) {
	docID := c.Param("id")
	userID := c.GetString("user_id")
	_, err := database.DB.Exec(`UPDATE md_documents SET locked_by=?, locked_at=NOW() WHERE id=?`, userID, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已锁定"})
}

func TeamUnlockDocument(c *gin.Context) {
	docID := c.Param("id")
	_, err := database.DB.Exec(`UPDATE md_documents SET locked_by='', locked_at=NULL WHERE id=?`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已解锁"})
}

// ==================== 分享 ====================

func TeamCreateShare(c *gin.Context) {
	docID := c.Param("id")
	userID := c.GetString("user_id")
	teamID := getTeamID(c)

	var req struct {
		Permission string `json:"permission"`
		Expires    string `json:"expires"`
		Password   string `json:"password"`
	}
	c.ShouldBindJSON(&req)
	if req.Permission == "" {
		req.Permission = "read"
	}

	token := uuid.New().String()
	id := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_shares (id, document_id, team_id, token, permission, password, expires_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, docID, teamID, token, req.Permission, req.Password, req.Expires, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": id, "token": token, "permission": req.Permission,
	}})
}

func TeamListShares(c *gin.Context) {
	docID := c.Param("id")
	rows, err := database.DB.Query(
		`SELECT id, token, permission, expires_at, created_by, created_at
		 FROM md_shares WHERE document_id=?`, docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var shares []map[string]interface{}
	for rows.Next() {
		var id, token, perm, createdBy, createdAt string
		var expires string
		rows.Scan(&id, &token, &perm, &expires, &createdBy, &createdAt)
		shares = append(shares, map[string]interface{}{
			"id": id, "token": token, "permission": perm,
			"expires_at": expires, "created_by": createdBy, "created_at": createdAt,
		})
	}
	if shares == nil { shares = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": shares})
}

// ==================== 协作者 ====================

func TeamListCollaborators(c *gin.Context) {
	docID := c.Param("id")
	rows, err := database.DB.Query(
		`SELECT p.id, p.target_id, p.permission, p.created_by,
		 IFNULL(u.display_name, '') as user_name
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
		var id, targetID, perm, createdBy, userName string
		rows.Scan(&id, &targetID, &perm, &createdBy, &userName)
		collabs = append(collabs, map[string]interface{}{
			"id": id, "target_id": targetID, "permission": perm,
			"created_by": createdBy, "user_name": userName,
		})
	}
	if collabs == nil { collabs = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": collabs})
}

func TeamAddCollaborator(c *gin.Context) {
	docID := c.Param("id")
	userID := c.GetString("user_id")
	var req struct {
		TargetID string `json:"target_id" binding:"required"`
		Perm     string `json:"permission" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	id := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_permissions (id, resource_type, resource_id, target_type, target_id, permission, inherit, created_by)
		 VALUES (?, 'document', ?, 'user', ?, ?, 0, ?)`,
		id, docID, req.TargetID, req.Perm, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已添加"})
}

// ==================== 评论 ====================

func TeamListComments(c *gin.Context) {
	docID := c.Param("id")
	rows, err := database.DB.Query(
		`SELECT c.id, c.content, c.user_id, c.parent_id, c.created_at, c.updated_at,
		 IFNULL(u.display_name,'') as user_name
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
	if comments == nil { comments = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": comments})
}

func TeamCreateComment(c *gin.Context) {
	docID := c.Param("id")
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
	id := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_comments (id, team_id, document_id, user_id, content, parent_id) VALUES (?, ?, ?, ?, ?, ?)`,
		id, teamID, docID, userID, req.Content, req.ParentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": id, "content": req.Content, "user_id": userID,
	}})
}

// ==================== 导出 ====================

func TeamExportDocument(c *gin.Context) {
	docID := c.Param("id")
	var title, docType string
	database.DB.QueryRow(`SELECT title, type FROM md_documents WHERE id=?`, docID).Scan(&title, &docType)
	content := readDocContent(docID)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.html\"", title))
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}

// ==================== 审计 ====================

func TeamListAudits(c *gin.Context) {
	if !service.RequirePlanFeature(c, "audit") {
		return
	}
	teamID := getTeamID(c)
	role := getTeamRole(c)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可查看审计日志"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	offset := (page - 1) * pageSize

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_audits WHERE team_id=?", teamID).Scan(&total)

	rows, err := database.DB.Query(
		`SELECT a.id, a.user_id, a.action, a.resource_type, a.resource_id, a.resource_name, a.detail, a.created_at,
		 IFNULL(u.display_name,'') as user_name
		 FROM md_audits a LEFT JOIN users u ON a.user_id COLLATE utf8mb4_unicode_ci = u.id
		 WHERE a.team_id=? ORDER BY a.created_at DESC LIMIT ? OFFSET ?`,
		teamID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var audits []map[string]interface{}
	for rows.Next() {
		var id, userID, action, resType, resID, resName, detail, createdAt, userName string
		rows.Scan(&id, &userID, &action, &resType, &resID, &resName, &detail, &createdAt, &userName)
		audits = append(audits, map[string]interface{}{
			"id": id, "user_id": userID, "action": action,
			"resource_type": resType, "resource_id": resID,
			"resource_name": resName, "detail": detail,
			"created_at": createdAt, "user_name": userName,
		})
	}
	if audits == nil { audits = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": audits, "total": total})
}

func TeamExportAudits(c *gin.Context) {
	if !service.RequirePlanFeature(c, "audit") {
		return
	}
	teamID := getTeamID(c)
	role := getTeamRole(c)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可导出"})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=audits.csv")
	c.Writer.Write([]byte("\xEF\xBB\xBF"))
	// Simple CSV export
	rows, err := database.DB.Query(
		`SELECT a.id, a.user_id, a.action, a.resource_type, a.resource_id, a.created_at,
		 IFNULL(u.display_name,'')
		 FROM md_audits a LEFT JOIN users u ON a.user_id COLLATE utf8mb4_unicode_ci = u.id
		 WHERE a.team_id=? ORDER BY a.created_at DESC LIMIT 10000`, teamID)
	if err != nil {
		return
	}
	defer rows.Close()
	c.Writer.Write([]byte("ID,用户,操作,资源类型,资源ID,时间\n"))
	for rows.Next() {
		var id, userID, action, resType, resID, createdAt, userName string
		rows.Scan(&id, &userID, &action, &resType, &resID, &createdAt, &userName)
		c.Writer.Write([]byte(fmt.Sprintf("%s,%s,%s,%s,%s,%s\n", id, userName, action, resType, resID, createdAt)))
	}
}

func TeamAuditStats(c *gin.Context) {
	teamID := getTeamID(c)
	role := getTeamRole(c)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可查看"})
		return
	}
	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_audits WHERE team_id=?", teamID).Scan(&total)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"total": total}})
}

// ==================== 权限 ====================

func TeamListPermissions(c *gin.Context) {
	resType := c.Query("resource_type")
	resID := c.Query("resource_id")
	if resType == "" || resID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 resource_type 和 resource_id"})
		return
	}
	rows, err := database.DB.Query(
		`SELECT p.id, p.target_type, p.target_id, p.permission, p.inherit, p.created_by,
		 IFNULL(u.display_name,'') as user_name
		 FROM md_permissions p LEFT JOIN users u ON p.target_type='user' AND p.target_id COLLATE utf8mb4_unicode_ci = u.id
		 WHERE p.resource_type=? AND p.resource_id=?`, resType, resID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var perms []map[string]interface{}
	for rows.Next() {
		var id, tType, tID, perm, createdBy, userName string
		var inherit bool
		rows.Scan(&id, &tType, &tID, &perm, &inherit, &createdBy, &userName)
		perms = append(perms, map[string]interface{}{
			"id": id, "target_type": tType, "target_id": tID,
			"permission": perm, "inherit": inherit,
			"created_by": createdBy, "user_name": userName,
		})
	}
	if perms == nil { perms = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": perms})
}

func TeamSetPermission(c *gin.Context) {
	role := getTeamRole(c)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可设置权限"})
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
	id := uuid.New().String()
	userID := c.GetString("user_id")
	_, err := database.DB.Exec(
		`INSERT INTO md_permissions (id, resource_type, resource_id, target_type, target_id, permission, inherit, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.ResourceType, req.ResourceID, req.TargetType, req.TargetID, req.Permission, req.Inherit, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": req})
}

func TeamRemovePermission(c *gin.Context) {
	role := getTeamRole(c)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可删除权限"})
		return
	}
	_, err := database.DB.Exec(`DELETE FROM md_permissions WHERE id=?`, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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

	// Team admin = full access
	role := getTeamRole(c)
	if role == "admin" {
		c.JSON(http.StatusOK, gin.H{"permission": "admin"})
		return
	}

	// Check direct permission
	var perm string
	database.DB.QueryRow(
		`SELECT permission FROM md_permissions WHERE resource_type=? AND resource_id=? AND target_type='user' AND target_id=?`,
		resType, resID, userID).Scan(&perm)
	if perm == "" {
		perm = "read" // team member default = read
	}
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
	if templates == nil { templates = []map[string]interface{}{} }
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

func TeamGetTemplate(c *gin.Context) {
	id := c.Param("id")
	var name, docType, content string
	err := database.DB.QueryRow(`SELECT name, type, content FROM md_templates WHERE id=?`, id).Scan(&name, &docType, &content)
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
	_, err := database.DB.Exec(`UPDATE md_templates SET name=?, content=? WHERE id=?`, req.Name, req.Content, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func TeamDeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(`DELETE FROM md_templates WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== 存储 ====================

func TeamStorageStatus(c *gin.Context) {
	teamID := getTeamID(c)
	var totalSize int64
	database.DB.QueryRow(`SELECT COALESCE(SUM(file_size), 0) FROM md_documents WHERE team_id=?`, teamID).Scan(&totalSize)
	var docCount int
	database.DB.QueryRow(`SELECT COUNT(*) FROM md_documents WHERE team_id=? AND status=1`, teamID).Scan(&docCount)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"total_size": totalSize, "doc_count": docCount,
	}})
}

// ==================== Webhooks ====================

func TeamListWebhooks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

func TeamCreateWebhook(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func TeamDeleteWebhook(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func TeamToggleWebhook(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func TeamListWebhookLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

// ==================== 文档统计 ====================

func TeamDocStats(c *gin.Context) {
	docID := c.Param("id")
	var version int
	var fileSize int64
	database.DB.QueryRow(`SELECT version, file_size FROM md_documents WHERE id=?`, docID).Scan(&version, &fileSize)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"version": version, "file_size": fileSize}})
}

// ==================== Helpers ====================

func writeDocContent(docID string, content []byte) {
	var deptID string
	database.DB.QueryRow("SELECT department_id FROM md_documents WHERE id=?", docID).Scan(&deptID)
	_, _, err := store.WriteVersion(deptID, docID, 1, content)
	if err != nil {
		log.Printf("writeDocContent error: %v", err)
	}
}

func readDocContent(docID string) []byte {
	var deptID string
	database.DB.QueryRow("SELECT department_id FROM md_documents WHERE id=?", docID).Scan(&deptID)
	data, err := store.ReadCurrent(deptID, docID)
	if err != nil {
		log.Printf("readDocContent error: %v", err)
		return []byte{}
	}
	return data
}

func readBody(c *gin.Context) ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

// Ensure model types are referenced (avoid unused import errors during dev)
var _ = model.Document{}
