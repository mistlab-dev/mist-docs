package model

import "time"

// ==================== 交期看板 ====================

// Deadline is one order/deliverable being tracked against a due date.
//
// This is deliberately a DB row rather than a spreadsheet: the reminder scan
// needs `WHERE due_date = ...`, which is impossible against an encrypted
// document blob.
type Deadline struct {
	ID      string `json:"id" db:"id"`
	TeamID  string `json:"team_id" db:"team_id"`
	OrderNo string `json:"order_no" db:"order_no"`
	Title   string `json:"title" db:"title"`
	Customer string `json:"customer,omitempty" db:"customer"`
	Quantity int    `json:"quantity" db:"quantity"`

	StartDate *time.Time `json:"-" db:"start_date"`
	DueDate   time.Time  `json:"-" db:"due_date"`

	// Status is an operational state, not a derived value.
	//
	// "overdue" must be settable independently of `today > due_date`, otherwise
	// a renegotiated-but-late order is indistinguishable from a real slip, and
	// reminders cannot be paused for it.
	Status   string `json:"status" db:"status"`     // pending|running|done|overdue
	Progress int    `json:"progress" db:"progress"` // 0-100
	Priority string `json:"priority" db:"priority"` // normal|urgent|inserted
	OwnerID  string `json:"owner_id,omitempty" db:"owner_id"`
	Remark   string `json:"remark,omitempty" db:"remark"`

	CreatedBy string    `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedBy string    `json:"updated_by,omitempty" db:"updated_by"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Non-DB fields, filled in by the handler for the UI.
	OwnerName     string `json:"owner_name,omitempty"`
	StartDateStr  string `json:"start_date,omitempty"`
	DueDateStr    string `json:"due_date"`
	DaysLeft      int    `json:"days_left"`      // computed at read time, never stored
	RiskLevel     string `json:"risk_level"`     // overdue|critical|warning|ok
	IsArchived    bool   `json:"is_archived,omitempty"`
}

// ReminderRule is a configurable "T minus N days" notification rule.
type ReminderRule struct {
	ID         string `json:"id" db:"id"`
	TeamID     string `json:"team_id" db:"team_id"`
	Name       string `json:"name" db:"name"`
	OffsetDays int    `json:"offset_days" db:"offset_days"`
	Channel    string `json:"channel" db:"channel"` // inapp|webhook
	Target     string `json:"target" db:"target"`   // owner|creator
	Enabled    bool   `json:"enabled" db:"enabled"`

	CreatedBy string    `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// DeadlineEvent records a change so history survives.
//
// Stored from day one because history cannot be reconstructed later; Reason is
// optional but is the most valuable field for explaining a slip down the line.
type DeadlineEvent struct {
	ID         string    `json:"id" db:"id"`
	TeamID     string    `json:"team_id" db:"team_id"`
	DeadlineID string    `json:"deadline_id" db:"deadline_id"`
	EventType  string    `json:"event_type" db:"event_type"`
	Field      string    `json:"field,omitempty" db:"field"`
	OldValue   string    `json:"old_value,omitempty" db:"old_value"`
	NewValue   string    `json:"new_value,omitempty" db:"new_value"`
	Reason     string    `json:"reason,omitempty" db:"reason"`
	ActorID    string    `json:"actor_id,omitempty" db:"actor_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`

	ActorName string `json:"actor_name,omitempty"`
}

// ==================== 交期计算 ====================

// TodayStart returns local midnight, used as the reference for day counts.
func TodayStart() time.Time {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, n.Location())
}

// DaysLeft returns whole days from today until due (negative when overdue).
func DaysLeft(due time.Time) int {
	d := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, due.Location())
	return int(d.Sub(TodayStart()).Hours() / 24)
}

// RiskLevel buckets a deadline for display. Thresholds mirror the field
// practice found in the research: <=3 days is where schedules start to break.
func RiskLevel(daysLeft int, status string) string {
	switch {
	case status == "done":
		return "ok"
	case daysLeft < 0:
		return "overdue"
	case daysLeft <= 3:
		return "critical"
	case daysLeft <= 7:
		return "warning"
	default:
		return "ok"
	}
}

// DefaultReminderRules seeds a new team's board.
// 7/3/1 days ahead plus due-day and one-day-overdue, matching the cadence
// described by practitioners (a 3-day warning is already too late).
func DefaultReminderRules() []ReminderRule {
	return []ReminderRule{
		{Name: "到期前 7 天", OffsetDays: 7, Channel: "inapp", Target: "owner", Enabled: true},
		{Name: "到期前 3 天", OffsetDays: 3, Channel: "inapp", Target: "owner", Enabled: true},
		{Name: "到期前 1 天", OffsetDays: 1, Channel: "inapp", Target: "owner", Enabled: true},
		{Name: "今天到期", OffsetDays: 0, Channel: "inapp", Target: "owner", Enabled: true},
		{Name: "已逾期 1 天", OffsetDays: -1, Channel: "inapp", Target: "owner", Enabled: true},
	}
}
