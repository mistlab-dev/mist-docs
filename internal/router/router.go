// Package router registers the HTTP API. cmd/server and the integration
// tests share it, so tests exercise exactly the routes production serves.
package router

import (
	"github.com/c-wind/mist-docs/internal/handler"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/ws"
	"github.com/gin-gonic/gin"
)

// RegisterAPI mounts every /api route on r.
func RegisterAPI(r gin.IRouter) {
	api := r.Group("/api")
	{
		// 公开
		api.POST("/auth/login", handler.Login)
		api.GET("/files/:filename", handler.GetFile)
		api.GET("/openapi.json", handler.APIDocs)

		// 公开分享链接
		api.GET("/s/:token", handler.AccessShare)
		api.GET("/s/:token/info", handler.AccessShareInfo)

		// 需认证
		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.POST("/auth/logout", handler.Logout)
			auth.GET("/auth/me", handler.Me)
			auth.PUT("/auth/password", handler.ChangePassword)

			// 团队级 API
			teams := auth.Group("/teams/:team_id")
			teams.Use(middleware.TeamAuth())
			{
				// 文件夹树
				teams.GET("/folders/tree", handler.TeamFolderTree)
				teams.POST("/folders", handler.CreateTeamFolder)
				teams.PUT("/folders/:id", handler.UpdateTeamFolder)
				teams.DELETE("/folders/:id", handler.DeleteTeamFolder)

				// 文档
				teams.GET("/documents", handler.TeamListDocuments)
				teams.GET("/documents/search", handler.TeamSearchDocuments)
				teams.GET("/docs/search", handler.TeamDocParagraphSearch)
				teams.GET("/documents/recent", handler.TeamRecentDocuments)
				teams.POST("/documents", handler.TeamCreateDocument)
				teams.GET("/documents/:id", handler.TeamGetDocument)
				teams.PUT("/documents/:id", handler.TeamUpdateDocument)
				teams.DELETE("/documents/:id", handler.TeamDeleteDocument)
				teams.GET("/documents/:id/content", handler.TeamGetDocumentContent)
				teams.PUT("/documents/:id/content", handler.TeamSaveDocumentContent)
				teams.GET("/documents/:id/stats", handler.TeamDocStats)
				teams.GET("/documents/:id/versions", handler.TeamListVersions)
				teams.GET("/documents/:id/versions/:ver/content", handler.TeamGetVersionContent)
				teams.POST("/documents/:id/restore", handler.TeamRestoreVersion)
				teams.POST("/documents/:id/lock", handler.TeamLockDocument)
				teams.POST("/documents/:id/unlock", handler.TeamUnlockDocument)
				teams.POST("/documents/:id/share", handler.TeamCreateShare)
				teams.GET("/documents/:id/shares", handler.TeamListShares)
				teams.GET("/documents/:id/collaborators", handler.TeamListCollaborators)
				teams.POST("/documents/:id/collaborators", handler.TeamAddCollaborator)
				teams.GET("/documents/:id/comments", handler.TeamListComments)
				teams.POST("/documents/:id/comments", handler.TeamCreateComment)
				teams.GET("/documents/:id/export", handler.TeamExportDocument)

				// 文档 ↔ 团队片段联动
				teams.GET("/documents/:id/fragments", handler.TeamListDocFragments)
				teams.POST("/documents/:id/fragments", handler.TeamAttachDocFragment)
				teams.DELETE("/documents/:id/fragments/:fragment_id", handler.TeamDetachDocFragment)
				teams.GET("/fragments-search", handler.TeamSearchTeamFragments)

				// 回收站
				teams.GET("/trash", handler.TeamListTrash)
				teams.POST("/trash/restore/:id", handler.TeamRestoreFromTrash)
				teams.DELETE("/trash/purge/:id", handler.TeamPurgeFromTrash)
				teams.DELETE("/trash/empty", handler.TeamEmptyTrash)

				// 标签
				teams.GET("/tags", handler.TeamListTags)
				teams.POST("/tags", handler.TeamCreateTag)
				teams.DELETE("/tags/:id", handler.TeamDeleteTag)
				teams.GET("/documents/:id/tags", handler.TeamGetDocTags)
				teams.PUT("/documents/:id/tags", handler.TeamSetDocTags)

				// 模板
				teams.GET("/templates", handler.TeamListTemplates)
				teams.GET("/templates/:id", handler.TeamGetTemplate)
				teams.POST("/templates", handler.TeamCreateTemplate)
				teams.PUT("/templates/:id", handler.TeamUpdateTemplate)
				teams.DELETE("/templates/:id", handler.TeamDeleteTemplate)

				// 权限
				teams.GET("/permissions", handler.TeamListPermissions)
				teams.POST("/permissions", handler.TeamSetPermission)
				teams.DELETE("/permissions/:id", handler.TeamRemovePermission)
				teams.GET("/permissions/check", handler.TeamCheckPermission)

				// 审计
				teams.GET("/audits", handler.TeamListAudits)
				teams.GET("/audits/export", handler.TeamExportAudits)
				teams.GET("/audits/stats", handler.TeamAuditStats)

				// 收藏
				teams.GET("/favorites", handler.TeamListFavorites)
				teams.POST("/favorites/:id", handler.TeamAddFavorite)
				teams.DELETE("/favorites/:id", handler.TeamRemoveFavorite)

				// 存储
				teams.GET("/storage/status", handler.TeamStorageStatus)

				// Webhooks
				teams.GET("/webhooks", handler.TeamListWebhooks)
				teams.POST("/webhooks", handler.TeamCreateWebhook)
				teams.PUT("/webhooks/:id", handler.TeamUpdateWebhook)
				teams.DELETE("/webhooks/:id", handler.TeamDeleteWebhook)
				teams.PUT("/webhooks/:id/toggle", handler.TeamToggleWebhook)
				teams.GET("/webhooks/:id/logs", handler.TeamListWebhookLogs)

				// Media
				teams.POST("/upload", handler.TeamUploadFile)
				teams.GET("/media", handler.TeamListMedia)
				teams.GET("/media/:filename", handler.TeamGetMedia)
				teams.DELETE("/media/:filename", handler.TeamDeleteMedia)

				// Shares
				teams.DELETE("/shares/:id", handler.TeamDeleteShare)

				// Collaborators
				teams.PUT("/collaborators/:id", handler.TeamUpdateCollaborator)
				teams.DELETE("/collaborators/:id", handler.TeamRemoveCollaborator)

				// Comments
				teams.PUT("/comments/:id", handler.TeamUpdateComment)
				teams.DELETE("/comments/:id", handler.TeamDeleteComment)

				// Search targets and members
				teams.GET("/search-targets", handler.TeamSearchTargets)
				teams.GET("/members", handler.TeamListMembers)

				// Tags (documents by tag)
				teams.GET("/tags/:id/documents", handler.TeamGetDocsByTag)

				// Import
				teams.POST("/import", handler.TeamImportDocument)

				// Dashboard
				teams.GET("/dashboard", handler.TeamDashboardStats)
				teams.GET("/system-info", handler.TeamSystemInfo)

				// Notifications
				teams.GET("/notifications", handler.TeamListNotifications)
				teams.PUT("/notifications/:id/read", handler.TeamMarkNotificationRead)
				teams.PUT("/notifications/read-all", handler.TeamMarkAllNotificationsRead)
				teams.DELETE("/notifications/:id", handler.TeamDeleteNotification)
				teams.GET("/notifications/unread-count", handler.TeamUnreadCount)

				// 交期看板（deadlines are DB rows, not documents）
				teams.GET("/deadlines", handler.TeamListDeadlines)
				teams.GET("/deadlines/board", handler.TeamDeadlineBoard)
				teams.POST("/deadlines", handler.TeamCreateDeadline)
				teams.GET("/deadlines/:id", handler.TeamGetDeadline)
				teams.PUT("/deadlines/:id", handler.TeamUpdateDeadline)
				teams.DELETE("/deadlines/:id", handler.TeamDeleteDeadline)
				teams.GET("/deadlines/:id/events", handler.TeamDeadlineEvents)

				// 交期提醒规则
				teams.GET("/reminder-rules", handler.TeamListReminderRules)
				teams.POST("/reminder-rules", handler.TeamCreateReminderRule)
				teams.POST("/reminder-rules/seed", handler.TeamSeedDeadlineRules)
				teams.PUT("/reminder-rules/:id", handler.TeamUpdateReminderRule)
				teams.DELETE("/reminder-rules/:id", handler.TeamDeleteReminderRule)
				teams.GET("/reminder-log", handler.TeamListReminderLog)
			}
		}
	}

}

// RegisterWS mounts the collaboration websocket served by hub.
func RegisterWS(r gin.IRouter, hub *ws.Hub) {
	// Lock/unlock and permission changes re-check open sessions immediately.
	handler.OnDocAccessChanged = hub.Reauthorize
	r.GET("/ws/teams/:team_id/docs/:doc_id", func(c *gin.Context) {
		ws.ServeWS(hub, c)
	})
}
