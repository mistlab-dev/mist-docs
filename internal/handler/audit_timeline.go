package handler

// 合规时间线（#79 T5）：把 MistDocs 审计（md_audits）与 MistTerm 汇入事件
// （md_notifications 中 term.attention / audit.alert，来自 #77 ingest）按
// 时间合并成一条只读视图，供等保/内审「谁在什么时候对什么做了什么」。
//
// 数据面刻意保持薄：MistTerm 侧只有已经走 ingest 端点进来的事件会出现，
// 交互式 PTY 手输命令若 MistTerm 未上报则不在本视图（响应带
// coverage_note，前端须展示覆盖边界文案，验收标准第三条）。不做 SIEM 对
// 接、不做新的存储架构。
//
// 权限：与 TeamListAudits 同口径——团队 admin + plan 含 audit（plan gate
// 打开且套餐不含时 402），成员/游客 403。不借本票上新 plan gate，沿用 #29
// 精神。

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// source / actor 过滤白名单：校验不通过的按未传处理（视图容错优先于
// 严格报错，与 audits 列表的 filter 行为一致）。
var timelineSources = map[string]bool{
	"docs": true, // md_audits：文档/团队操作审计
	"term": true, // md_notifications 中 term.* / audit.alert 汇入事件
}

var timelineActors = map[string]bool{
	"user":  true,
	"agent": true,
}

type timelineFilter struct {
	source    string // "" | docs | term
	actor     string // "" | user | agent
	userName  string // actor_name LIKE（audit 臂查 display_name/user_name，term 臂查 title 里的 [agent] 前缀段）
	action    string // audits.action 精确值 / notifications.type 精确值
	startDate string // YYYY-MM-DD
	endDate   string // YYYY-MM-DD
}

func parseTimelineFilter(c *gin.Context) timelineFilter {
	f := timelineFilter{
		source:    strings.TrimSpace(c.Query("source")),
		actor:     strings.TrimSpace(c.Query("actor")),
		action:    strings.TrimSpace(c.Query("action")),
		startDate: strings.TrimSpace(c.Query("start_date")),
		endDate:   strings.TrimSpace(c.Query("end_date")),
	}
	if !timelineSources[f.source] {
		f.source = ""
	}
	if !timelineActors[f.actor] {
		f.actor = ""
	}
	if name := strings.TrimSpace(c.Query("user_name")); name != "" {
		runes := []rune(name)
		if len(runes) > 50 {
			name = string(runes[:50])
		}
		// 与 buildAuditFilter 同一套 LIKE 转义（反斜杠是 MySQL 默认
		// ESCAPE 字符，无需 ESCAPE 子句）。
		f.userName = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(name)
	}
	// action 同时匹配 audits.action（如 update_doc）与
	// notifications.type（term.attention——含 '.'，所以不能复用
	// validAuditAction；term 事件名是 ingest 白名单写死的，无注入面）。
	if len(f.action) > 40 {
		f.action = ""
	}
	for _, r := range f.action {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '.' {
			f.action = ""
			break
		}
	}
	if _, err := time.Parse("2006-01-02", f.startDate); err != nil {
		f.startDate = ""
	}
	if _, err := time.Parse("2006-01-02", f.endDate); err != nil {
		f.endDate = ""
	}
	return f
}

