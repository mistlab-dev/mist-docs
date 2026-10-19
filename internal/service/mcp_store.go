package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/google/uuid"
)

// MCPStore implements the mcp.Store interface against the shared database so
// the MCP endpoint reads exactly the same rows, permission checks and content
// bucket layout as the REST API. No MCP-specific caching or access rules.
type MCPStore struct{}

// SearchDocuments matches keyword against title and body, newest first,
// filtering out any documents the user cannot read.
func (MCPStore) SearchDocuments(teamID, userID, keyword string, limit int) ([]MCPStoreHit, error) {
	return mcpSearch(teamID, userID, keyword, limit)
}

// RecentDocuments lists recently updated documents of a team that the user can read.
func (MCPStore) RecentDocuments(teamID, userID string, limit int) ([]MCPStoreMeta, error) {
	return mcpRecent(teamID, userID, limit)
}

// ReadDocument returns the document body plus metadata.
func (MCPStore) ReadDocument(teamID, docID string) (*MCPStoreContent, bool, error) {
	return mcpRead(teamID, docID)
}

// CreateDocument inserts a new document and returns its id.
func (MCPStore) CreateDocument(teamID, title, folderID, content, userID string) (string, error) {
	return mcpCreate(teamID, title, folderID, content, userID)
}

// CanCreate checks if the user has editor/admin role in teamID and has not exceeded document limit.
func (MCPStore) CanCreate(teamID, userID string) (bool, string, error) {
	var role string
	err := database.DB.QueryRow(
		`SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return false, "not a member of team " + teamID, nil
	}
	if err != nil {
		return false, "", err
	}
	switch role {
	case "owner", "admin", "editor", "member":
		// allowed role
	default:
		return false, "editor or admin role required to create documents", nil
	}

	var docCount int
	if err := database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents WHERE status = 1 AND team_id = ?`, teamID,
	).Scan(&docCount); err != nil {
		return false, "", err
	}
	limits := GetPlanLimits(userID)
	if limits.MaxDocuments > 0 && docCount >= limits.MaxDocuments {
		return false, "document limit reached (" + strconv.Itoa(limits.MaxDocuments) + ")", nil
	}
	return true, "", nil
}

// CanRead mirrors the REST requireDoc permission check through the same
// service.CheckTeamPermission path.
func (MCPStore) CanRead(teamID, docID, userID string) (bool, error) {
	var role string
	err := database.DB.QueryRow(
		`SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var n int
	if err := database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents WHERE id=? AND team_id=? AND status=1`, docID, teamID,
	).Scan(&n); err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	return HasTeamPermission(context.Background(), userID, teamID, role, "document", docID, "read"), nil
}

