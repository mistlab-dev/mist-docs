package handler

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"unicode/utf8"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// 反向链接（Backlinks）：[[ 双链只解决了“去”，这里解决“谁引用了我”。
//
// 双链插入的是 `<a target="_blank" rel="noopener" class="editor-link" href="https://.../docs/{id}">标题</a>`，
// 历史导入中也可能有纯 Markdown 的 `[标题](/docs/{id})`。无论哪种，在 content_text
// 中都包含 `/docs/{id}` 字符串。
//
// 性能与准确性设计：
// 1. 查询阶段：在主查询中直接用 SUBSTRING 取目标链接前后的局部正文切片（~1000 字符），
//    避免对每篇候选文档单独发起二次查询去拉取整段 LONGTEXT。
// 2. 权限过滤：主查询仅作为粗筛候选，逐篇过 HasTeamPermission(read) 校验，
//    确保用户无权阅读的文档绝不泄漏标题和摘要。
// 3. Excerpt 处理：先定位指向 targetID 的 <a> 标签并用锚文本替换，再去标签、反转义 HTML 实体，
//    最后复用 rune 安全的边界截取，防止将 HTML 标签截半或切碎 UTF-8 字符。

// targetAnchorRe 匹配指向特定文档 id 的 <a> 标签及其锚文本：<a ... href=".../docs/{id}...">锚文本</a>
func makeTargetAnchorRe(targetID string) *regexp.Regexp {
	pattern := fmt.Sprintf(`(?is)<a\b[^>]*\bhref=["'][^"']*/docs/%s(?:\?[^"']*)?["'][^>]*>(.*?)</a>`, regexp.QuoteMeta(targetID))
	return regexp.MustCompile(pattern)
}

// formatBacklinkExcerpt 将包含指向 targetID 链接的 HTML 片段提炼为适合侧边栏展示的纯文本摘要。
func formatBacklinkExcerpt(rawHTML, targetID string) string {
	if rawHTML == "" {
		return ""
	}

	// 1. 先用指向目标的 <a> 标签的锚文本直接替换掉整段 <a ...>...</a>
	// 这样可以避免切片边界恰好切在 <a href="..."> 标签中间而留下残缺的属性文本
	anchorRe := makeTargetAnchorRe(targetID)
	anchorHit := anchorRe.FindStringIndex(rawHTML)
	replaced := anchorRe.ReplaceAllString(rawHTML, " $1 ")

	// 2. 清理 script / style 块、剥离其他 HTML 标签、反转义 HTML 实体、归一化连续空白
	cleaned := stripScriptStyle(replaced)
	cleaned = htmlTagRe.ReplaceAllString(cleaned, " ")
	cleaned = html.UnescapeString(cleaned)
	cleaned = collapseSpace(cleaned)
	if cleaned == "" {
		return ""
	}

	// 3. 定位焦点中心：如果替换前找到了 <a> 的位置，在清洗后的文本中尽量居中展现
	const targetWindow = 100
	runes := []rune(cleaned)
	if len(runes) <= targetWindow {
		return cleaned
	}

	// 找到原锚文本在清洗后文本里的位置以居中窗口
	focusIndex := 0
	if anchorHit != nil {
		// 估算焦点位置在清洗文本中的大致比例
		ratio := float64(anchorHit[0]) / float64(len(rawHTML))
		focusIndex = int(float64(len(runes)) * ratio)
		if focusIndex >= len(runes) {
			focusIndex = len(runes) / 2
		}
	} else {
		focusIndex = len(runes) / 2
	}

	start := focusIndex - targetWindow/2
	if start < 0 {
		start = 0
	}
	end := start + targetWindow
	if end > len(runes) {
		end = len(runes)
		start = end - targetWindow
		if start < 0 {
			start = 0
		}
	}

	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet = snippet + "…"
	}
	return snippet
}

// TeamListBacklinks GET /teams/:team_id/documents/:id/backlinks
// 返回引用了当前文档的来源文档列表，严格经过当前用户的团队 ACL 读权限过滤。
func TeamListBacklinks(c *gin.Context) {
	teamID := getTeamID(c)
	docID := c.Param("id")
	userID := c.GetString("user_id")
	teamRole := getTeamRole(c)

	// 本文档必须属于当前团队且当前用户具备读权限
	if !requireDoc(c, docID, "read", true) {
		return
	}

	// 在同团队活跃文档中反查正文包含 `/docs/{docID}` 的文档
	// 主查询中直接使用 LOCATE + SUBSTRING 获取链接附近的局部正文（前后约 400 字符），
	// 避免对 200 条候选逐个进行二次查询拉取整篇大字段 content_text
	needle := "/docs/" + docID
	likePattern := "%" + needle + "%"

	query := `SELECT d.id, d.title, d.type, d.updated_at, IFNULL(d.folder_id, ''),
		        COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), '') as updated_by_name,
		        SUBSTRING(d.content_text, GREATEST(LOCATE(?, d.content_text) - 400, 1), 900) as excerpt_raw
		 FROM md_documents d
		 LEFT JOIN users u ON d.updated_by COLLATE utf8mb4_unicode_ci = u.id
		 WHERE d.team_id = ? AND d.status = 1 AND d.id <> ? AND d.content_text LIKE ?
		 ORDER BY d.updated_at DESC
		 LIMIT 200`

	rows, err := database.DB.Query(query, needle, teamID, docID, likePattern)
	if err != nil {
		log.Printf("list backlinks query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type backlinkCandidate struct {
		ID            string
		Title         string
		Type          string
		UpdatedAt     string
		FolderID      string
		UpdatedByName string
		ExcerptRaw    string
	}

	var candidates []backlinkCandidate
	for rows.Next() {
		var b backlinkCandidate
		var excerptRaw sql.NullString
		if err := rows.Scan(&b.ID, &b.Title, &b.Type, &b.UpdatedAt, &b.FolderID, &b.UpdatedByName, &excerptRaw); err != nil {
			log.Printf("scan backlink row error: %v", err)
			continue
		}
		b.ExcerptRaw = excerptRaw.String
		candidates = append(candidates, b)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate backlinks error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 逐篇进行权限过滤：调用方没有读权限的文档，其标题和摘要绝不返回
	ctx := c.Request.Context()
	out := make([]gin.H, 0, len(candidates))
	for _, b := range candidates {
		if !service.HasTeamPermission(ctx, userID, teamID, teamRole, "document", b.ID, "read") {
			continue
		}
		out = append(out, gin.H{
			"id":              b.ID,
			"title":           b.Title,
			"type":            b.Type,
			"updated_at":      b.UpdatedAt,
			"folder_id":       b.FolderID,
			"updated_by_name": b.UpdatedByName,
			"excerpt":         formatBacklinkExcerpt(b.ExcerptRaw, docID),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
}

// 确保引用到了 utf8 包以防依赖警告
var _ = utf8.RuneStart