// timelineUnion 返回不带 ORDER BY/LIMIT 的 UNION 子查询（别名 t）和参数。
// 参数顺序与占位符一致：audit 臂 = [teamID, auditArgs...]，term 臂 =
// [teamID, termArgs...]。
//
// term 臂按 (type,title,related_id,document_id,created_at) GROUP BY 去重：
// ingest 给每个团队成员插一行相同的通知，created_at 是秒级 DATETIME，同一
// 次 ingest 的批量插入通常落在同一秒；极端大团队跨秒会拆成相邻两行，可
// 接受且如实。
func timelineUnion(teamID string, f timelineFilter) (string, []interface{}) {
	// auditFromClause() 自带 "WHERE a.team_id=?"，过滤条件以 AND 续接。
	auditWhere := ""
	auditArgs := []interface{}{}
	termWhere := ""
	termArgs := []interface{}{}

	if f.actor == "agent" {
		auditWhere += " AND a.actor_type='agent'"
	} else if f.actor == "user" {
		auditWhere += " AND a.actor_type='user'"
		// ingest 端点只允许 agt_ token，term 臂主体恒为 agent；
		// actor=user 时整臂为空。
		termWhere += " AND 1=0"
	}
	if f.action != "" {
		// 同一参数分别匹配两臂：查 "term.attention" 只留 term 臂，
		// 查 "update_doc" 只留 audit 臂，互不串。
		auditWhere += " AND a.action=?"
		auditArgs = append(auditArgs, f.action)
		termWhere += " AND n.type=?"
		termArgs = append(termArgs, f.action)
	}
	if f.userName != "" {
		pat := "%" + f.userName + "%"
		auditWhere += " AND (IFNULL(u.display_name,'') LIKE ? OR IFNULL(a.user_name,'') LIKE ?)"
		auditArgs = append(auditArgs, pat, pat)
		termWhere += " AND n.title LIKE ?"
		termArgs = append(termArgs, pat)
	}
	if f.startDate != "" {
		auditWhere += " AND a.created_at>=?"
		auditArgs = append(auditArgs, f.startDate+" 00:00:00")
		termWhere += " AND n.created_at>=?"
		termArgs = append(termArgs, f.startDate+" 00:00:00")
	}
	if f.endDate != "" {
		auditWhere += " AND a.created_at<=?"
		auditArgs = append(auditArgs, f.endDate+" 23:59:59")
		termWhere += " AND n.created_at<=?"
		termArgs = append(termArgs, f.endDate+" 23:59:59")
	}
	if f.source == "docs" {
		termWhere += " AND 1=0"
	} else if f.source == "term" {
		auditWhere += " AND 1=0"
	}

	// 统一列形：source, event, actor_type, actor_name, resource_type,
	// resource_id, resource_name, detail, created_at
	// ⚠️ term 臂的 detail 只吐原始 document_id（可能为空串），由
	// scanTimelineRows 在 Go 侧用 json.Marshal 组 JSON——不要在 SQL 里
	// CONCAT 拼 JSON：document_id 来自 ingest 仅做长度校验，含 " 或 \
	// 时拼出的会是非法 JSON（#85 review 中1）。
	// ⚠️ 每个字符串输出列都显式 CONVERT 到同一 collation：库里 users /
	// md_documents 与 md_audits / md_notifications 的默认 collation 不一致
	//（auditFromClause 里的 COLLATE utf8mb4_unicode_ci 是同款坑的先例），
	// UNION 两臂列 collation 不同会直接 Error 1271 Illegal mix of
	// collations for operation 'UNION'（CI 实证）。
	union := `SELECT * FROM (
		SELECT 'docs' AS source, CONVERT(a.action USING utf8mb4) COLLATE utf8mb4_unicode_ci AS event,
		 CONVERT(a.actor_type USING utf8mb4) COLLATE utf8mb4_unicode_ci AS actor_type,
		 CONVERT(COALESCE(NULLIF(u.display_name,''), NULLIF(a.user_name,''), '') USING utf8mb4) COLLATE utf8mb4_unicode_ci AS actor_name,
		 CONVERT(a.resource_type USING utf8mb4) COLLATE utf8mb4_unicode_ci AS resource_type,
		 CONVERT(a.resource_id USING utf8mb4) COLLATE utf8mb4_unicode_ci AS resource_id,
		 CONVERT(` + auditNameExpr + ` USING utf8mb4) COLLATE utf8mb4_unicode_ci AS resource_name,
		 CONVERT(IFNULL(a.detail,'') USING utf8mb4) COLLATE utf8mb4_unicode_ci AS detail, a.created_at
		` + auditFromClause() + auditWhere + `
		UNION ALL
		SELECT 'term', CONVERT(n.type USING utf8mb4) COLLATE utf8mb4_unicode_ci, 'agent',
		 CONVERT(CASE WHEN n.title LIKE '[%]%' THEN SUBSTRING_INDEX(SUBSTRING_INDEX(n.title,']',1),'[',-1) ELSE '' END USING utf8mb4) COLLATE utf8mb4_unicode_ci,
		 'event', CONVERT(IFNULL(n.related_id,'') USING utf8mb4) COLLATE utf8mb4_unicode_ci,
		 CONVERT(n.title USING utf8mb4) COLLATE utf8mb4_unicode_ci,
		 CONVERT(IFNULL(n.document_id,'') USING utf8mb4) COLLATE utf8mb4_unicode_ci,
		 n.created_at
		FROM md_notifications n
		WHERE n.team_id=? AND n.type IN ('term.attention','audit.alert')` + termWhere + `
		GROUP BY n.type, n.title, n.related_id, n.document_id, n.created_at
	) t`

	args := []interface{}{teamID}
	args = append(args, auditArgs...)
	args = append(args, teamID)
	args = append(args, termArgs...)
	return union, args
}

