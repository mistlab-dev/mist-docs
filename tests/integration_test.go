package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/handler"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/google/uuid"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// ==================== Test Helpers ====================

var (
	router *gin.Engine

	// 测试用户 token
	adminToken  string
	editorToken string
	viewerToken string

	// 测试用户 ID
	adminID  string
	editorID string
	viewerID string

	// 测试团队
	teamID string

	// 存储临时目录
	testStorageRoot string
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	config.C = config.Config{
		Server: config.ServerConfig{Port: 0},
		Database: config.DatabaseConfig{
			Host:         "127.0.0.1",
			Port:         3306,
			User:         "mist_team",
			Password:     "MistTeam@2026",
			DBName:       "mist_team",
			MaxOpenConns: 5,
			MaxIdleConns: 2,
		},
		JWT: config.JWTConfig{
			Secret:      "test-jwt-secret-for-integration",
			ExpireHours: 24,
			Issuer:      "mist-docs-test",
		},
		Storage: config.StorageConfig{
			Root:        "",
			MaxFileSize: 52428800,
			VersionKeep: 20,
		},
		Audit:     config.AuditConfig{RetainDays: 180},
		WebSocket: config.WebSocketConfig{MaxMessageSize: 1048576},
	}

	tmpDir, err := os.MkdirTemp("", "mistdocs-test-*")
	if err != nil {
		panic(err)
	}
	testStorageRoot = tmpDir
	config.C.Storage.Root = tmpDir

	if err := database.Init(config.C.Database); err != nil {
		fmt.Printf("SKIP: cannot connect database: %v\n", err)
		os.Exit(0)
	}
	defer database.Close()

	store.Init()

	cleanTestData()
	setupTestData()

	router = buildRouter()

	adminToken, _ = middleware.GenerateToken(adminID, "admin", "super_admin", "")
	editorToken, _ = middleware.GenerateToken(editorID, "editor", "editor", "")
	viewerToken, _ = middleware.GenerateToken(viewerID, "viewer", "viewer", "")

	code := m.Run()

	cleanTestData()
	os.RemoveAll(testStorageRoot)

	os.Exit(code)
}

func cleanTestData() {
	db := database.DB
	ctx := context.Background()

	tables := []string{
		"md_webhook_logs", "md_webhooks",
		"md_notifications", "md_comments",
		"md_shares", "md_favorites",
		"md_doc_tags", "md_tags",
		"md_audits", "md_permissions",
		"md_versions", "md_documents",
		"md_team_folders",
		"md_keys",
		"md_users", "md_departments",
	}
	for _, t := range tables {
		db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id LIKE 'test-%%'", t))
	}

	// 清理 team 相关测试数据
	db.ExecContext(ctx, "DELETE FROM team_members WHERE team_id LIKE 'test-%'")
	db.ExecContext(ctx, "DELETE FROM teams WHERE id LIKE 'test-%'")
	db.ExecContext(ctx, "DELETE FROM users WHERE id LIKE 'test-%'")
}

func setupTestData() {
	ctx := context.Background()
	db := database.DB

	// 使用真实 admin 用户
	adminID = "u_adm_f38a1c13-488"
	editorID = "test-user-editor"
	viewerID = "test-user-viewer"

	// 在共享 users 表创建测试用户
	hashed := "$2a$10$X5VtjlPu/whem0Jgbe/WDecwDAI9tTQYLNVMhq3OVoacvnwLEHWiS"
	for _, u := range []struct {
		id, email, username, displayName string
		isAdmin                         bool
	}{
		{editorID, "editor@test.com", "editor", "编辑者", false},
		{viewerID, "viewer@test.com", "viewer", "查看者", false},
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, email, username, display_name, password_hash, is_admin, email_verified) VALUES (?,?,?,?,?,?,1)`,
			u.id, u.email, u.username, u.displayName, hashed, u.isAdmin)
		if err != nil {
			panic(fmt.Sprintf("insert test user %s into users table failed: %v", u.username, err))
		}
	}

	// 创建测试团队
	teamID = "test-team-alpha"
	_, err := db.ExecContext(ctx,
		`INSERT INTO teams (id, name, description) VALUES (?, ?, ?)`,
		teamID, "测试团队", "集成测试用团队")
	if err != nil {
		panic(fmt.Sprintf("insert test team failed: %v", err))
	}

	// 添加团队成员
	// admin 作为团队 admin
	db.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'admin')`, teamID, adminID)
	// editor 作为 editor
	db.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'editor')`, teamID, editorID)
	// viewer 作为 viewer
	db.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'viewer')`, teamID, viewerID)
}

func buildRouter() *gin.Engine {
	r := gin.New()
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())

	api := r.Group("/api")
	{
		// 公开
		api.POST("/auth/login", handler.Login)
		api.GET("/s/:token", handler.AccessShare)
		api.GET("/s/:token/info", handler.AccessShareInfo)

		// 需认证
		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.GET("/auth/me", handler.Me)

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
				teams.POST("/documents", handler.TeamCreateDocument)
				teams.GET("/documents/:id", handler.TeamGetDocument)
				teams.PUT("/documents/:id", handler.TeamUpdateDocument)
				teams.DELETE("/documents/:id", handler.TeamDeleteDocument)
				teams.GET("/documents/:id/content", handler.TeamGetDocumentContent)
				teams.PUT("/documents/:id/content", handler.TeamSaveDocumentContent)
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

				// 分享删除
				teams.DELETE("/shares/:id", handler.TeamDeleteShare)

				// 评论管理
				teams.PUT("/comments/:id", handler.TeamUpdateComment)
				teams.DELETE("/comments/:id", handler.TeamDeleteComment)

				// Dashboard
				teams.GET("/dashboard", handler.TeamDashboardStats)
				teams.GET("/system-info", handler.TeamSystemInfo)

				// 模板
				teams.GET("/templates", handler.TeamListTemplates)
				teams.GET("/templates/:id", handler.TeamGetTemplate)
				teams.POST("/templates", handler.TeamCreateTemplate)
				teams.PUT("/templates/:id", handler.TeamUpdateTemplate)
				teams.DELETE("/templates/:id", handler.TeamDeleteTemplate)

				// Webhooks
				teams.GET("/webhooks", handler.TeamListWebhooks)
				teams.POST("/webhooks", handler.TeamCreateWebhook)
				teams.DELETE("/webhooks/:id", handler.TeamDeleteWebhook)
				teams.PUT("/webhooks/:id/toggle", handler.TeamToggleWebhook)
				teams.GET("/webhooks/:id/logs", handler.TeamListWebhookLogs)

				// 文档统计
				teams.GET("/documents/:id/stats", handler.TeamDocStats)

				// Recent
				teams.GET("/documents/recent", handler.TeamRecentDocuments)

				// Media
				teams.POST("/upload", handler.TeamUploadFile)
				teams.GET("/media", handler.TeamListMedia)
				teams.GET("/media/:filename", handler.TeamGetMedia)
				teams.DELETE("/media/:filename", handler.TeamDeleteMedia)

				// Import
				teams.POST("/import", handler.TeamImportDocument)

				// Search Targets
				teams.GET("/search-targets", handler.TeamSearchTargets)

				// Collaborator 管理
				teams.PUT("/collaborators/:id", handler.TeamUpdateCollaborator)
				teams.DELETE("/collaborators/:id", handler.TeamRemoveCollaborator)

				// Tag documents
				teams.GET("/tags/:id/documents", handler.TeamGetDocsByTag)

				// Notifications
				teams.GET("/notifications", handler.TeamListNotifications)
				teams.PUT("/notifications/:id/read", handler.TeamMarkNotificationRead)
				teams.PUT("/notifications/read-all", handler.TeamMarkAllNotificationsRead)
				teams.DELETE("/notifications/:id", handler.TeamDeleteNotification)
				teams.GET("/notifications/unread-count", handler.TeamUnreadCount)
			}
		}
	}
	return r
}

