package model

import "time"

// Document is a row of md_documents as read by service.GetDocumentByID.
type Document struct {
	ID           string     `json:"id" db:"id"`
	FolderID     string     `json:"folder_id" db:"folder_id"`
	DepartmentID string     `json:"department_id" db:"department_id"`
	TeamID       string     `json:"team_id,omitempty" db:"team_id"`
	Title        string     `json:"title" db:"title"`
	Type         string     `json:"type" db:"type"` // doc / sheet
	FilePath     string     `json:"-" db:"file_path"`
	FileSize     int64      `json:"file_size" db:"file_size"`
	Version      int        `json:"version" db:"version"` // 1=正常 0=回收站
	LockedBy     string     `json:"locked_by" db:"locked_by"`
	LockedAt     *time.Time `json:"locked_at,omitempty" db:"locked_at"`
	Status       int        `json:"status" db:"status"`
	CreatedBy    string     `json:"created_by" db:"created_by"`
	UpdatedBy    string     `json:"updated_by" db:"updated_by"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`

	// 非数据库字段
	CreatedByName string `json:"created_by_name,omitempty"`
	UpdatedByName string `json:"updated_by_name,omitempty"`
}
