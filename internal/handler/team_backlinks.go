package handler

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// 反向链接（Backlinks）：[[ 双链只解决了“去”，这里解决“谁引用了我”。
//
// 双链插入的是 <a href="/docs/{id}">标题</a>（见 DocEditor.insertWikiDocLink），
// 也有历史内容直接写 [标题](/docs/{id})，两种都在 content_text 里留有
// "/docs/{id}" 片段，反查用 LIKE 即可。结果按 ACL 逐篇过滤，只返回调用者
// 能读到的文档。

// backlinkIDRe 用于从正文中提取 /docs/{uuid} 形式的目标文档 id。
var backlinkIDRe = regexp.MustCompile(`(?:https?://[^\s"'<>]+)?/docs/([0-9a-fA-F][0-9a-fA-F-]{10,36})`)

// parseBacklinkTargets returns the distinct document ids referenced by body.
func parseBacklinkTargets(body string) map[string]bool {
	targets := map[string]bool{}
	for _, m := range backlinkIDRe.FindAllStringSubmatch(body, -1) {
		if id := strings.TrimSpace(m[1]); id != "" {
			targets[id] = true
		}
	}
	return targets
}

// TeamListBacklinks GET /teams/:team_id/documents/:id/backlinks
// 返回引用了本文档的文档列表，仅含调用者可读的。
func TeamListBacklinks(c *gin.Context) {
	teamID := getTeamID(c)
	docID := c.Param("id")
	userID := c.GetString("user_id")
	teamRole := getTeamRole(c)

	// 本文档必须属于当前团队且可读（requireDoc 兼查 404 与无权）。
	if !requireDoc(c, docID, "read", true) {
		return
	}

	// 同团队活跃文档里反查含 /docs/{docID} 的候选。
	needle := "%/docs/" + docID + "%"
	rows, err := database.DB.Query(
		`SELECT d.id, d.title, d.type, d.updated_at, IFNULL(d.folder_id, ''),
		        COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), '')
		 FROM md_documents d
		 LEFT JOIN users u ON d.updated_by COLLATE utf8mb4_unicode_ci = u.id
		 WHERE d.team_id = ? AND d.status = 1 AND d.id <> ? AND d.content_text LIKE ?
		 ORDER BY d.updated_at DESC
		 LIMIT 200`, teamID, docID, needle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type backlink struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Type       string `json:"type"`
		UpdatedAt  string `json:"updated_at"`
		FolderID   string `json:"folder_id"`
		UpdatedBy  string `json:"updated_by_name"`
		Excerpt    string `json:"excerpt"`
	}
	var candidates []backlink
	for rows.Next() {
		var b backlink
		if err := rows.Scan(&b.ID, &b.Title, &b.Type, &b.UpdatedAt, &b.FolderID, &b.UpdatedBy); err != nil {
			continue
		}
		candidates = append(candidates, b)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// ACL 过滤：只保留调用者能读的文档（与 search_docs / recent 一致）。
	ctx := c.Request.Context()
	out := make([]gin.H, 0, len(candidates))
	for _, b := range candidates {
		if !service.HasTeamPermission(ctx, userID, teamID, teamRole, "document", b.ID, "read") {
			continue
		}
		out = append(out, gin.H{
			"id": b.ID, "title": b.Title, "type": b.Type,
			"updated_at": b.UpdatedAt, "folder_id": b.FolderID,
			"updated_by_name": b.UpdatedBy,
			"excerpt": backlinkExcerpt(b.ID, docID),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
}

// backlinkExcerpt 取 source 文档里指向 target 的一小段上下文，
// 帮助用户不打开就能判断引用位置。
func backlinkExcerpt(sourceID, targetID string) string {
	var content string
	database.DB.QueryRow(
		`SELECT IFNULL(content_text, '') FROM md_documents WHERE id=?`, sourceID).Scan(&content)
	if content == "" {
		return ""
	}
	idx := strings.Index(content, "/docs/"+targetID)
	if idx < 0 {
		return ""
	}
	const window = 120
	start := idx - window/2
	if start < 0 {
		start = 0
	}
	end := idx + window + len(targetID)
	if end > len(content) {
		end = len(content)
	}
	snippet := collapseSpace(htmlTagRe.ReplaceAllString(content[start:end], " "))
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(content) {
		snippet += "…"
	}
	return snippet
}
