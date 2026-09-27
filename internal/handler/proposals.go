package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/schedule"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==================== 两段式提议：预演 → 确认落库 ====================
//
// Every insert preview stores a pending proposal holding the preview result,
// the exact changes that confirming would make, and a fingerprint of the
// data it was computed from. Confirming (editor+, D17: one person) re-checks
// the fingerprint inside a transaction with the open orders locked; if
// anything moved it refuses with 409 and marks the proposal stale.
// The changes applied are the stored ones, never a list sent by the client.

const (
	proposalPending  = "pending"
	proposalApplied  = "applied"
	proposalRejected = "rejected"
	proposalStale    = "stale"
)

// proposalChange is one write that confirming will make.
type proposalChange struct {
	DeadlineID string `json:"deadline_id,omitempty"` // empty for __create__
	OrderNo    string `json:"order_no"`
	Title      string `json:"title,omitempty"`
	Field      string `json:"field"` // __create__ | due_date | priority
	Old        string `json:"old,omitempty"`
	New        string `json:"new,omitempty"`
}

type proposalPayload struct {
	Request previewInput     `json:"request"`
	Result  schedule.Result  `json:"result"`
	Changes []proposalChange `json:"changes"`
}

type proposalBaseline struct {
	Day         string `json:"day"`
	Fingerprint string `json:"fingerprint"`
}

