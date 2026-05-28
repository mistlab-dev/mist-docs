package service

import (
	"context"
	"database/sql"
	"encoding/csv"
	"io"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/google/uuid"
)

// ==================== 审计日志 ====================

func CreateAudit(ctx context.Context, userID, userName, deptID, action, resourceType, resourceID, resourceName, detail, ip string) error {
	_, err := database.DB.ExecContext(ctx,
		`INSERT INTO md_audits (id, user_id, user_name, department_id, action, resource_type, resource_id, resource_name, detail, ip)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), userID, userName, deptID, action, resourceType, resourceID, resourceName, detail, ip,
	)
	return err
}

func ListAudits(ctx context.Context, userID, deptID, action, resourceID string, startDate, endDate string, page, pageSize int) ([]*model.DocAudit, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}

	if userID != "" {
		where += " AND user_id = ?"
		args = append(args, userID)
	}
	if deptID != "" {
		where += " AND department_id = ?"
		args = append(args, deptID)
	}
	if action != "" {
		where += " AND action = ?"
		args = append(args, action)
	}
	if resourceID != "" {
		where += " AND resource_id = ?"
		args = append(args, resourceID)
	}
	if startDate != "" {
		where += " AND created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}

	var total int
	database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM md_audits "+where, args...).Scan(&total)

	offset := (page - 1) * pageSize
	listSQL := `SELECT id, user_id, user_name, department_id, action, resource_type, resource_id, resource_name, detail, ip, created_at
	            FROM md_audits ` + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := database.DB.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	audits := []*model.DocAudit{}
	for rows.Next() {
		a := &model.DocAudit{}
		var detail sql.NullString
		var deptID sql.NullString
		var resourceType, resourceID, resourceName sql.NullString
		if err := rows.Scan(&a.ID, &a.UserID, &a.UserName, &deptID, &a.Action,
			&resourceType, &resourceID, &resourceName, &detail, &a.IP, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		a.DepartmentID = ns(deptID)
		a.ResourceType = ns(resourceType)
		a.ResourceID = ns(resourceID)
		a.ResourceName = ns(resourceName)
		a.Detail = ns(detail)
		audits = append(audits, a)
	}
	return audits, total, nil
}

func ExportAuditsCSV(ctx context.Context, userID, deptID, action, resourceID, startDate, endDate string, w io.Writer) error {
	audits, _, err := ListAudits(ctx, userID, deptID, action, resourceID, startDate, endDate, 1, 10000)
	if err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	cw.Write([]string{"时间", "用户", "部门", "操作", "资源类型", "资源名称", "详情", "IP"})

	for _, a := range audits {
		cw.Write([]string{
			a.CreatedAt.Format("2006-01-02 15:04:05"),
			a.UserName,
			a.DepartmentID,
			a.Action,
			a.ResourceType,
			a.ResourceName,
			a.Detail,
			a.IP,
		})
	}
	cw.Flush()
	return cw.Error()
}

// ==================== 统计 ====================

func AuditStats(ctx context.Context, startDate, endDate string) (map[string]interface{}, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	if startDate != "" {
		where += " AND created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}

	stats := map[string]interface{}{}

	// Total count
	var total int
	database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM md_audits "+where, args...).Scan(&total)
	stats["total"] = total

	// By action
	actionRows, err := database.DB.QueryContext(ctx, "SELECT action, COUNT(*) as cnt FROM md_audits "+where+" GROUP BY action", args...)
	if err == nil {
		defer actionRows.Close()
		byAction := map[string]int{}
		for actionRows.Next() {
			var act string
			var cnt int
			actionRows.Scan(&act, &cnt)
			byAction[act] = cnt
		}
		stats["by_action"] = byAction
	}

	// Active users (top 10)
	userRows, err := database.DB.QueryContext(ctx,
		`SELECT user_name, COUNT(*) as cnt FROM md_audits `+where+` GROUP BY user_id, user_name ORDER BY cnt DESC LIMIT 10`, args...)
	if err == nil {
		defer userRows.Close()
		activeUsers := []map[string]interface{}{}
		for userRows.Next() {
			var name string
			var cnt int
			userRows.Scan(&name, &cnt)
			activeUsers = append(activeUsers, map[string]interface{}{"name": name, "count": cnt})
		}
		stats["active_users"] = activeUsers
	}

	// Popular documents (top 10)
	docRows, err := database.DB.QueryContext(ctx,
		`SELECT resource_name, COUNT(*) as cnt FROM md_audits WHERE resource_type='document' `+where+` GROUP BY resource_id, resource_name ORDER BY cnt DESC LIMIT 10`, args...)
	if err == nil {
		defer docRows.Close()
		popularDocs := []map[string]interface{}{}
		for docRows.Next() {
			var name string
			var cnt int
			docRows.Scan(&name, &cnt)
			popularDocs = append(popularDocs, map[string]interface{}{"title": name, "count": cnt})
		}
		stats["popular_documents"] = popularDocs
	}

	return stats, nil
}

// ==================== 清理 ====================

func CleanOldAudits(ctx context.Context, retainDays int) error {
	if retainDays <= 0 {
		retainDays = 180
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays).Format("2006-01-02")
	_, err := database.DB.ExecContext(ctx, `DELETE FROM md_audits WHERE created_at < ?`, cutoff)
	return err
}