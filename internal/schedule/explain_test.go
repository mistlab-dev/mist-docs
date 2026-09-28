package schedule

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var exToday = time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

func md(s string) time.Time {
	t, err := time.Parse("2006-01-02", "2026-"+s)
	if err != nil {
		panic(err)
	}
	return t
}

func xord(id, pri, due string, created string) Order {
	return Order{ID: id, OrderNo: id, Title: id, Priority: pri, Status: "pending", Due: md(due), CreatedAt: md(created)}
}

func hasEvidence(ex Explanation, typ, contains string) bool {
	for _, e := range ex.Evidence {
		if e.Type == typ && strings.Contains(e.Text, contains) {
			return true
		}
	}
	return false
}

func hasRef(ex Explanation, ref string) bool {
	for _, e := range ex.Evidence {
		if e.Ref == ref {
			return true
		}
	}
	return false
}

// A full queue plus recorded history: the explanation cites the plan, the
// load and the history, each with a reference.
func TestExplainLateWithHistory(t *testing.T) {
	target := xord("SO-1", "normal", "09-29", "09-01")
	orders := []Order{
		xord("SO-A", "urgent", "10-05", "09-20"),   // created later, jumps ahead
		xord("SO-B", "inserted", "10-06", "09-21"), // created later, jumps ahead
		xord("SO-C", "normal", "09-28", "08-30"),
		target,
	}
	events := []Event{
		{ID: "evt-created", Type: "created", At: md("09-01")},
		{ID: "evt-due", Type: "date_changed", Field: "due_date", Old: "2026-09-27", New: "2026-09-29", Reason: "铝材到货延迟", Actor: "陈航", At: md("09-20")},
	}
	ex := Explain(ExplainInput{Today: exToday, Target: target, Orders: orders,
		Capacity: []CapacityStep{{From: md("01-01"), PerDay: 1}}, Events: events})

	if ex.Verdict != VerdictLate || ex.LateDays != 2 || ex.PlannedFinish != "2026-10-01" {
		t.Fatalf("verdict=%s late=%d finish=%s (%s)", ex.Verdict, ex.LateDays, ex.PlannedFinish, ex.Conclusion)
	}
	if len(ex.Evidence) < 2 {
		t.Fatalf("want at least 2 evidence items, got %d: %+v", len(ex.Evidence), ex.Evidence)
	}
	if !hasEvidence(ex, EvidencePlan, "排第 4 位") || !hasEvidence(ex, EvidenceLoad, "前面的还有 3 单") {
		t.Errorf("plan/load evidence missing: %+v", ex.Evidence)
	}
	if !hasEvidence(ex, EvidenceLoad, "SO-A（加急）") {
		t.Errorf("orders that jumped the queue should be named: %+v", ex.Evidence)
	}
	if !hasEvidence(ex, EvidenceEvent, "09-27 改到 09-29，原因：铝材到货延迟（陈航）") || !hasRef(ex, "evt-due") {
		t.Errorf("due change evidence missing: %+v", ex.Evidence)
	}
	if ex.Confidence != ConfidenceHigh || len(ex.Missing) != 0 {
		t.Errorf("capacity configured and reasons recorded: confidence=%s missing=%v", ex.Confidence, ex.Missing)
	}
	if ex.Preview == nil || ex.Preview.DeadlineID != "SO-1" || ex.Preview.Priority != "urgent" {
		t.Errorf("late normal order should suggest an expedite preview: %+v", ex.Preview)
	}
	if strings.HasPrefix(ex.Conclusion, "数据不足") {
		t.Errorf("has history, must not be data-insufficient: %s", ex.Conclusion)
	}
}

// An order with no recorded changes says "数据不足" instead of guessing.
func TestExplainNoEventsIsDataInsufficient(t *testing.T) {
	target := xord("SO-9", "normal", "10-20", "09-01")
	ex := Explain(ExplainInput{Today: exToday, Target: target, Orders: []Order{target},
		Events: []Event{{ID: "evt-c", Type: "created", At: md("09-01")}}})
	if !strings.HasPrefix(ex.Conclusion, "数据不足") {
		t.Fatalf("conclusion = %q", ex.Conclusion)
	}
	if ex.Confidence != ConfidenceLow {
		t.Errorf("confidence = %s, want low", ex.Confidence)
	}
	joined := strings.Join(ex.Missing, "|")
	if !strings.Contains(joined, "没有任何变更记录") || !strings.Contains(joined, "没有配置产能") {
		t.Errorf("missing = %v", ex.Missing)
	}
	// Same with no events at all.
	ex2 := Explain(ExplainInput{Today: exToday, Target: target, Orders: []Order{target}})
	if !strings.HasPrefix(ex2.Conclusion, "数据不足") || ex2.Confidence != ConfidenceLow {
		t.Errorf("no events: %q %s", ex2.Conclusion, ex2.Confidence)
	}
}

