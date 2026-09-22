package scheduler

import (
	"database/sql"
	"log"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/google/uuid"
)

// ReminderScanInterval is how often the due-date scan runs.
//
// The scan is a plain indexed date range query, so running it hourly is cheap.
// Correctness does not depend on the interval: the UNIQUE key on
// md_reminder_log (rule_id, deadline_id) makes repeated runs idempotent, so a
// deadline is never notified twice for the same rule no matter how often we
// look at it.
const ReminderScanInterval = 30 * time.Minute

// StartReminderScheduler runs the due-date reminder scan until the process stops.
// It performs one scan immediately on startup, then ticks.
func StartReminderScheduler(interval time.Duration) {
	if interval <= 0 {
		interval = ReminderScanInterval
	}
	go func() {
		// Small delay so the DB pool and schema are settled before first scan.
		time.Sleep(10 * time.Second)
		RunReminderScanOnce(time.Now())

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			RunReminderScanOnce(time.Now())
		}
	}()
	log.Printf("交期提醒调度器已启动，扫描间隔 %s", interval)
}

// RunReminderScanOnce executes a single reminder scan cycle.
//
// For every enabled rule it finds deadlines whose due_date matches the rule
// offset, then notifies the target user once. Exported so it can be driven
// directly from tests or a cron runner.
func RunReminderScanOnce(now time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Load all enabled rules across teams in one query.
	ruleRows, err := database.DB.Query(
		`SELECT id, team_id, name, offset_days, channel, target FROM md_reminder_rules WHERE enabled = 1`)
	if err != nil {
		log.Printf("[reminder] 加载提醒规则失败: %v", err)
		return
	}
	type rule struct {
		id, teamID, name, channel, target string
		offsetDays                        int
	}
	var rules []rule
	for ruleRows.Next() {
		var r rule
		if err := ruleRows.Scan(&r.id, &r.teamID, &r.name, &r.offsetDays, &r.channel, &r.target); err != nil {
			continue
		}
		rules = append(rules, r)
	}
	ruleRows.Close()
	if len(rules) == 0 {
		return
	}

	total := 0
	for _, r := range rules {
		// offset_days semantics: N = notify N days before the due date;
		// 0 = on the due date; -N = N days after.
		//
		// "7 days before due" on day T means the due date sits at T+7, so the
		// scan date moves forward for positive offsets. Getting this sign wrong
		// silently notifies about the wrong orders (or nothing at all), which is
		// why model.TestOffsetDaysToScanDate pins the contract.
		dueDay := today.AddDate(0, 0, r.offsetDays)

		rows, err := database.DB.Query(`
			SELECT id, order_no, title, due_date, status, owner_id, created_by
			FROM md_deadlines
			WHERE team_id = ? AND due_date = ? AND status <> 'done' AND deleted_at IS NULL`,
			r.teamID, dueDay.Format("2006-01-02"))
		if err != nil {
			log.Printf("[reminder] 查询规则 %s 的到期订单失败: %v", r.name, err)
			continue
		}

		type hit struct {
			id, orderNo, title, status, ownerID, createdBy string
			dueDate                                        time.Time
		}
		var hits []hit
		for rows.Next() {
			var h hit
			if err := rows.Scan(&h.id, &h.orderNo, &h.title, &h.dueDate, &h.status, &h.ownerID, &h.createdBy); err != nil {
				continue
			}
			hits = append(hits, h)
		}
		rows.Close()

		for _, h := range hits {
			target := h.ownerID
			if r.target == "creator" || target == "" {
				target = h.createdBy
			}
			if target == "" {
				continue
			}
			if notifyOnce(r.id, r.name, r.channel, r.teamID, h.id, target, h.orderNo, h.title, h.dueDate, r.offsetDays) {
				total++
			}
		}
	}

	if total > 0 {
		log.Printf("[reminder] 本轮发送 %d 条交期提醒", total)
	}
}

