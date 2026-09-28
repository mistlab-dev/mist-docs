package handler

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/schedule"
	"github.com/gin-gonic/gin"
)

// ==================== 插单预演（只算不写） ====================
//
// The preview is computed by internal/schedule from the team's open orders
// and md_team_capacity. It never writes to the deadline tables.

type previewInput struct {
	DeadlineID string `json:"deadline_id"` // set = move an existing order instead of adding one
	OrderNo    string `json:"order_no"`
	Title      string `json:"title"`
	Customer   string `json:"customer"`
	Remark     string `json:"remark"`
	StartDate  string `json:"start_date"`
	DueDate    string `json:"due_date"`
	Priority   string `json:"priority"`
}

// teamCapacity is the team's capacity steps and key-customer list.
type teamCapacity struct {
	Steps        []schedule.CapacityStep
	KeyCustomers []string
}

func loadTeamCapacity(teamID string) (teamCapacity, error) {
	var tc teamCapacity
	rows, err := database.DB.Query(`SELECT effective_from, per_day, COALESCE(key_customers,'')
		FROM md_team_capacity WHERE team_id=? ORDER BY effective_from`, teamID)
	if err != nil {
		return tc, err
	}
	defer rows.Close()
	keys := ""
	for rows.Next() {
		var s schedule.CapacityStep
		if err := rows.Scan(&s.From, &s.PerDay, &keys); err != nil {
			return tc, err
		}
		tc.Steps = append(tc.Steps, s)
	}
	tc.KeyCustomers = splitNames(keys)
	return tc, rows.Err()
}

