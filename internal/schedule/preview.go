// Package schedule is the deterministic "what happens if we insert this order"
// calculation behind the deadline board's insert preview (design doc
// DESIGN-DEADLINE-AI.md §4). It is a pure function: no database, no clock,
// no AI. The handler loads the rows, calls Preview, and returns the result.
//
// Model (§4.1, D14): the team finishes up to N orders per calendar day. Open
// orders are queued by priority (urgent → inserted → normal), then due date,
// then creation time, and each takes the first day, from max(today, start
// date), that still has a free slot. The result is advice, not a promise:
// no weekends, holidays, routings or machines are modelled.
package schedule

import (
	"sort"
	"strings"
	"time"
)

// Order is one open deadline row, reduced to what the model needs.
type Order struct {
	ID        string
	OrderNo   string
	Title     string
	Customer  string
	Remark    string
	Priority  string // urgent | inserted | normal
	Status    string // pending | running | overdue (done rows are not passed in)
	Progress  int    // 0-100
	Start     *time.Time
	Due       time.Time
	CreatedAt time.Time
}

// CapacityStep sets the orders-per-day from a date on (the md_team_capacity
// rows). The step with the latest From on or before a day applies.
type CapacityStep struct {
	From   time.Time
	PerDay int
}

// Input is everything Preview needs.
type Input struct {
	Today    time.Time
	Orders   []Order        // current open orders
	Insert   Order          // the order to insert; ID set = an existing order being re-prioritised / re-dated
	Capacity []CapacityStep // empty = 1 per day
	// KeyCustomers are matched case-insensitively against Order.Customer (D16).
	KeyCustomers []string
}

// Flags explain why an affected order deserves attention (D16).
const (
	FlagPenalty     = "penalty"      // remark mentions 违约 / 赔偿 / 扣款
	FlagKeyCustomer = "key_customer" // customer is on the team's key-customer list
	FlagAlreadyLate = "already_late" // it was going to miss its date anyway
)

// Outcome of one order.
const (
	OutcomeNewBreach  = "new_breach" // met its due date before, misses it after
	OutcomeDelayed    = "delayed"    // finishes later, but the verdict did not flip
	OutcomeUnaffected = "unaffected"
	OutcomeStarted    = "started" // progress > 0: holds its slot, not re-planned (D15)
)

// Conclusion for the whole preview.
const (
	ConclusionBreach = "breach" // at least one order newly misses its date
	ConclusionDelay  = "delay"  // some orders slip, none newly misses
	ConclusionNone   = "none"
)

// Item is the before/after for one order.
type Item struct {
	ID        string   `json:"id"`
	OrderNo   string   `json:"order_no"`
	Title     string   `json:"title"`
	Customer  string   `json:"customer,omitempty"`
	Priority  string   `json:"priority"`
	Due       string   `json:"due_date"`
	OldFinish string   `json:"old_finish,omitempty"` // empty for a brand-new insert
	NewFinish string   `json:"new_finish"`
	DelayDays int      `json:"delay_days"` // new - old, in days (0 if not later)
	LateDays  int      `json:"late_days"`  // new finish - due date, if positive
	Outcome   string   `json:"outcome"`
	Flags     []string `json:"flags,omitempty"`
}

// Result is what the preview endpoint returns.
type Result struct {
	Insert      Item   `json:"insert"`
	NewBreaches []Item `json:"new_breaches"`
	Delayed     []Item `json:"delayed"`
	Unaffected  []Item `json:"unaffected"`
	Started     []Item `json:"started"`
	Conclusion  string `json:"conclusion"`
	PerDay      int    `json:"per_day"` // capacity on Today
}

var penaltyWords = []string{"违约", "赔偿", "扣款"}

const dateFmt = "2006-01-02"

func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func days(a, b time.Time) int { return int(day(a).Sub(day(b)).Hours() / 24) }

func priorityRank(p string) int {
	switch p {
	case "urgent":
		return 0
	case "inserted":
		return 1
	default:
		return 2
	}
}

// Started reports whether an order is treated as already in production (D15).
func Started(o Order) bool { return o.Progress > 0 }

// PerDayOn is the capacity that applies on day d.
func PerDayOn(steps []CapacityStep, d time.Time) int { return perDay(steps, day(d)) }

func perDay(steps []CapacityStep, d time.Time) int {
	n := 1
	var best time.Time
	found := false
	for _, s := range steps {
		f := day(s.From)
		if !f.After(d) && (!found || f.After(best)) {
			best, n, found = f, s.PerDay, true
		}
	}
	if n < 1 {
		n = 1
	}
	return n
}

