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

// ==================== Google Docs 风格协作者管理 ====================

// Collaborator 协作者信息（API 返回用）
type Collaborator struct {
	ID         string `json:"id"`
	TargetType string `json:"target_type"` // user / department
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Avatar     string `json:"avatar,omitempty"`
	Role       string `json:"role"`       // owner / editor / commenter / viewer
	Inherited  bool   `json:"inherited"`   // 是否从父文件夹继承
	InheritFrom string `json:"inherit_from,omitempty"` // 继承来源
}

// permToFrontendRole 数据库 permission → 前端角色名
var permToFrontendRole = map[string]string{"read": "viewer", "write": "editor", "admin": "admin"}

func normalizeRole(role string) string {
	if r, ok := permToFrontendRole[role]; ok {
		return r
	}
	return role
}

// ListCollaborators 列出文档/文件夹的协作者
func ListCollaborators(ctx context.Context, resourceType, resourceID string) ([]*Collaborator, error) {
	// 获取文档/文件夹的 owner
	var ownerID string
	if resourceType == "document" {
		doc, err := GetDocumentByID(ctx, resourceID)
		if err != nil || doc == nil {
			return nil, errors.New("文档不存在")
		}
		ownerID = doc.CreatedBy
	} else {
		folder, err := GetFolderByID(ctx, resourceID)
		if err != nil || folder == nil {
			return nil, errors.New("文件夹不存在")
		}
		ownerID = folder.CreatedBy
	}

	results := []*Collaborator{}

	// 1. Get owner info
	ownerName := ""
	database.DB.QueryRowContext(ctx, `SELECT name FROM md_users WHERE id=?`, ownerID).Scan(&ownerName)
	results = append(results, &Collaborator{
		ID:         "owner",
		TargetType: "user",
		TargetID:   ownerID,
		TargetName: ownerName,
		Role:       "owner",
	})

	// 2. Get direct permissions
	rows, err := database.DB.QueryContext(ctx,
		`SELECT p.id, p.target_type, p.target_id, p.permission,
		        CASE WHEN p.target_type='user' THEN u.name ELSE d.name END as target_name
		 FROM md_permissions p
		 LEFT JOIN md_users u ON p.target_type='user' AND p.target_id=u.id
		 LEFT JOIN md_departments d ON p.target_type='department' AND p.target_id=d.id
		 WHERE p.resource_type=? AND p.resource_id=?
		 ORDER BY p.created_at`,
		resourceType, resourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		c := &Collaborator{}
		var targetName sql.NullString
		if err := rows.Scan(&c.ID, &c.TargetType, &c.TargetID, &c.Role, &targetName); err != nil {
			return nil, err
		}
		c.TargetName = ns(targetName)
		c.Role = normalizeRole(c.Role)
		results = append(results, c)
	}

	// 3. Get inherited permissions from parent folders
	if resourceType == "document" {
		doc, _ := GetDocumentByID(ctx, resourceID)
		if doc != nil && doc.FolderID != "" {
			inherited := getInheritedPerms(ctx, "folder", doc.FolderID)
			for _, ih := range inherited {
				// Skip if already has direct permission
				found := false
				for _, r := range results {
					if r.TargetType == ih.TargetType && r.TargetID == ih.TargetID {
						found = true
						break
					}
				}
				if !found {
					ih.Inherited = true
					ih.InheritFrom = doc.FolderID
					results = append(results, ih)
				}
			}
		}
	}

	return results, nil
}

// getInheritedPerms 递归获取从父文件夹继承的权限
func getInheritedPerms(ctx context.Context, resourceType, resourceID string) []*Collaborator {
	results := []*Collaborator{}

	rows, err := database.DB.QueryContext(ctx,
		`SELECT p.id, p.target_type, p.target_id, p.permission,
		        CASE WHEN p.target_type='user' THEN u.name ELSE d.name END as target_name
		 FROM md_permissions p
		 LEFT JOIN md_users u ON p.target_type='user' AND p.target_id=u.id
		 LEFT JOIN md_departments d ON p.target_type='department' AND p.target_id=d.id
		 WHERE p.resource_type=? AND p.resource_id=? AND p.inherit=1`,
		resourceType, resourceID,
	)
	if err != nil {
		return results
	}
	defer rows.Close()

	for rows.Next() {
		c := &Collaborator{}
		var targetName sql.NullString
		if err := rows.Scan(&c.ID, &c.TargetType, &c.TargetID, &c.Role, &targetName); err != nil {
			continue
		}
		c.TargetName = ns(targetName)
		c.Role = normalizeRole(c.Role)
		c.Inherited = true
		results = append(results, c)
	}

	// 递归查父文件夹
	if resourceType == "folder" {
		folder, err := GetFolderByID(ctx, resourceID)
		if err == nil && folder != nil && folder.ParentID != "" {
			parentPerms := getInheritedPerms(ctx, "folder", folder.ParentID)
			results = append(results, parentPerms...)
		}
	}

	return results
}

