package handler

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/google/uuid"
)

// Version files are append-only: v<N>.dat is written once, when version N is
// created, and never rewritten afterwards. Restoring an old version or saving
// new content always produces a new N. Only current.dat is replaced.

var errVersionConflict = errors.New("文档版本已变化，请重试")

// docBucket returns the storage directory of a document (team, or the legacy
// department for documents created before teams).
func docBucket(docID string) string {
	var teamID, deptID string
	database.DB.QueryRow("SELECT team_id, department_id FROM md_documents WHERE id=?", docID).Scan(&teamID, &deptID)
	if teamID != "" {
		return teamID
	}
	return deptID
}

// saveDocVersion stores content as the next version of docID.
//
// It returns the document's version afterwards and whether anything changed.
// When content equals the current content no version is created, so opening a
// document (which re-saves the same HTML) does not flood the history.
func saveDocVersion(docID, teamID, userID string, content []byte) (int, bool, error) {
	for attempt := 0; attempt < 3; attempt++ {
		var current int
		if err := database.DB.QueryRow(
			"SELECT version FROM md_documents WHERE id=? AND team_id=?", docID, teamID).Scan(&current); err != nil {
			return 0, false, err
		}
		if hasCurrentContent(docID) && bytes.Equal(readDocContent(docID), content) {
			return current, false, nil
		}

		next := current + 1
		// Claim the version number first so two concurrent saves can never
		// write the same v<N>.dat.
		res, err := database.DB.Exec(
			`UPDATE md_documents SET content_text=?, version=?, file_size=?, updated_by=?, updated_at=NOW()
			 WHERE id=? AND team_id=? AND version=?`,
			string(content), next, int64(len(content)), userID, docID, teamID, current)
		if err != nil {
			return 0, false, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue // someone else saved in between; re-read and retry
		}
		if err := writeVersionFile(docID, next, userID, content); err != nil {
			return next, true, err
		}
		service.PruneDocumentVersions(docID)
		return next, true, nil
	}
	return 0, false, errVersionConflict
}

// writeInitialVersion stores the first content of a freshly created document
// as version 1 (the column default), with its history row.
func writeInitialVersion(docID, userID string, content []byte) error {
	database.DB.Exec(`UPDATE md_documents SET content_text=?, file_size=? WHERE id=?`,
		string(content), int64(len(content)), docID)
	return writeVersionFile(docID, 1, userID, content)
}

func writeVersionFile(docID string, version int, userID string, content []byte) error {
	bucket := docBucket(docID)
	path, size, err := store.WriteVersion(bucket, docID, version, content)
	if err != nil {
		return fmt.Errorf("写入版本文件失败: %w", err)
	}
	_, err = database.DB.Exec(
		`INSERT INTO md_versions (id, document_id, version, file_path, file_size, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), docID, version, path, size, userID)
	return err
}

func hasCurrentContent(docID string) bool {
	_, err := store.ReadCurrent(docBucket(docID), docID)
	return err == nil
}
