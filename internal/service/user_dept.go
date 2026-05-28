package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ==================== helpers ====================

func ns(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// ==================== 用户 ====================

func CreateUser(ctx context.Context, u *model.User, rawPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	u.ID = uuid.New().String()
	u.Password = string(hash)
	if u.Role == "" {
		u.Role = "member"
	}
	u.Status = 1

	_, err = database.DB.ExecContext(ctx,
		`INSERT INTO md_users (id, username, password, name, email, phone, department_id, role, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.Password, u.Name, nullStr(u.Email), nullStr(u.Phone), nullStr(u.DepartmentID), u.Role, u.Status,
	)
	return err
}

func scanUser(row *sql.Row) (*model.User, error) {
	u := &model.User{}
	var email, phone, deptID sql.NullString
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Name, &email, &phone, &deptID,
		&u.Role, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.Email = ns(email)
	u.Phone = ns(phone)
	u.DepartmentID = ns(deptID)
	return u, nil
}

func GetUserByID(ctx context.Context, id string) (*model.User, error) {
	row := database.DB.QueryRowContext(ctx,
		`SELECT id, username, password, name, email, phone, department_id, role, status, last_login_at, created_at, updated_at
		 FROM md_users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	row := database.DB.QueryRowContext(ctx,
		`SELECT id, username, password, name, email, phone, department_id, role, status, last_login_at, created_at, updated_at
		 FROM md_users WHERE username = ? AND status = 1`, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func ListUsers(ctx context.Context, departmentID, keyword string, page, pageSize int) ([]*model.User, int, error) {
	where := "WHERE u.status = 1"
	args := []interface{}{}

	if departmentID != "" {
		where += " AND u.department_id = ?"
		args = append(args, departmentID)
	}
	if keyword != "" {
		where += " AND (u.name LIKE ? OR u.username LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM md_users u " + where
	if err := database.DB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	listSQL := `SELECT u.id, u.username, u.name, u.email, u.phone, u.department_id, u.role, u.status,
	            u.last_login_at, u.created_at, u.updated_at, d.name as dept_name
	            FROM md_users u LEFT JOIN md_departments d ON u.department_id = d.id ` +
		where + " ORDER BY u.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := database.DB.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []*model.User{}
	for rows.Next() {
		u := &model.User{}
		var email, phone, deptID, deptName sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &email, &phone, &deptID,
			&u.Role, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt, &deptName); err != nil {
			return nil, 0, err
		}
		u.Email = ns(email)
		u.Phone = ns(phone)
		u.DepartmentID = ns(deptID)
		u.DepartmentName = ns(deptName)
		users = append(users, u)
	}
	return users, total, nil
}

func UpdateUser(ctx context.Context, u *model.User) error {
	_, err := database.DB.ExecContext(ctx,
		`UPDATE md_users SET name=?, email=?, phone=?, department_id=?, role=?, status=?, updated_at=NOW() WHERE id=?`,
		u.Name, nullStr(u.Email), nullStr(u.Phone), nullStr(u.DepartmentID), u.Role, u.Status, u.ID,
	)
	return err
}

func ResetPassword(ctx context.Context, userID, rawPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = database.DB.ExecContext(ctx,
		`UPDATE md_users SET password=?, updated_at=NOW() WHERE id=?`, string(hash), userID)
	return err
}

func UpdateLastLogin(ctx context.Context, userID string) {
	database.DB.ExecContext(ctx,
		`UPDATE md_users SET last_login_at=? WHERE id=?`, time.Now(), userID)
}

// ==================== 部门 ====================

func CreateDepartment(ctx context.Context, d *model.Department) error {
	d.ID = uuid.New().String()
	d.Status = 1

	if d.ParentID != "" {
		depth, err := getDeptDepth(ctx, d.ParentID)
		if err != nil {
			return err
		}
		if depth >= 2 {
			return fmt.Errorf("部门层级不能超过三级")
		}
	}

	_, err := database.DB.ExecContext(ctx,
		`INSERT INTO md_departments (id, name, parent_id, sort_order, status) VALUES (?, ?, ?, ?, ?)`,
		d.ID, d.Name, nullStr(d.ParentID), d.SortOrder, d.Status,
	)
	return err
}

func getDeptDepth(ctx context.Context, deptID string) (int, error) {
	depth := 0
	current := deptID
	for i := 0; i < 10; i++ {
		var parentID sql.NullString
		err := database.DB.QueryRowContext(ctx,
			`SELECT parent_id FROM md_departments WHERE id = ?`, current,
		).Scan(&parentID)
		if err != nil {
			return 0, err
		}
		if !parentID.Valid || parentID.String == "" {
			break
		}
		depth++
		current = parentID.String
	}
	return depth, nil
}

func ListDepartments(ctx context.Context) ([]*model.Department, error) {
	rows, err := database.DB.QueryContext(ctx,
		`SELECT id, name, parent_id, sort_order, status, created_at, updated_at
		 FROM md_departments WHERE status = 1 ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	depts := []*model.Department{}
	for rows.Next() {
		d := &model.Department{}
		var parentID sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &parentID, &d.SortOrder, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.ParentID = ns(parentID)
		depts = append(depts, d)
	}
	return depts, nil
}

func GetDepartmentTree(ctx context.Context) ([]*model.Department, error) {
	all, err := ListDepartments(ctx)
	if err != nil {
		return nil, err
	}

	nodeMap := map[string]*model.Department{}
	for _, d := range all {
		d.Children = []*model.Department{}
		nodeMap[d.ID] = d
	}

	roots := []*model.Department{}
	for _, d := range all {
		if d.ParentID == "" {
			roots = append(roots, d)
		} else if parent, ok := nodeMap[d.ParentID]; ok {
			parent.Children = append(parent.Children, d)
		}
	}
	return roots, nil
}

func UpdateDepartment(ctx context.Context, d *model.Department) error {
	_, err := database.DB.ExecContext(ctx,
		`UPDATE md_departments SET name=?, parent_id=?, sort_order=?, updated_at=NOW() WHERE id=?`,
		d.Name, nullStr(d.ParentID), d.SortOrder, d.ID,
	)
	return err
}

func DeleteDepartment(ctx context.Context, id string) error {
	_, err := database.DB.ExecContext(ctx,
		`UPDATE md_departments SET status=0, updated_at=NOW() WHERE id=?`, id)
	return err
}

func FindDeptIDByName(ctx context.Context, name string) (string, error) {
	var id string
	err := database.DB.QueryRowContext(ctx,
		`SELECT id FROM md_departments WHERE name = ? AND status = 1 LIMIT 1`, name,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}