// scheduleFingerprint changes whenever anything the preview depends on
// changes: an open order appears, disappears or changes date, priority,
// status, progress or start date, or the capacity changes.
func scheduleFingerprint(orders []schedule.Order, steps []schedule.CapacityStep) string {
	lines := make([]string, 0, len(orders)+len(steps))
	for _, o := range orders {
		start := ""
		if o.Start != nil {
			start = o.Start.Format(dateLayout)
		}
		lines = append(lines, fmt.Sprintf("o|%s|%s|%s|%s|%d|%s", o.ID, o.Due.Format(dateLayout), o.Priority, o.Status, o.Progress, start))
	}
	for _, s := range steps {
		lines = append(lines, fmt.Sprintf("c|%s|%d", s.From.Format(dateLayout), s.PerDay))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// proposalChanges lists what confirming the preview writes: the new order
// (or the moved existing one), and for every order that newly misses its
// date, its due date moved to the new planned finish (design §6.1). Delayed
// orders that still make their date, and orders that were late anyway, are
// left alone.
func proposalChanges(in previewInput, sin schedule.Input, res schedule.Result) []proposalChange {
	var ch []proposalChange
	due := sin.Insert.Due.Format(dateLayout)
	if in.DeadlineID == "" {
		ch = append(ch, proposalChange{Field: "__create__", OrderNo: sin.Insert.OrderNo, Title: sin.Insert.Title, New: due})
	} else {
		for _, o := range sin.Orders {
			if o.ID != in.DeadlineID {
				continue
			}
			if o.Due.Format(dateLayout) != due {
				ch = append(ch, proposalChange{DeadlineID: o.ID, OrderNo: o.OrderNo, Title: o.Title, Field: "due_date", Old: o.Due.Format(dateLayout), New: due})
			}
			if o.Priority != sin.Insert.Priority {
				ch = append(ch, proposalChange{DeadlineID: o.ID, OrderNo: o.OrderNo, Title: o.Title, Field: "priority", Old: o.Priority, New: sin.Insert.Priority})
			}
		}
	}
	for _, it := range res.NewBreaches {
		ch = append(ch, proposalChange{DeadlineID: it.ID, OrderNo: it.OrderNo, Title: it.Title, Field: "due_date", Old: it.Due, New: it.NewFinish})
	}
	return ch
}

func proposalTitle(in previewInput, sin schedule.Input) string {
	name := sin.Insert.OrderNo
	if name == "" {
		name = sin.Insert.Title
	}
	if in.DeadlineID != "" {
		return "调整 " + name
	}
	return "插单 " + name
}

// createProposal stores a pending proposal for a preview and returns its id
// and the changes confirming would make.
func createProposal(teamID, userID string, in previewInput, sin schedule.Input, res schedule.Result) (string, []proposalChange, error) {
	if in.Priority == "" {
		in.Priority = sin.Insert.Priority
	}
	changes := proposalChanges(in, sin, res)
	payload, _ := json.Marshal(proposalPayload{Request: in, Result: res, Changes: changes})
	baseline, _ := json.Marshal(proposalBaseline{
		Day:         sin.Today.Format(dateLayout),
		Fingerprint: scheduleFingerprint(sin.Orders, sin.Capacity),
	})
	id := uuid.New().String()
	_, err := database.DB.Exec(`INSERT INTO md_proposals (id, team_id, user_id, kind, title, payload, baseline, status)
		VALUES (?, ?, ?, 'insert', ?, ?, ?, 'pending')`, id, teamID, userID, proposalTitle(in, sin), string(payload), string(baseline))
	return id, changes, err
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertDeadlineEvent(ex execer, teamID, deadlineID, eventType, field, oldValue, newValue, reason, actorID string) error {
	_, err := ex.Exec(`
		INSERT INTO md_deadline_events
			(id, team_id, deadline_id, event_type, field, old_value, new_value, reason, actor_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), teamID, deadlineID, eventType, field, oldValue, newValue, reason, actorID)
	return err
}

func markProposalStale(teamID, id string) {
	database.DB.Exec(`UPDATE md_proposals SET status='stale', decided_at=NOW()
		WHERE id=? AND team_id=? AND status='pending'`, id, teamID)
}

// TeamApplyProposal POST /teams/:team_id/proposals/:id/apply (editor+)
//
// Body: {reason?}. Applies the stored changes in one transaction.
func TeamApplyProposal(c *gin.Context) {
	// Design R3: the role is checked here, not left to a middleware.
	if !requireRole(c, RoleEditor) {
		return
	}
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	id := c.Param("id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	tx, err := database.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	var title, payloadText, baselineText, status string
	err = tx.QueryRow(`SELECT title, payload, baseline, status FROM md_proposals
		WHERE id=? AND team_id=? FOR UPDATE`, id, teamID).Scan(&title, &payloadText, &baselineText, &status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "提议不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if status != proposalPending {
		c.JSON(http.StatusConflict, gin.H{"error": "这条提议已经处理过了", "status": status})
		return
	}
	var payload proposalPayload
	var baseline proposalBaseline
	if json.Unmarshal([]byte(payloadText), &payload) != nil || json.Unmarshal([]byte(baselineText), &baseline) != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提议数据损坏"})
		return
	}

	stale := func(msg string) {
		tx.Rollback()
		markProposalStale(teamID, id)
		c.JSON(http.StatusConflict, gin.H{"error": msg, "status": proposalStale})
	}
	if baseline.Day != time.Now().Format(dateLayout) {
		stale("预演是按前一天的数据算的，请重新预演")
		return
	}
	orders, err := loadOpenOrders(tx, teamID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tc, err := loadTeamCapacity(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if scheduleFingerprint(orders, tc.Steps) != baseline.Fingerprint {
		stale("订单数据已变化，请重新预演")
		return
	}

	reason := title + " 影响面确认"
	if r := strings.TrimSpace(body.Reason); r != "" {
		reason += "：" + r
	}
	createdID := ""
	var notes []proposalNote // owners of changed orders, told after commit
	for _, ch := range payload.Changes {
		switch ch.Field {
		case "__create__":
			req := payload.Request
			createdID = uuid.New().String()
			if _, err := tx.Exec(`INSERT INTO md_deadlines
				(id, team_id, order_no, title, customer, start_date, due_date, status, progress, priority, owner_id, remark, created_by)
				VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', 0, ?, ?, ?, ?)`,
				createdID, teamID, strings.TrimSpace(req.OrderNo), strings.TrimSpace(req.Title), req.Customer,
				nullDate(req.StartDate), ch.New, req.Priority, userID, req.Remark, userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if err := insertDeadlineEvent(tx, teamID, createdID, "created", "", "", strings.TrimSpace(ch.OrderNo+" "+ch.Title), reason, userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if err := insertDeadlineEvent(tx, teamID, createdID, "proposal_applied", "__create__", "", ch.New, reason, userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		case "due_date", "priority":
			res, err := tx.Exec(`UPDATE md_deadlines SET `+ch.Field+`=?, updated_by=?
				WHERE id=? AND team_id=? AND deleted_at IS NULL AND `+ch.Field+`=?`,
				ch.New, userID, ch.DeadlineID, teamID, ch.Old)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if n, _ := res.RowsAffected(); n != 1 {
				stale("订单数据已变化，请重新预演")
				return
			}
			if err := insertDeadlineEvent(tx, teamID, ch.DeadlineID, "proposal_applied", ch.Field, ch.Old, ch.New, reason, userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			var owner string
			tx.QueryRow(`SELECT COALESCE(owner_id,'') FROM md_deadlines WHERE id=?`, ch.DeadlineID).Scan(&owner)
			if owner != "" && owner != userID {
				notes = append(notes, proposalNote{owner: owner, deadlineID: ch.DeadlineID, line: proposalNotifyLine(ch)})
			}
		}
	}
	if _, err := tx.Exec(`UPDATE md_proposals SET status='applied', decided_by=?, decided_at=NOW(), reason=? WHERE id=?`,
		userID, body.Reason, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, n := range notes {
		if err := createDeadlineNotification(n.owner, teamID, n.deadlineID, n.line+"（"+title+"）"); err != nil {
			log.Printf("proposal %s: notify %s: %v", id, n.owner, err)
		}
	}
	detail, _ := json.Marshal(gin.H{"proposal_id": id, "changes": payload.Changes, "created_id": createdID, "reason": reason})
	fireWebhooks(teamID, "deadline.proposal_applied", "proposal", id, title, string(detail))

	c.JSON(http.StatusOK, gin.H{"ok": true, "created_id": createdID, "applied": len(payload.Changes)})
}

type proposalNote struct{ owner, deadlineID, line string }

func proposalNotifyLine(ch proposalChange) string {
	name := ch.OrderNo
	if name == "" {
		name = ch.Title
	}
	if ch.Field == "priority" {
		return fmt.Sprintf("订单 %s 优先级 %s → %s", name, ch.Old, ch.New)
	}
	return fmt.Sprintf("订单 %s 交期 %s → %s", name, ch.Old, ch.New)
}

// createDeadlineNotification writes an in-app notification the board links
// to (same shape as the reminder scheduler's).
func createDeadlineNotification(userID, teamID, deadlineID, title string) error {
	_, err := database.DB.Exec(`
		INSERT INTO md_notifications (id, user_id, team_id, type, title, document_id, related_id, is_read, created_at)
		VALUES (?, ?, ?, 'deadline', ?, NULL, ?, 0, NOW())`,
		uuid.New().String(), userID, teamID, title, deadlineID)
	return err
}

func nullDate(s string) any {
	if s == "" {
		return nil
	}
	if t, err := time.ParseInLocation(dateLayout, s, time.Local); err == nil {
		return t.Format(dateLayout)
	}
	return nil
}

// TeamRejectProposal POST /teams/:team_id/proposals/:id/reject
//
// The proposer can withdraw their own proposal; editors and admins can
// reject any. Rejections are kept (design §5.1: cancel is recorded too).
func TeamRejectProposal(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	id := c.Param("id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	var owner, status string
	err := database.DB.QueryRow(`SELECT user_id, status FROM md_proposals WHERE id=? AND team_id=?`, id, teamID).Scan(&owner, &status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "提议不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if owner != userID && !roleAtLeast(c, RoleEditor) {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有提议人或编辑者可以驳回", "required_role": RoleEditor})
		return
	}
	res, err := database.DB.Exec(`UPDATE md_proposals SET status='rejected', decided_by=?, decided_at=NOW(), reason=?
		WHERE id=? AND team_id=? AND status='pending'`, userID, body.Reason, id, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n != 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "这条提议已经处理过了", "status": status})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// expirePendingProposals marks pending proposals stale when they can no
// longer be applied: computed on an earlier day, or the open orders or
// capacity changed since (same check as apply, so the history does not show
// 待确认 for something that would be refused).
func expirePendingProposals(teamID string) {
	database.DB.Exec(`UPDATE md_proposals SET status='stale', decided_at=NOW()
		WHERE team_id=? AND status='pending' AND created_at < CURDATE()`, teamID)
	rows, err := database.DB.Query(`SELECT id, baseline FROM md_proposals WHERE team_id=? AND status='pending'`, teamID)
	if err != nil {
		return
	}
	type pend struct{ id, fp string }
	var list []pend
	for rows.Next() {
		var id, text string
		var b proposalBaseline
		if rows.Scan(&id, &text) == nil && json.Unmarshal([]byte(text), &b) == nil {
			list = append(list, pend{id, b.Fingerprint})
		}
	}
	rows.Close()
	if len(list) == 0 {
		return
	}
	orders, err := loadOpenOrders(database.DB, teamID, false)
	if err != nil {
		return
	}
	tc, err := loadTeamCapacity(teamID)
	if err != nil {
		return
	}
	current := scheduleFingerprint(orders, tc.Steps)
	for _, p := range list {
		if p.fp != current {
			markProposalStale(teamID, p.id)
		}
	}
}

// TeamListProposals GET /teams/:team_id/proposals?status=&limit=
//
// History for every member, newest first, rejected and stale included.
// Pending proposals that can no longer apply are marked stale on the way.
func TeamListProposals(c *gin.Context) {
	teamID := getTeamID(c)
	expirePendingProposals(teamID)

	limit := 50
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}
	where := "p.team_id=?"
	args := []any{teamID}
	if s := c.Query("status"); s != "" {
		where += " AND p.status=?"
		args = append(args, s)
	}
	args = append(args, limit)
	rows, err := database.DB.Query(`SELECT p.id, p.kind, p.title, p.payload, p.status, p.user_id, COALESCE(p.decided_by,''),
		COALESCE(p.reason,''), p.created_at, p.decided_at
		FROM md_proposals p WHERE `+where+` ORDER BY p.created_at DESC, p.id LIMIT ?`, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	names := map[string]string{}
	name := func(id string) string {
		if id == "" {
			return ""
		}
		if n, ok := names[id]; ok {
			return n
		}
		names[id] = userDisplayName(id)
		return names[id]
	}
	for rows.Next() {
		var id, kind, title, payloadText, status, userID, decidedBy, reason string
		var created time.Time
		var decided sql.NullTime
		if err := rows.Scan(&id, &kind, &title, &payloadText, &status, &userID, &decidedBy, &reason, &created, &decided); err != nil {
			continue
		}
		var p proposalPayload
		json.Unmarshal([]byte(payloadText), &p)
		item := gin.H{
			"id": id, "kind": kind, "title": title, "status": status,
			"user_id": userID, "user_name": name(userID),
			"decided_by": decidedBy, "decided_by_name": name(decidedBy),
			"reason": reason, "created_at": created,
			"conclusion": p.Result.Conclusion, "breaches": len(p.Result.NewBreaches), "delayed": len(p.Result.Delayed),
			"changes": p.Changes,
		}
		if decided.Valid {
			item["decided_at"] = decided.Time
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}
