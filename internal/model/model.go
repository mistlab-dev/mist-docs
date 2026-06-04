package model

import "time"

// ==================== 部门 ====================

type Department struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	ParentID  string    `json:"parent_id" db:"parent_id"`
	SortOrder int       `json:"sort_order" db:"sort_order"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// 非数据库字段
	Children  []*Department `json:"children"`
	UserCount int           `json:"user_count,omitempty"`
}

// ==================== 用户 ====================

type User struct {
	ID           string     `json:"id" db:"id"`
	Username     string     `json:"username" db:"username"`
	Password     string     `json:"-" db:"password"`
	Name         string     `json:"name" db:"name"`
	Email        string     `json:"email,omitempty" db:"email"`
	Phone        string     `json:"phone,omitempty" db:"phone"`
	DepartmentID string     `json:"department_id" db:"department_id"`
	Role         string     `json:"role" db:"role"` // super_admin / dept_admin / member
	Status       int        `json:"status" db:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`

	// 非数据库字段
	DepartmentName string `json:"department_name,omitempty"`
}

// ==================== 文件夹 ====================

type DocFolder struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	ParentID     string    `json:"parent_id" db:"parent_id"`
	DepartmentID string    `json:"department_id" db:"department_id"`
	CreatedBy    string    `json:"created_by" db:"created_by"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`

	// 非数据库字段
	Children []*DocFolder `json:"children,omitempty"`
	DocCount  int          `json:"doc_count,omitempty"`
}

// ==================== 文档 ====================

type Document struct {
	ID           string    `json:"id" db:"id"`
	FolderID     string    `json:"folder_id" db:"folder_id"`
	DepartmentID string    `json:"department_id" db:"department_id"`
	Title        string    `json:"title" db:"title"`
	Type         string    `json:"type" db:"type"` // doc / sheet
	FilePath     string    `json:"-" db:"file_path"`
	FileSize     int64     `json:"file_size" db:"file_size"`
	Version      int       `json:"version" db:"version"` // 1=正常 0=回收站
	LockedBy     string    `json:"locked_by" db:"locked_by"`
	LockedAt     *time.Time `json:"locked_at,omitempty" db:"locked_at"`
	Status       int       `json:"status" db:"status"`
	CreatedBy    string    `json:"created_by" db:"created_by"`
	UpdatedBy    string    `json:"updated_by" db:"updated_by"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`

	// 非数据库字段
	CreatedByName string `json:"created_by_name,omitempty"`
	UpdatedByName string `json:"updated_by_name,omitempty"`
}

// ==================== 版本 ====================

type DocVersion struct {
	ID         string    `json:"id" db:"id"`
	DocumentID string    `json:"document_id" db:"document_id"`
	Version    int       `json:"version" db:"version"`
	FilePath   string    `json:"-" db:"file_path"`
	FileSize   int64     `json:"file_size" db:"file_size"`
	CreatedBy  string    `json:"created_by" db:"created_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`

	// 非数据库字段
	CreatedByName string `json:"created_by_name,omitempty"`
}

// ==================== 权限 ====================

type DocPermission struct {
	ID           string    `json:"id" db:"id"`
	ResourceType string    `json:"resource_type" db:"resource_type"` // folder / document
	ResourceID   string    `json:"resource_id" db:"resource_id"`
	TargetType   string    `json:"target_type" db:"target_type"` // department / user
	TargetID     string    `json:"target_id" db:"target_id"`
	Permission   string    `json:"permission" db:"permission"` // read / write / admin
	Inherit      bool      `json:"inherit" db:"inherit"`
	CreatedBy    string    `json:"created_by" db:"created_by"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`

	// 非数据库字段
	TargetName   string `json:"target_name,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`
}

// ==================== 审计 ====================

type DocAudit struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	UserName     string    `json:"user_name" db:"user_name"`
	DepartmentID string    `json:"department_id" db:"department_id"`
	Action       string    `json:"action" db:"action"`
	ResourceType string    `json:"resource_type" db:"resource_type"`
	ResourceID   string    `json:"resource_id" db:"resource_id"`
	ResourceName string    `json:"resource_name" db:"resource_name"`
	Detail       string    `json:"detail,omitempty" db:"detail"`
	IP           string    `json:"ip" db:"ip"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// ==================== 收藏 ====================

type DocFavorite struct {
	ID         string    `json:"id" db:"id"`
	UserID     string    `json:"user_id" db:"user_id"`
	DocumentID string    `json:"document_id" db:"document_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`

	// 非数据库字段
	DocTitle   string `json:"doc_title,omitempty"`
	DocType    string `json:"doc_type,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// ==================== 标签 ====================

type DocTag struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Color     string    `json:"color" db:"color"` // hex color like #409eff
	UserID    string    `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// 非数据库字段
	DocCount int `json:"doc_count,omitempty"`
}

// 文档-标签关联
type DocTagRelation struct {
	ID         string    `json:"id" db:"id"`
	DocumentID string    `json:"document_id" db:"document_id"`
	TagID      string    `json:"tag_id" db:"tag_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
