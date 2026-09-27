package schedule

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ==================== 交期解释器（规则版，无 AI） ====================
//
// Explain answers "why might this order be late?" from facts only: the
// order's change history (md_deadline_events), the same queue model as the
// insert preview, and the overdue history. Output follows the structure in
// DESIGN-DEADLINE-AI.md §5.2 (conclusion, confidence, evidence, missing,
// suggestion). Every sentence is built from a rule; nothing is generated.
// When the facts are thin it says so ("数据不足") instead of guessing.

// Event is one md_deadline_events row, reduced to what the explainer reads.
type Event struct {
	ID     string
	Type   string // created | updated | status_changed | date_changed | proposal_applied | deleted
	Field  string
	Old    string
	New    string
	Reason string
	Actor  string // display name, may be empty
	At     time.Time
}

// ExplainInput is everything Explain needs. Target may be done (then it is
// not in Orders); otherwise it must also appear in Orders.
type ExplainInput struct {
	Today        time.Time
	Target       Order
	Orders       []Order // current open orders (status <> done)
	Capacity     []CapacityStep
	KeyCustomers []string
	Events       []Event // any order; oldest or newest first, both fine
}

// Evidence types.
const (
	EvidenceEvent    = "event"
	EvidenceLoad     = "load"
	EvidenceHistory  = "history"
	EvidencePlan     = "plan"
	EvidenceProgress = "progress"
	EvidenceFlag     = "flag"
)

// Verdicts.
const (
	VerdictDone     = "done"
	VerdictOverdue  = "overdue"   // due date already passed and not done
	VerdictLate     = "late"      // the queue puts the finish after the due date
	VerdictTight    = "tight"     // finishes on the due date: no slack
	VerdictOnTrack  = "on_track"  // finishes before the due date
	VerdictStarted  = "started"   // in production; the model does not re-plan it
	VerdictNoSignal = "no_signal" // nothing recorded to reason from
)

// Confidence levels.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// EvidenceItem is one fact behind the conclusion. Ref points at its source:
// an event id, "schedule" for the queue model, or an order id.
type EvidenceItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
}

// SuggestedPreview pre-fills the insert preview for "what if we expedite".
type SuggestedPreview struct {
	DeadlineID string `json:"deadline_id"`
	Priority   string `json:"priority"`
	DueDate    string `json:"due_date"`
}

// Explanation is what the explain endpoint returns.
type Explanation struct {
	DeadlineID    string            `json:"deadline_id"`
	OrderNo       string            `json:"order_no"`
	Verdict       string            `json:"verdict"`
	Conclusion    string            `json:"conclusion"`
	Confidence    string            `json:"confidence"`
	DueDate       string            `json:"due_date"`
	PlannedFinish string            `json:"planned_finish,omitempty"`
	LateDays      int               `json:"late_days"`
	Evidence      []EvidenceItem    `json:"evidence"`
	Missing       []string          `json:"missing"`
	Assumptions   []string          `json:"assumptions"`
	Suggestion    string            `json:"suggestion"`
	Preview       *SuggestedPreview `json:"preview,omitempty"`
}

// StaleProgressDays: no progress update for this long on an unfinished,
// not-yet-due order is worth pointing out.
const StaleProgressDays = 7

func mmdd(t time.Time) string { return day(t).Format("01-02") }

func orderLabel(o Order) string {
	if o.OrderNo != "" {
		return o.OrderNo
	}
	return o.Title
}

var priorityZh = map[string]string{"urgent": "加急", "inserted": "插单", "normal": "普通"}

func zh(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return k
}