// notifyOnce records and delivers a reminder, returning false if it was already sent.
//
// The guard is the INSERT itself: uk_once (rule_id, deadline_id) means a
// duplicate attempt fails, so we skip delivery. Doing the claim *before* the
// send keeps concurrent or repeated scans from double-notifying.
func notifyOnce(ruleID, ruleName, channel, teamID, deadlineID, targetUser, orderNo, title string, due time.Time, offsetDays int) bool {
	logID := uuid.New().String()
	res, err := database.DB.Exec(`
		INSERT IGNORE INTO md_reminder_log
			(id, rule_id, deadline_id, team_id, target_user_id, channel, result, detail, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, NOW())`,
		logID, ruleID, deadlineID, teamID, targetUser, channel, ruleName)
	if err != nil {
		log.Printf("[reminder] 写提醒日志失败: %v", err)
		return false
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false // already notified for this rule+deadline
	}

	msg := buildMessage(orderNo, title, due, offsetDays)

	detail := ""
	switch channel {
	case "webhook":
		detail = deliverWebhook(teamID, deadlineID, orderNo, msg)
	default:
		if err := createInAppNotification(targetUser, teamID, deadlineID, msg); err != nil {
			detail = err.Error()
		}
	}

	result := "ok"
	if detail != "" {
		result = "failed"
	}
	database.DB.Exec(`UPDATE md_reminder_log SET result = ?, detail = ? WHERE id = ?`, result, truncate(detail, 250), logID)
	return result == "ok"
}

// buildMessage renders the human-facing reminder text.
func buildMessage(orderNo, title string, due time.Time, offsetDays int) string {
	label := orderNo
	if label == "" {
		label = title
	} else if title != "" {
		label = orderNo + " " + title
	}
	if label == "" {
		// Both fields empty: still produce a readable message rather than "[]".
		label = "未命名订单"
	}
	switch {
	case offsetDays > 0:
		return "[" + label + "] 将在 " + itoa(offsetDays) + " 天后（" + due.Format("01-02") + "）到期"
	case offsetDays == 0:
		return "[" + label + "] 今天到期（" + due.Format("01-02") + "）"
	default:
		return "[" + label + "] 已逾期 " + itoa(-offsetDays) + " 天（交期 " + due.Format("01-02") + "）"
	}
}

// createInAppNotification writes a row the web UI already knows how to render.
//
// team_id is required: the team-scoped notification endpoint filters on
// (user_id, team_id), so a row without it is silently invisible in the UI even
// though it was written successfully.
func createInAppNotification(userID, teamID, deadlineID, title string) error {
	_, err := database.DB.Exec(`
		INSERT INTO md_notifications (id, user_id, team_id, type, title, document_id, related_id, is_read, created_at)
		VALUES (?, ?, ?, 'deadline', ?, NULL, ?, 0, NOW())`,
		uuid.New().String(), userID, teamID, title, deadlineID)
	return err
}

// deliverWebhook posts the reminder through the team's existing webhook config.
// Returns "" on success or a failure detail.
func deliverWebhook(teamID, deadlineID, orderNo, message string) string {
	rows, err := database.DB.Query(
		`SELECT id, url, secret FROM md_webhooks WHERE enabled = 1`)
	if err != nil {
		return err.Error()
	}
	defer rows.Close()

	var (
		anySent bool
		lastErr string
	)
	for rows.Next() {
		var id, url, secret string
		rows.Scan(&id, &url, &secret)
		if err := postWebhook(url, secret, teamID, map[string]any{
			"event":       "deadline.reminder",
			"deadline_id": deadlineID,
			"order_no":    orderNo,
			"message":     message,
			"team_id":     teamID,
			"timestamp":   time.Now().Format(time.RFC3339),
		}); err != nil {
			lastErr = err.Error()
			database.DB.Exec(`INSERT INTO md_webhook_logs (id, webhook_id, event, status, created_at) VALUES (?,?,?,?,NOW())`,
				uuid.New().String(), id, "deadline.reminder", "error:"+err.Error())
			continue
		}
		anySent = true
		database.DB.Exec(`INSERT INTO md_webhook_logs (id, webhook_id, event, status, created_at) VALUES (?,?,?,?,NOW())`,
			uuid.New().String(), id, "deadline.reminder", "ok")
	}
	if anySent {
		return ""
	}
	if lastErr == "" {
		return "no webhook configured"
	}
	return lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ensure sql import is used even if future edits drop the query above
var _ = sql.ErrNoRows
