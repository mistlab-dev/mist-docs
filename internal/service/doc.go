package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/google/uuid"
)

// ==================== 文件夹 ====================

func CreateFolder(ctx context.Context, f *model.DocFolder, userID string) error {
	f.ID = uuid.New().String()
	f.CreatedBy = userID

	_, err := database.DB.ExecContext(ctx,
		`INSERT INTO md_folders (id, name, parent_id, department_id, created_by) VALUES (?, ?, ?, ?, ?)`,
		f.ID, f.Name, nullStr(f.ParentID), f.DepartmentID, f.CreatedBy,
	)
	return err
}

func GetFolderByID(ctx context.Context, id string) (*model.DocFolder, error) {
	f := &model.DocFolder{}
	var parentID sql.NullString
	err := database.DB.QueryRowContext(ctx,
		`SELECT id, name, parent_id, department_id, created_by, created_at, updated_at FROM md_folders WHERE id = ?`, id,
	).Scan(&f.ID, &f.Name, &parentID, &f.DepartmentID, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	f.ParentID = ns(parentID)
	return f, err
}

func UpdateFolder(ctx context.Context, f *model.DocFolder) error {
	_, err := database.DB.ExecContext(ctx,
		`UPDATE md_folders SET name=?, updated_at=NOW() WHERE id=?`, f.Name, f.ID,
	)
	return err
}

func DeleteFolder(ctx context.Context, id string) error {
	// Check for children
	var count int
	database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM md_documents WHERE folder_id = ? AND status = 1`, id).Scan(&count)
	if count > 0 {
		return fmt.Errorf("文件夹内还有文档，无法删除")
	}
	database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM md_folders WHERE parent_id = ?`, id).Scan(&count)
	if count > 0 {
		return fmt.Errorf("文件夹内还有子文件夹，无法删除")
	}

	_, err := database.DB.ExecContext(ctx, `DELETE FROM md_folders WHERE id=?`, id)
	return err
}

func GetFolderTree(ctx context.Context, deptID string) ([]*model.DocFolder, error) {
	q := `SELECT id, name, parent_id, department_id, created_by, created_at, updated_at
		 FROM md_folders `
	var args []interface{}
	if deptID != "" {
		q += ` WHERE department_id = ? `
		args = append(args, deptID)
	}
	q += ` ORDER BY name`
	rows, err := database.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	all := []*model.DocFolder{}
	for rows.Next() {
		f := &model.DocFolder{}
		var parentID sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &parentID, &f.DepartmentID, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		f.ParentID = ns(parentID)
		all = append(all, f)
	}

	// build tree
	nodeMap := map[string]*model.DocFolder{}
	for _, f := range all {
		f.Children = []*model.DocFolder{}
		nodeMap[f.ID] = f
	}

	// 统计每个文件夹的文档数
	if len(all) > 0 {
		ids := make([]string, len(all))
		for i, f := range all {
			ids[i] = f.ID
		}
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		args2 := make([]interface{}, len(ids))
		for i, id := range ids {
			args2[i] = id
		}
		countRows, err := database.DB.QueryContext(ctx,
			`SELECT folder_id, COUNT(*) FROM md_documents WHERE folder_id IN (`+placeholders+`) AND status=1 GROUP BY folder_id`,
			args2...,
		)
		if err == nil {
			defer countRows.Close()
			for countRows.Next() {
				var folderID string
				var cnt int
				if countRows.Scan(&folderID, &cnt) == nil {
					if f, ok := nodeMap[folderID]; ok {
						f.DocCount = cnt
					}
				}
			}
		}
	}

	roots := []*model.DocFolder{}
	for _, f := range all {
		if f.ParentID == "" {
			roots = append(roots, f)
		} else if parent, ok := nodeMap[f.ParentID]; ok {
			parent.Children = append(parent.Children, f)
		}
	}
	return roots, nil
}

// ==================== 文档 ====================

func CreateDocument(ctx context.Context, doc *model.Document, initialContent []byte, userID string) error {
	doc.ID = uuid.New().String()
	doc.Version = 1
	doc.Status = 1
	doc.CreatedBy = userID
	doc.UpdatedBy = userID
	doc.UpdatedAt = time.Now()
	doc.CreatedAt = time.Now()

	if doc.Type == "" {
		doc.Type = "doc"
	}

	// Write file
	path, size, err := store.WriteVersion(doc.DepartmentID, doc.ID, 1, initialContent)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	doc.FilePath = path
	doc.FileSize = size

	// Insert document
	_, err = database.DB.ExecContext(ctx,
		`INSERT INTO md_documents (id, folder_id, department_id, title, type, file_path, file_size, version, status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, nullStr(doc.FolderID), doc.DepartmentID, doc.Title, doc.Type, doc.FilePath, doc.FileSize, doc.Version, doc.Status, doc.CreatedBy, doc.UpdatedBy,
	)
	if err != nil {
		return err
	}

	// Insert version record
	_, err = database.DB.ExecContext(ctx,
		`INSERT INTO md_versions (id, document_id, version, file_path, file_size, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), doc.ID, 1, path, size, userID,
	)
	return err
}

