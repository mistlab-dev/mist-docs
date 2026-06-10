package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/crypto"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/handler"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/ws"
	"github.com/gin-gonic/gin"
)

func main() {
	configFile := flag.String("c", "configs/config.yaml", "配置文件路径")
	showKey := flag.Bool("show-key", false, "显示 master key 信息")
	keyInfo := flag.Bool("key-info", false, "显示密钥管理信息")
	rotateDEK := flag.Bool("rotate-dek", false, "轮换 DEK")
	flag.Parse()

	// Subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "keygen":
			if err := crypto.Keygen(); err != nil {
				log.Fatalf("Keygen failed: %v", err)
			}
			return
		}
	}

	log.Println("[BOOT] loading config...")
	if err := config.Load(*configFile); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Println("[BOOT] config loaded")

	log.Println("[BOOT] connecting database...")
	if err := database.Init(config.C.Database); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Close()
	log.Println("[BOOT] database connected")

	// 密钥管理命令
	if *showKey {
		if err := crypto.InitMasterKey(); err != nil {
			fmt.Printf("Master key: NOT LOADED (%v)\n", err)
			return
		}
		crypto.ShowKey()
		return
	}

	if *keyInfo {
		if err := crypto.InitMasterKey(); err != nil {
			fmt.Printf("⚠️  Master key not loaded: %v\n", err)
		}
		info, err := crypto.KeyInfo(context.Background())
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Master key loaded: %v\n", info["master_key_loaded"])
		fmt.Printf("Keys:\n")
		if keys, ok := info["keys"].([]map[string]interface{}); ok {
			for _, k := range keys {
				fmt.Printf("  %s | %s | %s | %s | by %s\n",
					k["id"], k["type"], k["status"], k["created_at"], k["created_by"])
			}
		}
		return
	}

	if *rotateDEK {
		if err := crypto.InitMasterKey(); err != nil {
			log.Fatalf("需要 master key: %v", err)
		}
		if err := crypto.InitKeyTables(context.Background()); err != nil {
			log.Fatalf("初始化密钥表: %v", err)
		}
		if err := crypto.RotateDEK(context.Background(), "cli"); err != nil {
			log.Fatalf("轮换 DEK 失败: %v", err)
		}
		fmt.Println("DEK rotated successfully")
		return
	}

	// 初始化文件存储
	log.Println("[BOOT] init store...")
	if err := handler.InitStore(); err != nil {
		log.Fatalf("文件存储初始化失败: %v", err)
	}
	log.Println("[BOOT] store ready")

	// 初始化加密
	log.Println("[BOOT] init crypto...")
	if err := handler.InitCrypto(); err != nil {
		log.Printf("⚠️  加密初始化失败: %v", err)
	}
	log.Println("[BOOT] crypto ready")

	// 初始化路由
	r := gin.Default()
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())
	r.Use(middleware.RateLimit(30, 60)) // 30 req/s per IP, burst 60

	// 静态文件
	r.Static("/assets", "./web/dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	// API
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

				// Search targets
				teams.GET("/search-targets", handler.TeamSearchTargets)

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
			}
		}
	}

	// WebSocket
	hub := ws.NewHub()
	go hub.Run()
	r.GET("/ws/teams/:team_id/docs/:doc_id", func(c *gin.Context) {
		ws.ServeWS(hub, c)
	})

	addr := fmt.Sprintf("%s:%d", config.C.Server.Host, config.C.Server.Port)
	encStatus := "OFF"
	if crypto.IsMasterKeyLoaded() {
		encStatus = "ON (AES-256-GCM)"
	}
	log.Printf("MistDocs 启动: %s | 加密: %s", addr, encStatus)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
