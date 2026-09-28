package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/store"
)

// ==================== 文档 ====================

func GetDocumentByID(ctx context.Context, id string) (*model.Document, error) {
	doc := &model.Document{}
	var folderID sql.NullString
	var lockedAt sql.NullTime
	err := database.DB.QueryRowContext(ctx,
		`SELECT id, folder_id, department_id, IFNULL(team_id,''), title, type, file_path, file_size, version, locked_by, locked_at, status, created_by, updated_by, created_at, updated_at
		 FROM md_documents WHERE id = ?`, id,
	).Scan(&doc.ID, &folderID, &doc.DepartmentID, &doc.TeamID, &doc.Title, &doc.Type, &doc.FilePath, &doc.FileSize,
		&doc.Version, &doc.LockedBy, &lockedAt, &doc.Status, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	doc.FolderID = ns(folderID)
	doc.LockedAt = nt(lockedAt)
	return doc, err
}

// ==================== helpers ====================

func ns(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

func nt(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

// PruneDocumentVersions drops version files beyond the configured keep count.
// Called after a team save so history does not grow without a bound.
func PruneDocumentVersions(docID string) {
	cleanOldVersions(docID, "")
}

func cleanOldVersions(docID, deptID string) {
	keep := store.VersionKeep()

	// Find versions to delete
	rows, err := database.DB.QueryContext(context.Background(),
		`SELECT version, file_path FROM md_versions WHERE document_id = ? ORDER BY version DESC LIMIT 1000 OFFSET ?`, docID, keep)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ver int
		var path string
		if rows.Scan(&ver, &path) == nil {
			os.Remove(path) // delete file
			database.DB.ExecContext(context.Background(), `DELETE FROM md_versions WHERE document_id = ? AND version = ?`, docID, ver)
		}
	}
}
