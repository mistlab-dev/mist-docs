package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/google/uuid"
)

// ==================== 权限检查 ====================

// CheckTeamPermission returns the permission level for a user on a resource within a team.
// Layer 1: Team role (admin → full access)
// Layer 2: Direct permission on document/folder
// Layer 3: Inherited from parent folder (md_team_folders)
// Layer 4: Default — team member = read, editor = write
func CheckTeamPermission(ctx context.Context, userID, teamID, teamRole, resourceType, resourceID string) string {
	// Layer 1: team admin → full
	if teamRole == "admin" {
		return "admin"
	}

	// Layer 2: direct permission on resource
	var perm sql.NullString
	database.DB.QueryRowContext(ctx,
		`SELECT permission FROM md_permissions
		 WHERE resource_type=? AND resource_id=? AND target_type='user' AND target_id=?`,
		resourceType, resourceID, userID,
	).Scan(&perm)
	if perm.Valid && perm.String != "" {
		return perm.String
	}

	// Layer 3: inherited from parent folder
	if resourceType == "document" {
		var folderID string
		database.DB.QueryRowContext(ctx,
			`SELECT folder_id FROM md_documents WHERE id=?`, resourceID,
		).Scan(&folderID)
		if folderID != "" {
			return checkFolderPermissionRecursive(ctx, userID, folderID)
		}
	} else if resourceType == "folder" {
		var parentID string
		database.DB.QueryRowContext(ctx,
			`SELECT parent_id FROM md_team_folders WHERE id=?`, resourceID,
		).Scan(&parentID)
		if parentID != "" {
			return checkFolderPermissionRecursive(ctx, userID, parentID)
		}
	}

	// Layer 4: default by team role
	if teamRole == "editor" {
		return "write"
	}
	if teamRole == "viewer" {
		return "read"
	}
	return "none"
}

func checkFolderPermissionRecursive(ctx context.Context, userID, folderID string) string {
	// Check this folder
	var perm sql.NullString
	database.DB.QueryRowContext(ctx,
		`SELECT permission FROM md_permissions
		 WHERE resource_type='folder' AND resource_id=? AND target_type='user' AND target_id=?`,
		folderID, userID,
	).Scan(&perm)
	if perm.Valid && perm.String != "" {
		return perm.String
	}
	// Recurse up
	var parentID string
	database.DB.QueryRowContext(ctx,
		`SELECT parent_id FROM md_team_folders WHERE id=?`, folderID,
	).Scan(&parentID)
	if parentID != "" {
		return checkFolderPermissionRecursive(ctx, userID, parentID)
	}
	return ""
}

// HasTeamPermission checks if user has at least the required permission
func HasTeamPermission(ctx context.Context, userID, teamID, teamRole, resourceType, resourceID, required string) bool {
	perm := CheckTeamPermission(ctx, userID, teamID, teamRole, resourceType, resourceID)
	levels := map[string]int{"none": 0, "read": 1, "comment": 2, "write": 3, "admin": 4}
	return levels[perm] >= levels[required]
}

// ==================== Google Docs 风格协作者管理 ====================

type Collaborator struct {
	ID           string `json:"id"`
	TargetType   string `json:"target_type"`
	TargetID     string `json:"target_id"`
	TargetName   string `json:"target_name"`
	Role         string `json:"role"`
	Inherited    bool   `json:"inherited"`
	InheritFrom  string `json:"inherit_from,omitempty"`
}

var permToFrontendRole = map[string]string{"read": "viewer", "comment": "commenter", "write": "editor", "admin": "admin"}

func normalizeRole(role string) string {
	if r, ok := permToFrontendRole[role]; ok {
		return r
	}
	return role
}