func GetDocumentByID(ctx context.Context, id string) (*model.Document, error) {
	doc := &model.Document{}
	var folderID sql.NullString
	var lockedAt sql.NullTime
	err := database.DB.QueryRowContext(ctx,
		`SELECT id, folder_id, department_id, title, type, file_path, file_size, version, locked_by, locked_at, status, created_by, updated_by, created_at, updated_at
		 FROM md_documents WHERE id = ?`, id,
	).Scan(&doc.ID, &folderID, &doc.DepartmentID, &doc.Title, &doc.Type, &doc.FilePath, &doc.FileSize,
		&doc.Version, &doc.LockedBy, &lockedAt, &doc.Status, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	doc.FolderID = ns(folderID)
	doc.LockedAt = nt(lockedAt)
	return doc, err
}

func nt(t sql.NullTime) *time.Time {
	if t.Valid { return &t.Time }
	return nil
}

func ListDocuments(ctx context.Context, folderID, deptID, docType string, page, pageSize int) ([]*model.Document, int, error) {
	where := "WHERE d.status = 1"
	args := []interface{}{}

	if folderID != "" {
		where += " AND IFNULL(d.folder_id,'') = ?"
		args = append(args, folderID)
	}
	if deptID != "" {
		where += " AND d.department_id = ?"
		args = append(args, deptID)
	}
	if docType != "" {
		where += " AND d.type = ?"
		args = append(args, docType)
	}

	var total int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM md_documents d "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	listSQL := `SELECT d.id, IFNULL(d.folder_id,''), d.department_id, d.title, d.type, d.file_size, d.version, d.status,
	            IFNULL(d.created_by,''), IFNULL(d.updated_by,''), d.created_at, d.updated_at,
	            u1.name as creator_name, u2.name as updater_name
	            FROM md_documents d
	            LEFT JOIN md_users u1 ON IFNULL(d.created_by,'') = u1.id
	            LEFT JOIN md_users u2 ON IFNULL(d.updated_by,'') = u2.id ` +
		where + " ORDER BY d.updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := database.DB.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	docs := []*model.Document{}
	for rows.Next() {
		doc := &model.Document{}
		var folderID, creator, updater sql.NullString
		if err := rows.Scan(&doc.ID, &folderID, &doc.DepartmentID, &doc.Title, &doc.Type,
			&doc.FileSize, &doc.Version, &doc.Status, &doc.CreatedBy, &doc.UpdatedBy,
			&doc.CreatedAt, &doc.UpdatedAt, &creator, &updater); err != nil {
			return nil, 0, err
		}
		doc.FolderID = ns(folderID)
		doc.CreatedByName = ns(creator)
		doc.UpdatedByName = ns(updater)
		docs = append(docs, doc)
	}
	return docs, total, nil
}

func UpdateDocument(ctx context.Context, doc *model.Document) error {
	_, err := database.DB.ExecContext(ctx,
		`UPDATE md_documents SET title=?, folder_id=?, updated_at=NOW() WHERE id=?`,
		doc.Title, nullStr(doc.FolderID), doc.ID,
	)
	return err
}

func SaveDocumentContent(ctx context.Context, docID string, content []byte, userID string) (*model.Document, error) {
	doc, err := GetDocumentByID(ctx, docID)
	if err != nil || doc == nil {
		return nil, fmt.Errorf("文档不存在")
	}

	newVersion := doc.Version + 1

	// Write new version file
	path, size, err := store.WriteVersion(doc.DepartmentID, doc.ID, newVersion, content)
	if err != nil {
		return nil, err
	}

	// Insert version record
	_, err = database.DB.ExecContext(ctx,
		`INSERT INTO md_versions (id, document_id, version, file_path, file_size, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), doc.ID, newVersion, path, size, userID,
	)
	if err != nil {
		return nil, err
	}

	// Update document
	_, err = database.DB.ExecContext(ctx,
		`UPDATE md_documents SET file_path=?, file_size=?, version=?, updated_by=?, updated_at=NOW(), content_text=? WHERE id=?`,
		path, size, newVersion, userID, stripHTMLTags(string(content)), doc.ID,
	)
	if err != nil {
		return nil, err
	}

	doc.Version = newVersion
	doc.FilePath = path
	doc.FileSize = size
	doc.UpdatedBy = userID

	// Clean old versions
	go cleanOldVersions(doc.ID, doc.DepartmentID)

	return doc, nil
}