// Explain builds the rule-based explanation for in.Target.
func Explain(in ExplainInput) Explanation {
	t := in.Target
	today := day(in.Today)
	ex := Explanation{
		DeadlineID: t.ID, OrderNo: t.OrderNo,
		DueDate:  day(t.Due).Format(dateFmt),
		Evidence: []EvidenceItem{}, Missing: []string{}, Assumptions: []string{},
	}

	events := append([]Event(nil), in.Events...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })

	// ---- facts from the history ----
	var dueChanges, noReason, overdueMarks, progressEvents []Event
	var lastProgress *Event
	substantive := 0 // anything beyond the "created" row
	for i := range events {
		e := events[i]
		if e.Type != "created" {
			substantive++
		}
		switch {
		case e.Field == "due_date" && (e.Type == "date_changed" || e.Type == "proposal_applied"):
			dueChanges = append(dueChanges, e)
			if strings.TrimSpace(e.Reason) == "" {
				noReason = append(noReason, e)
			}
		case e.Field == "status" && e.New == "overdue":
			overdueMarks = append(overdueMarks, e)
		case e.Field == "progress":
			progressEvents = append(progressEvents, e)
			lastProgress = &events[i]
		}
	}

	// ---- the queue model ----
	capConfigured := len(in.Capacity) > 0
	perDayToday := perDay(in.Capacity, today)
	var finish time.Time
	planned := false
	var ahead []Order
	if t.Status != "done" && !Started(t) {
		fin := plan(in.Today, in.Orders, in.Capacity)
		if f, ok := fin[t.ID]; ok {
			finish, planned = f, true
			ex.PlannedFinish = f.Format(dateFmt)
			for _, o := range in.Orders {
				if o.ID == t.ID || Started(o) {
					continue
				}
				if f2, ok := fin[o.ID]; ok && !f2.After(f) && queuedBefore(o, t) {
					ahead = append(ahead, o)
				}
			}
		}
	}

	// ---- verdict and conclusion ----
	daysLeft := days(t.Due, today)
	switch {
	case t.Status == "done":
		ex.Verdict = VerdictDone
		ex.Conclusion = "已完成，不存在延期风险"
	case daysLeft < 0:
		ex.Verdict = VerdictOverdue
		ex.LateDays = -daysLeft
		ex.Conclusion = fmt.Sprintf("已逾期 %d 天（交期 %s）", -daysLeft, mmdd(t.Due))
	case Started(t):
		ex.Verdict = VerdictStarted
		ex.Conclusion = fmt.Sprintf("已开工（进度 %d%%），模型不重新估算完工日，交期 %s", t.Progress, mmdd(t.Due))
	case planned && finish.After(day(t.Due)):
		ex.Verdict = VerdictLate
		ex.LateDays = days(finish, t.Due)
		ex.Conclusion = fmt.Sprintf("按当前排队可能晚 %d 天（预计 %s 完成，交期 %s）", ex.LateDays, mmdd(finish), mmdd(t.Due))
	case planned && finish.Equal(day(t.Due)):
		ex.Verdict = VerdictTight
		ex.Conclusion = fmt.Sprintf("能赶上但没有余量（预计 %s 完成，正好是交期）", mmdd(finish))
	case planned:
		ex.Verdict = VerdictOnTrack
		ex.Conclusion = fmt.Sprintf("按当前排队可按期（预计 %s 完成，余量 %d 天）", mmdd(finish), days(t.Due, finish))
	default:
		ex.Verdict = VerdictNoSignal
		ex.Conclusion = "无法估算"
	}

	// ---- evidence: plan and load ----
	if planned {
		ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidencePlan, Ref: "schedule",
			Text: fmt.Sprintf("按每天 %d 单的产能排队，本单排第 %d 位，预计 %s 完成", perDayToday, len(ahead)+1, mmdd(finish))})
		if len(ahead) > 0 {
			ex.Evidence = append(ex.Evidence, loadEvidence(t, ahead))
		}
		if t.Start != nil && day(*t.Start).After(today) {
			ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidencePlan, Ref: "start_date",
				Text: fmt.Sprintf("计划开工日是 %s，最早从那天才开始排", mmdd(*t.Start))})
		}
		if squeezed := jumpedAhead(t, ahead); len(squeezed) > 0 {
			labels := make([]string, 0, len(squeezed))
			for _, o := range squeezed {
				labels = append(labels, orderLabel(o)+"（"+zh(priorityZh, o.Priority)+"）")
			}
			ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceLoad, Ref: squeezed[0].ID,
				Text: fmt.Sprintf("有 %d 单比本单晚建、但优先级更高而排在前面：%s", len(squeezed), strings.Join(limit(labels, 3), "、"))})
		}
	}

	// ---- evidence: history ----
	for _, e := range lastN(dueChanges, 3) {
		ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceEvent, Ref: e.ID, Text: dueChangeText(e)})
	}
	if len(dueChanges) > 3 {
		ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceHistory, Ref: "events",
			Text: fmt.Sprintf("交期一共改过 %d 次（上面只列最近 3 次）", len(dueChanges))})
	}
	if len(overdueMarks) > 0 {
		last := overdueMarks[len(overdueMarks)-1]
		ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceHistory, Ref: last.ID,
			Text: fmt.Sprintf("被标记过 %d 次“已逾期”，最近一次在 %s", len(overdueMarks), mmdd(last.At))})
	}

	// ---- evidence: progress ----
	if t.Status != "done" && t.Progress < 100 {
		if lastProgress != nil {
			idle := days(today, lastProgress.At)
			if idle >= StaleProgressDays {
				ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceProgress, Ref: lastProgress.ID,
					Text: fmt.Sprintf("进度停在 %d%%，已经 %d 天没有更新", t.Progress, idle)})
			} else {
				ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceProgress, Ref: lastProgress.ID,
					Text: fmt.Sprintf("进度 %d%%，%s 更新过", t.Progress, mmdd(lastProgress.At))})
			}
		} else if t.Status == "running" || t.Progress > 0 {
			ex.Missing = append(ex.Missing, "没有进度更新记录，无法判断生产是否推进")
		}
	}

	// ---- evidence: flags ----
	for _, f := range flags(t, in.KeyCustomers) {
		switch f {
		case FlagPenalty:
			ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceFlag, Ref: "remark", Text: "备注提到违约/赔偿/扣款，晚交有直接损失"})
		case FlagKeyCustomer:
			ex.Evidence = append(ex.Evidence, EvidenceItem{Type: EvidenceFlag, Ref: "customer", Text: "客户 " + t.Customer + " 在重点客户名单里"})
		}
	}

	// ---- missing data ----
	if len(noReason) > 0 {
		ex.Missing = append(ex.Missing, fmt.Sprintf("%d 次交期变更没有填写原因", len(noReason)))
	}
	insufficient := substantive == 0 && t.Status != "done" && ex.Verdict != VerdictOverdue
	if insufficient {
		ex.Missing = append(ex.Missing, "这张单除了创建之外没有任何变更记录（交期、状态、进度都没改过），看不出延误原因")
	}
	if !capConfigured {
		ex.Missing = append(ex.Missing, "团队没有配置产能，按默认每天 1 单估算")
	}

	ex.Assumptions = append(ex.Assumptions,
		"排队模型只按“每天可完成 N 单”估算，不考虑工序、机台、外协和节假日",
		"已开工（进度 > 0）的单按原交期占位，不重新估算")

	// ---- confidence ----
	switch {
	case ex.Verdict == VerdictDone || ex.Verdict == VerdictOverdue:
		ex.Confidence = ConfidenceHigh // a fact, not an estimate
	case insufficient || ex.Verdict == VerdictStarted || ex.Verdict == VerdictNoSignal:
		ex.Confidence = ConfidenceLow
	case capConfigured && len(ex.Missing) == 0:
		ex.Confidence = ConfidenceHigh
	case len(ex.Missing) >= 2:
		ex.Confidence = ConfidenceLow
	default:
		ex.Confidence = ConfidenceMedium
	}
	if insufficient {
		ex.Conclusion = "数据不足：" + ex.Conclusion
	}

	// ---- suggestion ----
	switch ex.Verdict {
	case VerdictDone:
		ex.Suggestion = "无需处理"
	case VerdictOverdue:
		ex.Suggestion = "先和客户确认新的交期，改交期时写明原因，下次解释会更准"
	case VerdictLate, VerdictTight:
		if t.Priority != "urgent" {
			ex.Suggestion = "如需保交期，可在预演里看看把本单改为加急对其它单的影响"
			ex.Preview = &SuggestedPreview{DeadlineID: t.ID, Priority: "urgent", DueDate: day(t.Due).Format(dateFmt)}
		} else {
			ex.Suggestion = "本单已是加急，仍排不进交期：需要加产能、外协，或与客户协商交期"
		}
	case VerdictStarted:
		ex.Suggestion = "请负责人更新进度；进度长期不动时要尽早和客户沟通"
	case VerdictOnTrack:
		ex.Suggestion = "按当前排队可按期，无需调整"
	default:
		ex.Suggestion = "补充交期、进度的变更记录后再看"
	}
	if len(noReason) > 0 && ex.Verdict != VerdictDone {
		ex.Suggestion += "；以后改交期请填写原因"
	}
	return ex
}

