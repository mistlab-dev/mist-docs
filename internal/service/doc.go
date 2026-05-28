package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
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
	rows, err := database.DB.QueryContext(ctx,
		`SELECT id, name, parent_id, department_id, created_by, created_at, updated_at
		 FROM md_folders WHERE department_id = ? ORDER BY name`, deptID)
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
	err := database.DB.QueryRowContext(ctx,
		`SELECT id, folder_id, department_id, title, type, file_path, file_size, version, status, created_by, updated_by, created_at, updated_at
		 FROM md_documents WHERE id = ?`, id,
	).Scan(&doc.ID, &folderID, &doc.DepartmentID, &doc.Title, &doc.Type, &doc.FilePath, &doc.FileSize,
		&doc.Version, &doc.Status, &doc.CreatedBy, &doc.UpdatedBy, &doc.CreatedAt, &doc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	doc.FolderID = ns(folderID)
	return doc, err
}

func ListDocuments(ctx context.Context, folderID, deptID, docType string, page, pageSize int) ([]*model.Document, int, error) {
	where := "WHERE d.status = 1"
	args := []interface{}{}

	if folderID != "" {
		where += " AND d.folder_id = ?"
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
	listSQL := `SELECT d.id, d.folder_id, d.department_id, d.title, d.type, d.file_size, d.version, d.status,
	            d.created_by, d.updated_by, d.created_at, d.updated_at,
	            u1.name as creator_name, u2.name as updater_name
	            FROM md_documents d
	            LEFT JOIN md_users u1 ON d.created_by = u1.id
	            LEFT JOIN md_users u2 ON d.updated_by = u2.id ` +
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
		`UPDATE md_documents SET file_path=?, file_size=?, version=?, updated_by=?, updated_at=NOW() WHERE id=?`,
		path, size, newVersion, userID, doc.ID,
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
	listSQL := `SELECT d.id, d.folder_id, d.department_id, d.title, d.type, d.file_size, d.version,
	            d.created_by, d.updated_by, d.created_at, d.updated_at
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
