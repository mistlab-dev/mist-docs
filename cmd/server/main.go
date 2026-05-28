package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/handler"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/ws"
	"github.com/gin-gonic/gin"
)

func main() {
	configFile := flag.String("c", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	if err := config.Load(*configFile); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	if err := database.Init(config.C.Database); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Close()

	// 初始化路由
	r := gin.Default()
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())

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

			// 文档（stub）
			auth.GET("/docs/tree", handler.DocTree)
			auth.POST("/docs/folders", handler.CreateFolder)
			auth.PUT("/docs/folders/:id", handler.UpdateFolder)
			auth.DELETE("/docs/folders/:id", handler.DeleteFolder)
			auth.GET("/docs/documents", handler.ListDocuments)
			auth.POST("/docs/documents", handler.CreateDocument)
			auth.PUT("/docs/documents/:id", handler.UpdateDocument)
			auth.DELETE("/docs/documents/:id", handler.DeleteDocument)
			auth.GET("/docs/documents/:id/content", handler.GetDocumentContent)
			auth.PUT("/docs/documents/:id/content", handler.SaveDocumentContent)
			auth.GET("/docs/documents/:id/versions", handler.ListVersions)
			auth.POST("/docs/documents/:id/restore", handler.RestoreVersion)
			auth.GET("/docs/trash", handler.ListTrash)
			auth.POST("/docs/trash/restore/:id", handler.RestoreFromTrash)
			auth.DELETE("/docs/trash/purge/:id", handler.PurgeFromTrash)

			// 权限（stub）
			auth.GET("/permissions", handler.ListPermissions)
			auth.POST("/permissions", handler.SetPermission)
			auth.DELETE("/permissions/:id", handler.RemovePermission)
			auth.GET("/permissions/check", handler.CheckPermission)

			// 审计（stub）
			auth.GET("/audits", handler.ListAudits)
			auth.GET("/audits/export", handler.ExportAudits)
			auth.GET("/audits/stats", handler.AuditStats)
		}
	}

	// WebSocket
	hub := ws.NewHub()
	go hub.Run()
	r.GET("/ws/docs/:doc_id", func(c *gin.Context) {
		ws.ServeWS(hub, c)
	})

	addr := fmt.Sprintf("%s:%d", config.C.Server.Host, config.C.Server.Port)
	log.Printf("MistDocs 启动: %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
