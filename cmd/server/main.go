package main

import (
	"log"
	"os"

	"github.com/c-wind/mist-docs/internal/handler"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/ws"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Middleware
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())

	// Static files
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/", "./web/dist/index.html")

	// API group
	api := r.Group("/api")
	{
		// Public
		api.POST("/auth/login", handler.Login)

		// Authenticated
		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.POST("/auth/logout", handler.Logout)
			auth.GET("/auth/me", handler.Me)

			// Departments
			auth.GET("/departments", handler.ListDepartments)
			auth.POST("/departments", handler.CreateDepartment)
			auth.PUT("/departments/:id", handler.UpdateDepartment)
			auth.DELETE("/departments/:id", handler.DeleteDepartment)
			auth.POST("/departments/import", handler.ImportDepartments)

			// Users
			auth.GET("/users", handler.ListUsers)
			auth.POST("/users", handler.CreateUser)
			auth.PUT("/users/:id", handler.UpdateUser)
			auth.DELETE("/users/:id", handler.DeleteUser)
			auth.PUT("/users/:id/reset-password", handler.ResetPassword)
			auth.POST("/users/import", handler.ImportUsers)

			// Documents
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

			// Permissions
			auth.GET("/permissions", handler.ListPermissions)
			auth.POST("/permissions", handler.SetPermission)
			auth.DELETE("/permissions/:id", handler.RemovePermission)
			auth.GET("/permissions/check", handler.CheckPermission)

			// Audit
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8900"
	}
	log.Printf("MistDocs listening on :%s", port)
	r.Run(":" + port)
}