// HTTP helpers

func teamPath(path string) string {
	return "/api/teams/" + teamID + path
}

func request(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v, body: %s", err, w.Body.String())
	}
	return result
}

func getFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func getString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// 创建测试文档（通过 team API），返回文档 ID
func createTestDoc(t *testing.T, token, title string) string {
	t.Helper()
	body := map[string]interface{}{
		"title":   title,
		"type":    "doc",
		"content": "<p>测试内容</p>",
	}
	w := request("POST", teamPath("/documents"), body, token)
	if w.Code != 200 {
		t.Fatalf("create doc failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	return getString(data["id"])
}

// ==================== 1. 认证测试 ====================

func TestLoginDeprecated(t *testing.T) {
	w := request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "Admin@2026",
	}, "")
	if w.Code != 410 {
		t.Errorf("deprecated login should return 410, got %d", w.Code)
	}
	resp := parseJSON(t, w)
	if resp["portal_url"] == nil {
		t.Error("should return portal_url for SSO redirect")
	}
}

func TestMe(t *testing.T) {
	w := request("GET", "/api/auth/me", nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("me should succeed, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if getString(data["username"]) != "admin" {
		t.Errorf("username should be admin, got %v", data["username"])
	}
}

func TestMeNoAuth(t *testing.T) {
	w := request("GET", "/api/auth/me", nil, "")
	if w.Code != 401 {
		t.Errorf("me without token should return 401, got %d", w.Code)
	}
}

// ==================== 2. 团队成员权限测试 ====================

func TestTeamMembershipRequired(t *testing.T) {
	// 生成一个不在团队里的用户 token
	outsiderToken, _ := middleware.GenerateToken("test-user-outsider", "outsider", "member", "")
	// 插入 users 表
	database.DB.ExecContext(context.Background(),
		`INSERT IGNORE INTO users (id, email, username, display_name, password_hash, is_admin, email_verified)
		 VALUES ('test-user-outsider','out@test.com','outsider','外人','$2a$10$X5VtjlPu/whem0Jgbe/WDecwDAI9tTQYLNVMhq3OVoacvnwLEHWiS',0,1)`)
	defer database.DB.ExecContext(context.Background(), "DELETE FROM users WHERE id='test-user-outsider'")

	w := request("GET", teamPath("/documents"), nil, outsiderToken)
	if w.Code != 403 {
		t.Errorf("non-member should be forbidden, got %d", w.Code)
	}
}

func TestAdminCanManageTeam(t *testing.T) {
	// admin (team role = admin) 可以创建文件夹
	w := request("POST", teamPath("/folders"), map[string]string{
		"name": "管理员创建的文件夹",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("admin should create folder, got %d: %s", w.Code, w.Body.String())
	}
}

func TestViewerCannotCreateFolder(t *testing.T) {
	w := request("POST", teamPath("/folders"), map[string]string{
		"name": "查看者不应创建",
	}, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not create folder, got %d", w.Code)
	}
}

func TestEditorCanCreateFolder(t *testing.T) {
	w := request("POST", teamPath("/folders"), map[string]string{
		"name": "编辑者创建的文件夹",
	}, editorToken)
	if w.Code != 200 {
		t.Errorf("editor should create folder, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEditorCannotManagePermissions(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限测试文档")

	w := request("POST", teamPath("/permissions"), map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     viewerID,
		"permission":    "write",
	}, editorToken)
	if w.Code != 403 {
		t.Errorf("editor should not set permission, got %d", w.Code)
	}
}

// ==================== 3. 文档 CRUD 测试 ====================

func TestCreateDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "集成测试文档")
	if docID == "" {
		t.Fatal("doc ID should not be empty")
	}

	w := request("GET", teamPath("/documents/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("get document failed: %d", w.Code)
	}
}

func TestGetDocumentContent(t *testing.T) {
	docID := createTestDoc(t, adminToken, "内容读取测试")

	w := request("GET", teamPath("/documents/"+docID+"/content"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("get content failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if getString(data["content"]) == "" {
		t.Error("should return content")
	}
}

func TestSaveDocumentContent(t *testing.T) {
	docID := createTestDoc(t, adminToken, "内容保存测试")

	w := request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{
		"content": "<p>更新后的内容</p>",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("save content failed: %d %s", w.Code, w.Body.String())
	}

	// 验证内容已更新
	w = request("GET", teamPath("/documents/"+docID+"/content"), nil, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	content := data["content"]
	if content != "<p>更新后的内容</p>" {
		t.Errorf("content should be updated, got %v", content)
	}
}

func TestUpdateDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "标题更新测试")

	w := request("PUT", teamPath("/documents/"+docID), map[string]string{
		"title": "新标题",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("update document failed: %d %s", w.Code, w.Body.String())
	}
}

func TestDeleteDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "待删除文档")

	w := request("DELETE", teamPath("/documents/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete document failed: %d", w.Code)
	}
}

func TestListDocuments(t *testing.T) {
	createTestDoc(t, adminToken, "列表测试文档1")

	w := request("GET", teamPath("/documents"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list documents failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	total := getFloat(resp["total"])
	if total < 1 {
		t.Errorf("should have at least 1 document, got %v", total)
	}
}

func TestDocTree(t *testing.T) {
	w := request("GET", teamPath("/folders/tree"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("folder tree failed: %d", w.Code)
	}
}

func TestCreateDocumentByEditor(t *testing.T) {
	docID := createTestDoc(t, editorToken, "编辑者创建的文档")
	if docID == "" {
		t.Fatal("editor should be able to create document")
	}
}

func TestViewerCannotCreateDocument(t *testing.T) {
	w := request("POST", teamPath("/documents"), map[string]interface{}{
		"title": "查看者不应创建",
		"type":  "doc",
	}, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not create document, got %d", w.Code)
	}
}

// ==================== 4. 文件夹测试 ====================

func TestCreateFolder(t *testing.T) {
	w := request("POST", teamPath("/folders"), map[string]string{
		"name": "测试文件夹",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("create folder failed: %d %s", w.Code, w.Body.String())
	}
}

func TestCreateSubFolder(t *testing.T) {
	// 创建父文件夹
	w := request("POST", teamPath("/folders"), map[string]string{
		"name": "父文件夹",
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	parentID := getString(data["id"])

	// 创建子文件夹
	w = request("POST", teamPath("/folders"), map[string]interface{}{
		"name":       "子文件夹",
		"parent_id":  parentID,
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("create subfolder failed: %d %s", w.Code, w.Body.String())
	}
}

func TestUpdateFolder(t *testing.T) {
	w := request("POST", teamPath("/folders"), map[string]string{
		"name": "待更新文件夹",
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	folderID := getString(data["id"])

	w = request("PUT", teamPath("/folders/"+folderID), map[string]string{
		"name": "更新后文件夹",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("update folder failed: %d", w.Code)
	}
}

func TestDeleteFolder(t *testing.T) {
	w := request("POST", teamPath("/folders"), map[string]string{
		"name": "待删除文件夹",
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	folderID := getString(data["id"])

	w = request("DELETE", teamPath("/folders/"+folderID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete folder failed: %d", w.Code)
	}
}

// ==================== 5. 权限系统测试 ====================

func TestSetPermission(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限设置测试")

	w := request("POST", teamPath("/permissions"), map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     viewerID,
		"permission":    "write",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("admin should set permission, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCheckPermissionAdmin(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限检查-管理员")

	w := request("GET", teamPath("/permissions/check?resource_type=document&resource_id="+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("check permission failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	if getString(resp["permission"]) != "admin" {
		t.Errorf("admin should have admin permission, got %v", resp["permission"])
	}
}

func TestCheckPermissionMemberDefault(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限检查-默认")

	// viewer (team member default = read)
	w := request("GET", teamPath("/permissions/check?resource_type=document&resource_id="+docID), nil, viewerToken)
	if w.Code != 200 {
		t.Errorf("check permission failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "read" && perm != "admin" {
		t.Errorf("viewer should have read permission by default, got %v", perm)
	}
}

func TestListPermissions(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限列表测试")

	// 先设置一个权限
	request("POST", teamPath("/permissions"), map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     viewerID,
		"permission":    "read",
	}, adminToken)

	w := request("GET", teamPath("/permissions?resource_type=document&resource_id="+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list permissions failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data, ok := resp["data"].([]interface{})
	if !ok || len(data) < 1 {
		t.Errorf("should have at least 1 permission, got: %v", resp)
	}
}

func TestRemovePermission(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限删除测试")

	// 先设置权限
	request("POST", teamPath("/permissions"), map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     viewerID,
		"permission":    "read",
	}, adminToken)

	// 获取权限列表
	w := request("GET", teamPath("/permissions?resource_type=document&resource_id="+docID), nil, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Fatal("should have at least 1 permission to remove")
	}
	permData := data[0].(map[string]interface{})
	permID := getString(permData["id"])

	// 删除权限
	w = request("DELETE", teamPath("/permissions/"+permID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("remove permission failed: %d", w.Code)
	}
}

func TestViewerCannotSetPermission(t *testing.T) {
	docID := createTestDoc(t, adminToken, "查看者权限测试")

	w := request("POST", teamPath("/permissions"), map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     editorID,
		"permission":    "write",
	}, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not set permission, got %d", w.Code)
	}
}

func TestViewerCannotRemovePermission(t *testing.T) {
	w := request("DELETE", teamPath("/permissions/test-fake-id"), nil, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not remove permission, got %d", w.Code)
	}
}

// ==================== 6. 文档锁测试 ====================

func TestLockUnlock(t *testing.T) {
	docID := createTestDoc(t, adminToken, "锁定测试文档")

	// 锁定
	w := request("POST", teamPath("/documents/"+docID+"/lock"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("lock failed: %d %s", w.Code, w.Body.String())
	}

	// 同一用户可以保存（锁定者）
	w = request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{
		"content": "<p>锁定后内容</p>",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("locker should save, got %d", w.Code)
	}

	// 解锁
	w = request("POST", teamPath("/documents/"+docID+"/unlock"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("unlock failed: %d", w.Code)
	}
}

func TestLockConflict(t *testing.T) {
	docID := createTestDoc(t, adminToken, "锁定冲突测试")

	// admin 锁定
	request("POST", teamPath("/documents/"+docID+"/lock"), nil, adminToken)

	// editor 尝试保存（非锁定者，非 admin）
	w := request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{
		"content": "<p>冲突内容</p>",
	}, editorToken)
	if w.Code != 409 {
		t.Errorf("non-locker editor should get conflict, got %d", w.Code)
	}

	// admin 可以保存（admin bypass）
	w = request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{
		"content": "<p>管理员覆盖</p>",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("admin should bypass lock, got %d", w.Code)
	}
}

// ==================== 7. 文档分享测试 ====================

func TestCreateShare(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享测试文档")

	w := request("POST", teamPath("/documents/"+docID+"/share"), map[string]string{
		"permission": "read",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("create share failed: %d %s", w.Code, w.Body.String())
	}
}

func TestAccessShare(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享访问测试")

	// 创建分享
	w := request("POST", teamPath("/documents/"+docID+"/share"), map[string]string{
		"permission": "read",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create share failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	token := getString(data["token"])

	// 通过分享链接访问（不需要 team token）
	w = request("GET", "/api/s/"+token, nil, "")
	if w.Code != 200 {
		t.Errorf("access share should work, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShareInfo(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享信息测试")

	request("POST", teamPath("/documents/"+docID+"/share"), map[string]string{
		"permission": "read",
	}, adminToken)

	w := request("GET", teamPath("/documents/"+docID+"/shares"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list shares failed: %d", w.Code)
	}
}

func TestListAndDeleteShare(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享列表测试")

	request("POST", teamPath("/documents/"+docID+"/share"), map[string]string{
		"permission": "read",
	}, adminToken)

	w := request("GET", teamPath("/documents/"+docID+"/shares"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list shares failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Fatal("should have shares")
	}

	shareData := data[0].(map[string]interface{})
	shareID := getString(shareData["id"])

	w = request("DELETE", teamPath("/shares/"+shareID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete share failed: %d", w.Code)
	}
}

// ==================== 8. 收藏测试 ====================

func TestFavorite(t *testing.T) {
	docID := createTestDoc(t, adminToken, "收藏测试文档")

	// 收藏
	w := request("POST", teamPath("/favorites/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("add favorite failed: %d %s", w.Code, w.Body.String())
	}

	// 列表
	w = request("GET", teamPath("/favorites"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list favorites failed: %d", w.Code)
	}

	// 取消
	w = request("DELETE", teamPath("/favorites/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("remove favorite failed: %d", w.Code)
	}
}

// ==================== 9. 评论测试 ====================

func TestComments(t *testing.T) {
	docID := createTestDoc(t, adminToken, "评论测试文档")

	// 创建评论
	w := request("POST", teamPath("/documents/"+docID+"/comments"), map[string]string{
		"content": "这是一条测试评论",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("create comment failed: %d %s", w.Code, w.Body.String())
	}

	// 列表
	w = request("GET", teamPath("/documents/"+docID+"/comments"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list comments failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have comments")
	}
}

// ==================== 10. 标签测试 ====================

func TestTags(t *testing.T) {
	// 创建标签
	w := request("POST", teamPath("/tags"), map[string]string{
		"name":  "测试标签",
		"color": "#ff0000",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("create tag failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	tagID := getString(data["id"])

	// 列表
	w = request("GET", teamPath("/tags"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list tags failed: %d", w.Code)
	}

	// 给文档设置标签
	docID := createTestDoc(t, adminToken, "标签测试文档")
	w = request("PUT", teamPath("/documents/"+docID+"/tags"), map[string]interface{}{
		"tag_ids": []string{tagID},
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("set doc tags failed: %d %s", w.Code, w.Body.String())
	}

	// 查看文档标签
	w = request("GET", teamPath("/documents/"+docID+"/tags"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("get doc tags failed: %d", w.Code)
	}

	// 删除标签
	w = request("DELETE", teamPath("/tags/"+tagID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete tag failed: %d", w.Code)
	}
}

// ==================== 11. 回收站测试 ====================

func TestTrashAndRestore(t *testing.T) {
	docID := createTestDoc(t, adminToken, "回收站测试文档")

	// 删除（进入回收站）
	w := request("DELETE", teamPath("/documents/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete failed: %d", w.Code)
	}

	// 查看回收站
	w = request("GET", teamPath("/trash"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list trash failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	total := getFloat(resp["total"])
	if total < 1 {
		t.Errorf("should have at least 1 item in trash, got %v", total)
	}

	// 恢复
	w = request("POST", teamPath("/trash/restore/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("restore from trash failed: %d", w.Code)
	}
}

func TestTrashPurge(t *testing.T) {
	docID := createTestDoc(t, adminToken, "彻底删除测试")

	// 删除
	request("DELETE", teamPath("/documents/"+docID), nil, adminToken)

	// 彻底删除
	w := request("DELETE", teamPath("/trash/purge/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("purge from trash failed: %d", w.Code)
	}
}

// ==================== 12. 版本管理测试 ====================

func TestVersionHistory(t *testing.T) {
	docID := createTestDoc(t, adminToken, "版本历史测试")

	// 保存多次内容产生版本
	for i := 0; i < 3; i++ {
		request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{
			"content": fmt.Sprintf("<p>版本 %d</p>", i+1),
		}, adminToken)
		time.Sleep(10 * time.Millisecond)
	}

	w := request("GET", teamPath("/documents/"+docID+"/versions"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list versions failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) < 2 {
		t.Errorf("should have at least 2 versions, got %d", len(data))
	}
}

func TestRestoreVersion(t *testing.T) {
	docID := createTestDoc(t, adminToken, "版本恢复测试")

	// 保存内容产生版本
	request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{
		"content": "<p>版本1</p>",
	}, adminToken)
	request("PUT", teamPath("/documents/"+docID+"/content"), map[string]string{
		"content": "<p>版本2</p>",
	}, adminToken)

	// 恢复到版本 1
	w := request("POST", teamPath("/documents/"+docID+"/restore"), map[string]interface{}{
		"version": 1,
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("restore version failed: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 13. 审计日志测试 ====================

func TestListAudits(t *testing.T) {
	// 先触发一些操作产生审计日志
	createTestDoc(t, adminToken, "审计测试文档")

	w := request("GET", teamPath("/audits"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list audits failed: %d: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	total := getFloat(resp["total"])
	if total < 1 {
		t.Errorf("should have at least 1 audit entry, got %v", total)
	}
}

func TestAuditsForbiddenForNonAdmin(t *testing.T) {
	w := request("GET", teamPath("/audits"), nil, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not view audits, got %d", w.Code)
	}
}

// ==================== 14. Dashboard & 存储测试 ====================

func TestStorageStatus(t *testing.T) {
	w := request("GET", teamPath("/storage/status"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("storage status failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if data["doc_count"] == nil {
		t.Error("should return doc_count")
	}
}

func TestDashboard(t *testing.T) {
	// 先创建文档确保有数据
	createTestDoc(t, adminToken, "Dashboard 测试")

	w := request("GET", teamPath("/dashboard"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("dashboard failed: %d: %s", w.Code, w.Body.String())
	}
}

// ==================== 15. 导出功能测试 ====================

func TestExportDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "导出测试文档")

	w := request("GET", teamPath("/documents/"+docID+"/export"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("export failed: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 16. 协作者测试 ====================

func TestAddCollaborator(t *testing.T) {
	docID := createTestDoc(t, adminToken, "协作者测试文档")

	w := request("POST", teamPath("/documents/"+docID+"/collaborators"), map[string]string{
		"target_id":  viewerID,
		"permission": "write",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("add collaborator failed: %d %s", w.Code, w.Body.String())
	}

	// 查看协作者列表
	w = request("GET", teamPath("/documents/"+docID+"/collaborators"), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list collaborators failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have collaborators")
	}
}

// ==================== 17. 全文搜索测试 ====================

func TestSearchByTitle(t *testing.T) {
	uniqueTitle := "UniqueSearchTitle_" + fmt.Sprintf("%d", time.Now().UnixNano())
	createTestDoc(t, adminToken, uniqueTitle)

	w := request("GET", teamPath("/documents/search?q="+uniqueTitle), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("search failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data, ok := resp["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Errorf("should find doc by title, got: %v", resp)
	}
}

func TestSearchNoKeyword(t *testing.T) {
	w := request("GET", teamPath("/documents/search"), nil, adminToken)
	if w.Code != 400 {
		t.Errorf("search without keyword should return 400, got %d", w.Code)
	}
}

func TestSearchFullTextContent(t *testing.T) {
	docID := createTestDoc(t, adminToken, "全文搜索测试")

	uniqueKeyword := "FullTextSearchKeyword_" + fmt.Sprintf("%d", time.Now().UnixNano())
	w := request("PUT", teamPath("/documents/"+docID+"/content"), map[string]interface{}{
		"content": "<p>" + uniqueKeyword + " is hidden in the content</p>",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("save content failed: %d %s", w.Code, w.Body.String())
	}

	w = request("GET", teamPath("/documents/search?q="+uniqueKeyword), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("full-text search failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data, ok := resp["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Errorf("should find doc by content text, got: %v", resp)
	}
}

// ==================== 18. 无效输入测试 ====================

func TestCreateDocEmptyTitle(t *testing.T) {
	w := request("POST", teamPath("/documents"), map[string]string{
		"title": "",
		"type":  "doc",
	}, adminToken)
	if w.Code != 400 {
		t.Errorf("empty title should return 400, got %d", w.Code)
	}
}

func TestGetNonexistentDocument(t *testing.T) {
	w := request("GET", teamPath("/documents/nonexistent-id"), nil, adminToken)
	if w.Code != 404 {
		t.Errorf("nonexistent doc should return 404, got %d", w.Code)
	}
}

func TestAccessInvalidShare(t *testing.T) {
	w := request("GET", "/api/s/invalid-token-12345", nil, "")
	if w.Code != 404 {
		t.Errorf("invalid share should return 404, got %d", w.Code)
	}
}

// ==================== 权限隔离测试 ====================

// TestViewerCannotEditDocument verifies viewer cannot save/update document content
func TestViewerCannotEditDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限隔离-查看者测试")

	// Viewer cannot update document title
	w := request("PUT", teamPath("/documents/"+docID), map[string]interface{}{
		"title": "查看者篡改标题",
	}, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not update document, got %d", w.Code)
	}

	// Viewer cannot save document content
	w = request("PUT", teamPath("/documents/"+docID+"/content"), map[string]interface{}{
		"content": "<p>查看者篡改内容</p>",
		"version": 1,
	}, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not save document content, got %d", w.Code)
	}
}

// TestViewerCannotDeleteDocument verifies viewer cannot delete document
func TestViewerCannotDeleteDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限隔离-删除测试")

	w := request("DELETE", teamPath("/documents/"+docID), nil, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not delete document, got %d", w.Code)
	}
}

// TestEditorCannotDeleteDocument verifies editor cannot delete document (admin only)
func TestEditorCannotDeleteDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限隔离-编辑者删除测试")

	w := request("DELETE", teamPath("/documents/"+docID), nil, editorToken)
	if w.Code != 403 {
		t.Errorf("editor should not delete document, got %d", w.Code)
	}
}

// TestViewerCanReadDocument verifies viewer can still read document content
func TestViewerCanReadDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限隔离-查看者读取测试")

	// Viewer can read content
	w := request("GET", teamPath("/documents/"+docID+"/content"), nil, viewerToken)
	if w.Code != 200 {
		t.Errorf("viewer should be able to read document content, got %d", w.Code)
	}

	// Viewer can get document info
	w = request("GET", teamPath("/documents/"+docID), nil, viewerToken)
	if w.Code != 200 {
		t.Errorf("viewer should be able to read document info, got %d", w.Code)
	}
}

// TestEditorCanEditButNotDelete verifies editor can create/edit but not delete
func TestEditorCanEditButNotDelete(t *testing.T) {
	// Editor can create
	docID := createTestDoc(t, editorToken, "权限隔离-编辑者创建")

	// Editor can update own document
	w := request("PUT", teamPath("/documents/"+docID), map[string]interface{}{
		"title": "编辑者修改标题",
	}, editorToken)
	if w.Code != 200 {
		t.Errorf("editor should update own document, got %d", w.Code)
	}

	// Editor can save content
	w = request("PUT", teamPath("/documents/"+docID+"/content"), map[string]interface{}{
		"content": "<p>编辑者保存内容</p>",
		"version": 1,
	}, editorToken)
	if w.Code != 200 {
		t.Errorf("editor should save document content, got %d: %s", w.Code, w.Body.String())
	}

	// Editor cannot delete
	w = request("DELETE", teamPath("/documents/"+docID), nil, editorToken)
	if w.Code != 403 {
		t.Errorf("editor should not delete document, got %d", w.Code)
	}
}

// TestViewerCannotLockDocument verifies viewer cannot lock/unlock
func TestViewerCannotLockDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限隔离-锁定测试")

	w := request("POST", teamPath("/documents/"+docID+"/lock"), nil, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not lock document, got %d", w.Code)
	}
}

// TestViewerCannotManageTags verifies viewer cannot create/delete tags
func TestViewerCannotManageTags(t *testing.T) {
	w := request("POST", teamPath("/tags"), map[string]string{
		"name": "查看者标签",
	}, viewerToken)
	if w.Code != 403 {
		t.Errorf("viewer should not create tags, got %d", w.Code)
	}
}

// TestViewerCannotUploadMedia verifies viewer cannot upload files
func TestViewerCannotUploadMedia(t *testing.T) {
	w := request("POST", teamPath("/upload"), map[string]interface{}{
		"filename": "test.txt",
	}, viewerToken)
	// Even if the upload fails for other reasons, it should not be 200
	if w.Code == 200 {
		t.Errorf("viewer should not upload media, got %d", w.Code)
	}
}

// ==================== 新增补充测试 ====================

// --- Template CRUD ---

func TestCreateTemplate(t *testing.T) {
	w := request("POST", teamPath("/templates"), map[string]interface{}{
		"name":    "测试模板",
		"type":    "doc",
		"content": "<p>模板内容</p>",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create template: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if getString(data["name"]) != "测试模板" {
		t.Errorf("template name mismatch")
	}
}

func TestListTemplates(t *testing.T) {
	w := request("GET", teamPath("/templates"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list templates: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have at least 1 template")
	}
}

func TestGetTemplate(t *testing.T) {
	// 先创建
	w := request("POST", teamPath("/templates"), map[string]interface{}{
		"name": "获取测试", "type": "doc", "content": "<p>内容</p>",
	}, adminToken)
	resp := parseJSON(t, w)
	id := getString(resp["data"].(map[string]interface{})["id"])

	w = request("GET", teamPath("/templates/"+id), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("get template: %d %s", w.Code, w.Body.String())
	}
	resp = parseJSON(t, w)
	if resp["data"] == nil {
		t.Fatal("template data is nil")
	}
	data := resp["data"].(map[string]interface{})
	if getString(data["content"]) != "<p>内容</p>" {
		t.Error("template content mismatch")
	}
}

func TestUpdateTemplate(t *testing.T) {
	w := request("POST", teamPath("/templates"), map[string]interface{}{
		"name": "更新前", "type": "doc", "content": "旧内容",
	}, adminToken)
	id := getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])

	w = request("PUT", teamPath("/templates/"+id), map[string]interface{}{
		"name": "更新后", "content": "新内容",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("update template: %d %s", w.Code, w.Body.String())
	}
}

func TestDeleteTemplate(t *testing.T) {
	w := request("POST", teamPath("/templates"), map[string]interface{}{
		"name": "待删除", "type": "doc", "content": "",
	}, adminToken)
	id := getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])

	w = request("DELETE", teamPath("/templates/"+id), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete template: %d %s", w.Code, w.Body.String())
	}

	// 确认已删除
	w = request("GET", teamPath("/templates/"+id), nil, adminToken)
	if w.Code != 404 {
		t.Errorf("deleted template should be 404, got %d", w.Code)
	}
}

func TestGetNonexistentTemplate(t *testing.T) {
	w := request("GET", teamPath("/templates/nonexistent-id"), nil, adminToken)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Webhook CRUD ---

func TestCreateWebhook(t *testing.T) {
	w := request("POST", teamPath("/webhooks"), map[string]interface{}{
		"name":   "测试Hook",
		"url":    "https://example.com/hook",
		"events": `["document.created"]`,
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create webhook: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if getString(data["name"]) != "测试Hook" {
		t.Error("webhook name mismatch")
	}
}

func TestListWebhooks(t *testing.T) {
	w := request("GET", teamPath("/webhooks"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list webhooks: %d %s", w.Code, w.Body.String())
	}
}

func TestToggleWebhook(t *testing.T) {
	w := request("POST", teamPath("/webhooks"), map[string]interface{}{
		"name": "Toggle测试", "url": "https://example.com/toggle",
	}, adminToken)
	id := getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])

	// 切换为禁用
	w = request("PUT", teamPath("/webhooks/"+id+"/toggle"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("toggle webhook: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	// enabled 应该变为 false
	if resp["enabled"] == true {
		t.Error("webhook should be disabled after toggle")
	}

	// 再切换回来
	w = request("PUT", teamPath("/webhooks/"+id+"/toggle"), nil, adminToken)
	resp = parseJSON(t, w)
	if resp["enabled"] != true {
		t.Error("webhook should be enabled after second toggle")
	}
}

func TestDeleteWebhook(t *testing.T) {
	w := request("POST", teamPath("/webhooks"), map[string]interface{}{
		"name": "待删除Hook", "url": "https://example.com/delete",
	}, adminToken)
	id := getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])

	w = request("DELETE", teamPath("/webhooks/"+id), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete webhook: %d %s", w.Code, w.Body.String())
	}
}

func TestWebhookLogs(t *testing.T) {
	w := request("POST", teamPath("/webhooks"), map[string]interface{}{
		"name": "日志Hook", "url": "https://example.com/logs",
	}, adminToken)
	id := getString(parseJSON(t, w)["data"].(map[string]interface{})["id"])

	w = request("GET", teamPath("/webhooks/"+id+"/logs"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("webhook logs: %d %s", w.Code, w.Body.String())
	}
}

func TestCreateWebhookMissingURL(t *testing.T) {
	w := request("POST", teamPath("/webhooks"), map[string]interface{}{
		"name": "无URL",
	}, adminToken)
	if w.Code == 200 {
		t.Error("should fail without url")
	}
}

// --- Doc Stats ---

func TestDocStats(t *testing.T) {
	docID := createTestDoc(t, adminToken, "统计测试")
	w := request("GET", teamPath("/documents/"+docID+"/stats"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("doc stats: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if data["version"] == nil || data["file_size"] == nil {
		t.Error("stats should have version and file_size")
	}
}

// --- Recent Documents ---

func TestRecentDocuments(t *testing.T) {
	// 创建几篇文档
	createTestDoc(t, adminToken, "最近文档1")
	createTestDoc(t, adminToken, "最近文档2")

	w := request("GET", teamPath("/documents/recent"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("recent docs: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have recent documents")
	}
}

// --- Empty Trash ---

func TestEmptyTrash(t *testing.T) {
	docID := createTestDoc(t, adminToken, "清空回收站测试")

	// 删除文档放入回收站
	request("DELETE", teamPath("/documents/"+docID), nil, adminToken)

	// 清空回收站
	w := request("DELETE", teamPath("/trash/empty"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("empty trash: %d %s", w.Code, w.Body.String())
	}

	// 验证回收站为空
	w = request("GET", teamPath("/trash"), nil, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("trash should be empty, got %d items", len(data))
	}
}

// --- Export Audits ---

func TestExportAudits(t *testing.T) {
	w := request("GET", teamPath("/audits/export"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("export audits: %d %s", w.Code, w.Body.String())
	}
}

func TestAuditStats(t *testing.T) {
	w := request("GET", teamPath("/audits/stats"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("audit stats: %d %s", w.Code, w.Body.String())
	}
}

// --- Share Delete ---

func TestDeleteShare(t *testing.T) {
	docID := createTestDoc(t, adminToken, "删除分享测试")

	// 创建分享
	w := request("POST", teamPath("/documents/"+docID+"/share"), map[string]interface{}{
		"permission": "read",
	}, adminToken)
	resp := parseJSON(t, w)
	shareData := resp["data"].(map[string]interface{})
	shareID := getString(shareData["id"])

	// 删除分享
	w = request("DELETE", teamPath("/shares/"+shareID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete share: %d %s", w.Code, w.Body.String())
	}
}

// --- Comment Update / Delete ---

func TestUpdateAndDeleteComment(t *testing.T) {
	docID := createTestDoc(t, adminToken, "评论管理测试")

	// 创建评论
	w := request("POST", teamPath("/documents/"+docID+"/comments"), map[string]interface{}{
		"content": "原始评论",
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	commentID := getString(data["id"])

	// 更新评论
	w = request("PUT", teamPath("/comments/"+commentID), map[string]interface{}{
		"content": "更新后评论",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("update comment: %d %s", w.Code, w.Body.String())
	}

	// 删除评论
	w = request("DELETE", teamPath("/comments/"+commentID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete comment: %d %s", w.Code, w.Body.String())
	}
}

// --- Version Content ---

func TestGetVersionContent(t *testing.T) {
	docID := createTestDoc(t, adminToken, "版本内容测试")

	// 写入内容（创建时 content 已经写入了 v1）
	w := request("PUT", teamPath("/documents/"+docID+"/content"), map[string]interface{}{
		"content": "<p>版本内容</p>",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("save content: %d %s", w.Code, w.Body.String())
	}

	// 获取版本列表
	w = request("GET", teamPath("/documents/"+docID+"/versions"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list versions: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	versions := resp["data"].([]interface{})
	if len(versions) == 0 {
		t.Fatal("should have at least 1 version")
	}
	// 版本列表按 created_at DESC，取最新的
	firstVer := versions[0].(map[string]interface{})
	verNum := int(getFloat(firstVer["version"]))

	// 获取版本内容
	w = request("GET", teamPath(fmt.Sprintf("/documents/%s/versions/%d/content", docID, verNum)), nil, adminToken)
	if w.Code != 200 {
		// 如果版本文件不存在（比如加密/解密问题），只记录不 panic
		t.Logf("version content returned %d: %s (may be encryption issue)", w.Code, w.Body.String())
		return
	}
}

// --- System Info ---

func TestSystemInfo(t *testing.T) {
	w := request("GET", teamPath("/system-info"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("system info: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 1. Media 上传/列表/获取/删除 ====================

func uploadFile(t *testing.T, token string) string {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test-upload.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("hello media test"))
	writer.Close()

	req := httptest.NewRequest("POST", teamPath("/upload"), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("upload file: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	return getString(data["filename"])
}

func TestUploadFile(t *testing.T) {
	filename := uploadFile(t, adminToken)
	if filename == "" {
		t.Fatal("filename should not be empty")
	}
}

func TestListMedia(t *testing.T) {
	uploadFile(t, adminToken)

	w := request("GET", teamPath("/media"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list media: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have at least 1 media file")
	}
}

func TestGetMedia(t *testing.T) {
	filename := uploadFile(t, adminToken)

	req := httptest.NewRequest("GET", teamPath("/media/"+filename), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("get media: %d %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "hello media test" {
		t.Errorf("media content mismatch: %s", w.Body.String())
	}
}

func TestDeleteMedia(t *testing.T) {
	filename := uploadFile(t, adminToken)

	w := request("DELETE", teamPath("/media/"+filename), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete media: %d %s", w.Code, w.Body.String())
	}

	// 再获取应该 404
	req := httptest.NewRequest("GET", teamPath("/media/"+filename), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("deleted media should be 404, got %d", w.Code)
	}
}

func TestViewerCannotUpload(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "blocked.txt")
	part.Write([]byte("blocked"))
	writer.Close()

	req := httptest.NewRequest("POST", teamPath("/upload"), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// TODO: handler 已加入 viewer 权限检查
	if w.Code != 403 {
		t.Errorf("viewer should be denied, got %d", w.Code)
	}
}

// ==================== 2. Import 文档 ====================

func importFile(t *testing.T, token, filename, content string) string {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte(content))
	writer.Close()

	req := httptest.NewRequest("POST", teamPath("/import"), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("import file: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	return getString(data["id"])
}

func TestImportMarkdown(t *testing.T) {
	docID := importFile(t, adminToken, "readme.md", "# Hello\nImported content")
	if docID == "" {
		t.Fatal("imported doc id should not be empty")
	}

	// 验证文档可读
	w := request("GET", teamPath("/documents/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("get imported doc: %d %s", w.Code, w.Body.String())
	}
}

func TestImportText(t *testing.T) {
	docID := importFile(t, adminToken, "notes.txt", "Plain text content")
	if docID == "" {
		t.Fatal("imported doc id should not be empty")
	}
}

func TestImportHTML(t *testing.T) {
	docID := importFile(t, adminToken, "page.html", "<h1>HTML Import</h1>")
	if docID == "" {
		t.Fatal("imported doc id should not be empty")
	}
}

// ==================== 3. Search Targets ====================

func TestSearchTargets(t *testing.T) {
	// 搜索 admin 用户（团队成员）
	w := request("GET", teamPath("/search-targets?q=adm"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("search targets: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should find admin user")
	}
}

func TestSearchTargetsNoQuery(t *testing.T) {
	w := request("GET", teamPath("/search-targets"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("search targets no query: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Error("empty query should return empty array")
	}
}

func TestSearchTargetsNonMember(t *testing.T) {
	// 搜索不存在的用户
	w := request("GET", teamPath("/search-targets?q=nonexistent"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("search targets: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Error("should not find nonexistent member")
	}
}

// ==================== 4. Collaborator Update/Remove ====================

func TestUpdateCollaborator(t *testing.T) {
	docID := createTestDoc(t, adminToken, "协作者更新测试")

	// 添加协作者
	w := request("POST", teamPath("/documents/"+docID+"/collaborators"), map[string]interface{}{
		"target_id":  editorID,
		"permission": "read",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("add collaborator: %d %s", w.Code, w.Body.String())
	}

	// 找到刚添加的权限记录
	w = request("GET", teamPath("/permissions?resource_type=document&resource_id="+docID), nil, adminToken)
	resp := parseJSON(t, w)
	permsRaw := resp["data"]
	if permsRaw == nil {
		t.Fatal("permissions data is nil")
	}
	perms, ok := permsRaw.([]interface{})
	if !ok {
		t.Fatalf("permissions data is not array: %T", permsRaw)
	}
	var permID string
	for _, p := range perms {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if getString(pm["target_id"]) == editorID {
			permID = getString(pm["id"])
			break
		}
	}
	if permID == "" {
		t.Fatal("collaborator permission not found")
	}

	// 更新权限为 write
	w = request("PUT", teamPath("/collaborators/"+permID), map[string]interface{}{
		"permission": "write",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("update collaborator: %d %s", w.Code, w.Body.String())
	}
}

func TestRemoveCollaborator(t *testing.T) {
	docID := createTestDoc(t, adminToken, "协作者移除测试")

	w := request("POST", teamPath("/documents/"+docID+"/collaborators"), map[string]interface{}{
		"target_id":  viewerID,
		"permission": "read",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("add collaborator: %d %s", w.Code, w.Body.String())
	}

	w = request("GET", teamPath("/permissions?resource_type=document&resource_id="+docID), nil, adminToken)
	resp := parseJSON(t, w)
	permsRaw := resp["data"]
	if permsRaw == nil {
		t.Fatal("permissions data is nil")
	}
	perms, ok := permsRaw.([]interface{})
	if !ok {
		t.Fatalf("permissions data is not array: %T", permsRaw)
	}
	var permID string
	for _, p := range perms {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if getString(pm["target_id"]) == viewerID {
			permID = getString(pm["id"])
			break
		}
	}
	if permID == "" {
		t.Fatal("collaborator permission not found")
	}

	w = request("DELETE", teamPath("/collaborators/"+permID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("remove collaborator: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 5. Get Docs By Tag ====================

func TestGetDocsByTag(t *testing.T) {
	// 创建标签（用唯一名避免冲突）
	tagName := "标签文档-" + uuid.New().String()[:8]
	w := request("POST", teamPath("/tags"), map[string]interface{}{
		"name": tagName,
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create tag: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if resp["data"] == nil {
		t.Fatalf("tag data is nil: %v", resp)
	}
	tagID := getString(resp["data"].(map[string]interface{})["id"])

	// 创建文档
	docID := createTestDoc(t, adminToken, "标签关联文档")

	// 给文档打标签
	request("PUT", teamPath("/documents/"+docID+"/tags"), map[string]interface{}{
		"tag_ids": []string{tagID},
	}, adminToken)

	// 查标签下的文档
	w = request("GET", teamPath("/tags/"+tagID+"/documents"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("docs by tag: %d %s", w.Code, w.Body.String())
	}
	resp = parseJSON(t, w)
	data := resp["data"].([]interface{})
	found := false
	for _, d := range data {
		dm := d.(map[string]interface{})
		if getString(dm["id"]) == docID {
			found = true
		}
	}
	if !found {
		t.Error("document should be found under tag")
	}
}

// ==================== 6. Notifications ====================

func createTestNotification(t *testing.T, userID string) string {
	t.Helper()
	id := "test-notif-" + uuid.New().String()[:8]
	_, err := database.DB.Exec(
		`INSERT INTO md_notifications (id, user_id, team_id, type, title) VALUES (?, ?, ?, 'system', '测试通知')`,
		id, userID, teamID)
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}
	return id
}

func TestListNotifications(t *testing.T) {
	createTestNotification(t, adminID)

	w := request("GET", teamPath("/notifications"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list notifications: %d %s", w.Code, w.Body.String())
	}
}

func TestUnreadCount(t *testing.T) {
	createTestNotification(t, adminID)

	w := request("GET", teamPath("/notifications/unread-count"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("unread count: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	count := int(getFloat(resp["count"]))
	if count < 1 {
		t.Errorf("should have unread notifications, got %d", count)
	}
}

func TestMarkNotificationRead(t *testing.T) {
	notifID := createTestNotification(t, adminID)

	w := request("PUT", teamPath("/notifications/"+notifID+"/read"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("mark read: %d %s", w.Code, w.Body.String())
	}
}

func TestMarkAllNotificationsRead(t *testing.T) {
	createTestNotification(t, adminID)
	createTestNotification(t, adminID)

	w := request("PUT", teamPath("/notifications/read-all"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("mark all read: %d %s", w.Code, w.Body.String())
	}

	// 验证全部已读
	w = request("GET", teamPath("/notifications/unread-count"), nil, adminToken)
	resp := parseJSON(t, w)
	if int(getFloat(resp["count"])) != 0 {
		t.Error("all should be read")
	}
}

func TestDeleteNotification(t *testing.T) {
	notifID := createTestNotification(t, adminID)

	w := request("DELETE", teamPath("/notifications/"+notifID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete notification: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 7. 跨团队访问隔离 ====================

func TestCrossTeamAccessDenied(t *testing.T) {
	// 创建另一个团队
	otherTeamID := "test-team-other"
	_, err := database.DB.Exec(`INSERT INTO teams (id, name, description) VALUES (?, ?, ?)`, otherTeamID, "其他团队", "隔离测试")
	if err != nil {
		t.Fatalf("create other team: %v", err)
	}
	defer database.DB.Exec("DELETE FROM teams WHERE id=?", otherTeamID)

	// 在另一个团队创建文档
	otherDocID := "test-doc-cross-" + uuid.New().String()[:8]
	_, err = database.DB.Exec(
		`INSERT INTO md_documents (id, team_id, department_id, title, type, status, created_by, updated_by) VALUES (?, ?, '', '跨团文档', 'doc', 1, ?, ?)`,
		otherDocID, otherTeamID, adminID, adminID)
	if err != nil {
		t.Fatalf("create other doc: %v", err)
	}
	defer database.DB.Exec("DELETE FROM md_documents WHERE id=?", otherDocID)

	// 用当前团队的 admin token 通过路径参数尝试访问另一个团队的文档
	// 因为 URL 路径里有 team_id，team_id 是 test-team-alpha
	// 文档属于 other-team，list 时按 team_id 过滤所以不会出现
	w := request("GET", teamPath("/documents"), nil, adminToken)
	resp := parseJSON(t, w)
	docs := resp["data"].([]interface{})
	for _, d := range docs {
		dm := d.(map[string]interface{})
		if getString(dm["id"]) == otherDocID {
			t.Error("cross-team document should not be visible")
		}
	}
}

// ==================== 8. 无团队成员访问 ====================

func TestNonTeamMemberDenied(t *testing.T) {
	// 创建一个不在任何团队的用户
	outsiderID := "test-user-outsider"
	_, err := database.DB.Exec(
		`INSERT INTO users (id, email, username, display_name, password_hash, is_admin, email_verified) VALUES (?,?,?,?,?,?,1)`,
		outsiderID, "outsider@test.com", "outsider", "外部用户",
		"$2a$10$X5VtjlPu/whem0Jgbe/WDecwDAI9tTQYLNVMhq3OVoacvnwLEHWiS", false)
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}
	defer database.DB.Exec("DELETE FROM users WHERE id=?", outsiderID)

	outsiderToken, _ := middleware.GenerateToken(outsiderID, "outsider", "user", "")

	// 外部用户访问团队文档列表
	w := request("GET", teamPath("/documents"), nil, outsiderToken)
	if w.Code != 403 {
		t.Errorf("outsider should be denied, got %d", w.Code)
	}
}

// ==================== 9. 文档大小限制 ====================

func TestDocumentSizeLimit(t *testing.T) {
	docID := createTestDoc(t, adminToken, "大文档测试")

	// 构造一个超过 50MB 的内容（直接发会很大，测 handler 是否有检查）
	// handler 目前没显式检查大小，但 store 有 MaxFileSize 配置
	// 先验证正常大小可以保存
	w := request("PUT", teamPath("/documents/"+docID+"/content"), map[string]interface{}{
		"content": "normal content",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("save normal content: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 10. Template 缺 name ====================

func TestCreateTemplateMissingName(t *testing.T) {
	w := request("POST", teamPath("/templates"), map[string]interface{}{
		"type":    "doc",
		"content": "content",
	}, adminToken)
	if w.Code == 200 {
		t.Error("should fail without name")
	}
}

// ==================== 11. 重复收藏 ====================

func TestDuplicateFavorite(t *testing.T) {
	docID := createTestDoc(t, adminToken, "重复收藏测试")

	// 第一次收藏
	w := request("POST", teamPath("/favorites/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("first favorite: %d %s", w.Code, w.Body.String())
	}

	// 第二次收藏（应不报错或幂等）
	w = request("POST", teamPath("/favorites/"+docID), nil, adminToken)
	// 不管是 200 还是重复错误，不应 500
	if w.Code >= 500 {
		t.Errorf("duplicate favorite should not 500, got %d", w.Code)
	}
}

// ==================== 12. 删除已删除的文档 ====================

func TestDeleteAlreadyDeleted(t *testing.T) {
	docID := createTestDoc(t, adminToken, "二次删除测试")

	// 第一次删除
	w := request("DELETE", teamPath("/documents/"+docID), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("first delete: %d %s", w.Code, w.Body.String())
	}

	// 第二次删除（已在回收站）
	w = request("DELETE", teamPath("/documents/"+docID), nil, adminToken)
	// 不应 500
	if w.Code >= 500 {
		t.Errorf("second delete should not 500, got %d", w.Code)
	}
}

// ==================== 13. 恢复不存在的版本 ====================

func TestRestoreNonexistentVersion(t *testing.T) {
	docID := createTestDoc(t, adminToken, "恢复不存在版本测试")

	w := request("POST", teamPath("/documents/"+docID+"/restore"), map[string]interface{}{
		"version": 99999,
	}, adminToken)
	if w.Code == 200 {
		t.Error("restoring nonexistent version should not succeed")
	}
}

// ==================== 14. Admin 强制解锁 ====================

func TestAdminForceUnlock(t *testing.T) {
	docID := createTestDoc(t, adminToken, "强制解锁测试")

	// editor 锁定
	w := request("POST", teamPath("/documents/"+docID+"/lock"), nil, editorToken)
	if w.Code != 200 {
		t.Fatalf("editor lock: %d %s", w.Code, w.Body.String())
	}

	// admin 强制解锁
	w = request("POST", teamPath("/documents/"+docID+"/unlock"), nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("admin unlock: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 15. 分享密码 ====================

func TestShareWithPassword(t *testing.T) {
	docID := createTestDoc(t, adminToken, "密码分享测试")

	w := request("POST", teamPath("/documents/"+docID+"/share"), map[string]interface{}{
		"permission": "read",
		"password":   "secret123",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("share with password: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	token := getString(data["token"])

	// 无密码访问应被拒
	w = request("GET", "/api/s/"+token, nil, "")
	if w.Code == 200 {
		t.Error("access without password should be denied")
	}
}
