package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// ==================== 文件夹 ====================

func DocTree(c *gin.Context) {
	deptID := c.Query("department_id")
	role := c.GetString("role")
	if role == "dept_admin" {
		deptID = c.GetString("department_id")
	}

	tree, err := service.GetFolderTree(c.Request.Context(), deptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tree})
}

func CreateFolder(c *gin.Context) {
	var f model.DocFolder
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if f.Name == "" || f.DepartmentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称和部门不能为空"})
		return
	}

	role := c.GetString("role")
	userID := c.GetString("user_id")
	if role == "dept_admin" && f.DepartmentID != c.GetString("department_id") {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能在本部门创建"})
		return
	}

	if err := service.CreateFolder(c.Request.Context(), &f, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "create_folder", "folder", f.ID, f.Name, "")
	c.JSON(http.StatusOK, gin.H{"data": f})
}

func UpdateFolder(c *gin.Context) {
	var f model.DocFolder
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	f.ID = c.Param("id")

	if err := service.UpdateFolder(c.Request.Context(), &f); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func DeleteFolder(c *gin.Context) {
	id := c.Param("id")
	folder, _ := service.GetFolderByID(c.Request.Context(), id)
	name := ""
	if folder != nil {
		name = folder.Name
	}

	if err := service.DeleteFolder(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	audit(c, "delete_folder", "folder", id, name, "")
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== 文档 ====================

func ListDocuments(c *gin.Context) {
	folderID := c.Query("folder_id")
	deptID := c.Query("department_id")
	docType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	role := c.GetString("role")
	if role == "dept_admin" {
		deptID = c.GetString("department_id")
	}

	docs, total, err := service.ListDocuments(c.Request.Context(), folderID, deptID, docType, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": docs, "total": total, "page": page, "page_size": pageSize})
}

func CreateDocument(c *gin.Context) {
	var req struct {
		Title        string `json:"title" binding:"required"`
		Type         string `json:"type"`
		FolderID     string `json:"folder_id"`
		DepartmentID string `json:"department_id" binding:"required"`
		Content      string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	role := c.GetString("role")
	userID := c.GetString("user_id")
	if role == "dept_admin" && req.DepartmentID != c.GetString("department_id") {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能在本部门创建"})
		return
	}

	doc := &model.Document{
		Title:        req.Title,
		Type:         req.Type,
		FolderID:     req.FolderID,
		DepartmentID: req.DepartmentID,
	}

	initialContent := []byte("{}")
	if req.Content != "" {
		initialContent = []byte(req.Content)
	}

	if err := service.CreateDocument(c.Request.Context(), doc, initialContent, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "create_doc", "document", doc.ID, doc.Title, fmt.Sprintf(`{"type":"%s"}`, doc.Type))
	c.JSON(http.StatusOK, gin.H{"data": doc})
}

func UpdateDocument(c *gin.Context) {
	var req struct {
		Title    string `json:"title"`
		FolderID string `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	doc := &model.Document{ID: c.Param("id"), Title: req.Title, FolderID: req.FolderID}
	if err := service.UpdateDocument(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func DeleteDocument(c *gin.Context) {
	id := c.Param("id")
	doc, _ := service.GetDocumentByID(c.Request.Context(), id)
	title := ""
	if doc != nil {
		title = doc.Title
	}

	if err := service.DeleteDocument(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	audit(c, "delete_doc", "document", id, title, "")
	c.JSON(http.StatusOK, gin.H{"message": "已移入回收站"})
}

func GetDocumentContent(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	role := c.GetString("role")
	deptID := c.GetString("department_id")

	if !service.HasPermission(c.Request.Context(), userID, deptID, role, "document", id, "read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	data, doc, err := service.GetDocumentContent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	audit(c, "view", "document", id, doc.Title, "")
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"content":    string(data),
		"version":    doc.Version,
		"title":      doc.Title,
		"type":       doc.Type,
		"updated_at": doc.UpdatedAt,
	}})
}

func SaveDocumentContent(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	role := c.GetString("role")
	deptID := c.GetString("department_id")

	if !service.HasPermission(c.Request.Context(), userID, deptID, role, "document", id, "write") {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取内容失败"})
		return
	}

	doc, err := service.SaveDocumentContent(c.Request.Context(), id, body, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "edit_doc", "document", id, doc.Title, fmt.Sprintf(`{"version":%d}`, doc.Version))
	c.JSON(http.StatusOK, gin.H{"message": "已保存", "version": doc.Version})
}

func ListVersions(c *gin.Context) {
	versions, err := service.ListVersions(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": versions})
}

func RestoreVersion(c *gin.Context) {
	var req struct {
		Version int `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定版本号"})
		return
	}

	userID := c.GetString("user_id")
	if err := service.RestoreVersion(c.Request.Context(), c.Param("id"), req.Version, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "restore", "document", c.Param("id"), "", fmt.Sprintf(`{"version":%d}`, req.Version))
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

// ==================== 回收站 ====================

func ListTrash(c *gin.Context) {
	deptID := c.Query("department_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	role := c.GetString("role")
	if role == "dept_admin" {
		deptID = c.GetString("department_id")
	}

	docs, total, err := service.ListTrash(c.Request.Context(), deptID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": docs, "total": total})
}

func RestoreFromTrash(c *gin.Context) {
	id := c.Param("id")
	if err := service.RestoreDocument(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit(c, "restore_doc", "document", id, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

func PurgeFromTrash(c *gin.Context) {
	id := c.Param("id")
	if err := service.PurgeDocument(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit(c, "purge_doc", "document", id, "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已永久删除"})
}

// ==================== 权限 ====================

func ListPermissions(c *gin.Context) {
	resourceType := c.Query("resource_type")
	resourceID := c.Query("resource_id")
	if resourceType == "" || resourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 resource_type 和 resource_id"})
		return
	}

	perms, err := service.ListPermissions(c.Request.Context(), resourceType, resourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": perms})
}

func SetPermission(c *gin.Context) {
	role := c.GetString("role")
	userID := c.GetString("user_id")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
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

	p := &model.DocPermission{
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		TargetType:   req.TargetType,
		TargetID:     req.TargetID,
		Permission:   req.Permission,
		Inherit:      req.Inherit,
	}

	if err := service.SetPermission(c.Request.Context(), p, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resourceName, _ := service.GetResourceName(c.Request.Context(), req.ResourceType, req.ResourceID)
	audit(c, "set_permission", req.ResourceType, req.ResourceID, resourceName,
		fmt.Sprintf(`{"target_type":"%s","target_id":"%s","permission":"%s"}`, req.TargetType, req.TargetID, req.Permission))
	c.JSON(http.StatusOK, gin.H{"data": p})
}

func RemovePermission(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	if err := service.RemovePermission(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit(c, "remove_permission", "", c.Param("id"), "", "")
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func CheckPermission(c *gin.Context) {
	resourceType := c.DefaultQuery("resource_type", "document")
	resourceID := c.Query("resource_id")
	if resourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 resource_id"})
		return
	}

	userID := c.GetString("user_id")
	role := c.GetString("role")
	deptID := c.GetString("department_id")

	perm, err := service.CheckPermission(c.Request.Context(), userID, deptID, resourceType, resourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if role == "super_admin" {
		perm = "admin"
	}
	c.JSON(http.StatusOK, gin.H{"permission": perm})
}

// ==================== 审计 ====================

func ListAudits(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	deptID := c.Query("department_id")
	if role == "dept_admin" {
		deptID = c.GetString("department_id")
	}

	audits, total, err := service.ListAudits(c.Request.Context(),
		c.Query("user_id"), deptID, c.Query("action"), c.Query("resource_id"),
		c.Query("start_date"), c.Query("end_date"),
		atoi(c.DefaultQuery("page", "1"), 1),
		atoi(c.DefaultQuery("page_size", "20"), 20),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": audits, "total": total})
}

func ExportAudits(c *gin.Context) {
	if c.GetString("role") != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅超级管理员可导出"})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=audits.csv")
	c.Writer.Write([]byte("\xEF\xBB\xBF"))

	if err := service.ExportAuditsCSV(c.Request.Context(),
		c.Query("user_id"), c.Query("department_id"), c.Query("action"),
		c.Query("resource_id"), c.Query("start_date"), c.Query("end_date"),
		c.Writer,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func AuditStats(c *gin.Context) {
	if c.GetString("role") != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅超级管理员可查看统计"})
		return
	}

	stats, err := service.AuditStats(c.Request.Context(), c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

func atoi(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
