package handler

import (
	"net/http"
	"path/filepath"

	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
)

// ================== 文件上传 ==================

func GetFile(c *gin.Context) {
	filename, ok := safeBaseName(c.Param("filename"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件名无效"})
		return
	}
	fullPath := filepath.Join(store.RootPath(), "uploads", filename)
	c.File(fullPath)
}