// plan returns the planned finish day for every order ID.
func plan(today time.Time, orders []Order, steps []CapacityStep) map[string]time.Time {
	today = day(today)
	used := map[time.Time]int{}
	finish := make(map[string]time.Time, len(orders))
	var queue []Order
	for _, o := range orders {
		if Started(o) {
			// D15: holds one slot on its due date; its date does not move.
			d := day(o.Due)
			used[d]++
			finish[o.ID] = d
			continue
		}
		queue = append(queue, o)
	}
	sort.SliceStable(queue, func(i, j int) bool {
		a, b := queue[i], queue[j]
		if ra, rb := priorityRank(a.Priority), priorityRank(b.Priority); ra != rb {
			return ra < rb
		}
		if !day(a.Due).Equal(day(b.Due)) {
			return a.Due.Before(b.Due)
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
	for _, o := range queue {
		d := today
		if o.Start != nil && day(*o.Start).After(d) {
			d = day(*o.Start)
		}
		for used[d] >= perDay(steps, d) {
			d = d.AddDate(0, 0, 1)
		}
		used[d]++
		finish[o.ID] = d
	}
	return finish
}

func flags(o Order, keys []string) []string {
	var f []string
	for _, w := range penaltyWords {
		if strings.Contains(o.Remark, w) {
			f = append(f, FlagPenalty)
			break
		}
	}
	c := strings.TrimSpace(strings.ToLower(o.Customer))
	if c != "" {
		for _, k := range keys {
			if strings.TrimSpace(strings.ToLower(k)) == c {
				f = append(f, FlagKeyCustomer)
				break
			}
		}
	}
	return f
}

// InsertID is the placeholder ID used for a brand-new order.
const InsertID = "__insert__"

// Preview compares the plan without and with the inserted order.
func Preview(in Input) Result {
	ins := in.Insert
	if ins.ID == "" {
		ins.ID = InsertID
	}
	if ins.Priority == "" {
		ins.Priority = "inserted"
	}
	if ins.CreatedAt.IsZero() {
		ins.CreatedAt = in.Today.Add(24 * time.Hour) // after every existing order of the same key
	}

	before := make([]Order, 0, len(in.Orders))
	after := make([]Order, 0, len(in.Orders)+1)
	existing := false
	for _, o := range in.Orders {
		before = append(before, o)
		if o.ID == ins.ID {
			existing = true
			// Re-prioritising an existing order keeps its identity and history.
			merged := o
			merged.Priority = ins.Priority
			merged.Due = ins.Due
			if ins.Start != nil {
				merged.Start = ins.Start
			}
			ins = merged
			after = append(after, merged)
			continue
		}
		after = append(after, o)
	}
	if !existing {
		after = append(after, ins)
	}

	old := plan(in.Today, before, in.Capacity)
	neu := plan(in.Today, after, in.Capacity)

	res := Result{
		NewBreaches: []Item{}, Delayed: []Item{}, Unaffected: []Item{}, Started: []Item{},
		PerDay: perDay(in.Capacity, day(in.Today)),
	}

	mk := func(o Order) Item {
		it := Item{
			ID: o.ID, OrderNo: o.OrderNo, Title: o.Title, Customer: o.Customer, Priority: o.Priority,
			Due: day(o.Due).Format(dateFmt), NewFinish: neu[o.ID].Format(dateFmt),
			Flags: flags(o, in.KeyCustomers),
		}
		if late := days(neu[o.ID], o.Due); late > 0 {
			it.LateDays = late
		}
		if f, ok := old[o.ID]; ok {
			it.OldFinish = f.Format(dateFmt)
			if d := days(neu[o.ID], f); d > 0 {
				it.DelayDays = d
			}
		}
		return it
	}

	res.Insert = mk(ins)
	if res.Insert.LateDays > 0 {
		res.Insert.Outcome = OutcomeNewBreach
	} else {
		res.Insert.Outcome = OutcomeUnaffected
	}
	if res.Insert.ID == InsertID {
		res.Insert.ID = ""
	}

	for _, o := range in.Orders {
		if o.ID == ins.ID {
			continue
		}
		it := mk(o)
		wasLate := days(old[o.ID], o.Due) > 0
		switch {
		case Started(o):
			it.Outcome = OutcomeStarted
			res.Started = append(res.Started, it)
		case it.DelayDays > 0 && it.LateDays > 0 && !wasLate:
			it.Outcome = OutcomeNewBreach
			res.NewBreaches = append(res.NewBreaches, it)
		case it.DelayDays > 0:
			it.Outcome = OutcomeDelayed
			if wasLate {
				it.Flags = append(it.Flags, FlagAlreadyLate)
			}
			res.Delayed = append(res.Delayed, it)
		default:
			it.Outcome = OutcomeUnaffected
			res.Unaffected = append(res.Unaffected, it)
		}
	}
	byDue := func(s []Item) {
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Due != s[j].Due {
				return s[i].Due < s[j].Due
			}
			return s[i].OrderNo < s[j].OrderNo
		})
	}
	byDue(res.NewBreaches)
	byDue(res.Delayed)
	byDue(res.Unaffected)
	byDue(res.Started)

	switch {
	case len(res.NewBreaches) > 0:
		res.Conclusion = ConclusionBreach
	case len(res.Delayed) > 0:
		res.Conclusion = ConclusionDelay
	default:
		res.Conclusion = ConclusionNone
	}
	return res
}