func TestExplainDoneAndOverdueAreFacts(t *testing.T) {
	done := xord("SO-D", "normal", "09-20", "09-01")
	done.Status = "done"
	ex := Explain(ExplainInput{Today: exToday, Target: done})
	if ex.Verdict != VerdictDone || ex.Confidence != ConfidenceHigh || ex.Preview != nil {
		t.Errorf("done: %+v", ex)
	}

	late := xord("SO-O", "normal", "09-25", "09-01")
	late.Status = "overdue"
	ex = Explain(ExplainInput{Today: exToday, Target: late, Orders: []Order{late}, Events: []Event{
		{ID: "e1", Type: "status_changed", Field: "status", Old: "running", New: "overdue", At: md("09-26")},
	}})
	if ex.Verdict != VerdictOverdue || ex.LateDays != 3 || ex.Confidence != ConfidenceHigh {
		t.Errorf("overdue: verdict=%s late=%d conf=%s", ex.Verdict, ex.LateDays, ex.Confidence)
	}
	if !hasEvidence(ex, EvidenceHistory, "被标记过 1 次") || !hasRef(ex, "e1") {
		t.Errorf("overdue history missing: %+v", ex.Evidence)
	}
}

func TestExplainMissingReasonsAndStaleProgress(t *testing.T) {
	target := xord("SO-2", "normal", "10-30", "09-01")
	target.Status, target.Progress = "running", 0
	events := []Event{
		{ID: "a", Type: "date_changed", Field: "due_date", Old: "2026-10-10", New: "2026-10-20", At: md("09-05")},
		{ID: "b", Type: "date_changed", Field: "due_date", Old: "2026-10-20", New: "2026-10-30", At: md("09-10"), Reason: "客户改图"},
		{ID: "p", Type: "updated", Field: "progress", Old: "0", New: "0", At: md("09-12")},
	}
	ex := Explain(ExplainInput{Today: exToday, Target: target, Orders: []Order{target},
		Capacity: []CapacityStep{{From: md("01-01"), PerDay: 2}}, Events: events})
	if ex.Verdict != VerdictOnTrack {
		t.Fatalf("verdict = %s", ex.Verdict)
	}
	if !strings.Contains(strings.Join(ex.Missing, "|"), "1 次交期变更没有填写原因") {
		t.Errorf("missing = %v", ex.Missing)
	}
	if !hasEvidence(ex, EvidenceEvent, "没有写原因") || !hasEvidence(ex, EvidenceProgress, "已经 16 天没有更新") {
		t.Errorf("evidence = %+v", ex.Evidence)
	}
	if ex.Confidence != ConfidenceMedium || !strings.Contains(ex.Suggestion, "填写原因") {
		t.Errorf("confidence=%s suggestion=%s", ex.Confidence, ex.Suggestion)
	}
}

func TestExplainStartedOrderIsLowConfidence(t *testing.T) {
	target := xord("SO-S", "normal", "10-02", "09-01")
	target.Progress, target.Status = 40, "running"
	ex := Explain(ExplainInput{Today: exToday, Target: target, Orders: []Order{target},
		Capacity: []CapacityStep{{From: md("01-01"), PerDay: 1}},
		Events:   []Event{{ID: "p", Type: "updated", Field: "progress", Old: "10", New: "40", At: md("09-27")}}})
	if ex.Verdict != VerdictStarted || ex.Confidence != ConfidenceLow || ex.PlannedFinish != "" {
		t.Errorf("started: verdict=%s conf=%s finish=%s", ex.Verdict, ex.Confidence, ex.PlannedFinish)
	}
}

func TestExplainUrgentLateHasNoPreview(t *testing.T) {
	a := xord("SO-X", "urgent", "09-28", "09-01")
	target := xord("SO-Y", "urgent", "09-28", "09-02")
	ex := Explain(ExplainInput{Today: exToday, Target: target, Orders: []Order{a, target},
		Capacity: []CapacityStep{{From: md("01-01"), PerDay: 1}},
		Events:   []Event{{ID: "u", Type: "updated", Field: "progress", Old: "0", New: "0", At: md("09-27")}}})
	if ex.Verdict != VerdictLate || ex.Preview != nil || !strings.Contains(ex.Suggestion, "已是加急") {
		t.Errorf("urgent late: verdict=%s preview=%+v suggestion=%s", ex.Verdict, ex.Preview, ex.Suggestion)
	}
}

func TestExplainTightAndStartDate(t *testing.T) {
	target := xord("SO-T", "normal", "10-03", "09-01")
	s := md("10-03")
	target.Start = &s
	ex := Explain(ExplainInput{Today: exToday, Target: target, Orders: []Order{target},
		Capacity: []CapacityStep{{From: md("01-01"), PerDay: 1}},
		Events:   []Event{{ID: "u", Type: "updated", Field: "progress", Old: "0", New: "0", At: md("09-27")}}})
	if ex.Verdict != VerdictTight || !hasEvidence(ex, EvidencePlan, "计划开工日是 10-03") {
		t.Errorf("tight: %s %+v", ex.Verdict, ex.Evidence)
	}
}

// Same facts in any order give the same answer.
func TestExplainIsDeterministic(t *testing.T) {
	target := xord("SO-1", "normal", "09-29", "09-01")
	orders := []Order{xord("SO-A", "urgent", "10-05", "09-20"), target}
	ev := []Event{
		{ID: "1", Type: "date_changed", Field: "due_date", Old: "2026-09-20", New: "2026-09-25", At: md("09-10"), Reason: "r1"},
		{ID: "2", Type: "date_changed", Field: "due_date", Old: "2026-09-25", New: "2026-09-29", At: md("09-15"), Reason: "r2"},
	}
	rev := []Event{ev[1], ev[0]}
	a := Explain(ExplainInput{Today: exToday, Target: target, Orders: orders, Events: ev})
	b := Explain(ExplainInput{Today: exToday, Target: target, Orders: orders, Events: rev})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("event order changed the result:\n%+v\n%+v", a, b)
	}
}
