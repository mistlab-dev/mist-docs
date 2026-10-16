package service

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/google/uuid"
)

// MCPStore implements the mcp.Store interface against the shared database so
// the MCP endpoint reads exactly the same rows, permission checks and content
// bucket layout as the REST API. No MCP-specific caching or access rules.
type MCPStore struct{}

// SearchDocuments matches keyword against title and body, newest first.
func (MCPStore) SearchDocuments(teamID, keyword string, limit int) ([]MCPStoreHit, error) {
	return mcpSearch(teamID, keyword, limit)
}

// RecentDocuments lists recently updated documents of a team.
func (MCPStore) RecentDocuments(teamID string, limit int) ([]MCPStoreMeta, error) {
	return mcpRecent(teamID, limit)
}

// ReadDocument returns the document body plus metadata.
func (MCPStore) ReadDocument(teamID, docID string) (*MCPStoreContent, bool, error) {
	return mcpRead(teamID, docID)
}

// CreateDocument inserts a new document and returns its id.
func (MCPStore) CreateDocument(teamID, title, folderID, content, userID string) (string, error) {
	return mcpCreate(teamID, title, folderID, content, userID)
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

// --- internal row helpers ---

func mcpSearch(teamID, keyword string, limit int) ([]MCPStoreHit, error) {
	pattern := "%" + keyword + "%"
	rows, err := database.DB.Query(
		`SELECT id, title, IFNULL(content_text, '') FROM md_documents
		 WHERE team_id=? AND status=1 AND (title LIKE ? OR content_text LIKE ?)
		 ORDER BY updated_at DESC LIMIT ?`, teamID, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MCPStoreHit
	for rows.Next() {
		var d MCPStoreHit
		var body string
		if err := rows.Scan(&d.ID, &d.Title, &body); err != nil {
			continue
		}
		d.Excerpt = excerptAround(body, keyword, 200)
		out = append(out, d)
	}
	return out, rows.Err()
}

func mcpRecent(teamID string, limit int) ([]MCPStoreMeta, error) {
	rows, err := database.DB.Query(
		`SELECT id, title, type, updated_at FROM md_documents
		 WHERE team_id=? AND status=1 ORDER BY updated_at DESC LIMIT ?`, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MCPStoreMeta
	for rows.Next() {
		var d MCPStoreMeta
		if err := rows.Scan(&d.ID, &d.Title, &d.Type, &d.UpdatedAt); err != nil {
			continue
		}
		out = append(out, d)
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
		// Same initial-version path the REST create handler uses.
		database.DB.Exec(`UPDATE md_documents SET content_text=?, file_size=? WHERE id=?`,
			content, len(content), docID)
		if _, _, err := store.WriteVersion(teamID, docID, 1, []byte(content)); err == nil {
			database.DB.Exec(
				`INSERT INTO md_versions (id, document_id, version, file_size, created_by) VALUES (?, ?, 1, ?, ?)`,
				uuid.New().String(), docID, len(content), userID)
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
