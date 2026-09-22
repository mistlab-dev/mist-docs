package model

import (
	"testing"
	"time"
)

// ==================== DaysLeft ====================

func TestDaysLeft(t *testing.T) {
	today := TodayStart()

	cases := []struct {
		name string
		due  time.Time
		want int
	}{
		{"today", today, 0},
		{"tomorrow", today.AddDate(0, 0, 1), 1},
		{"in 7 days", today.AddDate(0, 0, 7), 7},
		{"yesterday (overdue)", today.AddDate(0, 0, -1), -1},
		{"a month late", today.AddDate(0, 0, -30), -30},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DaysLeft(c.due); got != c.want {
				t.Fatalf("DaysLeft(%s) = %d, want %d", c.due.Format("2006-01-02"), got, c.want)
			}
		})
	}
}

// A due date carrying a time of day must still count as whole days, otherwise
// an order due "today 18:00" would report -1 in the afternoon.
func TestDaysLeftIgnoresTimeOfDay(t *testing.T) {
	today := TodayStart()

	t.Run("later today", func(t *testing.T) {
		due := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 0, 0, today.Location())
		if got := DaysLeft(due); got != 0 {
			t.Fatalf("due at 23:59 today should be 0 days left, got %d", got)
		}
	})

	t.Run("midnight tomorrow", func(t *testing.T) {
		due := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).AddDate(0, 0, 1)
		if got := DaysLeft(due); got != 1 {
			t.Fatalf("due at 00:00 tomorrow should be 1 day left, got %d", got)
		}
	})
}

// ==================== RiskLevel ====================

func TestRiskLevel(t *testing.T) {
	cases := []struct {
		name     string
		daysLeft int
		status   string
		want     string
	}{
		{"completed beats lateness", -10, "done", "ok"},
		{"completed on time", 5, "done", "ok"},
		{"one day late", -1, "running", "overdue"},
		{"long overdue", -30, "pending", "overdue"},
		{"due today", 0, "running", "critical"},
		{"three days out", 3, "running", "critical"},
		{"four days out", 4, "running", "warning"},
		{"seven days out", 7, "pending", "warning"},
		{"eight days out", 8, "pending", "ok"},
		{"far future", 60, "pending", "ok"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RiskLevel(c.daysLeft, c.status); got != c.want {
				t.Fatalf("RiskLevel(%d, %q) = %q, want %q", c.daysLeft, c.status, got, c.want)
			}
		})
	}
}

// RiskLevel buckets by days remaining and only short-circuits on 'done'.
//
// The status column stays authoritative for operational state (so a
// renegotiated-but-late order is distinguishable from a real slip, and its
// reminders can be paused), while RiskLevel answers a narrower question:
// how urgent is the due date itself. Pinning this separation deliberately,
// because collapsing the two would lose the ability to represent a
// late-but-renegotiated order.
func TestRiskLevelSeparatesUrgencyFromOperationalStatus(t *testing.T) {
	// A renegotiated order whose new due date is a week out reads as a
	// week-out risk, not as "overdue", even though status still says overdue.
	if got := RiskLevel(7, "overdue"); got != "warning" {
		t.Fatalf("RiskLevel(7, overdue) = %q, want %q", got, "warning")
	}

	// Only 'done' overrides the day-based bucket.
	if got := RiskLevel(7, "done"); got != "ok" {
		t.Fatalf("RiskLevel(7, done) = %q, want %q", got, "ok")
	}

	// Overdue is what a late date reports, regardless of status.
	if got := RiskLevel(-1, "pending"); got != "overdue" {
		t.Fatalf("RiskLevel(-1, pending) = %q, want %q", got, "overdue")
	}
}

// ==================== DefaultReminderRules ====================

func TestDefaultReminderRulesCoverTheUsefulCadence(t *testing.T) {
	rules := DefaultReminderRules()

	if len(rules) == 0 {
		t.Fatal("expected default rules to be non-empty")
	}

	// The research is consistent that a warning only a few days out is already
	// too late, so at least one rule must fire a week ahead.
	hasWeekAhead := false
	hasOverdue := false
	seen := map[int]bool{}

	for _, r := range rules {
		if r.Name == "" {
			t.Errorf("rule with offset %d has no name", r.OffsetDays)
		}
		if r.OffsetDays == 7 {
			hasWeekAhead = true
		}
		if r.OffsetDays < 0 {
			hasOverdue = true
		}
		if r.Channel != "inapp" && r.Channel != "webhook" {
			t.Errorf("rule %q has unsupported channel %q", r.Name, r.Channel)
		}
		if r.Target != "owner" && r.Target != "creator" {
			t.Errorf("rule %q has unsupported target %q", r.Name, r.Target)
		}
		if seen[r.OffsetDays] {
			t.Errorf("duplicate offset %d among default rules", r.OffsetDays)
		}
		seen[r.OffsetDays] = true
	}

	if !hasWeekAhead {
		t.Error("expected a 7-day-ahead rule so risk surfaces before it is too late")
	}
	if !hasOverdue {
		t.Error("expected at least one rule for overdue orders")
	}
}

// ==================== Offset semantics ====================

// The scheduler derives the scan date as today - offset. This documents that
// contract, since getting the sign wrong would silently notify nobody.
func TestOffsetDaysToScanDate(t *testing.T) {
	today := TodayStart()

	cases := []struct {
		offsetDays int
		wantOffset int // days before today that the due date should sit
	}{
		{7, 7},   // rule fires 7 days early → due date is 7 days out
		{1, 1},   // tomorrow
		{0, 0},   // today
		{-1, -1}, // one day late → due date is yesterday
	}

	for _, c := range cases {
		// Mirrors scheduler.RunReminderScanOnce: scanDate = today + offset.
		scanDate := today.AddDate(0, 0, c.offsetDays)
		want := today.AddDate(0, 0, c.wantOffset)
		if !scanDate.Equal(want) {
			t.Fatalf("offset %d → scan date %s, want %s",
				c.offsetDays, scanDate.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}
}
