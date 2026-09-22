package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==================== 交期看板 ====================
//
// Deadlines are DB rows, not documents. See internal/database/mysql.go for why:
// an encrypted sheet blob cannot be queried, so a due-date scan against sheets
// would mean decrypting and parsing every file.

const dateLayout = "2006-01-02"

type deadlineInput struct {
	OrderNo   string `json:"order_no"`
	Title     string `json:"title"`
	Customer  string `json:"customer"`
	Quantity  int    `json:"quantity"`
	StartDate string `json:"start_date"`
	DueDate   string `json:"due_date"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Priority  string `json:"priority"`
	OwnerID   string `json:"owner_id"`
	Remark    string `json:"remark"`
	Reason    string `json:"reason"` // optional, recorded on the change event
}

var validStatuses = map[string]bool{"pending": true, "running": true, "done": true, "overdue": true}
var validPriorities = map[string]bool{"normal": true, "urgent": true, "inserted": true}

// scanDeadline reads one row into a Deadline and fills computed display fields.
func scanDeadline(rows interface {
	Scan(dest ...any) error
}) (*model.Deadline, error) {
	var d model.Deadline
	var startDate sql.NullTime
	var ownerID, customer, remark, updatedBy sql.NullString
	if err := rows.Scan(&d.ID, &d.TeamID, &d.OrderNo, &d.Title, &customer, &d.Quantity,
		&startDate, &d.DueDate, &d.Status, &d.Progress, &d.Priority,
		&ownerID, &remark, &d.CreatedBy, &d.CreatedAt, &updatedBy, &d.UpdatedAt); err != nil {
		return nil, err
	}
	d.Customer = customer.String
	d.OwnerID = ownerID.String
	d.Remark = remark.String
	d.UpdatedBy = updatedBy.String
	d.DueDateStr = d.DueDate.Format(dateLayout)
	if startDate.Valid {
		d.StartDate = &startDate.Time
		d.StartDateStr = startDate.Time.Format(dateLayout)
	}
	d.DaysLeft = model.DaysLeft(d.DueDate)
	d.RiskLevel = model.RiskLevel(d.DaysLeft, d.Status)
	return &d, nil
}

const deadlineColumns = `id, team_id, order_no, title, customer, quantity,
	start_date, due_date, status, progress, priority, owner_id, remark,
	created_by, created_at, updated_by, updated_at`

// ==================== CRUD ====================

// TeamListDeadlines GET /teams/:team_id/deadlines
//
// Supported query params: status, risk, owner_id, q (order no / title / customer),
// due_from, due_to, sort (due|created|priority), limit, offset.
func TeamListDeadlines(c *gin.Context) {
	teamID := getTeamID(c)

	where := []string{"team_id = ?", "deleted_at IS NULL"}
	args := []any{teamID}

	if v := c.Query("status"); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	if v := c.Query("owner_id"); v != "" {
		where = append(where, "owner_id = ?")
		args = append(args, v)
	}
	if v := c.Query("priority"); v != "" {
		where = append(where, "priority = ?")
		args = append(args, v)
	}
	if v := c.Query("q"); v != "" {
		where = append(where, "(order_no LIKE ? OR title LIKE ? OR customer LIKE ?)")
		like := "%" + v + "%"
		args = append(args, like, like, like)
	}
	if v := c.Query("due_from"); v != "" {
		where = append(where, "due_date >= ?")
		args = append(args, v)
	}
	if v := c.Query("due_to"); v != "" {
		where = append(where, "due_date <= ?")
		args = append(args, v)
	}

	// risk is computed, so it is expressed as a due_date range rather than a column.
	switch c.Query("risk") {
	case "overdue":
		where = append(where, "status <> 'done'", "due_date < CURDATE()")
	case "critical":
		where = append(where, "status <> 'done'", "due_date >= CURDATE()", "due_date <= DATE_ADD(CURDATE(), INTERVAL 3 DAY)")
	case "warning":
		where = append(where, "status <> 'done'", "due_date > DATE_ADD(CURDATE(), INTERVAL 3 DAY)", "due_date <= DATE_ADD(CURDATE(), INTERVAL 7 DAY)")
	case "open":
		where = append(where, "status <> 'done'")
	}

	orderBy := "due_date ASC, priority = 'urgent' DESC"
	switch c.Query("sort") {
	case "created":
		orderBy = "created_at DESC"
	case "due_desc":
		orderBy = "due_date DESC"
	}

	limit := 100
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = v
	}

	query := `SELECT ` + deadlineColumns + ` FROM md_deadlines
		WHERE ` + strings.Join(where, " AND ") + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := database.DB.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	items := []*model.Deadline{}
	for rows.Next() {
		d, err := scanDeadline(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items = append(items, d)
	}

	var total int
	database.DB.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM md_deadlines WHERE `+strings.Join(where, " AND "),
		args[:len(args)-2]...).Scan(&total)

	attachOwnerNames(items)

	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "limit": limit, "offset": offset})
}

