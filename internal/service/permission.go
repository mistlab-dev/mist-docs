package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/google/uuid"
)

// ==================== 权限检查 ====================

// CheckPermission returns the permission level for a user on a resource
func CheckPermission(ctx context.Context, userID, userDeptID, resourceType, resourceID string) (string, error) {
	// 1. Check user-level permission
	var perm sql.NullString
	err := database.DB.QueryRowContext(ctx,
		`SELECT permission FROM md_permissions WHERE resource_type=? AND resource_id=? AND target_type='user' AND target_id=?`,
		resourceType, resourceID, userID,
	).Scan(&perm)
	if err == nil && perm.Valid {
		return perm.String, nil
	}

	// 2. Check department-level permission
	err = database.DB.QueryRowContext(ctx,
		`SELECT permission FROM md_permissions WHERE resource_type=? AND resource_id=? AND target_type='department' AND target_id=?`,
		resourceType, resourceID, userDeptID,
	).Scan(&perm)
	if err == nil && perm.Valid {
		return perm.String, nil
	}

	// 3. Check inherited from parent folder
	if resourceType == "document" {
		doc, err := GetDocumentByID(ctx, resourceID)
		if err == nil && doc != nil && doc.FolderID != "" {
			return CheckPermission(ctx, userID, userDeptID, "folder", doc.FolderID)
		}
	} else if resourceType == "folder" {
		folder, err := GetFolderByID(ctx, resourceID)
		if err == nil && folder != nil && folder.ParentID != "" {
			return CheckPermission(ctx, userID, userDeptID, "folder", folder.ParentID)
		}
	}

	// 4. Default: own department = write, others = none
	if resourceType == "folder" {
		folder, err := GetFolderByID(ctx, resourceID)
		if err == nil && folder != nil {
			if folder.DepartmentID == userDeptID {
				return "write", nil
			}
		}
	} else if resourceType == "document" {
		doc, err := GetDocumentByID(ctx, resourceID)
		if err == nil && doc != nil {
			if doc.DepartmentID == userDeptID {
				return "write", nil
			}
		}
	}

	return "none", nil
}

// HasPermission checks if user has at least the required permission level
func HasPermission(ctx context.Context, userID, userDeptID, role, resourceType, resourceID, required string) bool {
	// Super admin bypasses all
	if role == "super_admin" {
		return true
	}

	perm, err := CheckPermission(ctx, userID, userDeptID, resourceType, resourceID)
	if err != nil {
		return false
	}

	levels := map[string]int{"none": 0, "read": 1, "write": 2, "admin": 3}
	return levels[perm] >= levels[required]
}

// ==================== 权限管理 ====================

func ListPermissions(ctx context.Context, resourceType, resourceID string) ([]*model.DocPermission, error) {
	rows, err := database.DB.QueryContext(ctx,
		`SELECT p.id, p.resource_type, p.resource_id, p.target_type, p.target_id, p.permission, p.inherit, p.created_by, p.created_at,
		        CASE WHEN p.target_type='user' THEN u.name ELSE d.name END as target_name
		 FROM md_permissions p
		 LEFT JOIN md_users u ON p.target_type='user' AND p.target_id=u.id
		 LEFT JOIN md_departments d ON p.target_type='department' AND p.target_id=d.id
		 WHERE p.resource_type=? AND p.resource_id=?`,
		resourceType, resourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := []*model.DocPermission{}
	for rows.Next() {
		p := &model.DocPermission{}
		var targetName sql.NullString
		if err := rows.Scan(&p.ID, &p.ResourceType, &p.ResourceID, &p.TargetType, &p.TargetID,
			&p.Permission, &p.Inherit, &p.CreatedBy, &p.CreatedAt, &targetName); err != nil {
			return nil, err
		}
		p.TargetName = ns(targetName)
		perms = append(perms, p)
	}
	return perms, nil
}

func SetPermission(ctx context.Context, p *model.DocPermission, userID string) error {
	p.ID = uuid.New().String()
	p.CreatedBy = userID

	// Validate permission level
	if p.Permission != "read" && p.Permission != "write" && p.Permission != "admin" {
		return fmt.Errorf("权限级别必须是 read/write/admin")
	}

	// Validate target exists
	if p.TargetType == "user" {
		var exists int
		database.DB.QueryRowContext(ctx, `SELECT 1 FROM md_users WHERE id=? AND status=1`, p.TargetID).Scan(&exists)
		if exists == 0 {
			return fmt.Errorf("用户不存在")
		}
	} else if p.TargetType == "department" {
		var exists int
		database.DB.QueryRowContext(ctx, `SELECT 1 FROM md_departments WHERE id=? AND status=1`, p.TargetID).Scan(&exists)
		if exists == 0 {
			return fmt.Errorf("部门不存在")
		}
	} else {
		return fmt.Errorf("target_type 必须是 user 或 department")
	}

	// Upsert
	_, err := database.DB.ExecContext(ctx,
		`INSERT INTO md_permissions (id, resource_type, resource_id, target_type, target_id, permission, inherit, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE permission=?, inherit=?`,
		p.ID, p.ResourceType, p.ResourceID, p.TargetType, p.TargetID, p.Permission, p.Inherit, p.CreatedBy,
		p.Permission, p.Inherit,
	)
	return err
}

func RemovePermission(ctx context.Context, id string) error {
	_, err := database.DB.ExecContext(ctx, `DELETE FROM md_permissions WHERE id=?`, id)
	return err
}

func GetResourceName(ctx context.Context, resourceType, resourceID string) (string, error) {
	if resourceType == "folder" {
		folder, err := GetFolderByID(ctx, resourceID)
		if err != nil || folder == nil {
			return "", errors.New("文件夹不存在")
		}
		return folder.Name, nil
	} else if resourceType == "document" {
		doc, err := GetDocumentByID(ctx, resourceID)
		if err != nil || doc == nil {
			return "", errors.New("文档不存在")
		}
		return doc.Title, nil
	}
	return "", nil
}