func GetDocumentContent(ctx context.Context, docID string) ([]byte, *model.Document, error) {
	doc, err := GetDocumentByID(ctx, docID)
	if err != nil || doc == nil {
		return nil, nil, fmt.Errorf("文档不存在")
	}

	data, err := store.ReadCurrent(doc.DepartmentID, doc.ID)
	if err != nil {
		return nil, doc, err
	}
	return data, doc, nil
}

func DeleteDocument(ctx context.Context, id string) error {
	doc, err := GetDocumentByID(ctx, id)
	if err != nil || doc == nil {
		return fmt.Errorf("文档不存在")
	}

	// Move files to trash
	if err := store.MoveToTrash(doc.DepartmentID, doc.ID); err != nil {
		return err
	}

	// Soft delete
	_, err = database.DB.ExecContext(ctx,
		`UPDATE md_documents SET status=0, updated_at=NOW() WHERE id=?`, id)
	return err
}

func RestoreDocument(ctx context.Context, id string) error {
	doc, err := GetDocumentByID(ctx, id)
	if err != nil || doc == nil {
		return fmt.Errorf("文档不存在")
	}

	if err := store.RestoreFromTrash(doc.DepartmentID, doc.ID); err != nil {
		return err
	}

	_, err = database.DB.ExecContext(ctx,
		`UPDATE md_documents SET status=1, updated_at=NOW() WHERE id=?`, id)
	return err
}

func PurgeDocument(ctx context.Context, id string) error {
	doc, err := GetDocumentByID(ctx, id)
	if err != nil || doc == nil {
		return fmt.Errorf("文档不存在")
	}

	// Delete versions from DB
	database.DB.ExecContext(ctx, `DELETE FROM md_versions WHERE document_id=?`, id)
	// Delete document
	database.DB.ExecContext(ctx, `DELETE FROM md_documents WHERE id=?`, id)
	// Delete files
	return store.PurgeFromTrash(doc.ID)
}

func EmptyTrash(ctx context.Context, deptID string) (int, error) {
	q := `SELECT id FROM md_documents WHERE status = 0`
	var args []interface{}
	if deptID != "" {
		q += ` AND department_id = ?`
		args = append(args, deptID)
	}
	rows, err := database.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	count := 0
	for _, id := range ids {
		if err := PurgeDocument(ctx, id); err == nil {
			count++
		}
	}
	return count, nil
}

// ==================== 版本 ====================