// ListTeamCollaborators lists collaborators for a team resource
func ListTeamCollaborators(ctx context.Context, resourceType, resourceID string) ([]*Collaborator, error) {
	results := []*Collaborator{}

	// Get owner
	var ownerID string
	if resourceType == "document" {
		database.DB.QueryRowContext(ctx, `SELECT created_by FROM md_documents WHERE id=?`, resourceID).Scan(&ownerID)
	} else {
		database.DB.QueryRowContext(ctx, `SELECT created_by FROM md_team_folders WHERE id=?`, resourceID).Scan(&ownerID)
	}
	if ownerID != "" {
		var name string
		database.DB.QueryRowContext(ctx, `SELECT display_name FROM users WHERE id=?`, ownerID).Scan(&name)
		results = append(results, &Collaborator{
			ID: "owner", TargetType: "user", TargetID: ownerID,
			TargetName: name, Role: "owner",
		})
	}

	// Direct permissions
	rows, err := database.DB.QueryContext(ctx,
		`SELECT p.id, p.target_type, p.target_id, p.permission,
		        IFNULL(u.display_name, '') as target_name
		 FROM md_permissions p
		 LEFT JOIN users u ON p.target_type='user' AND p.target_id=u.id
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
		if rows.Scan(&c.ID, &c.TargetType, &c.TargetID, &c.Role, &targetName) != nil {
			continue
		}
		c.TargetName = targetName.String
		c.Role = normalizeRole(c.Role)
		results = append(results, c)
	}

	// Inherited from parent folder
	if resourceType == "document" {
		var folderID string
		database.DB.QueryRowContext(ctx, `SELECT folder_id FROM md_documents WHERE id=?`, resourceID).Scan(&folderID)
		if folderID != "" {
			inherited := getInheritedPerms(ctx, "folder", folderID)
			for _, ih := range inherited {
				found := false
				for _, r := range results {
					if r.TargetType == ih.TargetType && r.TargetID == ih.TargetID {
						found = true
						break
					}
				}
				if !found {
					ih.Inherited = true
					ih.InheritFrom = folderID
					results = append(results, ih)
				}
			}
		}
	}

	return results, nil
}

func getInheritedPerms(ctx context.Context, resourceType, resourceID string) []*Collaborator {
	results := []*Collaborator{}

	rows, err := database.DB.QueryContext(ctx,
		`SELECT p.id, p.target_type, p.target_id, p.permission,
		        IFNULL(u.display_name, '') as target_name
		 FROM md_permissions p
		 LEFT JOIN users u ON p.target_type='user' AND p.target_id=u.id
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
		if rows.Scan(&c.ID, &c.TargetType, &c.TargetID, &c.Role, &targetName) != nil {
			continue
		}
		c.TargetName = targetName.String
		c.Role = normalizeRole(c.Role)
		c.Inherited = true
		results = append(results, c)
	}

	// Recurse up
	var parentID string
	database.DB.QueryRowContext(ctx, `SELECT parent_id FROM md_team_folders WHERE id=?`, resourceID).Scan(&parentID)
	if parentID != "" {
		parentPerms := getInheritedPerms(ctx, "folder", parentID)
		results = append(results, parentPerms...)
	}

	return results
}

// AddTeamCollaborator adds a collaborator to a resource
func AddTeamCollaborator(ctx context.Context, resourceType, resourceID, targetType, targetID, role, userID string) error {
	permMap := map[string]string{"viewer": "read", "commenter": "comment", "editor": "write", "admin": "admin"}
	perm, ok := permMap[role]
	if !ok {
		return fmt.Errorf("无效的角色，必须是 viewer/commenter/editor/admin")
	}

	if targetType == "user" {
		var exists int
		database.DB.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id=?`, targetID).Scan(&exists)
		if exists == 0 {
			return fmt.Errorf("用户不存在")
		}
	} else {
		return fmt.Errorf("target_type 目前仅支持 user")
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

// UpdateTeamCollaborator updates collaborator role
func UpdateTeamCollaborator(ctx context.Context, permID, role string) error {
	permMap := map[string]string{"viewer": "read", "commenter": "comment", "editor": "write", "admin": "admin"}
	perm, ok := permMap[role]
	if !ok {
		return fmt.Errorf("无效的角色")
	}
	_, err := database.DB.ExecContext(ctx, `UPDATE md_permissions SET permission=? WHERE id=?`, perm, permID)
	return err
}

// RemoveTeamCollaborator removes a collaborator
func RemoveTeamCollaborator(ctx context.Context, permID string) error {
	_, err := database.DB.ExecContext(ctx, `DELETE FROM md_permissions WHERE id=?`, permID)
	return err
}

// SearchTeamTargets searches users for autocomplete (within a team)
func SearchTeamTargets(ctx context.Context, teamID, query string) ([]map[string]interface{}, error) {
	results := []map[string]interface{}{}

	rows, err := database.DB.QueryContext(ctx,
		`SELECT u.id, u.display_name, u.username
		 FROM users u
		 JOIN team_members tm ON u.id = tm.user_id
		 WHERE tm.team_id = ? AND (u.display_name LIKE ? OR u.username LIKE ?)
		 LIMIT 10`,
		teamID, "%"+query+"%", "%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, username string
		if rows.Scan(&id, &name, &username) != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"type": "user", "id": id, "name": name, "username": username,
			"display": name + " (" + username + ")",
		})
	}

	return results, nil
}
