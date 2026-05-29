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

			// 部门
			auth.GET("/departments", handler.ListDepartments)
			auth.POST("/departments", handler.CreateDepartment)
			auth.PUT("/departments/:id", handler.UpdateDepartment)
			auth.DELETE("/departments/:id", handler.DeleteDepartment)
			auth.POST("/departments/import", handler.ImportDepartments)

			// 用户
			auth.GET("/users", handler.ListUsers)
			auth.POST("/users", handler.CreateUser)
			auth.PUT("/users/:id", handler.UpdateUser)
			auth.DELETE("/users/:id", handler.DeleteUser)
			auth.PUT("/users/:id/reset-password", handler.ResetPassword)
			auth.POST("/users/import", handler.ImportUsers)

			// 文档
			auth.GET("/docs/tree", handler.DocTree)
			auth.POST("/docs/folders", handler.CreateFolder)
			auth.PUT("/docs/folders/:id", handler.UpdateFolder)
			auth.DELETE("/docs/folders/:id", handler.DeleteFolder)
			auth.POST("/docs/upload", handler.UploadFile)
			auth.POST("/docs/import", handler.BatchImport)
			auth.GET("/docs/documents", handler.ListDocuments)
			auth.GET("/docs/documents/search", handler.SearchDocuments)
			auth.GET("/docs/documents/recent", handler.RecentDocuments)
			auth.GET("/docs/documents/:id", handler.GetDocument)
			auth.GET("/docs/favorites", handler.ListFavorites)
			auth.POST("/docs/favorites/:id", handler.AddFavorite)
			auth.DELETE("/docs/favorites/:id", handler.RemoveFavorite)
			// Tags
			auth.GET("/docs/tags", handler.ListTags)
			auth.POST("/docs/tags", handler.CreateTag)
			auth.DELETE("/docs/tags/:id", handler.DeleteTag)
			auth.GET("/docs/tags/:id/documents", handler.GetDocsByTag)
			auth.GET("/docs/documents/:id/tags", handler.GetDocTags)
			auth.PUT("/docs/documents/:id/tags", handler.SetDocTags)
			auth.POST("/docs/documents", handler.CreateDocument)
			auth.PUT("/docs/documents/:id", handler.UpdateDocument)
			auth.DELETE("/docs/documents/:id", handler.DeleteDocument)
			auth.GET("/docs/documents/:id/content", handler.GetDocumentContent)
			auth.PUT("/docs/documents/:id/content", handler.SaveDocumentContent)
			auth.GET("/docs/documents/:id/stats", handler.DocStats)
			auth.GET("/docs/documents/:id/versions", handler.ListVersions)
			auth.GET("/docs/documents/:id/versions/:ver/content", handler.GetVersionContent)
			auth.POST("/docs/documents/:id/restore", handler.RestoreVersion)
			auth.POST("/docs/documents/:id/lock", handler.LockDocument)
			auth.POST("/docs/documents/:id/unlock", handler.UnlockDocument)
			auth.GET("/docs/trash", handler.ListTrash)
			auth.POST("/docs/trash/restore/:id", handler.RestoreFromTrash)
			auth.DELETE("/docs/trash/purge/:id", handler.PurgeFromTrash)

			// 权限
			auth.GET("/permissions", handler.ListPermissions)
			auth.POST("/permissions", handler.SetPermission)
			auth.DELETE("/permissions/:id", handler.RemovePermission)
			auth.GET("/permissions/check", handler.CheckPermission)

			// 审计
			auth.GET("/audits", handler.ListAudits)
			auth.GET("/audits/export", handler.ExportAudits)
			auth.GET("/audits/stats", handler.AuditStats)

			// 存储监控
			auth.GET("/storage/status", handler.StorageStatus)

			// 管理后台
			auth.GET("/admin/dashboard", handler.DashboardStats)
			auth.GET("/admin/system-info", handler.SystemInfo)

			// Webhooks
			auth.GET("/admin/webhooks", handler.ListWebhooks)
			auth.POST("/admin/webhooks", handler.CreateWebhook)
			auth.DELETE("/admin/webhooks/:id", handler.DeleteWebhook)
			auth.PUT("/admin/webhooks/:id/toggle", handler.ToggleWebhook)
			auth.GET("/admin/webhooks/:id/logs", handler.ListWebhookLogs)

			// 文档分享
			auth.POST("/docs/documents/:id/share", handler.CreateShare)
			auth.GET("/docs/documents/:id/shares", handler.ListShares)
			auth.DELETE("/docs/shares/:id", handler.DeleteShare)

			// 文档导出
			auth.GET("/docs/documents/:id/export", handler.ExportDocument)

			// 评论
			auth.GET("/docs/documents/:id/comments", handler.ListComments)
			auth.POST("/docs/documents/:id/comments", handler.CreateComment)
			auth.PUT("/docs/comments/:id", handler.UpdateComment)
			auth.DELETE("/docs/comments/:id", handler.DeleteComment)

			// 通知
			auth.GET("/notifications", handler.ListNotifications)
			auth.PUT("/notifications/:id/read", handler.MarkNotificationRead)
			auth.PUT("/notifications/read-all", handler.MarkAllNotificationsRead)
			auth.DELETE("/notifications/:id", handler.DeleteNotification)
			auth.GET("/notifications/unread-count", handler.UnreadCount)
		}
	}

	// WebSocket
	hub := ws.NewHub()
	go hub.Run()
	r.GET("/ws/docs/:doc_id", func(c *gin.Context) {
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