// IsMember reports whether the user belongs to the team.
func (MCPStore) IsMember(teamID, userID string) (bool, error) {
	var n int
	err := database.DB.QueryRow(
		`SELECT COUNT(*) FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ErrDocLocked is returned by UpdateDocument when another user holds the
// document lock. The MCP layer maps it onto a friendlier message.
var ErrDocLocked = errors.New("document is locked by another user")

// CanWrite mirrors the REST requireDoc(docID, "write") gate: the document must
// belong to teamID, be active, and the user's ACL (or team role fallback) must
// reach write level. The string explains a denial to the agent.
func (MCPStore) CanWrite(teamID, docID, userID string) (bool, string, error) {
	var role string
	err := database.DB.QueryRow(
		`SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return false, "not a member of team " + teamID, nil
	}
	if err != nil {
		return false, "", err
	}

	var n int
	if err := database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents WHERE id=? AND team_id=? AND status=1`,
		docID, teamID,
	).Scan(&n); err != nil {
		return false, "", err
	}
	if n == 0 {
		// Same wording as read_doc so MCP leaks nothing about existence.
		return false, "document not found or not writable", nil
	}
	if !HasTeamPermission(context.Background(), userID, teamID, role, "document", docID, "write") {
		return false, "write permission required (owner/admin/editor role, or an explicit write grant)", nil
	}
	return true, "", nil
}

// UpdateDocument writes title and/or content to docID through the same
// version-bucket path the REST PUT handlers use: content is stored as the next
// version with an md_versions history row, title-only edits leave content
// untouched. It returns the document's new version (or current version for a
// title-only edit).
func (MCPStore) UpdateDocument(teamID, docID, title, content string, hasTitle, hasContent bool, userID string) (int, error) {
	// Someone else's lock blocks the write, mirroring lockedAgainst() in the
	// REST handler (MCP has no admin override: the agent is not a team admin
	// session, and admin overrides go through the UI).
	var lockedBy string
	database.DB.QueryRow(
		`SELECT IFNULL(locked_by,'') FROM md_documents WHERE id=? AND team_id=?`,
		docID, teamID).Scan(&lockedBy)
	if lockedBy != "" && lockedBy != userID {
		return 0, ErrDocLocked
	}

	if hasTitle {
		if _, err := database.DB.Exec(
			`UPDATE md_documents SET title=?, updated_by=?, updated_at=NOW()
			 WHERE id=? AND team_id=? AND status=1`,
			title, userID, docID, teamID); err != nil {
			return 0, err
		}
	}
	if !hasContent {
		var version int
		err := database.DB.QueryRow(
			`SELECT version FROM md_documents WHERE id=? AND team_id=?`, docID, teamID,
		).Scan(&version)
		return version, err
	}

	// Retry loop identical to handler.saveDocVersion: claim the version number
	// with a conditional UPDATE so concurrent saves never collide on v<N>.dat.
	for attempt := 0; attempt < 3; attempt++ {
		var current int
		if err := database.DB.QueryRow(
			`SELECT version FROM md_documents WHERE id=? AND team_id=?`, docID, teamID,
		).Scan(&current); err != nil {
			return 0, err
		}
		if mcpCurrentContentEquals(docID, content) {
			return current, nil
		}
		next := current + 1
		res, err := database.DB.Exec(
			`UPDATE md_documents SET content_text=?, version=?, file_size=?, updated_by=?, updated_at=NOW()
			 WHERE id=? AND team_id=? AND version=?`,
			content, next, int64(len(content)), userID, docID, teamID, current)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		path, size, err := store.WriteVersion(teamID, docID, next, []byte(content))
		if err != nil {
			return next, fmt.Errorf("write version file: %w", err)
		}
		// md_versions.file_path is NOT NULL (strict mode): mirror REST
		// writeVersionFile — write the real path from WriteVersion and surface
		// the INSERT error instead of swallowing it, otherwise a failed history
		// row leaves content_text/version ahead of history while MCP reports OK.
		if _, err := database.DB.Exec(
			`INSERT INTO md_versions (id, document_id, version, file_path, file_size, created_by)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), docID, next, path, size, userID); err != nil {
			return next, fmt.Errorf("insert version history row: %w", err)
		}
		PruneDocumentVersions(docID)
		return next, nil
	}
	return 0, errors.New("version conflict, please retry")
}

// IsLockConflict reports whether err came from a locked document.
func IsLockConflict(err error) bool { return errors.Is(err, ErrDocLocked) }

func mcpCurrentContentEquals(docID, content string) bool {
	body, err := store.ReadCurrent(docBucketOf(docID), docID)
	return err == nil && string(body) == content
}

// docBucketOf mirrors handler.docBucket (team bucket, legacy department
// fallback) without importing the handler package.
func docBucketOf(docID string) string {
	var teamID, deptID string
	database.DB.QueryRow(
		`SELECT team_id, department_id FROM md_documents WHERE id=?`, docID).Scan(&teamID, &deptID)
	if teamID != "" {
		return teamID
	}
	return deptID
}