func timelineAdminOnly(c *gin.Context) bool {
	// plan gate 与 requireTeamAdmin 的顺序跟 TeamListAudits 一致：
	// 先 402 再 403（前端按套餐决定是否展示入口）。
	if !service.RequirePlanFeature(c, "audit") {
		return false
	}
	return requireTeamAdmin(c)
}

func scanTimelineRows(rows interface {
	Next() bool
	Scan(...interface{}) error
}) []map[string]interface{} {
	var events []map[string]interface{}
	for rows.Next() {
		var source, event, actorType, actorName, resType, resID, resName, detail, createdAt string
		if rows.Scan(&source, &event, &actorType, &actorName, &resType, &resID, &resName, &detail, &createdAt) != nil {
			continue
		}
		if source == "term" && detail != "" {
			// term 臂 detail 列是原始 document_id，这里统一组成 JSON，
			// 保证前端拿到的 detail 永远是合法 JSON 字符串。
			detail = auditDetail(gin.H{"document_id": detail})
		}
		events = append(events, map[string]interface{}{
			"source": source, "event": event,
			"actor_type": actorType, "actor_name": actorName,
			"resource_type": resType, "resource_id": resID,
			"resource_name": resName, "detail": detail,
			"created_at": createdAt,
		})
	}
	if events == nil {
		events = []map[string]interface{}{}
	}
	return events
}

// coverageNote 是给前端直接展示的覆盖边界文案（验收标准：覆盖边界对用户
// 可见，避免过度承诺）。
const coverageNote = "term 分区仅覆盖已通过 ingest 端点汇入的 MistTerm 事件（term.attention/audit.alert）；交互式手输命令以 MistTerm 本地审计为准。"

// TeamAuditTimeline GET /teams/:team_id/timeline — 合并时间线（JSON）。
// Query: source(docs|term) / actor(user|agent) / action / user_name /
// start_date / end_date / page / page_size(<=200)。
func TeamAuditTimeline(c *gin.Context) {
	if !timelineAdminOnly(c) {
		return
	}
	teamID := getTeamID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	f := parseTimelineFilter(c)
	union, args := timelineUnion(teamID, f)

	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM (`+union+`) x`, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	listArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := database.DB.Query(union+` ORDER BY t.created_at DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	c.JSON(http.StatusOK, gin.H{
		"data":          scanTimelineRows(rows),
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
		"coverage_note": coverageNote,
	})
}

// TeamAuditTimelineExport GET /teams/:team_id/timeline/export?format=csv|json
// 与列表同一过滤面，上限 10000 行，CSV 带 BOM（Excel 直开不乱码）。
// 导出一个只读动作，不额外写 audit（与 TeamExportAudits 一致）。
func TeamAuditTimelineExport(c *gin.Context) {
	if !timelineAdminOnly(c) {
		return
	}
	teamID := getTeamID(c)
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	if format != "csv" && format != "json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be csv or json"})
		return
	}
	f := parseTimelineFilter(c)
	union, args := timelineUnion(teamID, f)
	args = append(args, 10000)

	rows, err := database.DB.Query(union+` ORDER BY t.created_at DESC LIMIT ?`, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	events := scanTimelineRows(rows)

	if format == "json" {
		c.Header("Content-Disposition", "attachment; filename=timeline.json")
		c.JSON(http.StatusOK, gin.H{"data": events, "count": len(events), "coverage_note": coverageNote})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=timeline.csv")
	c.Writer.Write([]byte("\xEF\xBB\xBF"))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"时间", "来源", "事件", "主体类型", "主体", "资源类型", "资源名称", "资源ID", "详情"})
	for _, e := range events {
		_ = w.Write([]string{
			e["created_at"].(string), e["source"].(string), e["event"].(string),
			e["actor_type"].(string), e["actor_name"].(string),
			e["resource_type"].(string), e["resource_name"].(string),
			e["resource_id"].(string), e["detail"].(string),
		})
	}
	w.Flush()
}
