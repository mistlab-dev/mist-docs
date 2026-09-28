package handler

import (
	"database/sql"
	"net/http"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/schedule"
	"github.com/gin-gonic/gin"
)

// ==================== 交期解释器（只读，规则版） ====================

// loadExplainEvents returns the order's change history with actor names.
func loadExplainEvents(teamID, deadlineID string) ([]schedule.Event, error) {
	rows, err := database.DB.Query(
		`SELECT id, event_type, COALESCE(field,''), COALESCE(old_value,''), COALESCE(new_value,''),
		        COALESCE(reason,''), COALESCE(actor_id,''), created_at
		 FROM md_deadline_events WHERE team_id = ? AND deadline_id = ?
		 ORDER BY created_at, id LIMIT 500`, teamID, deadlineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schedule.Event
	names := map[string]string{}
	for rows.Next() {
		var e schedule.Event
		var actor string
		if err := rows.Scan(&e.ID, &e.Type, &e.Field, &e.Old, &e.New, &e.Reason, &actor, &e.At); err != nil {
			return nil, err
		}
		if actor != "" {
			if _, ok := names[actor]; !ok {
				names[actor] = userDisplayName(actor)
			}
			e.Actor = names[actor]
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// TeamExplainDeadline GET /teams/:team_id/deadlines/:id/explain
//
// "为什么可能晚？" for one order: a deterministic explanation built from the
// order's events, the insert-preview queue model and its overdue history
// (DESIGN-DEADLINE-AI.md §5.2). Read-only; any team member may call it.
// There is no AI narration yet: that belongs to mist-team-server.
func TeamExplainDeadline(c *gin.Context) {
	teamID := getTeamID(c)
	d, err := loadDeadlineRaw(teamID, c.Param("id"))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "交期记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	orders, err := loadOpenOrders(database.DB, teamID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tc, err := loadTeamCapacity(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	events, err := loadExplainEvents(teamID, d.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	target := schedule.Order{
		ID: d.ID, OrderNo: d.OrderNo, Title: d.Title, Customer: d.Customer, Remark: d.Remark,
		Priority: d.Priority, Status: d.Status, Progress: d.Progress,
		Start: d.StartDate, Due: d.DueDate, CreatedAt: d.CreatedAt,
	}
	ex := schedule.Explain(schedule.ExplainInput{
		Today: model.TodayStart(), Target: target, Orders: orders,
		Capacity: tc.Steps, KeyCustomers: tc.KeyCustomers, Events: events,
	})
	c.JSON(http.StatusOK, gin.H{"data": ex})
}
