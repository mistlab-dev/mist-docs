package schedule

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var today = time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

func d(s string) time.Time {
	t, err := time.Parse(dateFmt, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ord(id, due string, created int) Order {
	return Order{ID: id, OrderNo: id, Title: id, Priority: "normal", Status: "pending", Due: d(due),
		CreatedAt: today.Add(-time.Duration(1000-created) * time.Hour)}
}

func ids(items []Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func eq(t *testing.T, what string, got, want interface{}) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// §9.1: an insert pushes two orders back and makes one miss its date.
func TestInsertTwoDelayedOneBreach(t *testing.T) {
	b, c, dd := ord("B", "2026-09-29", 1), ord("C", "2026-09-30", 2), ord("D", "2026-09-30", 3)
	dd.Remark = "逾期按合同违约金 5%/天"
	res := Preview(Input{Today: today, Orders: []Order{b, c, dd},
		Insert: Order{OrderNo: "A", Title: "A", Due: d("2026-09-28")}})

	eq(t, "conclusion", res.Conclusion, ConclusionBreach)
	eq(t, "breaches", ids(res.NewBreaches), []string{"D"})
	eq(t, "delayed", ids(res.Delayed), []string{"B", "C"})
	eq(t, "insert finish", res.Insert.NewFinish, "2026-09-28")
	eq(t, "insert outcome", res.Insert.Outcome, OutcomeUnaffected)
	eq(t, "insert id", res.Insert.ID, "")

	br := res.NewBreaches[0]
	eq(t, "D old→new", br.OldFinish+"→"+br.NewFinish, "2026-09-30→2026-10-01")
	eq(t, "D delay", br.DelayDays, 1)
	eq(t, "D late", br.LateDays, 1)
	eq(t, "D flags", br.Flags, []string{FlagPenalty})
}

// §9.1 / D15: progress > 0 holds a slot on its due date and is never re-planned.
func TestStartedOrdersHoldTheirSlot(t *testing.T) {
	run := ord("R", "2026-09-28", 1)
	run.Progress = 50
	run.Status = "running"
	x := ord("X", "2026-10-05", 2)
	res := Preview(Input{Today: today, Orders: []Order{run, x},
		Insert: Order{OrderNo: "N", Due: d("2026-09-28"), Priority: "urgent"}})

	eq(t, "started", ids(res.Started), []string{"R"})
	eq(t, "R finish unchanged", res.Started[0].OldFinish+"→"+res.Started[0].NewFinish, "2026-09-28→2026-09-28")
	// R occupies 09-28, so the urgent insert gets 09-29, X moves 09-29 → 09-30.
	eq(t, "insert finish", res.Insert.NewFinish, "2026-09-29")
	eq(t, "insert late", res.Insert.LateDays, 1)
	eq(t, "insert outcome", res.Insert.Outcome, OutcomeNewBreach)
	eq(t, "delayed", ids(res.Delayed), []string{"X"})
	eq(t, "conclusion", res.Conclusion, ConclusionDelay)
}

// §9.1: capacity 3 → six orders fill two days.
func TestCapacityThreeSixOrdersTwoDays(t *testing.T) {
	var os []Order
	for i := 1; i <= 5; i++ {
		os = append(os, ord(fmt.Sprintf("O%d", i), "2026-10-20", i))
	}
	res := Preview(Input{Today: today, Orders: os, Capacity: []CapacityStep{{From: d("2026-01-01"), PerDay: 3}},
		Insert: Order{OrderNo: "N", Due: d("2026-10-20"), Priority: "normal"}})
	eq(t, "per day", res.PerDay, 3)
	count := map[string]int{res.Insert.NewFinish: 1}
	for _, it := range append(res.Unaffected, res.Delayed...) {
		count[it.NewFinish]++
	}
	eq(t, "days", count, map[string]int{"2026-09-28": 3, "2026-09-29": 3})
	eq(t, "conclusion", res.Conclusion, ConclusionNone)
}

// §9.1: an insert that fits in spare capacity changes nothing.
func TestInsertWithoutImpact(t *testing.T) {
	os := []Order{ord("A", "2026-10-01", 1), ord("B", "2026-10-02", 2)}
	res := Preview(Input{Today: today, Orders: os, Capacity: []CapacityStep{{From: today, PerDay: 2}},
		Insert: Order{OrderNo: "N", Due: d("2026-10-10"), Priority: "normal"}})
	eq(t, "conclusion", res.Conclusion, ConclusionNone)
	eq(t, "breaches", len(res.NewBreaches), 0)
	eq(t, "delayed", len(res.Delayed), 0)
	eq(t, "unaffected", ids(res.Unaffected), []string{"A", "B"})
}

// §9.1: same priority is ordered by due date, then creation, deterministically.
func TestSamePriorityStableByDueDate(t *testing.T) {
	os := []Order{ord("late", "2026-10-09", 1), ord("early", "2026-10-01", 5), ord("mid2", "2026-10-05", 4), ord("mid1", "2026-10-05", 3)}
	for i := 0; i < 20; i++ {
		rand.Shuffle(len(os), func(a, b int) { os[a], os[b] = os[b], os[a] })
		f := plan(today, os, nil)
		eq(t, "order", []string{f["early"].Format(dateFmt), f["mid1"].Format(dateFmt), f["mid2"].Format(dateFmt), f["late"].Format(dateFmt)},
			[]string{"2026-09-28", "2026-09-29", "2026-09-30", "2026-10-01"})
	}
}

// §4.4: urgent → inserted → normal regardless of due date.
func TestPriorityOrder(t *testing.T) {
	n := ord("normal", "2026-09-28", 1)
	i := ord("inserted", "2026-10-30", 2)
	i.Priority = "inserted"
	u := ord("urgent", "2026-11-30", 3)
	u.Priority = "urgent"
	f := plan(today, []Order{n, i, u}, nil)
	eq(t, "urgent", f["urgent"].Format(dateFmt), "2026-09-28")
	eq(t, "inserted", f["inserted"].Format(dateFmt), "2026-09-29")
	eq(t, "normal", f["normal"].Format(dateFmt), "2026-09-30")
}

// §4.4: overdue orders keep queueing; slipping further is "delayed", flagged already late.
func TestOverdueOrdersStillQueue(t *testing.T) {
	o := ord("OD", "2026-09-20", 1)
	o.Status = "overdue"
	res := Preview(Input{Today: today, Orders: []Order{o},
		Insert: Order{OrderNo: "N", Due: d("2026-09-28"), Priority: "urgent"}})
	eq(t, "delayed", ids(res.Delayed), []string{"OD"})
	eq(t, "flags", res.Delayed[0].Flags, []string{FlagAlreadyLate})
	eq(t, "late days", res.Delayed[0].LateDays, 9)
	eq(t, "conclusion", res.Conclusion, ConclusionDelay)
}

func TestStartDateAndCapacitySteps(t *testing.T) {
	s := d("2026-10-01")
	a := ord("A", "2026-10-03", 1)
	a.Start = &s
	f := plan(today, []Order{a, ord("B", "2026-10-09", 2), ord("C", "2026-10-09", 3), ord("D", "2026-10-09", 4)},
		[]CapacityStep{{From: d("2026-01-01"), PerDay: 1}, {From: d("2026-09-29"), PerDay: 2}})
	eq(t, "A waits for its start date", f["A"].Format(dateFmt), "2026-10-01")
	eq(t, "B", f["B"].Format(dateFmt), "2026-09-28")
	eq(t, "C", f["C"].Format(dateFmt), "2026-09-29")
	eq(t, "D", f["D"].Format(dateFmt), "2026-09-29")
}

func TestKeyCustomerFlag(t *testing.T) {
	b := ord("B", "2026-09-28", 1)
	b.Customer = "  Acme 精密 "
	res := Preview(Input{Today: today, Orders: []Order{b}, KeyCustomers: []string{"acme 精密"},
		Insert: Order{OrderNo: "N", Due: d("2026-09-28"), Priority: "urgent"}})
	eq(t, "breach", ids(res.NewBreaches), []string{"B"})
	eq(t, "flags", res.NewBreaches[0].Flags, []string{FlagKeyCustomer})
}

// Re-prioritising an existing order reports it as the insert, not as affected.
func TestExistingOrderReprioritised(t *testing.T) {
	a, b, c := ord("A", "2026-09-28", 1), ord("B", "2026-09-29", 2), ord("C", "2026-10-09", 3)
	res := Preview(Input{Today: today, Orders: []Order{a, b, c},
		Insert: Order{ID: "C", Due: d("2026-09-28"), Priority: "urgent"}})
	eq(t, "insert id", res.Insert.ID, "C")
	eq(t, "insert old→new", res.Insert.OldFinish+"→"+res.Insert.NewFinish, "2026-09-30→2026-09-28")
	eq(t, "breaches", ids(res.NewBreaches), []string{"A", "B"})
}

// §4.3 / §9.1: inserting an order never makes any other order finish earlier.
func TestMonotonicProperty(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	prios := []string{"urgent", "inserted", "normal"}
	for round := 0; round < 3000; round++ {
		n := r.Intn(25)
		var os []Order
		for i := 0; i < n; i++ {
			o := ord(fmt.Sprintf("o%d", i), today.AddDate(0, 0, r.Intn(30)-5).Format(dateFmt), r.Intn(50))
			o.Priority = prios[r.Intn(3)]
			if r.Intn(5) == 0 {
				o.Progress = 10 + r.Intn(80)
			}
			if r.Intn(4) == 0 {
				s := today.AddDate(0, 0, r.Intn(10))
				o.Start = &s
			}
			os = append(os, o)
		}
		var steps []CapacityStep
		if r.Intn(2) == 0 {
			steps = append(steps, CapacityStep{From: today.AddDate(0, 0, -3), PerDay: 1 + r.Intn(3)})
			steps = append(steps, CapacityStep{From: today.AddDate(0, 0, r.Intn(10)), PerDay: 1 + r.Intn(3)})
		}
		ins := Order{OrderNo: "N", Due: today.AddDate(0, 0, r.Intn(20)), Priority: prios[r.Intn(3)]}
		if r.Intn(4) == 0 {
			s := today.AddDate(0, 0, r.Intn(10))
			ins.Start = &s
		}
		res := Preview(Input{Today: today, Orders: os, Insert: ins, Capacity: steps})
		all := append(append(append(append([]Item{}, res.NewBreaches...), res.Delayed...), res.Unaffected...), res.Started...)
		if len(all) != n {
			t.Fatalf("round %d: %d items for %d orders", round, len(all), n)
		}
		for _, it := range all {
			if it.NewFinish < it.OldFinish {
				t.Fatalf("round %d: %s finished earlier after insert: %s → %s", round, it.ID, it.OldFinish, it.NewFinish)
			}
		}
	}
}