// AddCollaborator 添加协作者
func AddCollaborator(ctx context.Context, resourceType, resourceID, targetType, targetID, role, userID string) error {
	// 权限级别映射：前端 Google Docs 角色 -> 数据库 permission
	permMap := map[string]string{"viewer": "read", "editor": "write", "admin": "admin"}
	perm, ok := permMap[role]
	if !ok {
		return fmt.Errorf("无效的角色，必须是 viewer/editor/admin")
	}

	// 验证目标存在
	if targetType == "user" {
		var exists int
		database.DB.QueryRowContext(ctx, `SELECT 1 FROM md_users WHERE id=? AND status=1`, targetID).Scan(&exists)
		if exists == 0 {
			return fmt.Errorf("用户不存在")
		}
	} else if targetType == "department" {
		var exists int
		database.DB.QueryRowContext(ctx, `SELECT 1 FROM md_departments WHERE id=? AND status=1`, targetID).Scan(&exists)
		if exists == 0 {
			return fmt.Errorf("部门不存在")
		}
	} else {
		return fmt.Errorf("target_type 必须是 user 或 department")
	}

	// 不能给自己设权限
	if targetType == "user" && targetID == userID {
		return fmt.Errorf("不能给自己设置权限")
	}

	id := uuid.New().String()
	_, err := database.DB.ExecContext(ctx,
		`INSERT INTO md_permissions (id, resource_type, resource_id, target_type, target_id, permission, inherit, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?)
		 ON DUPLICATE KEY UPDATE permission=VALUES(permission)`,
		id, resourceType, resourceID, targetType, targetID, perm, userID,
	)
	return err
}

// UpdateCollaborator 更新协作者角色
func UpdateCollaborator(ctx context.Context, permID, role string) error {
	permMap := map[string]string{"viewer": "read", "editor": "write", "admin": "admin"}
	perm, ok := permMap[role]
	if !ok {
		return fmt.Errorf("无效的角色")
	}
	_, err := database.DB.ExecContext(ctx, `UPDATE md_permissions SET permission=? WHERE id=?`, perm, permID)
	return err
}

// RemoveCollaborator 移除协作者
func RemoveCollaborator(ctx context.Context, permID string) error {
	_, err := database.DB.ExecContext(ctx, `DELETE FROM md_permissions WHERE id=?`, permID)
	return err
}

// SearchTargets 搜索可添加的用户/部门（用于自动补全）
func SearchTargets(ctx context.Context, query string, excludeIDs []string) ([]map[string]interface{}, error) {
	results := []map[string]interface{}{}

	// 搜索用户
	rows, err := database.DB.QueryContext(ctx,
		`SELECT id, name, username FROM md_users WHERE status=1 AND (name LIKE ? OR username LIKE ?) LIMIT 10`,
		"%"+query+"%", "%"+query+"%",
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, name, username string
			if err := rows.Scan(&id, &name, &username); err != nil {
				continue
			}
			skip := false
			for _, ex := range excludeIDs {
				if ex == id {
					skip = true; break }
			}
			if !skip {
				results = append(results, map[string]interface{}{
					"type": "user", "id": id, "name": name, "username": username,
					"display": name + " (" + username + ")",
				})
			}
		}
	}

	// 搜索部门
	rows2, err := database.DB.QueryContext(ctx,
		`SELECT id, name FROM md_departments WHERE status=1 AND name LIKE ? LIMIT 5`,
		"%"+query+"%",
	)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id, name string
			if err := rows2.Scan(&id, &name); err != nil {
				continue
			}
			results = append(results, map[string]interface{}{
				"type": "department", "id": id, "name": name,
				"display": name + " (部门)",
			})
		}
	}

	return results, nil
}