// queuedBefore mirrors the sort key in plan: priority, due date, created, id.
func queuedBefore(a, b Order) bool {
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
}

func loadEvidence(t Order, ahead []Order) EvidenceItem {
	byPri := map[string]int{}
	for _, o := range ahead {
		byPri[o.Priority]++
	}
	parts := []string{}
	for _, p := range []string{"urgent", "inserted", "normal"} {
		if byPri[p] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d 单", zh(priorityZh, p), byPri[p]))
		}
	}
	return EvidenceItem{Type: EvidenceLoad, Ref: "schedule",
		Text: fmt.Sprintf("排在本单前面的还有 %d 单（%s）", len(ahead), strings.Join(parts, "，"))}
}

// jumpedAhead lists orders created after t that are queued ahead of it only
// because of a higher priority: the "插单挤占" signal.
func jumpedAhead(t Order, ahead []Order) []Order {
	var out []Order
	for _, o := range ahead {
		if priorityRank(o.Priority) < priorityRank(t.Priority) && o.CreatedAt.After(t.CreatedAt) {
			out = append(out, o)
		}
	}
	return out
}

func dueChangeText(e Event) string {
	s := fmt.Sprintf("%s 交期从 %s 改到 %s", mmdd(e.At), shortDate(e.Old), shortDate(e.New))
	if e.Type == "proposal_applied" {
		s += "（预演确认）"
	}
	if r := strings.TrimSpace(e.Reason); r != "" {
		s += "，原因：" + r
	} else {
		s += "，没有写原因"
	}
	if e.Actor != "" {
		s += "（" + e.Actor + "）"
	}
	return s
}

func shortDate(s string) string {
	if t, err := time.Parse(dateFmt, s); err == nil {
		return t.Format("01-02")
	}
	if s == "" {
		return "—"
	}
	return s
}

func lastN(es []Event, n int) []Event {
	if len(es) <= n {
		return es
	}
	return es[len(es)-n:]
}

func limit(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return append(s[:n:n], "等 "+strconv.Itoa(len(s))+" 单")
}
