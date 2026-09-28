package handler

import (
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/schedule"
)

func TestScheduleFingerprint(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse(dateLayout, s); return v }
	a := schedule.Order{ID: "a", Due: d("2026-10-01"), Priority: "normal", Status: "pending"}
	b := schedule.Order{ID: "b", Due: d("2026-10-02"), Priority: "urgent", Status: "running", Progress: 20}
	steps := []schedule.CapacityStep{{From: d("2026-01-01"), PerDay: 2}}
	base := scheduleFingerprint([]schedule.Order{a, b}, steps)
	if scheduleFingerprint([]schedule.Order{b, a}, steps) != base {
		t.Fatal("fingerprint depends on row order")
	}
	b2 := b
	b2.Progress = 30
	s := d("2026-09-30")
	a2 := a
	a2.Start = &s
	for name, fp := range map[string]string{
		"progress": scheduleFingerprint([]schedule.Order{a, b2}, steps),
		"start":    scheduleFingerprint([]schedule.Order{a2, b}, steps),
		"removed":  scheduleFingerprint([]schedule.Order{a}, steps),
		"capacity": scheduleFingerprint([]schedule.Order{a, b}, []schedule.CapacityStep{{From: d("2026-01-01"), PerDay: 3}}),
		"no steps": scheduleFingerprint([]schedule.Order{a, b}, nil),
	} {
		if fp == base {
			t.Errorf("fingerprint unchanged after %s changed", name)
		}
	}
}

func TestProposalChanges(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse(dateLayout, s); return v }
	today := d("2026-09-28")
	x := schedule.Order{ID: "x", OrderNo: "X", Due: d("2026-09-28"), Priority: "inserted"}
	late := schedule.Order{ID: "late", OrderNo: "L", Due: d("2026-09-01"), Priority: "normal"}

	// New order: create it, and move the order it pushes past its date.
	in := schedule.Input{Today: today, Orders: []schedule.Order{x, late}, Insert: schedule.Order{OrderNo: "N", Due: d("2026-09-28"), Priority: "urgent"}}
	ch := proposalChanges(previewInput{OrderNo: "N"}, in, schedule.Preview(in))
	if len(ch) != 2 || ch[0].Field != "__create__" || ch[0].New != "2026-09-28" ||
		ch[1].DeadlineID != "x" || ch[1].Field != "due_date" || ch[1].Old != "2026-09-28" || ch[1].New != "2026-09-29" {
		t.Fatalf("changes = %+v", ch)
	}
	// The already-late order slips too, but its due date is left alone.
	for _, c := range ch {
		if c.DeadlineID == "late" {
			t.Fatalf("already-late order must not be re-dated: %+v", c)
		}
	}

	// Existing order moved up: its own date and priority change.
	in = schedule.Input{Today: today, Orders: []schedule.Order{x, late}, Insert: schedule.Order{ID: "late", Due: d("2026-09-30"), Priority: "urgent"}}
	ch = proposalChanges(previewInput{DeadlineID: "late"}, in, schedule.Preview(in))
	if len(ch) < 2 || ch[0].Field != "due_date" || ch[0].New != "2026-09-30" || ch[1].Field != "priority" || ch[1].New != "urgent" {
		t.Fatalf("changes = %+v", ch)
	}
}
