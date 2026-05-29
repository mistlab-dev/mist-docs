package handler

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
)

// DashboardStats returns admin dashboard statistics.
// GET /admin/dashboard
func DashboardStats(c *gin.Context) {
	db := database.DB

	stats := gin.H{}

	// 用户统计
	var userCount, activeUsers int
	db.QueryRow("SELECT COUNT(*) FROM md_users").Scan(&userCount)
	db.QueryRow("SELECT COUNT(*) FROM md_users WHERE status = 1").Scan(&activeUsers)
	stats["users"] = gin.H{"total": userCount, "active": activeUsers}

	// 部门统计
	var deptCount int
	db.QueryRow("SELECT COUNT(*) FROM md_departments").Scan(&deptCount)
	stats["departments"] = deptCount

	// 文档统计
	var docCount, docTotal, sheetCount int
	db.QueryRow("SELECT COUNT(*) FROM md_documents WHERE status = 1").Scan(&docCount)
	db.QueryRow("SELECT COUNT(*) FROM md_documents WHERE status = 1 AND type = 'doc'").Scan(&docTotal)
	db.QueryRow("SELECT COUNT(*) FROM md_documents WHERE status = 1 AND type = 'sheet'").Scan(&sheetCount)
	stats["documents"] = gin.H{"total": docCount, "docs": docTotal, "sheets": sheetCount}

	// 回收站
	var trashCount int
	db.QueryRow("SELECT COUNT(*) FROM md_documents WHERE status = 0").Scan(&trashCount)
	stats["trash"] = trashCount

	// 存储用量
	var totalSize sql.NullInt64
	db.QueryRow("SELECT COALESCE(SUM(file_size), 0) FROM md_documents WHERE status = 1").Scan(&totalSize)
	stats["storage_bytes"] = totalSize.Int64

	// 最近7天活跃（有更新的文档）
	var weekActive int
	db.QueryRow("SELECT COUNT(*) FROM md_documents WHERE updated_at > ? AND status = 1", time.Now().AddDate(0, 0, -7)).Scan(&weekActive)
	stats["week_active"] = weekActive

	// 最近7天新增
	var weekNew int
	db.QueryRow("SELECT COUNT(*) FROM md_documents WHERE created_at > ? AND status = 1", time.Now().AddDate(0, 0, -7)).Scan(&weekNew)
	stats["week_new"] = weekNew

	// 审计统计
	var auditCount int
	db.QueryRow("SELECT COUNT(*) FROM md_audits").Scan(&auditCount)
	var auditToday int
	db.QueryRow("SELECT COUNT(*) FROM md_audits WHERE created_at > ?", time.Now().Truncate(24*time.Hour)).Scan(&auditToday)
	stats["audits"] = gin.H{"total": auditCount, "today": auditToday}

	// 最近活动（最新10条）
	rows, err := db.Query(
		"SELECT action, resource_type, resource_name, user_name, created_at FROM md_audits ORDER BY created_at DESC LIMIT 10",
	)
	if err == nil {
		defer rows.Close()
		activities := []gin.H{}
		for rows.Next() {
			var action, rType, rName, userName string
			var createdAt time.Time
			rows.Scan(&action, &rType, &rName, &userName, &createdAt)
			activities = append(activities, gin.H{
				"action":        action,
				"resource_type": rType,
				"resource_name": rName,
				"user_name":     userName,
				"created_at":    createdAt,
			})
		}
		stats["recent_activities"] = activities
	}

	// 按天统计最近7天文档活跃
	rows2, err := db.Query(`
		SELECT DATE(created_at) as d, COUNT(*) as cnt
		FROM md_documents
		WHERE created_at > ? AND status = 1
		GROUP BY DATE(created_at)
		ORDER BY d
	`, time.Now().AddDate(0, 0, -7))
	if err == nil {
		defer rows2.Close()
		daily := []gin.H{}
		for rows2.Next() {
			var d time.Time
			var cnt int
			rows2.Scan(&d, &cnt)
			daily = append(daily, gin.H{"date": d.Format("2006-01-02"), "count": cnt})
		}
		stats["daily_new"] = daily
	}

	// 按天统计最近7天审计活跃
	rows3, err := db.Query(`
		SELECT DATE(created_at) as d, COUNT(*) as cnt
		FROM md_audits
		WHERE created_at > ?
		GROUP BY DATE(created_at)
		ORDER BY d
	`, time.Now().AddDate(0, 0, -7))
	if err == nil {
		defer rows3.Close()
		dailyAudit := []gin.H{}
		for rows3.Next() {
			var d time.Time
			var cnt int
			rows3.Scan(&d, &cnt)
			dailyAudit = append(dailyAudit, gin.H{"date": d.Format("2006-01-02"), "count": cnt})
		}
		stats["daily_audit"] = dailyAudit
	}

	// 分享统计
	var shareCount int
	db.QueryRow("SELECT COUNT(*) FROM md_shares WHERE status = 1").Scan(&shareCount)
	stats["shares"] = shareCount

	// 评论统计
	var commentCount int
	db.QueryRow("SELECT COUNT(*) FROM md_comments").Scan(&commentCount)
	var commentToday int
	db.QueryRow("SELECT COUNT(*) FROM md_comments WHERE created_at > ?", time.Now().Truncate(24*time.Hour)).Scan(&commentToday)
	stats["comments"] = gin.H{"total": commentCount, "today": commentToday}

	c.JSON(200, gin.H{"data": stats})
}

// SystemInfo returns system runtime info.
// GET /admin/system-info
func SystemInfo(c *gin.Context) {
	db := database.DB

	info := gin.H{}

	// 数据库统计
	var tableCount int
	db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE 'md_%'").Scan(&tableCount)
	info["version"] = "2.0.0"
	info["db_tables"] = tableCount

	// 服务运行时间
	var uptime string
	row := db.QueryRow("SELECT TIMEDIFF(NOW(), @@global.time_zone)")
	row.Scan(&uptime)
	info["db_time"] = fmt.Sprintf("%v", uptime)

	// 各表行数
	tables := []string{"md_users", "md_departments", "md_documents", "md_folders", "md_audits", "md_permissions", "md_versions", "md_favorites", "md_shares", "md_comments"}
	tableStats := []gin.H{}
	for _, t := range tables {
		var cnt int
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", t)).Scan(&cnt)
		tableStats = append(tableStats, gin.H{"table": t, "rows": cnt})
	}
	info["table_stats"] = tableStats

	c.JSON(200, gin.H{"data": info})
}