func ListVersions(ctx context.Context, docID string) ([]*model.DocVersion, error) {
	rows, err := database.DB.QueryContext(ctx,
		`SELECT v.id, v.document_id, v.version, v.file_size, v.created_by, v.created_at, u.name as creator_name
		 FROM md_versions v LEFT JOIN md_users u ON v.created_by = u.id
		 WHERE v.document_id = ? ORDER BY v.version DESC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := []*model.DocVersion{}
	for rows.Next() {
		v := &model.DocVersion{}
		var creator sql.NullString
		if err := rows.Scan(&v.ID, &v.DocumentID, &v.Version, &v.FileSize, &v.CreatedBy, &v.CreatedAt, &creator); err != nil {
			return nil, err
		}
		v.CreatedByName = ns(creator)
		versions = append(versions, v)
	}
	return versions, nil
}

func RestoreVersion(ctx context.Context, docID string, version int, userID string) error {
	doc, err := GetDocumentByID(ctx, docID)
	if err != nil || doc == nil {
		return fmt.Errorf("文档不存在")
	}

	// Read the old version
	data, err := store.ReadVersion(doc.DepartmentID, doc.ID, version)
	if err != nil {
		return fmt.Errorf("读取版本 %d 失败: %w", version, err)
	}

	// Save as new version
	_, err = SaveDocumentContent(ctx, docID, data, userID)
	return err
}

// GetVersionContent returns decrypted content of a specific version
func GetVersionContent(ctx context.Context, docID, version string) ([]byte, string, error) {
	doc, err := GetDocumentByID(ctx, docID)
	if err != nil || doc == nil {
		return nil, "", fmt.Errorf("文档不存在")
	}
	ver := 0
	fmt.Sscanf(version, "%d", &ver)
	data, err := store.ReadVersion(doc.DepartmentID, doc.ID, ver)
	if err != nil {
		return nil, "", fmt.Errorf("读取版本失败: %w", err)
	}
	return data, "text/html", nil
}

// ==================== 回收站 ====================

func ListTrash(ctx context.Context, deptID string, page, pageSize int) ([]*model.Document, int, error) {
	where := "WHERE d.status = 0"
	args := []interface{}{}
	if deptID != "" {
		where += " AND d.department_id = ?"
		args = append(args, deptID)
	}

	var total int
	database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM md_documents d "+where, args...).Scan(&total)

	offset := (page - 1) * pageSize
	listSQL := `SELECT d.id, IFNULL(d.folder_id,''), d.department_id, d.title, d.type, d.file_size, d.version,
	            IFNULL(d.created_by,''), IFNULL(d.updated_by,''), d.created_at, d.updated_at
	            FROM md_documents d ` + where + " ORDER BY d.updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := database.DB.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	docs := []*model.Document{}
	for rows.Next() {
		doc := &model.Document{}
		var folderID sql.NullString
		rows.Scan(&doc.ID, &folderID, &doc.DepartmentID, &doc.Title, &doc.Type, &doc.FileSize, &doc.Version,
			&doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt)
		doc.FolderID = ns(folderID)
		docs = append(docs, doc)
	}
	return docs, total, nil
}

// ==================== helpers ====================

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

// ==================== 搜索 ====================

func SearchDocuments(ctx context.Context, keyword, deptID string, page, pageSize int) ([]*model.Document, int, error) {
	return SearchDocumentsWithTags(ctx, keyword, deptID, "", page, pageSize)
}

// SearchDocumentsWithTags searches documents by keyword, dept, and optional tag filter.
func SearchDocumentsWithTags(ctx context.Context, keyword, deptID, tagID string, page, pageSize int) ([]*model.Document, int, error) {
	// First pass: search by title + creator + content_text (full-text)
	where := "WHERE d.status = 1 AND (d.title LIKE ?"
	args := []interface{}{"%" + keyword + "%"}

	where += " OR d.created_by IN (SELECT id FROM md_users WHERE name LIKE ?)"
	args = append(args, "%"+keyword+"%")

	// Full-text search on content
	where += " OR d.content_text LIKE ?"
	args = append(args, "%"+keyword+"%")

	where += ")"

	if deptID != "" {
		where += " AND d.department_id = ?"
		args = append(args, deptID)
	}
	if tagID != "" {
		where += " AND d.id IN (SELECT dt.document_id FROM md_doc_tags dt WHERE dt.tag_id = ?)"
		args = append(args, tagID)
	}

	var total int
	database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM md_documents d "+where, args...).Scan(&total)

	offset := (page - 1) * pageSize
	query := `SELECT d.id, IFNULL(d.folder_id,''), d.department_id, d.title, d.type, d.file_size, d.version, IFNULL(d.created_by,''), IFNULL(d.updated_by,''), d.created_at, d.updated_at
		FROM md_documents d ` + where + ` ORDER BY d.updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := database.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var docs []*model.Document
	for rows.Next() {
		doc := &model.Document{}
		if err := rows.Scan(&doc.ID, &doc.FolderID, &doc.DepartmentID, &doc.Title, &doc.Type, &doc.FileSize, &doc.Version, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	// Second pass: search document content
	foundIDs := map[string]bool{}
	for _, d := range docs {
		foundIDs[d.ID] = true
	}

	contentHits, err := searchDocumentContent(ctx, keyword, deptID, tagID)
	if err == nil {
		for _, doc := range contentHits {
			if !foundIDs[doc.ID] {
				docs = append(docs, doc)
				foundIDs[doc.ID] = true
			}
		}
		total = len(docs)
	}

	return docs, total, nil
}

// searchDocumentContent searches decrypted document content for a keyword
func searchDocumentContent(ctx context.Context, keyword, deptID, tagID string) ([]*model.Document, error) {
	where := "WHERE status = 1 AND type = 'doc'"
	args := []interface{}{}
	if deptID != "" {
		where += " AND department_id = ?"
		args = append(args, deptID)
	}
	where += " ORDER BY updated_at DESC LIMIT 50"

	if tagID != "" {
		where += `
		AND id IN (SELECT dt.document_id FROM md_doc_tags dt WHERE dt.tag_id = ?)`
		args = append(args, tagID)
	}

	rows, err := database.DB.QueryContext(ctx, "SELECT id, IFNULL(folder_id,''), department_id, title, type, file_size, version, IFNULL(created_by,''), IFNULL(updated_by,''), created_at, updated_at FROM md_documents "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lowerKW := strings.ToLower(keyword)
	var results []*model.Document

	for rows.Next() {
		doc := &model.Document{}
		if err := rows.Scan(&doc.ID, &doc.FolderID, &doc.DepartmentID, &doc.Title, &doc.Type, &doc.FileSize, &doc.Version, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			continue
		}

		// Read and decrypt content
		content, _, err := GetDocumentContent(ctx, doc.ID)
		if err != nil || len(content) == 0 {
			continue
		}

		// Strip HTML tags for text search
		plain := stripHTMLTags(string(content))
		if strings.Contains(strings.ToLower(plain), lowerKW) {
			results = append(results, doc)
		}
	}

	return results, nil
}

// stripHTMLTags removes HTML tags for plain text search
// StripHTMLTags exports stripHTMLTags for handler use.
func StripHTMLTags(s string) string { return stripHTMLTags(s) }

func stripHTMLTags(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// ==================== 最近文档 ====================

func RecentDocuments(ctx context.Context, userID string, limit int) ([]*model.Document, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := database.DB.QueryContext(ctx, `
		SELECT d.id, IFNULL(d.folder_id,''), d.department_id, d.title, d.type, d.file_size, d.version,
		       IFNULL(d.created_by,''), IFNULL(d.updated_by,''), d.created_at, d.updated_at,
		       u1.name as creator_name, u2.name as updater_name
		FROM md_documents d
		INNER JOIN md_audits a ON a.resource_id = d.id AND a.user_id = ? AND a.action = 'view'
		LEFT JOIN md_users u1 ON IFNULL(d.created_by,'') = u1.id
		LEFT JOIN md_users u2 ON IFNULL(d.updated_by,'') = u2.id
		WHERE d.status = 1
		GROUP BY d.id
		ORDER BY MAX(a.created_at) DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*model.Document
	for rows.Next() {
		doc := &model.Document{}
		var creator, updater sql.NullString
		if err := rows.Scan(&doc.ID, &doc.FolderID, &doc.DepartmentID, &doc.Title, &doc.Type, &doc.FileSize, &doc.Version, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt, &creator, &updater); err != nil {
			continue
		}
		doc.CreatedByName = ns(creator)
		doc.UpdatedByName = ns(updater)
		docs = append(docs, doc)
	}
	return docs, nil
}

// ==================== 收藏 ====================

func AddFavorite(ctx context.Context, userID, docID string) error {
	id := uuid.New().String()
	_, err := database.DB.ExecContext(ctx,
		"INSERT IGNORE INTO md_favorites (id, user_id, document_id) VALUES (?, ?, ?)",
		id, userID, docID)
	return err
}

func RemoveFavorite(ctx context.Context, userID, docID string) error {
	_, err := database.DB.ExecContext(ctx,
		"DELETE FROM md_favorites WHERE user_id = ? AND document_id = ?",
		userID, docID)
	return err
}

func ListFavorites(ctx context.Context, userID string) ([]*model.Document, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT d.id, IFNULL(d.folder_id,''), d.department_id, d.title, d.type, d.file_size, d.version,
		       IFNULL(d.created_by,''), IFNULL(d.updated_by,''), d.created_at, d.updated_at,
		       u1.name as creator_name, u2.name as updater_name
		FROM md_favorites f
		INNER JOIN md_documents d ON d.id = f.document_id AND d.status = 1
		LEFT JOIN md_users u1 ON IFNULL(d.created_by,'') = u1.id
		LEFT JOIN md_users u2 ON IFNULL(d.updated_by,'') = u2.id
		WHERE f.user_id = ?
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*model.Document
	for rows.Next() {
		doc := &model.Document{}
		var creator, updater sql.NullString
		if err := rows.Scan(&doc.ID, &doc.FolderID, &doc.DepartmentID, &doc.Title, &doc.Type, &doc.FileSize, &doc.Version, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt, &creator, &updater); err != nil {
			continue
		}
		doc.CreatedByName = ns(creator)
		doc.UpdatedByName = ns(updater)
		docs = append(docs, doc)
	}
	return docs, nil
}

func IsFavorite(ctx context.Context, userID, docID string) bool {
	var count int
	database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM md_favorites WHERE user_id = ? AND document_id = ?",
		userID, docID).Scan(&count)
	return count > 0
}