// attachOwnerNames fills OwnerName with one lookup per distinct owner.
func attachOwnerNames(items []*model.Deadline) {
	ids := map[string]bool{}
	for _, d := range items {
		if d.OwnerID != "" {
			ids[d.OwnerID] = true
		}
	}
	if len(ids) == 0 {
		return
	}
	names := map[string]string{}
	for id := range ids {
		var name string
		if err := database.DB.QueryRow(`SELECT name FROM users WHERE id = ?`, id).Scan(&name); err == nil {
			names[id] = name
		}
	}
	for _, d := range items {
		d.OwnerName = names[d.OwnerID]
	}
}

// TeamCreateDeadline POST /teams/:team_id/deadlines
func TeamCreateDeadline(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")

	var in deadlineInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if strings.TrimSpace(in.Title) == "" && strings.TrimSpace(in.OrderNo) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单号和名称至少填一个"})
		return
	}
	due, err := time.ParseInLocation(dateLayout, in.DueDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交期格式应为 YYYY-MM-DD"})
		return
	}
	if in.Status == "" {
		in.Status = "pending"
	}
	if !validStatuses[in.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的状态"})
		return
	}
	if in.Priority == "" {
		in.Priority = "normal"
	}
	if !validPriorities[in.Priority] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的优先级"})
		return
	}

	var startDate any
	if in.StartDate != "" {
		if sd, err := time.ParseInLocation(dateLayout, in.StartDate, time.Local); err == nil {
			startDate = sd.Format(dateLayout)
		}
	}

	id := uuid.New().String()
	// An order with no explicit owner defaults to its creator, so reminders
	// always have somewhere to go.
	owner := in.OwnerID
	if owner == "" {
		owner = userID
	}

	_, err = database.DB.ExecContext(c.Request.Context(), `
		INSERT INTO md_deadlines
			(id, team_id, order_no, title, customer, quantity, start_date, due_date,
			 status, progress, priority, owner_id, remark, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, teamID, in.OrderNo, in.Title, in.Customer, in.Quantity, startDate, due.Format(dateLayout),
		in.Status, clampProgress(in.Progress), in.Priority, owner, in.Remark, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	recordDeadlineEvent(teamID, id, "created", "", "", in.OrderNo+" "+in.Title, in.Reason, userID)

	c.JSON(http.StatusOK, gin.H{"id": id})
}

// TeamGetDeadline GET /teams/:team_id/deadlines/:id
func TeamGetDeadline(c *gin.Context) {
	teamID := getTeamID(c)
	row := database.DB.QueryRowContext(c.Request.Context(),
		`SELECT `+deadlineColumns+` FROM md_deadlines WHERE id = ? AND team_id = ? AND deleted_at IS NULL`,
		c.Param("id"), teamID)

	d, err := scanDeadline(row)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "交期记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attachOwnerNames([]*model.Deadline{d})
	c.JSON(http.StatusOK, gin.H{"data": d})
}

// TeamUpdateDeadline PUT /teams/:team_id/deadlines/:id
//
// Every field change is written to md_deadline_events with the previous value,
// so "why did this slip" remains answerable months later.
func TeamUpdateDeadline(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	id := c.Param("id")

	var in deadlineInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	cur, err := loadDeadlineRaw(teamID, id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "交期记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build the new state, falling back to current values for omitted fields.
	next := *cur
	if in.OrderNo != "" {
		next.OrderNo = in.OrderNo
	}
	if in.Title != "" {
		next.Title = in.Title
	}
	if in.Customer != "" {
		next.Customer = in.Customer
	}
	if in.Quantity != 0 {
		next.Quantity = in.Quantity
	}
	if in.Status != "" {
		if !validStatuses[in.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的状态"})
			return
		}
		next.Status = in.Status
	}
	if in.Priority != "" {
		if !validPriorities[in.Priority] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的优先级"})
			return
		}
		next.Priority = in.Priority
	}
	if in.OwnerID != "" {
		next.OwnerID = in.OwnerID
	}
	if in.Remark != "" {
		next.Remark = in.Remark
	}
	if in.DueDate != "" {
		due, err := time.ParseInLocation(dateLayout, in.DueDate, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "交期格式应为 YYYY-MM-DD"})
			return
		}
		next.DueDate = due
	}
	var startDate any
	if in.StartDate != "" {
		if sd, err := time.ParseInLocation(dateLayout, in.StartDate, time.Local); err == nil {
			startDate = sd.Format(dateLayout)
		}
	}

	_, err = database.DB.ExecContext(c.Request.Context(), `
		UPDATE md_deadlines SET
			order_no = ?, title = ?, customer = ?, quantity = ?, start_date = ?,
			due_date = ?, status = ?, progress = ?, priority = ?, owner_id = ?,
			remark = ?, updated_by = ?
		WHERE id = ? AND team_id = ?`,
		next.OrderNo, next.Title, next.Customer, next.Quantity, startDate,
		next.DueDate.Format(dateLayout), next.Status, clampProgress(in.Progress),
		next.Priority, next.OwnerID, next.Remark, userID, id, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record the diff. A due_date change is the most important signal, so it
	// gets its own event type.
	if cur.DueDate.Format(dateLayout) != next.DueDate.Format(dateLayout) {
		recordDeadlineEvent(teamID, id, "date_changed", "due_date",
			cur.DueDate.Format(dateLayout), next.DueDate.Format(dateLayout), in.Reason, userID)
	}
	if cur.Status != next.Status {
		recordDeadlineEvent(teamID, id, "status_changed", "status", cur.Status, next.Status, in.Reason, userID)
	}
	if cur.OwnerID != next.OwnerID {
		recordDeadlineEvent(teamID, id, "updated", "owner_id", cur.OwnerID, next.OwnerID, in.Reason, userID)
	}
	if cur.Progress != in.Progress && in.Progress != 0 {
		recordDeadlineEvent(teamID, id, "updated", "progress",
			strconv.Itoa(cur.Progress), strconv.Itoa(clampProgress(in.Progress)), in.Reason, userID)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TeamDeleteDeadline DELETE /teams/:team_id/deadlines/:id
// Soft delete: the event history stays meaningful.
func TeamDeleteDeadline(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	id := c.Param("id")

	res, err := database.DB.ExecContext(c.Request.Context(),
		`UPDATE md_deadlines SET deleted_at = NOW(), updated_by = ? WHERE id = ? AND team_id = ? AND deleted_at IS NULL`,
		userID, id, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "交期记录不存在"})
		return
	}

	recordDeadlineEvent(teamID, id, "deleted", "", "", "", c.Query("reason"), userID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ==================== 看板视图 ====================

// TeamDeadlineBoard GET /teams/:team_id/deadlines/board
//
// The single screen the owner actually looks at: what must ship today, what is
// late, and how loaded the coming week is.
func TeamDeadlineBoard(c *gin.Context) {
	teamID := getTeamID(c)
	ctx := c.Request.Context()

	type bucket struct {
		Overdue  []*model.Deadline `json:"overdue"`
		Today    []*model.Deadline `json:"today"`
		Next3    []*model.Deadline `json:"next_3_days"`
		Next7    []*model.Deadline `json:"next_7_days"`
	}

	out := bucket{
		Overdue: []*model.Deadline{},
		Today:   []*model.Deadline{},
		Next3:   []*model.Deadline{},
		Next7:   []*model.Deadline{},
	}

	load := func(where string) []*model.Deadline {
		rows, err := database.DB.QueryContext(ctx,
			`SELECT `+deadlineColumns+` FROM md_deadlines
			 WHERE team_id = ? AND deleted_at IS NULL AND status <> 'done' AND `+where+`
			 ORDER BY due_date ASC, priority = 'urgent' DESC LIMIT 200`, teamID)
		if err != nil {
			return []*model.Deadline{}
		}
		defer rows.Close()
		items := []*model.Deadline{}
		for rows.Next() {
			if d, err := scanDeadline(rows); err == nil {
				items = append(items, d)
			}
		}
		attachOwnerNames(items)
		return items
	}

	out.Overdue = load("due_date < CURDATE()")
	out.Today = load("due_date = CURDATE()")
	out.Next3 = load("due_date > CURDATE() AND due_date <= DATE_ADD(CURDATE(), INTERVAL 3 DAY)")
	out.Next7 = load("due_date > DATE_ADD(CURDATE(), INTERVAL 3 DAY) AND due_date <= DATE_ADD(CURDATE(), INTERVAL 7 DAY)")

	var doneThisWeek, totalOpen int
	database.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM md_deadlines WHERE team_id = ? AND deleted_at IS NULL
		 AND status = 'done' AND due_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)`, teamID).Scan(&doneThisWeek)
	database.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM md_deadlines WHERE team_id = ? AND deleted_at IS NULL AND status <> 'done'`, teamID).Scan(&totalOpen)

	// On-time rate over orders closed in the last 30 days, judged by whether the
	// due date passed without the order being marked overdue at close time.
	var closed30, onTime30 int
	database.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM md_deadlines WHERE team_id = ? AND deleted_at IS NULL
		 AND status = 'done' AND due_date >= DATE_SUB(CURDATE(), INTERVAL 30 DAY)`, teamID).Scan(&closed30)
	database.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM md_deadlines WHERE team_id = ? AND deleted_at IS NULL
		 AND status = 'done' AND due_date >= DATE_SUB(CURDATE(), INTERVAL 30 DAY)`, teamID).Scan(&onTime30)

	onTimeRate := 0
	if closed30 > 0 {
		onTimeRate = onTime30 * 100 / closed30
	}

	c.JSON(http.StatusOK, gin.H{
		"overdue":       out.Overdue,
		"today":         out.Today,
		"next_3_days":   out.Next3,
		"next_7_days":   out.Next7,
		"counts": gin.H{
			"overdue":     len(out.Overdue),
			"today":       len(out.Today),
			"next_3_days": len(out.Next3),
			"next_7_days": len(out.Next7),
			"open_total":  totalOpen,
			"done_7d":     doneThisWeek,
			"on_time_30d": onTimeRate,
		},
		"generated_at": time.Now(),
	})
}

// TeamDeadlineEvents GET /teams/:team_id/deadlines/:id/events
func TeamDeadlineEvents(c *gin.Context) {
	teamID := getTeamID(c)
	rows, err := database.DB.QueryContext(c.Request.Context(),
		`SELECT id, team_id, deadline_id, event_type, field, old_value, new_value, reason, actor_id, created_at
		 FROM md_deadline_events WHERE team_id = ? AND deadline_id = ?
		 ORDER BY created_at DESC LIMIT 200`, teamID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	events := []model.DeadlineEvent{}
	for rows.Next() {
		var e model.DeadlineEvent
		var field, oldV, newV, reason, actor sql.NullString
		if err := rows.Scan(&e.ID, &e.TeamID, &e.DeadlineID, &e.EventType, &field,
			&oldV, &newV, &reason, &actor, &e.CreatedAt); err != nil {
			continue
		}
		e.Field, e.OldValue, e.NewValue, e.Reason, e.ActorID = field.String, oldV.String, newV.String, reason.String, actor.String
		events = append(events, e)
	}
	c.JSON(http.StatusOK, gin.H{"data": events})
}

// ==================== 提醒规则 ====================

// TeamListReminderRules GET /teams/:team_id/reminder-rules
func TeamListReminderRules(c *gin.Context) {
	teamID := getTeamID(c)
	rows, err := database.DB.QueryContext(c.Request.Context(),
		`SELECT id, team_id, name, offset_days, channel, target, enabled, created_by, created_at, updated_at
		 FROM md_reminder_rules WHERE team_id = ? ORDER BY offset_days DESC`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	rules := []model.ReminderRule{}
	for rows.Next() {
		var r model.ReminderRule
		if err := rows.Scan(&r.ID, &r.TeamID, &r.Name, &r.OffsetDays, &r.Channel, &r.Target,
			&r.Enabled, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt); err != nil {
			continue
		}
		rules = append(rules, r)
	}
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// TeamCreateReminderRule POST /teams/:team_id/reminder-rules
func TeamCreateReminderRule(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")

	var in struct {
		Name       string `json:"name"`
		OffsetDays int    `json:"offset_days"`
		Channel    string `json:"channel"`
		Target     string `json:"target"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if in.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "规则名称不能为空"})
		return
	}
	if in.Channel == "" {
		in.Channel = "inapp"
	}
	if in.Channel != "inapp" && in.Channel != "webhook" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel 只支持 inapp 或 webhook"})
		return
	}
	if in.Target == "" {
		in.Target = "owner"
	}
	if in.Target != "owner" && in.Target != "creator" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target 只支持 owner 或 creator"})
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	id := uuid.New().String()
	if _, err := database.DB.ExecContext(c.Request.Context(), `
		INSERT INTO md_reminder_rules (id, team_id, name, offset_days, channel, target, enabled, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, teamID, in.Name, in.OffsetDays, in.Channel, in.Target, enabled, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// TeamUpdateReminderRule PUT /teams/:team_id/reminder-rules/:id
func TeamUpdateReminderRule(c *gin.Context) {
	teamID := getTeamID(c)
	var in struct {
		Name       string `json:"name"`
		OffsetDays *int   `json:"offset_days"`
		Channel    string `json:"channel"`
		Target     string `json:"target"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	sets := []string{}
	args := []any{}
	if in.Name != "" {
		sets = append(sets, "name = ?")
		args = append(args, in.Name)
	}
	if in.OffsetDays != nil {
		sets = append(sets, "offset_days = ?")
		args = append(args, *in.OffsetDays)
	}
	if in.Channel != "" {
		sets = append(sets, "channel = ?")
		args = append(args, in.Channel)
	}
	if in.Target != "" {
		sets = append(sets, "target = ?")
		args = append(args, in.Target)
	}
	if in.Enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, *in.Enabled)
	}
	if len(sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有需要更新的字段"})
		return
	}

	args = append(args, c.Param("id"), teamID)
	res, err := database.DB.ExecContext(c.Request.Context(),
		`UPDATE md_reminder_rules SET `+strings.Join(sets, ", ")+` WHERE id = ? AND team_id = ?`, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TeamDeleteReminderRule DELETE /teams/:team_id/reminder-rules/:id
func TeamDeleteReminderRule(c *gin.Context) {
	teamID := getTeamID(c)
	res, err := database.DB.ExecContext(c.Request.Context(),
		`DELETE FROM md_reminder_rules WHERE id = ? AND team_id = ?`, c.Param("id"), teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TeamSeedDeadlineRules POST /teams/:team_id/reminder-rules/seed
// Installs the default 7/3/1/0/-1 cadence for a team that has no rules yet.
func TeamSeedDeadlineRules(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")

	var existing int
	database.DB.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM md_reminder_rules WHERE team_id = ?`, teamID).Scan(&existing)
	if existing > 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "created": 0, "message": "已存在规则，未重复创建"})
		return
	}

	created := 0
	for _, r := range model.DefaultReminderRules() {
		if _, err := database.DB.ExecContext(c.Request.Context(), `
			INSERT INTO md_reminder_rules (id, team_id, name, offset_days, channel, target, enabled, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), teamID, r.Name, r.OffsetDays, r.Channel, r.Target, r.Enabled, userID); err == nil {
			created++
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "created": created})
}

// ==================== 提醒日志 ====================

// TeamListReminderLog GET /teams/:team_id/reminder-log
func TeamListReminderLog(c *gin.Context) {
	teamID := getTeamID(c)
	rows, err := database.DB.QueryContext(c.Request.Context(), `
		SELECT l.id, l.rule_id, l.deadline_id, l.target_user_id, l.channel, l.result, l.detail, l.sent_at,
		       d.order_no, d.title
		FROM md_reminder_log l
		LEFT JOIN md_deadlines d ON d.id = l.deadline_id
		WHERE l.team_id = ? ORDER BY l.sent_at DESC LIMIT 200`, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	items := []gin.H{}
	for rows.Next() {
		var id, ruleID, deadlineID, target, channel, result string
		var detail sql.NullString
		var orderNo, title sql.NullString
		var sentAt time.Time
		if err := rows.Scan(&id, &ruleID, &deadlineID, &target, &channel, &result, &detail, &sentAt, &orderNo, &title); err != nil {
			continue
		}
		items = append(items, gin.H{
			"id": id, "rule_id": ruleID, "deadline_id": deadlineID,
			"target_user_id": target, "channel": channel, "result": result,
			"detail": detail.String, "sent_at": sentAt,
			"order_no": orderNo.String, "title": title.String,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// ==================== 内部辅助 ====================

func loadDeadlineRaw(teamID, id string) (*model.Deadline, error) {
	row := database.DB.QueryRow(
		`SELECT `+deadlineColumns+` FROM md_deadlines WHERE id = ? AND team_id = ? AND deleted_at IS NULL`,
		id, teamID)
	return scanDeadline(row)
}

func recordDeadlineEvent(teamID, deadlineID, eventType, field, oldValue, newValue, reason, actorID string) {
	if reason == "" {
		reason = ""
	}
	database.DB.Exec(`
		INSERT INTO md_deadline_events
			(id, team_id, deadline_id, event_type, field, old_value, new_value, reason, actor_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), teamID, deadlineID, eventType, field, oldValue, newValue, reason, actorID)
}

func clampProgress(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
