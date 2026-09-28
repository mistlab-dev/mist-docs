package service

import (
	"context"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/google/uuid"
)

// ==================== 审计日志 ====================

func CreateAudit(ctx context.Context, userID, userName, deptID, teamID, action, resourceType, resourceID, resourceName, detail, ip string) error {
	_, err := database.DB.ExecContext(ctx,
		`INSERT INTO md_audits (id, team_id, user_id, user_name, department_id, action, resource_type, resource_id, resource_name, detail, ip)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), teamID, userID, userName, deptID, action, resourceType, resourceID, resourceName, detail, ip,
	)
	return err
}

// ==================== 清理 ====================

// CleanOldAudits deletes audit rows older than retainDays.
//
// NOTE: nothing schedules this yet, so audit.retain_days is not enforced.
// Wiring it into the scheduler deletes production rows, so that is left for
// an explicit decision rather than done as part of the legacy cleanup.
func CleanOldAudits(ctx context.Context, retainDays int) error {
	if retainDays <= 0 {
		retainDays = 180
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays).Format("2006-01-02")
	_, err := database.DB.ExecContext(ctx, `DELETE FROM md_audits WHERE created_at < ?`, cutoff)
	return err
}
