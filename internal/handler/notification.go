package handler

import (
	"database/sql"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListNotifications returns notifications for current user.
// GET /notifications
func ListNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")

	rows, err := database.DB.Query(
		"SELECT id, type, title, document_id, related_id, is_read, created_at FROM md_notifications WHERE user_id = ? ORDER BY created_at DESC LIMIT 50",
		userID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	notifications := []gin.H{}
	for rows.Next() {
		var id, nType, title string
		var createdAt time.Time
		var isRead bool
		var docID, relatedID sql.NullString
		rows.Scan(&id, &nType, &title, &docID, &relatedID, &isRead, &createdAt)
		notifications = append(notifications, gin.H{
			"id":          id,
			"type":        nType,
			"title":       title,
			"document_id": docID.String,
			"related_id":  relatedID.String,
			"is_read":     isRead,
			"created_at":  createdAt,
		})
	}

	// Unread count
	var unreadCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_notifications WHERE user_id = ? AND is_read = 0", userID).Scan(&unreadCount)

	c.JSON(200, gin.H{"data": notifications, "unread_count": unreadCount})
}

// MarkNotificationRead marks a notification as read.
// PUT /notifications/:id/read
func MarkNotificationRead(c *gin.Context) {
	notifID := c.Param("id")
	userID, _ := c.Get("user_id")

	database.DB.Exec("UPDATE md_notifications SET is_read = 1 WHERE id = ? AND user_id = ?", notifID, userID)
	c.JSON(200, gin.H{"message": "已标记已读"})
}

// MarkAllNotificationsRead marks all notifications as read.
// PUT /notifications/read-all
func MarkAllNotificationsRead(c *gin.Context) {
	userID, _ := c.Get("user_id")

	database.DB.Exec("UPDATE md_notifications SET is_read = 1 WHERE user_id = ? AND is_read = 0", userID)
	c.JSON(200, gin.H{"message": "已全部标记已读"})
}

// DeleteNotification deletes a notification.
// DELETE /notifications/:id
func DeleteNotification(c *gin.Context) {
	notifID := c.Param("id")
	userID, _ := c.Get("user_id")

	database.DB.Exec("DELETE FROM md_notifications WHERE id = ? AND user_id = ?", notifID, userID)
	c.JSON(200, gin.H{"message": "已删除"})
}

// UnreadCount returns the number of unread notifications.
// GET /notifications/unread-count
func UnreadCount(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_notifications WHERE user_id = ? AND is_read = 0", userID).Scan(&count)

	c.JSON(200, gin.H{"count": count})
}

// createNotification is an internal helper to create a notification.
func createNotification(userID, nType, title, documentID, relatedID string) {
	id := uuid.New().String()
	database.DB.Exec(
		"INSERT INTO md_notifications (id, user_id, type, title, document_id, related_id, is_read, created_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?)",
		id, userID, nType, title, documentID, relatedID, time.Now(),
	)
}