func splitNames(s string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' || r == '，' || r == ';' || r == '；' }) {
		p := strings.TrimSpace(part)
		if p != "" && !seen[strings.ToLower(p)] {
			seen[strings.ToLower(p)] = true
			out = append(out, p)
		}
	}
	return out
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// loadOpenOrders returns the team's orders that still need producing. With
// forUpdate (inside a transaction) the rows stay locked until commit.
func loadOpenOrders(q querier, teamID string, forUpdate bool) ([]schedule.Order, error) {
	query := `SELECT ` + deadlineColumns + ` FROM md_deadlines
		WHERE team_id=? AND status <> 'done' AND deleted_at IS NULL ORDER BY id`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	rows, err := q.Query(query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schedule.Order
	for rows.Next() {
		d, err := scanDeadline(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, schedule.Order{
			ID: d.ID, OrderNo: d.OrderNo, Title: d.Title, Customer: d.Customer, Remark: d.Remark,
			Priority: d.Priority, Status: d.Status, Progress: d.Progress,
			Start: d.StartDate, Due: d.DueDate, CreatedAt: d.CreatedAt,
		})
	}
	return out, rows.Err()
}

// buildPreview validates the request and runs the calculation. It is shared
// with the proposal flow so both use exactly the same numbers.
func buildPreview(teamID string, in previewInput) (schedule.Input, schedule.Result, int, string) {
	var zero schedule.Result
	due, err := time.ParseInLocation(dateLayout, in.DueDate, time.Local)
	if err != nil {
		return schedule.Input{}, zero, http.StatusBadRequest, "希望交期格式应为 YYYY-MM-DD"
	}
	if in.Priority == "" {
		in.Priority = "inserted"
	}
	if !validPriorities[in.Priority] {
		return schedule.Input{}, zero, http.StatusBadRequest, "无效的优先级"
	}
	orders, err := loadOpenOrders(database.DB, teamID, false)
	if err != nil {
		return schedule.Input{}, zero, http.StatusInternalServerError, err.Error()
	}
	ins := schedule.Order{OrderNo: strings.TrimSpace(in.OrderNo), Title: strings.TrimSpace(in.Title),
		Customer: in.Customer, Remark: in.Remark, Priority: in.Priority, Status: "pending", Due: due}
	if in.DeadlineID != "" {
		found := false
		for _, o := range orders {
			if o.ID == in.DeadlineID {
				found = true
				break
			}
		}
		if !found {
			return schedule.Input{}, zero, http.StatusNotFound, "订单不存在或已完成"
		}
		ins.ID = in.DeadlineID
	} else if ins.OrderNo == "" && ins.Title == "" {
		return schedule.Input{}, zero, http.StatusBadRequest, "订单号和名称至少填一个"
	}
	if in.StartDate != "" {
		if sd, err := time.ParseInLocation(dateLayout, in.StartDate, time.Local); err == nil {
			ins.Start = &sd
		}
	}
	tc, err := loadTeamCapacity(teamID)
	if err != nil {
		return schedule.Input{}, zero, http.StatusInternalServerError, err.Error()
	}
	sin := schedule.Input{Today: model.TodayStart(), Orders: orders, Insert: ins, Capacity: tc.Steps, KeyCustomers: tc.KeyCustomers}
	return sin, schedule.Preview(sin), http.StatusOK, ""
}

// TeamPreviewInsert POST /teams/:team_id/deadlines/preview-insert
//
// Any team member may run it (design §8: preview is read-only). It never
// touches the deadline tables; it stores a pending row in md_proposals.
func TeamPreviewInsert(c *gin.Context) {
	var in previewInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	teamID := getTeamID(c)
	sin, res, code, msg := buildPreview(teamID, in)
	if code != http.StatusOK {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	// Every preview is also a pending proposal (phase 5): confirming it
	// applies exactly these changes, if nothing moved in between.
	out := gin.H{"insert": res.Insert, "new_breaches": res.NewBreaches, "delayed": res.Delayed,
		"unaffected": res.Unaffected, "started": res.Started, "conclusion": res.Conclusion, "per_day": res.PerDay}
	if id, changes, err := createProposal(teamID, c.GetString("user_id"), in, sin, res); err == nil {
		out["proposal_id"] = id
		out["changes"] = changes
	} else {
		log.Printf("preview-insert: store proposal: %v", err)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// TeamGetCapacity GET /teams/:team_id/deadlines/capacity
func TeamGetCapacity(c *gin.Context) {
	tc, err := loadTeamCapacity(getTeamID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	today := model.TodayStart()
	type step struct {
		From   string `json:"effective_from"`
		PerDay int    `json:"per_day"`
	}
	steps := []step{}
	current := schedule.PerDayOn(tc.Steps, today)
	for _, s := range tc.Steps {
		steps = append(steps, step{From: s.From.Format(dateLayout), PerDay: s.PerDay})
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"per_day":       current,
		"steps":         steps,
		"key_customers": tc.KeyCustomers,
		"is_default":    len(tc.Steps) == 0,
	}})
}

// TeamUpdateCapacity PUT /teams/:team_id/deadlines/capacity (admin)
//
// Body: {per_day, effective_from?, key_customers?}. effective_from defaults to
// today; the key-customer list is team-wide.
func TeamUpdateCapacity(c *gin.Context) {
	if !requireRole(c, RoleAdmin) {
		return
	}
	var in struct {
		PerDay        int       `json:"per_day"`
		EffectiveFrom string    `json:"effective_from"`
		KeyCustomers  *[]string `json:"key_customers"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if in.PerDay < 1 || in.PerDay > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "每天产能应在 1～1000 单之间"})
		return
	}
	from := model.TodayString()
	if in.EffectiveFrom != "" {
		t, err := time.ParseInLocation(dateLayout, in.EffectiveFrom, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "生效日期格式应为 YYYY-MM-DD"})
			return
		}
		from = t.Format(dateLayout)
	}
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	tc, err := loadTeamCapacity(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	keys := tc.KeyCustomers
	if in.KeyCustomers != nil {
		keys = splitNames(strings.Join(*in.KeyCustomers, "\n"))
	}
	keyText := strings.Join(keys, "\n")
	if _, err := database.DB.Exec(`INSERT INTO md_team_capacity (team_id, effective_from, per_day, key_customers, updated_by)
		VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE per_day=VALUES(per_day), key_customers=VALUES(key_customers), updated_by=VALUES(updated_by)`,
		teamID, from, in.PerDay, keyText, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	database.DB.Exec(`UPDATE md_team_capacity SET key_customers=? WHERE team_id=?`, keyText, teamID)
	TeamGetCapacity(c)
}
