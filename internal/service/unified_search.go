package service

// Unified cross-product search (#78 T3 MVP).
//
// One endpoint, three sections:
//   - docs:     md_documents title/body match, per-doc permission filtered
//   - term:     ingested MistTerm events (term.attention / audit.alert,
//     #77) — the structured "execution/session summary" source; no PTY
//     byte indexing by design (上游 MistTerm#156 不硬等)
//   - audits:   md_audits action/resource/detail match — admin + plan
//     gated; a caller without the right sees the section omitted, not an
//     empty array ("无权限的源不返回")
//
// Degradation: a failing source yields an "error" field on that section
// instead of failing the whole request ("部分源失败降级").

import (
	"context"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
)

type UnifiedSearchHit struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Excerpt  string `json:"excerpt,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	DocID    string `json:"document_id,omitempty"`
	At       string `json:"at,omitempty"`
}

type UnifiedSearchSection struct {
	Items []UnifiedSearchHit `json:"items"`
	Error string             `json:"error,omitempty"`
}

type UnifiedSearchResult struct {
	Query  string               `json:"query"`
	Docs   UnifiedSearchSection `json:"docs"`
	Term   UnifiedSearchSection `json:"term"`
	Audits *UnifiedSearchSection `json:"audits,omitempty"`
}

func excerptOf(s, keyword string, width int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	i := strings.Index(strings.ToLower(s), strings.ToLower(keyword))
	if i < 0 {
		if len(s) > width {
			return s[:width]
		}
		return s
	}
	start := i - width/4
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

// searchDocsSection mirrors the paragraph-search candidate pool: fetch
// extra rows because per-doc ACL filtering drops some.
func searchDocsSection(ctx context.Context, teamID, userID, role, keyword string, limit int) UnifiedSearchSection {
	sec := UnifiedSearchSection{Items: []UnifiedSearchHit{}}
	pattern := "%" + keyword + "%"
	rows, err := database.DB.QueryContext(ctx,
		`SELECT d.id, d.title, IFNULL(d.content_text,'') FROM md_documents d
		 WHERE d.team_id=? AND d.status=1 AND (d.title LIKE ? OR d.content_text LIKE ?)
		 ORDER BY d.updated_at DESC LIMIT ?`, teamID, pattern, pattern, limit*3)
	if err != nil {
		sec.Error = "docs source failed"
		return sec
	}
	defer rows.Close()
	for rows.Next() {
		var id, title, body string
		if rows.Scan(&id, &title, &body) != nil {
			continue
		}
		if !PermAtLeast(CheckTeamPermission(ctx, userID, teamID, role, "document", id), "read") {
			continue
		}
		sec.Items = append(sec.Items, UnifiedSearchHit{
			ID: id, Title: title, Excerpt: excerptOf(body, keyword, 200),
		})
		if len(sec.Items) >= limit {
			break
		}
	}
	return sec
}

// searchTermSection reads ingested cross-product events (#77) for the
// team. Inbox rows are per-recipient; the team-wide view comes from the
// audit-style filter on distinct titles, but MVP keeps it simple: match
// any row (every member gets the same fan-out, so dedupe by related_id).
func searchTermSection(ctx context.Context, teamID, keyword string, limit int) UnifiedSearchSection {
	sec := UnifiedSearchSection{Items: []UnifiedSearchHit{}}
	pattern := "%" + keyword + "%"
	rows, err := database.DB.QueryContext(ctx,
		`SELECT id, title, IFNULL(document_id,''), IFNULL(related_id,''), created_at
		 FROM md_notifications
		 WHERE team_id=? AND type IN ('term.attention','audit.alert')
		   AND (title LIKE ? OR IFNULL(related_id,'') LIKE ?)
		 ORDER BY created_at DESC LIMIT ?`, teamID, pattern, pattern, limit*3)
	if err != nil {
		sec.Error = "term source failed"
		return sec
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var id, title, docID, relatedID, at string
		if rows.Scan(&id, &title, &docID, &relatedID, &at) != nil {
			continue
		}
		if seen[title] {
			continue // same fan-out row per member; keep first occurrence
		}
		seen[title] = true
		sec.Items = append(sec.Items, UnifiedSearchHit{
			ID: id, Title: title, DocID: docID, Subtitle: relatedID, At: at,
		})
		if len(sec.Items) >= limit {
			break
		}
	}
	return sec
}

// searchAuditsSection is admin-only (matches TeamListAudits gating); the
// caller passes canSee=false to omit the section entirely.
func searchAuditsSection(ctx context.Context, teamID, keyword string, limit int) UnifiedSearchSection {
	sec := UnifiedSearchSection{Items: []UnifiedSearchHit{}}
	pattern := "%" + keyword + "%"
	rows, err := database.DB.QueryContext(ctx,
		`SELECT a.id, a.action, IFNULL(a.resource_name,''), IFNULL(a.detail,''), a.created_at, a.user_id
		 FROM md_audits a
		 WHERE a.team_id=? AND (a.action LIKE ? OR IFNULL(a.resource_name,'') LIKE ? OR IFNULL(a.detail,'') LIKE ?)
		 ORDER BY a.created_at DESC LIMIT ?`, teamID, pattern, pattern, pattern, limit)
	if err != nil {
		sec.Error = "audit source failed"
		return sec
	}
	defer rows.Close()
	for rows.Next() {
		var id, action, resName, detail, at, userID string
		if rows.Scan(&id, &action, &resName, &detail, &at, &userID) != nil {
			continue
		}
		sub := resName
		if sub == "" {
			sub = excerptOf(detail, keyword, 120)
		}
		sec.Items = append(sec.Items, UnifiedSearchHit{
			ID: id, Title: action, Subtitle: sub, At: at,
		})
		if len(sec.Items) >= limit {
			break
		}
	}
	return sec
}

// UnifiedSearch runs all sections. docsPerms must be the caller's team
// role for per-doc ACL; canSeeAudits gates the audit section (admin only,
// same as TeamListAudits: plan feature + role checked by the handler).
func UnifiedSearch(ctx context.Context, teamID, userID, role, keyword string, limit int, canSeeAudits bool) *UnifiedSearchResult {
	res := &UnifiedSearchResult{Query: keyword}
	res.Docs = searchDocsSection(ctx, teamID, userID, role, keyword, limit)
	res.Term = searchTermSection(ctx, teamID, keyword, limit)
	if canSeeAudits {
		s := searchAuditsSection(ctx, teamID, keyword, limit)
		res.Audits = &s
	}
	return res
}
