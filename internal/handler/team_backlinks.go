package handler

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"strings"
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

// targetAnchorRe 匹配指向特定文档 id 的 <a> 标签及其锚文本：<a ... href=".../docs/{id}...">锚文本</a>
func makeTargetAnchorRe(targetID string) *regexp.Regexp {
	pattern := fmt.Sprintf(`(?is)<a\b[^>]*\bhref=["'][^"']*/docs/%s(?:\?[^"']*)?["'][^>]*>(.*?)</a>`, regexp.QuoteMeta(targetID))
	return regexp.MustCompile(pattern)
}

// markdownLinkRe 匹配 Markdown 格式的链接：[标题](.../docs/{id}...)
func makeMarkdownLinkRe(targetID string) *regexp.Regexp {
	pattern := fmt.Sprintf(`(?is)\[([^\]]+)\]\([^)]*/docs/%s(?:\?[^)]*)?\)`, regexp.QuoteMeta(targetID))
	return regexp.MustCompile(pattern)
}

// sanitizeSubstringBoundary 清理从正文中 SUBSTRING 截取时可能截出的残缺 HTML 标签：
// 1. 开头若有残损的属性片段（即在第一个 '<' 之前出现了 '>'），丢弃该前缀；
// 2. 结尾若有未闭合的标签（即在最后一个 '>' 之后出现了 '<'），丢弃该后缀。
func sanitizeSubstringBoundary(s string) string {
	firstOpen := strings.Index(s, "<")
	firstClose := strings.Index(s, ">")
	if firstClose >= 0 && (firstOpen < 0 || firstClose < firstOpen) {
		s = s[firstClose+1:]
	}

	lastOpen := strings.LastIndex(s, "<")
	lastClose := strings.LastIndex(s, ">")
	if lastOpen >= 0 && (lastClose < 0 || lastOpen > lastClose) {
		s = s[:lastOpen]
	}
	return s
}

const focusMarker = "\x00"

// formatBacklinkExcerpt 将包含指向 targetID 链接的 HTML 片段提炼为适合侧边栏展示的纯文本摘要。
func formatBacklinkExcerpt(rawHTML, targetID string) string {
	if rawHTML == "" {
		return ""
	}

	// 0. 清理从长文本切片边界切出来的残损 HTML 标签片段
	rawHTML = sanitizeSubstringBoundary(rawHTML)

	// 1. 先用标记字符包裹锚文本替换掉整段链接标签：
	//    - HTML <a> 标签：用 \x00$1\x00 替换
	//    - Markdown 链接 [标题](/docs/{id})：用 \x00$1\x00 替换
	anchorRe := makeTargetAnchorRe(targetID)
	replaced := anchorRe.ReplaceAllString(rawHTML, " "+focusMarker+"$1"+focusMarker+" ")

	mdRe := makeMarkdownLinkRe(targetID)
	replaced = mdRe.ReplaceAllString(replaced, " "+focusMarker+"$1"+focusMarker+" ")

	// 2. 清理 script / style 块、剥离其他 HTML 标签、反转义 HTML 实体、归一化连续空白
	cleaned := stripScriptStyle(replaced)
	cleaned = htmlTagRe.ReplaceAllString(cleaned, " ")
	cleaned = html.UnescapeString(cleaned)
	cleaned = collapseSpace(cleaned)
	if cleaned == "" {
		return ""
	}

	// 3. 依据标记字符 \x00 精确寻找链接锚文本在纯文本中的 rune 偏移量
	runes := []rune(cleaned)
	const targetWindow = 100

	focusIndex := -1
	for i, r := range runes {
		if r == '\x00' {
			focusIndex = i
			break
		}
	}

	// 去除所有 \x00 标记字符，并在去掉标记后的 runes 上做最终居中窗口截取
	var finalRunes []rune
	adjustedFocus := focusIndex
	for _, r := range runes {
		if r == '\x00' {
			continue
		}
		finalRunes = append(finalRunes, r)
	}

	if len(finalRunes) <= targetWindow {
		return string(finalRunes)
	}

	if adjustedFocus < 0 || adjustedFocus >= len(finalRunes) {
		adjustedFocus = len(finalRunes) / 2
	}

	start := adjustedFocus - targetWindow/2
	if start < 0 {
		start = 0
	}
	end := start + targetWindow
	if end > len(finalRunes) {
		end = len(finalRunes)
		start = end - targetWindow
		if start < 0 {
			start = 0
		}
	}

	snippet := string(finalRunes[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(finalRunes) {
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