// ListDocuments returns one page of team documents filtered by folder/type/tag,
// newest first, with the total across all filters. Every row passes the same
// per-document ACL check the REST list applies, so unreadable rows never appear.
func (MCPStore) ListDocuments(teamID, userID, folderID, docType, tag string, limit, offset int) ([]MCPStoreMeta, int, error) {
	var role string
	if err := database.DB.QueryRow(
		`SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&role); err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, fmt.Errorf("not a member of team %s", teamID)
		}
		return nil, 0, err
	}

	where := "WHERE d.team_id=? AND d.status=1"
	args := []interface{}{teamID}
	if folderID != "" {
		where += " AND d.folder_id=?"
		args = append(args, folderID)
	}
	if docType != "" {
		where += " AND d.type=?"
		args = append(args, docType)
	}
	if tag != "" {
		where += ` AND EXISTS (SELECT 1 FROM md_doc_tags dt JOIN md_tags t ON dt.tag_id=t.id
			WHERE dt.document_id=d.id AND t.name=?)`
		args = append(args, tag)
	}

	var total int
	if err := database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents d `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch extra rows so per-document ACL filtering can drop some while the
	// page still fills up; callers clamp limit so this stays bounded.
	fetchLimit := limit * 3
	if fetchLimit < limit+50 {
		fetchLimit = limit + 50
	}
	query := `SELECT d.id, d.title, d.type, IFNULL(d.folder_id,''), d.version, d.updated_at
		FROM md_documents d ` + where + ` ORDER BY d.updated_at DESC LIMIT ? OFFSET ?`
	rows, err := database.DB.Query(query, append(args, fetchLimit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	ctx := context.Background()
	var out []MCPStoreMeta
	for rows.Next() {
		var d MCPStoreMeta
		if err := rows.Scan(&d.ID, &d.Title, &d.Type, &d.FolderID, &d.Version, &d.UpdatedAt); err != nil {
			continue
		}
		if !HasTeamPermission(ctx, userID, teamID, role, "document", d.ID, "read") {
			continue
		}
		out = append(out, d)
		if len(out) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []MCPStoreMeta{}
	}
	return out, total, rows.Err()
}

// SearchDocContent is the paragraph search behind GET /docs/search: match the
// keyword in title or body, return a plain-text excerpt window around the hit.
// HTML is stripped (content_text stores editor HTML) and every row is
// ACL-filtered, so an agent only ever sees documents it may read.
func (MCPStore) SearchDocContent(teamID, userID, keyword string, limit int) ([]MCPStoreHit, error) {
	var role string
	if err := database.DB.QueryRow(
		`SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&role); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not a member of team %s", teamID)
		}
		return nil, err
	}

	pattern := "%" + keyword + "%"
	rows, err := database.DB.Query(
		`SELECT id, title, IFNULL(content_text,'') FROM md_documents
		 WHERE team_id=? AND status=1 AND (title LIKE ? OR content_text LIKE ?)
		 ORDER BY updated_at DESC LIMIT ?`,
		teamID, pattern, pattern, limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ctx := context.Background()
	var out []MCPStoreHit
	for rows.Next() {
		var d MCPStoreHit
		var body string
		if err := rows.Scan(&d.ID, &d.Title, &body); err != nil {
			continue
		}
		if !HasTeamPermission(ctx, userID, teamID, role, "document", d.ID, "read") {
			continue
		}
		d.Excerpt = mcpExcerpt(body, keyword, 200)
		out = append(out, d)
		if len(out) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []MCPStoreHit{}
	}
	return out, nil
}

// --- excerpt helpers (plain-text windows over stored HTML) ---

var (
	mcpScriptRe = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)
	mcpStyleRe  = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`)
	mcpTagRe    = regexp.MustCompile(`<[^>]*>`)
)

// mcpExcerpt strips script/style blocks and tags, decodes entities, then
// returns a rune-safe window centred on the first case-insensitive hit. It
// never returns HTML: the caller prints it as text.
func mcpExcerpt(body, keyword string, width int) string {
	clean := mcpTagRe.ReplaceAllString(mcpStyleRe.ReplaceAllString(
		mcpScriptRe.ReplaceAllString(body, " "), " "), " ")
	clean = collapseSpace(html.UnescapeString(clean))
	if clean == "" {
		return ""
	}
	idx := foldIndex(clean, keyword)
	if idx < 0 {
		if len([]rune(clean)) > width {
			r := []rune(clean)
			return string(r[:width]) + "…"
		}
		return clean
	}
	hit := len([]rune(clean[:idx]))
	total := len([]rune(clean))
	start := hit - width/2
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > total {
		end = total
	}
	r := []rune(clean)
	s := strings.Join(strings.Fields(string(r[start:end])), " ")
	if start > 0 {
		s = "…" + s
	}
	if end < total {
		s += "…"
	}
	return s
}

// foldIndex returns the byte offset of the first case-insensitive match of
// needle in body, or -1, comparing on rune sequences so multi-byte text never
// misaligns.
func foldIndex(body, needle string) int {
	if needle == "" {
		return 0
	}
	needles := []rune(strings.ToLower(needle))
	bodyRunes := []rune(body)
	for i := 0; i+len(needles) <= len(bodyRunes); i++ {
		match := true
		for j := range needles {
			if unicode.ToLower(bodyRunes[i+j]) != needles[j] {
				match = false
				break
			}
		}
		if match {
			return len(string(bodyRunes[:i]))
		}
	}
	return -1
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// --- internal row helpers ---

func mcpSearch(teamID, userID, keyword string, limit int) ([]MCPStoreHit, error) {
	var role string
	err := database.DB.QueryRow(
		`SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&role)
	if err != nil {
		return nil, err
	}

	pattern := "%" + keyword + "%"
	fetchLimit := limit * 3
	if fetchLimit < 50 {
		fetchLimit = 50
	}
	rows, err := database.DB.Query(
		`SELECT id, title, IFNULL(content_text, '') FROM md_documents
		 WHERE team_id=? AND status=1 AND (title LIKE ? OR content_text LIKE ?)
		 ORDER BY updated_at DESC LIMIT ?`, teamID, pattern, pattern, fetchLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ctx := context.Background()
	var out []MCPStoreHit
	for rows.Next() {
		var d MCPStoreHit
		var body string
		if err := rows.Scan(&d.ID, &d.Title, &body); err != nil {
			continue
		}
		if !HasTeamPermission(ctx, userID, teamID, role, "document", d.ID, "read") {
			continue
		}
		d.Excerpt = excerptAround(body, keyword, 200)
		out = append(out, d)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

func mcpRecent(teamID, userID string, limit int) ([]MCPStoreMeta, error) {
	var role string
	err := database.DB.QueryRow(
		`SELECT role FROM team_members WHERE team_id=? AND user_id=?`, teamID, userID,
	).Scan(&role)
	if err != nil {
		return nil, err
	}

	fetchLimit := limit * 3
	if fetchLimit < 50 {
		fetchLimit = 50
	}
	rows, err := database.DB.Query(
		`SELECT id, title, type, updated_at FROM md_documents
		 WHERE team_id=? AND status=1 ORDER BY updated_at DESC LIMIT ?`, teamID, fetchLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ctx := context.Background()
	var out []MCPStoreMeta
	for rows.Next() {
		var d MCPStoreMeta
		if err := rows.Scan(&d.ID, &d.Title, &d.Type, &d.UpdatedAt); err != nil {
			continue
		}
		if !HasTeamPermission(ctx, userID, teamID, role, "document", d.ID, "read") {
			continue
		}
		out = append(out, d)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

func mcpRead(teamID, docID string) (*MCPStoreContent, bool, error) {
	var d MCPStoreContent
	err := database.DB.QueryRow(
		`SELECT id, title, type, version, updated_at FROM md_documents
		 WHERE id=? AND team_id=? AND status=1`, docID, teamID,
	).Scan(&d.ID, &d.Title, &d.Type, &d.Version, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	// Content lives in the version bucket, same as the REST read path.
	var deptID string
	database.DB.QueryRow(`SELECT IFNULL(department_id,'') FROM md_documents WHERE id=?`, docID).Scan(&deptID)
	bucket := teamID
	if bucket == "" {
		bucket = deptID
	}
	body, err := store.ReadCurrent(bucket, docID)
	if err != nil {
		// Fall back to the cached text column if the blob store is unavailable.
		var cached string
		if e := database.DB.QueryRow(`SELECT IFNULL(content_text,'') FROM md_documents WHERE id=?`, docID).Scan(&cached); e == nil {
			d.Content = cached
			return &d, true, nil
		}
		return nil, false, err
	}
	d.Content = string(body)
	return &d, true, nil
}

func mcpCreate(teamID, title, folderID, content, userID string) (string, error) {
	docID := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_documents (id, team_id, folder_id, department_id, title, type, status, created_by, updated_by)
		 VALUES (?, ?, ?, '', ?, 'doc', 1, ?, ?)`,
		docID, teamID, folderID, title, userID, userID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(content) != "" {
		// Same initial-version path the REST create handler uses: content_text,
		// then version file + history row with file_path (NOT NULL in schema).
		database.DB.Exec(`UPDATE md_documents SET content_text=?, file_size=? WHERE id=?`,
			content, len(content), docID)
		path, size, err := store.WriteVersion(teamID, docID, 1, []byte(content))
		if err == nil {
			if _, err := database.DB.Exec(
				`INSERT INTO md_versions (id, document_id, version, file_path, file_size, created_by)
				 VALUES (?, ?, 1, ?, ?, ?)`,
				uuid.New().String(), docID, path, size, userID); err != nil {
				return "", fmt.Errorf("insert version history row: %w", err)
			}
		}
	}
	return docID, nil
}

// excerptAround returns a window of body centred on the first keyword hit.
func excerptAround(body, keyword string, width int) string {
	lowerBody := strings.ToLower(body)
	lowerKw := strings.ToLower(keyword)
	idx := strings.Index(lowerBody, lowerKw)
	if idx < 0 {
		if len(body) > width {
			return body[:width] + "…"
		}
		return body
	}
	start := idx - width/2
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > len(body) {
		end = len(body)
	}
	return strings.ReplaceAll(strings.TrimSpace(body[start:end]), "\n", " ")
}

// Hit/Meta/Content alias the handler-facing shapes without importing mcp here,
// keeping service independent of the protocol package.
type (
	MCPStoreHit    = MCPHit
	MCPStoreMeta   = MCPMeta
	MCPStoreContent = MCPContent
)

// MCPHit is one search result row.
type MCPHit struct {
	ID      string
	Title   string
	Excerpt string
}

// MCPMeta is document metadata without the body.
type MCPMeta struct {
	ID        string
	Title     string
	Type      string
	FolderID  string
	Version   int
	UpdatedAt string
}

// MCPContent is a full document row plus body.
type MCPContent struct {
	ID        string
	Title     string
	Type      string
	Version   int
	UpdatedAt string
	Content   string
}

var _ = time.Now
