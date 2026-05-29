package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// DocStats GET /docs/documents/:id/stats
func DocStats(c *gin.Context) {
	docID := c.Param("id")

	stats := gin.H{}

	// 1. Version count (edit count)
	var versionCount int
	database.DB.QueryRowContext(c.Request.Context(),
		"SELECT COUNT(*) FROM md_versions WHERE document_id = ?", docID).Scan(&versionCount)
	stats["edit_count"] = versionCount

	// 2. Word count from content
	content, _, err := service.GetDocumentContent(c.Request.Context(), docID)
	if err == nil && len(content) > 0 {
		plain := stripHTMLTagsFast(string(content))
		// Count words: CJK characters + space-separated words
		cjkCount := 0
		asciiWords := 0
		inWord := false
		for _, r := range plain {
			if r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF || r >= 0x3000 && r <= 0x303F {
				cjkCount++
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				if !inWord {
					asciiWords++
					inWord = true
				}
			} else {
				inWord = false
			}
		}
		stats["word_count"] = cjkCount + asciiWords
		stats["char_count"] = len([]rune(plain))
	} else {
		stats["word_count"] = 0
		stats["char_count"] = 0
	}

	// 3. Contributor count
	var contributors int
	database.DB.QueryRowContext(c.Request.Context(),
		"SELECT COUNT(DISTINCT created_by) FROM md_versions WHERE document_id = ?", docID).Scan(&contributors)
	stats["contributors"] = contributors

	// 4. Edit activity by hour (last 30 days)
	rows, err := database.DB.QueryContext(c.Request.Context(),
		`SELECT HOUR(created_at) AS h, COUNT(*) AS c FROM md_versions
		 WHERE document_id = ? AND created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)
		 GROUP BY h ORDER BY h`, docID)
	if err == nil {
		defer rows.Close()
		hourly := make([]int, 24)
		for rows.Next() {
			var h, cnt int
			rows.Scan(&h, &cnt)
			hourly[h] = cnt
		}
		stats["hourly_edits"] = hourly
	}

	// 5. Edit activity by day (last 30 days)
	rows2, err := database.DB.QueryContext(c.Request.Context(),
		`SELECT DATE(created_at) AS d, COUNT(*) AS c FROM md_versions
		 WHERE document_id = ? AND created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)
		 GROUP BY d ORDER BY d`, docID)
	if err == nil {
		defer rows2.Close()
		type dayEntry struct {
			Date  string `json:"date"`
			Count int    `json:"count"`
		}
		var daily []dayEntry
		for rows2.Next() {
			var d time.Time
			var cnt int
			rows2.Scan(&d, &cnt)
			daily = append(daily, dayEntry{Date: d.Format("2006-01-02"), Count: cnt})
		}
		stats["daily_edits"] = daily
	}

	// 6. Comment count
	var commentCount int
	database.DB.QueryRowContext(c.Request.Context(),
		"SELECT COUNT(*) FROM md_comments WHERE document_id = ?", docID).Scan(&commentCount)
	stats["comment_count"] = commentCount

	// 7. First and last edit
	var firstEdit, lastEdit string
	database.DB.QueryRowContext(c.Request.Context(),
		"SELECT created_at FROM md_versions WHERE document_id = ? ORDER BY version ASC LIMIT 1", docID).Scan(&firstEdit)
	database.DB.QueryRowContext(c.Request.Context(),
		"SELECT created_at FROM md_versions WHERE document_id = ? ORDER BY version DESC LIMIT 1", docID).Scan(&lastEdit)
	stats["first_edit"] = firstEdit
	stats["last_edit"] = lastEdit

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

func stripHTMLTagsFast(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
