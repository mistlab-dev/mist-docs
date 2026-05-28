package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
)

// ==================== 文件夹 ====================

func DocTree(c *gin.Context) {
	deptID := c.Query("department_id")
	role := c.GetString("role")

	// dept_admin 只能看本部门
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
	if role == "dept_admin" && f.DepartmentID != c.GetString("department_id") {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能在本部门创建"})
		return
	}

	if err := service.CreateFolder(c.Request.Context(), &f, c.GetString("user_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	if err := service.DeleteFolder(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
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
		Content      string `json:"content"` // base64 or raw text for initial content
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	role := c.GetString("role")
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

	// Initial content - empty or minimal
	initialContent := []byte("{}")
	if req.Content != "" {
		initialContent = []byte(req.Content)
	}

	if err := service.CreateDocument(c.Request.Context(), doc, initialContent, c.GetString("user_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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

	doc := &model.Document{
		ID:      c.Param("id"),
		Title:   req.Title,
		FolderID: req.FolderID,
	}

	if err := service.UpdateDocument(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func DeleteDocument(c *gin.Context) {
	if err := service.DeleteDocument(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已移入回收站"})
}

func GetDocumentContent(c *gin.Context) {
	data, doc, err := service.GetDocumentContent(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"content":   string(data),
		"version":   doc.Version,
		"title":     doc.Title,
		"type":      doc.Type,
		"updated_at": doc.UpdatedAt,
	}})
}

func SaveDocumentContent(c *gin.Context) {
	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取内容失败"})
		return
	}

	doc, err := service.SaveDocumentContent(c.Request.Context(), c.Param("id"), body, c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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

	if err := service.RestoreVersion(c.Request.Context(), c.Param("id"), req.Version, c.GetString("user_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	if err := service.RestoreDocument(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已恢复"})
}

func PurgeFromTrash(c *gin.Context) {
	if err := service.PurgeDocument(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已永久删除"})
}

// ==================== 权限 ====================

func ListPermissions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "message": "待实现"})
}

func SetPermission(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "待实现"})
}

func RemovePermission(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "待实现"})
}

func CheckPermission(c *gin.Context) {
	// 简单实现：检查用户对文档的访问权限
	docID := c.Query("document_id")
	folderID := c.Query("folder_id")

	role := c.GetString("role")
	userDeptID := c.GetString("department_id")

	var targetDeptID string
	if docID != "" {
		doc, err := service.GetDocumentByID(c.Request.Context(), docID)
		if err != nil || doc == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
			return
		}
		targetDeptID = doc.DepartmentID
	} else if folderID != "" {
		folder, err := service.GetFolderByID(c.Request.Context(), folderID)
		if err != nil || folder == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "文件夹不存在"})
			return
		}
		targetDeptID = folder.DepartmentID
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定文档或文件夹"})
		return
	}

	perm := "none"
	if role == "super_admin" {
		perm = "admin"
	} else if targetDeptID == userDeptID {
		perm = "write" // 本部门默认写权限
	}

	// TODO: check md_permissions table for explicit permissions

	c.JSON(http.StatusOK, gin.H{"permission": perm})
}

// ==================== 审计 ====================

func ListAudits(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "total": 0, "message": "待实现"})
}

func ExportAudits(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "待实现"})
}

func AuditStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{}, "message": "待实现"})
}

func InitStore() error {
	return store.Init()
}