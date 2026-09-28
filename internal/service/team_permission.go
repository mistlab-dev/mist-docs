package service

import (
	"context"
	"database/sql"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
)

// ==================== 权限检查 ====================

// CheckTeamPermission returns the permission level for a user on a resource within a team.
// Layer 1: Team role (admin/owner → full access)
// Layer 2: Direct permission on document/folder (explicit share wins, including a downgrade)
// Layer 3: Inherited from parent folder, only when a folder ACL actually exists
// Layer 4: Default — viewer is read-only; other team roles can write
func CheckTeamPermission(ctx context.Context, userID, teamID, teamRole, resourceType, resourceID string) string {
	if teamRole == "admin" || teamRole == "owner" {
		return "admin"
	}

	var perm sql.NullString
	database.DB.QueryRowContext(ctx,
		`SELECT permission FROM md_permissions
		 WHERE resource_type=? AND resource_id=? AND target_type='user' AND target_id=?`,
		resourceType, resourceID, userID,
	).Scan(&perm)
	if perm.Valid && perm.String != "" {
		if canon, ok := NormalizePermission(perm.String); ok {
			return canon
		}
	}

	if resourceType == "document" {
		var folderID string
		database.DB.QueryRowContext(ctx,
			`SELECT IFNULL(folder_id, '') FROM md_documents WHERE id=?`, resourceID,
		).Scan(&folderID)
		if folderID != "" {
			if inherited := checkFolderPermissionRecursive(ctx, userID, folderID); inherited != "" {
				return inherited
			}
		}
	} else if resourceType == "folder" {
		var parentID string
		database.DB.QueryRowContext(ctx,
			`SELECT parent_id FROM md_team_folders WHERE id=?`, resourceID,
		).Scan(&parentID)
		if parentID != "" {
			if inherited := checkFolderPermissionRecursive(ctx, userID, parentID); inherited != "" {
				return inherited
			}
		}
	}

	return defaultTeamPerm(teamRole)
}

// defaultTeamPerm is the team-role fallback used when no document or folder ACL applies.
// An empty folder must not wipe this fallback — that used to return "" and lock editors out.
func defaultTeamPerm(teamRole string) string {
	switch teamRole {
	case "admin", "owner":
		return "admin"
	case "viewer":
		return "read"
	case "":
		return "none"
	default:
		return "write"
	}
}

// NormalizePermission maps UI roles and stored ACL values onto read/comment/write/admin.
func NormalizePermission(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "read", "viewer":
		return "read", true
	case "comment", "commenter":
		return "comment", true
	case "write", "editor":
		return "write", true
	case "admin":
		return "admin", true
	default:
		return "", false
	}
}

// PermAtLeast reports whether have is at least the required level.
func PermAtLeast(have, need string) bool {
	levels := map[string]int{"": 0, "none": 0, "read": 1, "comment": 2, "write": 3, "admin": 4}
	return levels[have] >= levels[need]
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
		if canon, ok := NormalizePermission(perm.String); ok {
			return canon
		}
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
	return PermAtLeast(CheckTeamPermission(ctx, userID, teamID, teamRole, resourceType, resourceID), required)
}

// ==================== Google Docs 风格协作者管理 ====================

type Collaborator struct {
	ID          string `json:"id"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	TargetName  string `json:"target_name"`
	Role        string `json:"role"`
	Inherited   bool   `json:"inherited"`
	InheritFrom string `json:"inherit_from,omitempty"`
}

var permToFrontendRole = map[string]string{"read": "viewer", "comment": "commenter", "write": "editor", "admin": "admin"}

func normalizeRole(role string) string {
	if canon, ok := NormalizePermission(role); ok {
		role = canon
	}
	if r, ok := permToFrontendRole[role]; ok {
		return r
	}
	return role
}

// FrontendRole is the share-dialog role (viewer/editor/admin) for a stored ACL value.
func FrontendRole(stored string) string {
	return normalizeRole(stored)
}
