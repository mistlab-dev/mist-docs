package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
)

// MediaItem represents an uploaded file
type MediaItem struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Size      int64     `json:"size"`
	Type      string    `json:"type"` // image / document / other
	MimeType  string    `json:"mime_type"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListMedia GET /docs/media
func ListMedia(c *gin.Context) {
	uploadsDir := filepath.Join(store.RootPath(), "uploads")
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"data": []MediaItem{}})
		return
	}

	entries, err := os.ReadDir(uploadsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取失败"})
		return
	}

	// Filter by type param
	filterType := c.DefaultQuery("type", "") // image / document
	search := c.DefaultQuery("q", "")

	var items []MediaItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		// Classify
		mType := "other"
		mimeType := "application/octet-stream"
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp":
			mType = "image"
			if ext == ".svg" {
				mimeType = "image/svg+xml"
			} else {
				mimeType = "image/" + strings.TrimPrefix(ext, ".")
			}
		case ".pdf":
			mType = "document"
			mimeType = "application/pdf"
		case ".doc", ".docx":
			mType = "document"
			mimeType = "application/msword"
		case ".xls", ".xlsx":
			mType = "document"
			mimeType = "application/vnd.ms-excel"
		case ".txt", ".md":
			mType = "document"
			mimeType = "text/plain"
		case ".mp4", ".webm":
			mType = "video"
			mimeType = "video/" + strings.TrimPrefix(ext, ".")
		case ".mp3", ".wav", ".ogg":
			mType = "audio"
			mimeType = "audio/" + strings.TrimPrefix(ext, ".")
		}

		if filterType != "" && mType != filterType {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(search)) {
			continue
		}

		items = append(items, MediaItem{
			Name:      name,
			URL:       "/api/files/" + name,
			Size:      info.Size(),
			Type:      mType,
			MimeType:  mimeType,
			UpdatedAt: info.ModTime(),
		})
	}

	// Sort by date descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	// Pagination
	page := 1
	limit := 50
	if p := c.Query("page"); p != "" {
		if v := parseInt(p); v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v := parseInt(l); v > 0 && v <= 200 {
			limit = v
		}
	}

	total := len(items)
	start := (page - 1) * limit
	if start >= total {
		items = []MediaItem{}
	} else {
		end := start + limit
		if end > total {
			end = total
		}
		items = items[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  items,
		"total": total,
		"page":  page,
	})
}

// DeleteMedia DELETE /docs/media/:filename
func DeleteMedia(c *gin.Context) {
	filename := c.Param("filename")
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	path := filepath.Join(store.RootPath(), "uploads", filename)
	if err := os.Remove(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
