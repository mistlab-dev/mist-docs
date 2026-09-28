package handler

import (
	"database/sql"
	"net/http"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
)

// OnDocAccessChanged is called after a document's lock or permissions change
// so live editing sessions re-check at once. cmd/server wires it to the
// websocket hub; nil in tests that do not run a hub.
var OnDocAccessChanged func(docID string)

func notifyDocAccessChanged(docID string) {
	if OnDocAccessChanged != nil {
		OnDocAccessChanged(docID)
	}
}

type docLock struct {
	By     string `json:"locked_by"`
	ByName string `json:"locked_by_name"`
	At     string `json:"locked_at"`
}

func loadDocLock(docID string) docLock {
	var l docLock
	var at sql.NullString
	database.DB.QueryRow(
		`SELECT IFNULL(d.locked_by,''), d.locked_at,
		 COALESCE(NULLIF(u.display_name,''), NULLIF(u.username,''), '')
		 FROM md_documents d LEFT JOIN users u ON d.locked_by COLLATE utf8mb4_unicode_ci = u.id
		 WHERE d.id=?`, docID).Scan(&l.By, &at, &l.ByName)
	l.At = at.String
	return l
}

func (l docLock) fields() gin.H {
	return gin.H{"locked_by": l.By, "locked_by_name": l.ByName, "locked_at": l.At}
}

// lockedAgainst answers 409 (with who holds the lock) when someone else holds
// the lock and the caller is not a team admin. Admins may override.
func lockedAgainst(c *gin.Context, docID string) bool {
	l := loadDocLock(docID)
	if l.By == "" || l.By == c.GetString("user_id") || isTeamAdmin(c) {
		return false
	}
	resp := l.fields()
	resp["error"] = "文档已被 " + nameOr(l.ByName, "其他成员") + " 锁定"
	c.JSON(http.StatusConflict, resp)
	return true
}

func nameOr(name, fallback string) string {
	if name == "" {
		return fallback
	}
	return name
}

// TeamLockDocument POST /teams/:team_id/documents/:id/lock
// Takes the lock, or refreshes it when the caller already holds it. Someone
// else's lock is not taken over (admins must unlock first).
func TeamLockDocument(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	userID := c.GetString("user_id")
	res, err := database.DB.Exec(
		`UPDATE md_documents SET locked_by=?, locked_at=NOW()
		 WHERE id=? AND team_id=? AND (locked_by IS NULL OR locked_by='' OR locked_by=?)`,
		userID, docID, getTeamID(c), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	l := loadDocLock(docID)
	if n, _ := res.RowsAffected(); n == 0 && l.By != userID {
		resp := l.fields()
		resp["error"] = "文档已被 " + nameOr(l.ByName, "其他成员") + " 锁定"
		c.JSON(http.StatusConflict, resp)
		return
	}
	audit(c, "lock_doc", "document", docID, "", "")
	notifyDocAccessChanged(docID)
	resp := l.fields()
	resp["message"] = "已锁定"
	c.JSON(http.StatusOK, resp)
}

// TeamUnlockDocument POST /teams/:team_id/documents/:id/unlock
// Only the holder or a team admin can release a lock.
func TeamUnlockDocument(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "write", true) {
		return
	}
	l := loadDocLock(docID)
	if l.By == "" {
		c.JSON(http.StatusOK, gin.H{"message": "未锁定", "locked_by": ""})
		return
	}
	if l.By != c.GetString("user_id") && !isTeamAdmin(c) {
		resp := l.fields()
		resp["error"] = "只有锁定人或管理员可以解锁"
		c.JSON(http.StatusForbidden, resp)
		return
	}
	if _, err := database.DB.Exec(`UPDATE md_documents SET locked_by='', locked_at=NULL WHERE id=? AND team_id=?`, docID, getTeamID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "unlock_doc", "document", docID, "", "")
	notifyDocAccessChanged(docID)
	c.JSON(http.StatusOK, gin.H{"message": "已解锁", "locked_by": ""})
}
