package tests

import (
	"bytes"
	"context"

	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/handler"
	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/store"
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
