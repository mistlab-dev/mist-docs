package tests

// #79 T5: 合规时间线（md_audits ∪ term 汇入事件）。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
)

// ingestEvent 造一条 term 事件（走真 ingest 端点，和 #77 集成测试同路），
// 并断言通知确实落库（ingest 是静默 200，单测里拿到 false 成功也该炸）。
func ingestEvent(t *testing.T, token, event, title string) {
	t.Helper()
	w := request("POST", teamPath("/events/ingest"),
		map[string]any{"event": event, "title": title}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("ingest %s: %d %s", event, w.Code, w.Body.String())
	}
	var out struct {
		Recipients int `json:"recipients"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Recipients < 1 {
		t.Fatalf("ingest %s: recipients=0, event never landed: %s", event, w.Body.String())
	}
}

// createAuditRow 直插一条审计（不触发 webhook），保证 term 臂断言不混入
// audit 臂噪音。
func createAuditRow(t *testing.T, userID, userName, action, resType, resID, resName string) {
	t.Helper()
	_, err := database.DB.Exec(
		`INSERT INTO md_audits (id, team_id, user_id, user_name, actor_type, action, resource_type, resource_id, resource_name, detail, created_at)
		 VALUES (UUID(), ?, ?, ?, 'user', ?, ?, ?, ?, '{}', NOW())`,
		teamID, userID, userName, action, resType, resID, resName)
	if err != nil {
		t.Fatalf("insert audit: %v", err)
	}
}

// 成员（非 admin）403 —— 时间线是 admin only。
func TestTimelineRequiresAdmin(t *testing.T) {
	for name, tok := range map[string]string{"editor": editorToken, "viewer": viewerToken} {
		w := request("GET", teamPath("/timeline"), nil, tok)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s must be 403, got %d %s", name, w.Code, w.Body.String())
		}
	}
}

// 核心验收：文档审计 + term 事件出现在同一条时间线。
func TestTimelineMergesAuditsAndTermEvents(t *testing.T) {
	createAuditRow(t, adminID, "管理员", "update_doc", "document", "test-doc-t1", "时间线测试文档")
	token := createIngestAgent(t)
	ingestEvent(t, token, "term.attention", "[ingest-bot] 长任务完成: backup.sh")
	ingestEvent(t, token, "audit.alert", "[ingest-bot] 检测到危险命令: rm -rf")

	w := request("GET", teamPath("/timeline"), nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("timeline: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []struct {
			Source  string `json:"source"`
			Event   string `json:"event"`
			Actor   string `json:"actor_name"`
			ResName string `json:"resource_name"`
		} `json:"data"`
		Total        int    `json:"total"`
		CoverageNote string `json:"coverage_note"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sawDoc, sawAttention, sawAlert := false, false, false
	for _, e := range out.Data {
		if e.Source == "docs" && e.Event == "update_doc" && e.ResName == "时间线测试文档" {
			sawDoc = true
		}
		if e.Source == "term" && e.Event == "term.attention" && strings.Contains(e.ResName, "backup.sh") {
			sawAttention = true
		}
		if e.Source == "term" && e.Event == "audit.alert" && strings.Contains(e.ResName, "rm -rf") {
			sawAlert = true
		}
	}
	if !sawDoc || !sawAttention || !sawAlert {
		t.Fatalf("timeline incomplete: doc=%v attention=%v alert=%v\ndata=%s", sawDoc, sawAttention, sawAlert, w.Body.String())
	}
	if out.CoverageNote == "" {
		t.Fatal("coverage_note must be present (覆盖边界对用户可见)")
	}
}

// 通知按团队成员扇出，时间线必须去重：同一事件只出现一次。
func TestTimelineDedupsTermFanout(t *testing.T) {
	token := createIngestAgent(t)
	ingestEvent(t, token, "term.attention", "[dedup-bot] 唯一事件标记xyz")

	w := request("GET", teamPath("/timeline?source=term"), nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("timeline: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []struct {
			Event string `json:"event"`
			Title string `json:"resource_name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	n := 0
	for _, e := range out.Data {
		if strings.Contains(e.Title, "唯一事件标记xyz") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("term event must appear exactly once after dedup, got %d: %s", n, w.Body.String())
	}
}

// source/actor 过滤：docs 只出审计臂，term 只出事件臂；actor=user 时
// term 臂为空。
func TestTimelineSourceActorFilters(t *testing.T) {
	createAuditRow(t, adminID, "管理员", "import_doc", "document", "test-doc-t2", "过滤测试文档")
	token := createIngestAgent(t)
	ingestEvent(t, token, "term.attention", "[filter-bot] 过滤测试事件")

	// source=docs
	w := request("GET", teamPath("/timeline?source=docs"), nil, adminToken)
	var docsOut struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &docsOut)
	for _, e := range docsOut.Data {
		if e["source"] != "docs" {
			t.Fatalf("source=docs leaked term rows: %v", e)
		}
	}

	// source=term
	w = request("GET", teamPath("/timeline?source=term"), nil, adminToken)
	var termOut struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &termOut)
	for _, e := range termOut.Data {
		if e["source"] != "term" {
			t.Fatalf("source=term leaked audit rows: %v", e)
		}
	}

	// actor=user → 零 term 行
	w = request("GET", teamPath("/timeline?actor=user"), nil, adminToken)
	var userOut struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &userOut)
	for _, e := range userOut.Data {
		if e["source"] == "term" {
			t.Fatalf("actor=user must exclude term rows: %v", e)
		}
	}
}

// 导出：csv 带表头、json 带 count。
func TestTimelineExportCSVAndJSON(t *testing.T) {
	token := createIngestAgent(t)
	ingestEvent(t, token, "term.attention", "[export-bot] 导出测试事件")

	w := request("GET", teamPath("/timeline/export?format=json"), nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("json export: %d %s", w.Code, w.Body.String())
	}
	var jout struct {
		Count int              `json:"count"`
		Data  []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &jout)
	if jout.Count < 1 {
		t.Fatalf("json export empty: %s", w.Body.String())
	}

	w = request("GET", teamPath("/timeline/export"), nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("csv export: %d %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("csv content-type: %s", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "时间") || !strings.Contains(body, "导出测试事件") {
		t.Fatalf("csv missing header/event rows: %q", body[:min(200, len(body))])
	}
}

// 非法 format 400。
func TestTimelineExportBadFormat(t *testing.T) {
	w := request("GET", teamPath("/timeline/export?format=xml"), nil, adminToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad format must be 400, got %d", w.Code)
	}
}

var _ = time.Now // 保留 import 位置
