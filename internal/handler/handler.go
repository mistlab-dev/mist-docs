package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ==================== 认证 ====================

func Login(c *gin.Context) {
	// TODO: implement
	c.JSON(http.StatusOK, gin.H{"message": "login"})
}

func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "logout"})
}

func Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"user_id": c.GetString("user_id"),
		"role":    c.GetString("role"),
	})
}

// ==================== 部门 ====================

func ListDepartments(c *gin.Context)   { c.JSON(http.StatusOK, gin.H{}) }
func CreateDepartment(c *gin.Context)  { c.JSON(http.StatusOK, gin.H{}) }
func UpdateDepartment(c *gin.Context)  { c.JSON(http.StatusOK, gin.H{}) }
func DeleteDepartment(c *gin.Context)  { c.JSON(http.StatusOK, gin.H{}) }
func ImportDepartments(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) }

// ==================== 用户 ====================

func ListUsers(c *gin.Context)       { c.JSON(http.StatusOK, gin.H{}) }
func CreateUser(c *gin.Context)      { c.JSON(http.StatusOK, gin.H{}) }
func UpdateUser(c *gin.Context)      { c.JSON(http.StatusOK, gin.H{}) }
func DeleteUser(c *gin.Context)      { c.JSON(http.StatusOK, gin.H{}) }
func ResetPassword(c *gin.Context)   { c.JSON(http.StatusOK, gin.H{}) }
func ImportUsers(c *gin.Context)     { c.JSON(http.StatusOK, gin.H{}) }

// ==================== 文档 ====================

func DocTree(c *gin.Context)            { c.JSON(http.StatusOK, gin.H{}) }
func CreateFolder(c *gin.Context)       { c.JSON(http.StatusOK, gin.H{}) }
func UpdateFolder(c *gin.Context)       { c.JSON(http.StatusOK, gin.H{}) }
func DeleteFolder(c *gin.Context)       { c.JSON(http.StatusOK, gin.H{}) }
func ListDocuments(c *gin.Context)      { c.JSON(http.StatusOK, gin.H{}) }
func CreateDocument(c *gin.Context)     { c.JSON(http.StatusOK, gin.H{}) }
func UpdateDocument(c *gin.Context)     { c.JSON(http.StatusOK, gin.H{}) }
func DeleteDocument(c *gin.Context)     { c.JSON(http.StatusOK, gin.H{}) }
func GetDocumentContent(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) }
func SaveDocumentContent(c *gin.Context){ c.JSON(http.StatusOK, gin.H{}) }
func ListVersions(c *gin.Context)       { c.JSON(http.StatusOK, gin.H{}) }
func RestoreVersion(c *gin.Context)     { c.JSON(http.StatusOK, gin.H{}) }
func ListTrash(c *gin.Context)          { c.JSON(http.StatusOK, gin.H{}) }
func RestoreFromTrash(c *gin.Context)   { c.JSON(http.StatusOK, gin.H{}) }
func PurgeFromTrash(c *gin.Context)     { c.JSON(http.StatusOK, gin.H{}) }

// ==================== 权限 ====================

func ListPermissions(c *gin.Context)  { c.JSON(http.StatusOK, gin.H{}) }
func SetPermission(c *gin.Context)    { c.JSON(http.StatusOK, gin.H{}) }
func RemovePermission(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) }
func CheckPermission(c *gin.Context)  { c.JSON(http.StatusOK, gin.H{}) }

// ==================== 审计 ====================

func ListAudits(c *gin.Context)   { c.JSON(http.StatusOK, gin.H{}) }
func ExportAudits(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) }
func AuditStats(c *gin.Context)   { c.JSON(http.StatusOK, gin.H{}) }
