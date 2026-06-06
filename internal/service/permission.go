package service

// This file is DEPRECATED. All permission logic has moved to team_permission.go.
// Kept only for backward compatibility with legacy handlers during transition.
// Remove after Phase 5 frontend migration is complete.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/c-wind/mist-docs/internal/database"
)

// CheckPermission DEPRECATED — use CheckTeamPermission
func CheckPermission(ctx context.Context, userID, userDeptID, resourceType, resourceID string) (string, error) {
	perm := CheckTeamPermission(ctx, userID, "", "", resourceType, resourceID)
	return perm, nil
}

// HasPermission DEPRECATED — use HasTeamPermission
func HasPermission(ctx context.Context, userID, userDeptID, role, resourceType, resourceID, required string) bool {
	if role == "super_admin" {
		return true
	}
	return HasTeamPermission(ctx, userID, "", role, resourceType, resourceID, required)
}

// SetPermission DEPRECATED
func SetPermission(ctx context.Context, p interface{}, userID string) error {
	return fmt.Errorf("deprecated: use team API")
}

// RemovePermission DEPRECATED
func RemovePermission(ctx context.Context, id string) error {
	_, err := database.DB.ExecContext(ctx, `DELETE FROM md_permissions WHERE id=?`, id)
	return err
}

// GetResourceName DEPRECATED
func GetResourceName(ctx context.Context, resourceType, resourceID string) (string, error) {
	if resourceType == "folder" {
		var name string
		err := database.DB.QueryRowContext(ctx, `SELECT name FROM md_team_folders WHERE id=?`, resourceID).Scan(&name)
		return name, err
	} else if resourceType == "document" {
		var title string
		err := database.DB.QueryRowContext(ctx, `SELECT title FROM md_documents WHERE id=?`, resourceID).Scan(&title)
		return title, err
	}
	return "", nil
}

// ListPermissions DEPRECATED — queries users table instead of md_users
func ListPermissions(ctx context.Context, resourceType, resourceID string) ([]map[string]interface{}, error) {
	rows, err := database.DB.QueryContext(ctx,
		`SELECT p.id, p.target_type, p.target_id, p.permission, p.inherit, p.created_by, p.created_at,
		        IFNULL(u.display_name, '') as target_name
		 FROM md_permissions p
		 LEFT JOIN users u ON p.target_type='user' AND p.target_id=u.id
		 WHERE p.resource_type=? AND p.resource_id=?`,
		resourceType, resourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := []map[string]interface{}{}
	for rows.Next() {
		var id, tType, tID, perm, createdBy, createdAt, targetName string
		var inherit bool
		var nullTarget sql.NullString
		if rows.Scan(&id, &tType, &tID, &perm, &inherit, &createdBy, &createdAt, &nullTarget) != nil {
			continue
		}
		targetName = nullTarget.String
		perms = append(perms, map[string]interface{}{
			"id": id, "target_type": tType, "target_id": tID,
			"permission": perm, "inherit": inherit,
			"created_by": createdBy, "created_at": createdAt,
			"target_name": targetName,
		})
	}
	return perms, nil
}

// ListCollaborators DEPRECATED — use ListTeamCollaborators
func ListCollaborators(ctx context.Context, resourceType, resourceID string) ([]*Collaborator, error) {
	return ListTeamCollaborators(ctx, resourceType, resourceID)
}
func SearchTargets(ctx context.Context, query string, excludeIDs []string) ([]map[string]interface{}, error) {
	return SearchTeamTargets(ctx, "", query)
}

// AddCollaborator DEPRECATED — use AddTeamCollaborator
func AddCollaborator(ctx context.Context, resourceType, resourceID, targetType, targetID, role, userID string) error {
	return AddTeamCollaborator(ctx, resourceType, resourceID, targetType, targetID, role, userID)
}

// UpdateCollaborator DEPRECATED — use UpdateTeamCollaborator
func UpdateCollaborator(ctx context.Context, permID, role string) error {
	return UpdateTeamCollaborator(ctx, permID, role)
}

// RemoveCollaborator DEPRECATED — use RemoveTeamCollaborator
func RemoveCollaborator(ctx context.Context, permID string) error {
	return RemoveTeamCollaborator(ctx, permID)
}

// GetFolderByIDFromTeam returns folder info from md_team_folders
func GetFolderByIDFromTeam(ctx context.Context, id string) (*struct {
	ID       string
	Name     string
	ParentID string
}, error) {
	f := &struct {
		ID       string
		Name     string
		ParentID string
	}{}
	var parentID sql.NullString
	err := database.DB.QueryRowContext(ctx,
		`SELECT id, name, parent_id FROM md_team_folders WHERE id=?`, id,
	).Scan(&f.ID, &f.Name, &parentID)
	f.ParentID = parentID.String
	return f, err
}

func nsPermHelper(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
