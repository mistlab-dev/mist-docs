package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ==================== 文档 ====================

func DocTree(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

func CreateFolder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func UpdateFolder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func DeleteFolder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func ListDocuments(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "total": 0})
}

func CreateDocument(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func UpdateDocument(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func DeleteDocument(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func GetDocumentContent(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": nil})
}

func SaveDocumentContent(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func ListVersions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

func RestoreVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func ListTrash(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

func RestoreFromTrash(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func PurgeFromTrash(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

// ==================== 权限 ====================

func ListPermissions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

func SetPermission(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func RemovePermission(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func CheckPermission(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"permission": "none"})
}

// ==================== 审计 ====================

func ListAudits(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "total": 0})
}

func ExportAudits(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "TODO"})
}

func AuditStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{}})
}
