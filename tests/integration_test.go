package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	adminToken    string
	deptAdmin1Tkn string
	member1Tkn    string
	member2Tkn    string

	// 测试用户 ID
	adminID    string
	deptAdmin1ID string
	member1ID  string
	member2ID  string

	// 测试部门 ID
	dept1ID string // 研发部
	dept2ID string // 市场部

	// 存储临时目录
	testStorageRoot string
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// 配置
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "mist_team:MistTeam@2026@tcp(127.0.0.1:3306)/mist_team?charset=utf8mb4&parseTime=true&loc=Local"
	}
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

	// 临时存储目录
	tmpDir, err := os.MkdirTemp("", "mistdocs-test-*")
	if err != nil {
		panic(err)
	}
	testStorageRoot = tmpDir
	config.C.Storage.Root = tmpDir

	// 连接数据库
	if err := database.Init(config.C.Database); err != nil {
		fmt.Printf("SKIP: cannot connect database: %v\n", err)
		os.Exit(0)
	}
	defer database.Close()

	// 初始化文件存储
	store.Init()

	// 清理测试数据并创建测试数据
	cleanTestData()
	setupTestData()

	// 构建路由
	router = buildRouter()

	// 生成 token
	adminToken, _ = middleware.GenerateToken(adminID, "admin", "super_admin", "")
	deptAdmin1Tkn, _ = middleware.GenerateToken(deptAdmin1ID, "dept_admin1", "dept_admin", dept1ID)
	member1Tkn, _ = middleware.GenerateToken(member1ID, "member1", "member", dept1ID)
	member2Tkn, _ = middleware.GenerateToken(member2ID, "member2", "member", dept2ID)

	code := m.Run()

	// 清理
	cleanTestData()
	os.RemoveAll(testStorageRoot)

	os.Exit(code)
}

func cleanTestData() {
	db := database.DB
	ctx := context.Background()

	// 按外键依赖顺序删除
	tables := []string{
		"md_webhook_logs", "md_webhooks",
		"md_notifications", "md_comments",
		"md_shares", "md_favorites",
		"md_doc_tags", "md_tags",
		"md_audits", "md_permissions",
		"md_versions", "md_documents",
		"md_folders",
		"md_keys",
		"md_users", "md_departments",
	}
	for _, t := range tables {
		db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id LIKE 'test-%%'", t))
	}
}

func setupTestData() {
	ctx := context.Background()
	db := database.DB

	// 创建部门
	dept1ID = "test-dept-research"
	dept2ID = "test-dept-market"
	db.ExecContext(ctx, "INSERT INTO md_departments (id,name,parent_id,sort_order,status) VALUES (?,?,?,?,1)",
		dept1ID, "研发部-测试", nil, 0)
	db.ExecContext(ctx, "INSERT INTO md_departments (id,name,parent_id,sort_order,status) VALUES (?,?,?,?,1)",
		dept2ID, "市场部-测试", nil, 1)

	// 创建用户
	adminID = "u_admin" // 已存在
	deptAdmin1ID = "test-user-deptadmin1"
	member1ID = "test-user-member1"
	member2ID = "test-user-member2"

	insertUser(db, ctx, deptAdmin1ID, "dept_admin1", "部门管理员1", "dept_admin", dept1ID)
	insertUser(db, ctx, member1ID, "member1", "普通成员1", "member", dept1ID)
	insertUser(db, ctx, member2ID, "member2", "普通成员2", "member", dept2ID)
}

func insertUser(db *sql.DB, ctx context.Context, id, username, name, role, deptID string) {
	// bcrypt hash of "Test123!" — generated freshly
	hashed := "$2a$10$X5VtjlPu/whem0Jgbe/WDecwDAI9tTQYLNVMhq3OVoacvnwLEHWiS"
	result, err := db.ExecContext(ctx,
		"INSERT INTO md_users (id,username,password,name,role,department_id,status) VALUES (?,?,?,?,?,?,?)",
		id, username, hashed, name, role, deptID, 1)
	if err != nil {
		panic(fmt.Sprintf("insert test user %s failed: %v", username, err))
	}
	_ = result
}

func buildRouter() *gin.Engine {
	r := gin.New()
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery())

	api := r.Group("/api")
	{
		api.POST("/auth/login", handler.Login)
		api.GET("/s/:token", handler.AccessShare)
		api.GET("/s/:token/info", handler.AccessShareInfo)

		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.GET("/auth/me", handler.Me)

			auth.GET("/departments", handler.ListDepartments)
			auth.POST("/departments", handler.CreateDepartment)
			auth.PUT("/departments/:id", handler.UpdateDepartment)
			auth.DELETE("/departments/:id", handler.DeleteDepartment)

			auth.GET("/users", handler.ListUsers)
			auth.POST("/users", handler.CreateUser)
			auth.PUT("/users/:id", handler.UpdateUser)
			auth.DELETE("/users/:id", handler.DeleteUser)
			auth.PUT("/users/:id/reset-password", handler.ResetPassword)

			auth.GET("/docs/tree", handler.DocTree)
			auth.POST("/docs/folders", handler.CreateFolder)
			auth.PUT("/docs/folders/:id", handler.UpdateFolder)
			auth.DELETE("/docs/folders/:id", handler.DeleteFolder)

			auth.GET("/docs/documents", handler.ListDocuments)
			auth.POST("/docs/documents", handler.CreateDocument)
			auth.GET("/docs/documents/:id", handler.GetDocument)
			auth.PUT("/docs/documents/:id", handler.UpdateDocument)
			auth.DELETE("/docs/documents/:id", handler.DeleteDocument)
			auth.GET("/docs/documents/:id/content", handler.GetDocumentContent)
			auth.PUT("/docs/documents/:id/content", handler.SaveDocumentContent)

			auth.POST("/docs/documents/:id/lock", handler.LockDocument)
			auth.POST("/docs/documents/:id/unlock", handler.UnlockDocument)
			auth.GET("/docs/documents/:id/versions", handler.ListVersions)
			auth.POST("/docs/documents/:id/restore", handler.RestoreVersion)
			auth.GET("/docs/documents/:id/versions/:ver/content", handler.GetVersionContent)

			auth.GET("/docs/trash", handler.ListTrash)
			auth.POST("/docs/trash/restore/:id", handler.RestoreFromTrash)
			auth.DELETE("/docs/trash/purge/:id", handler.PurgeFromTrash)

			auth.GET("/permissions", handler.ListPermissions)
			auth.POST("/permissions", handler.SetPermission)
			auth.DELETE("/permissions/:id", handler.RemovePermission)
			auth.GET("/permissions/check", handler.CheckPermission)

			auth.POST("/docs/documents/:id/share", handler.CreateShare)
			auth.GET("/docs/documents/:id/shares", handler.ListShares)
			auth.DELETE("/docs/shares/:id", handler.DeleteShare)

			auth.GET("/docs/documents/:id/comments", handler.ListComments)
			auth.POST("/docs/documents/:id/comments", handler.CreateComment)
			auth.PUT("/docs/comments/:id", handler.UpdateComment)
			auth.DELETE("/docs/comments/:id", handler.DeleteComment)

			auth.GET("/docs/favorites", handler.ListFavorites)
			auth.POST("/docs/favorites/:id", handler.AddFavorite)
			auth.DELETE("/docs/favorites/:id", handler.RemoveFavorite)

			auth.GET("/docs/tags", handler.ListTags)
			auth.POST("/docs/tags", handler.CreateTag)
			auth.DELETE("/docs/tags/:id", handler.DeleteTag)
			auth.GET("/docs/documents/:id/tags", handler.GetDocTags)
			auth.PUT("/docs/documents/:id/tags", handler.SetDocTags)

			auth.GET("/audits", handler.ListAudits)
			auth.GET("/admin/dashboard", handler.DashboardStats)
			auth.GET("/storage/status", handler.StorageStatus)
		}
	}
	return r
}

// HTTP helpers

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

// 创建测试文档，返回文档 ID
func createTestDoc(t *testing.T, token, title, deptID string) string {
	t.Helper()
	body := map[string]interface{}{
		"title":         title,
		"type":          "doc",
		"department_id": deptID,
		"content":       "<p>测试内容</p>",
	}
	w := request("POST", "/api/docs/documents", body, token)
	if w.Code != 200 {
		t.Fatalf("create doc failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	return getString(data["id"])
}

// 创建测试文件夹，返回文件夹 ID
func createTestFolder(t *testing.T, token, name, deptID string) string {
	t.Helper()
	body := map[string]interface{}{
		"name":          name,
		"department_id": deptID,
	}
	w := request("POST", "/api/docs/folders", body, token)
	if w.Code != 200 {
		t.Fatalf("create folder failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	return getString(data["id"])
}

// ==================== 1. 认证测试 ====================

func TestLogin(t *testing.T) {
	// 正确密码登录
	w := request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "Admin@2026",
	}, "")
	if w.Code != 200 {
		t.Errorf("admin login should succeed, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if resp["token"] == nil {
		t.Error("should return token")
	}
	user := resp["user"].(map[string]interface{})
	if getString(user["role"]) != "super_admin" {
		t.Errorf("role should be super_admin, got %v", user["role"])
	}
}

func TestLoginWrongPassword(t *testing.T) {
	w := request("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrong",
	}, "")
	if w.Code != 401 {
		t.Errorf("wrong password should return 401, got %d", w.Code)
	}
}

func TestLoginNonexistentUser(t *testing.T) {
	w := request("POST", "/api/auth/login", map[string]string{
		"username": "noone",
		"password": "whatever",
	}, "")
	if w.Code != 401 {
		t.Errorf("nonexistent user should return 401, got %d", w.Code)
	}
}

func TestMe(t *testing.T) {
	w := request("GET", "/api/auth/me", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("me should succeed, got %d", w.Code)
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

// ==================== 2. 部门管理测试 ====================

func TestListDepartments(t *testing.T) {
	w := request("GET", "/api/departments", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list departments failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have departments")
	}
}

func TestCreateDepartment(t *testing.T) {
	// super_admin 创建
	w := request("POST", "/api/departments", map[string]string{
		"name": "测试部-IT",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("super_admin should create department, got %d: %s", w.Code, w.Body.String())
	}

	// member 不能创建
	w = request("POST", "/api/departments", map[string]string{
		"name": "测试部-不应创建",
	}, member1Tkn)
	if w.Code != 403 {
		t.Errorf("member should not create department, got %d", w.Code)
	}
}

func TestDeleteDepartment(t *testing.T) {
	// 先创建一个临时部门
	w := request("POST", "/api/departments", map[string]string{
		"name": "待删除部门",
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	deptID := getString(data["id"])

	// member 不能删除
	w = request("DELETE", "/api/departments/"+deptID, nil, member1Tkn)
	if w.Code != 403 {
		t.Errorf("member should not delete department, got %d", w.Code)
	}

	// super_admin 删除
	w = request("DELETE", "/api/departments/"+deptID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("super_admin should delete department, got %d", w.Code)
	}
}

// ==================== 3. 用户管理测试 ====================

func TestListUsersByAdmin(t *testing.T) {
	w := request("GET", "/api/users", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list users failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	total := getFloat(resp["total"])
	if total < 4 {
		t.Errorf("should have at least 4 users, got %v", total)
	}
}

func TestListUsersByMember(t *testing.T) {
	w := request("GET", "/api/users", nil, member1Tkn)
	if w.Code != 403 {
		t.Errorf("member should not list users, got %d", w.Code)
	}
}

func TestDeptAdminListUsersSeesOwnDept(t *testing.T) {
	w := request("GET", "/api/users", nil, deptAdmin1Tkn)
	if w.Code != 200 {
		t.Errorf("dept_admin should list users, got %d", w.Code)
	}
	// dept_admin 只能看到本部门
	resp := parseJSON(t, w)
	total := getFloat(resp["total"])
	if total < 1 {
		t.Error("dept_admin should see at least 1 user in own dept")
	}
}

func TestCreateUser(t *testing.T) {
	w := request("POST", "/api/users", map[string]interface{}{
		"username":      "test-new-user",
		"password":      "Test123!",
		"name":          "新建用户",
		"department_id": dept1ID,
		"role":          "member",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("create user failed: %d %s", w.Code, w.Body.String())
	}

	// 清理
	database.DB.ExecContext(context.Background(),
		"DELETE FROM md_users WHERE username = 'test-new-user'")
}

func TestCreateUserByDeptAdmin(t *testing.T) {
	// dept_admin 应该可以创建本部门用户
	w := request("POST", "/api/users", map[string]interface{}{
		"username":      "test-deptadmin-user",
		"password":      "Test123!",
		"name":          "部门管理员创建",
		"department_id": dept1ID,
		"role":          "member",
	}, deptAdmin1Tkn)
	if w.Code != 200 {
		t.Errorf("dept_admin should create user in own dept, got %d: %s", w.Code, w.Body.String())
	}

	database.DB.ExecContext(context.Background(),
		"DELETE FROM md_users WHERE username = 'test-deptadmin-user'")
}

func TestDeleteUser(t *testing.T) {
	// 先创建一个临时用户
	w := request("POST", "/api/users", map[string]interface{}{
		"username":      "test-to-delete",
		"password":      "Test123!",
		"name":          "待删除",
		"department_id": dept1ID,
		"role":          "member",
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	userID := getString(data["id"])

	// member 不能删除
	w = request("DELETE", "/api/users/"+userID, nil, member1Tkn)
	if w.Code != 403 {
		t.Errorf("member should not delete user, got %d", w.Code)
	}

	// super_admin 删除
	w = request("DELETE", "/api/users/"+userID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("super_admin should delete user, got %d", w.Code)
	}
}

func TestResetPassword(t *testing.T) {
	w := request("PUT", "/api/users/"+member1ID+"/reset-password", map[string]string{
		"password": "NewPass123!",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("reset password failed: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 4. 文档 CRUD 测试 ====================

func TestCreateDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "集成测试文档", dept1ID)
	if docID == "" {
		t.Fatal("doc ID should not be empty")
	}

	// 验证可以读取
	w := request("GET", "/api/docs/documents/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("get document failed: %d", w.Code)
	}
}

func TestGetDocumentContent(t *testing.T) {
	docID := createTestDoc(t, adminToken, "内容读取测试", dept1ID)

	w := request("GET", "/api/docs/documents/"+docID+"/content", nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("get content failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if getString(data["content"]) == "" {
		t.Error("content should not be empty")
	}
}

func TestSaveDocumentContent(t *testing.T) {
	docID := createTestDoc(t, adminToken, "保存内容测试", dept1ID)

	newContent := "<p>更新后的内容 " + time.Now().Format(time.RFC3339) + "</p>"
	w := request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": newContent,
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("save content failed: %d %s", w.Code, w.Body.String())
	}

	// 验证保存成功
	w = request("GET", "/api/docs/documents/"+docID+"/content", nil, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if getString(data["content"]) != newContent {
		t.Errorf("content mismatch, got %v", data["content"])
	}
}

func TestUpdateDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "更新标题测试", dept1ID)

	w := request("PUT", "/api/docs/documents/"+docID, map[string]string{
		"title": "更新后的标题",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("update doc failed: %d %s", w.Code, w.Body.String())
	}

	// 验证标题
	w = request("GET", "/api/docs/documents/"+docID, nil, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if getString(data["title"]) != "更新后的标题" {
		t.Errorf("title not updated, got %v", data["title"])
	}
}

func TestDeleteDocument(t *testing.T) {
	docID := createTestDoc(t, adminToken, "删除测试文档", dept1ID)

	// 软删除
	w := request("DELETE", "/api/docs/documents/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete doc failed: %d %s", w.Code, w.Body.String())
	}

	// 验证在回收站
	w = request("GET", "/api/docs/trash", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list trash failed: %d", w.Code)
	}
}

func TestListDocuments(t *testing.T) {
	w := request("GET", "/api/docs/documents", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list docs failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	if resp["data"] == nil {
		t.Error("data should not be nil")
	}
}

func TestDocTree(t *testing.T) {
	w := request("GET", "/api/docs/tree", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("doc tree failed: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 5. 文件夹测试 ====================

func TestCreateFolder(t *testing.T) {
	folderID := createTestFolder(t, adminToken, "测试文件夹", dept1ID)
	if folderID == "" {
		t.Fatal("folder ID should not be empty")
	}
}

func TestCreateSubFolder(t *testing.T) {
	parentID := createTestFolder(t, adminToken, "父文件夹", dept1ID)

	w := request("POST", "/api/docs/folders", map[string]interface{}{
		"name":          "子文件夹",
		"parent_id":     parentID,
		"department_id": dept1ID,
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("create sub-folder failed: %d %s", w.Code, w.Body.String())
	}
}

func TestDeptAdminCreateFolderInOtherDept(t *testing.T) {
	// dept_admin1 属于 dept1，不应该在 dept2 创建文件夹
	w := request("POST", "/api/docs/folders", map[string]interface{}{
		"name":          "跨部门文件夹",
		"department_id": dept2ID,
	}, deptAdmin1Tkn)
	if w.Code != 403 {
		t.Errorf("dept_admin should not create folder in other dept, got %d", w.Code)
	}
}

func TestUpdateFolder(t *testing.T) {
	folderID := createTestFolder(t, adminToken, "待更新文件夹", dept1ID)

	w := request("PUT", "/api/docs/folders/"+folderID, map[string]string{
		"name": "已更新文件夹",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("update folder failed: %d %s", w.Code, w.Body.String())
	}
}

func TestDeleteFolder(t *testing.T) {
	folderID := createTestFolder(t, adminToken, "待删除文件夹", dept1ID)

	w := request("DELETE", "/api/docs/folders/"+folderID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete folder failed: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 6. 权限系统测试 ====================

func TestSetPermission(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限测试文档", dept1ID)

	// 给 member2（dept2）设置 read 权限
	w := request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "read",
		"inherit":       true,
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("set permission failed: %d %s", w.Code, w.Body.String())
	}
}

func TestCheckPermissionDefault(t *testing.T) {
	// member1 属于 dept1，文档也属于 dept1 → 默认 write
	docID := createTestDoc(t, adminToken, "默认权限测试", dept1ID)

	w := request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, member1Tkn)
	if w.Code != 200 {
		t.Fatalf("check permission failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "write" {
		t.Errorf("same dept member should have write, got %s", perm)
	}
}

func TestCheckPermissionOtherDept(t *testing.T) {
	// member2 属于 dept2，文档属于 dept1 → 默认 none
	docID := createTestDoc(t, adminToken, "跨部门权限测试", dept1ID)

	w := request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, member2Tkn)
	if w.Code != 200 {
		t.Fatalf("check permission failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "none" {
		t.Errorf("other dept member should have none, got %s", perm)
	}
}

func TestCheckPermissionExplicitGrant(t *testing.T) {
	docID := createTestDoc(t, adminToken, "显式授权测试", dept1ID)

	// 给 member2 显式 read 权限
	request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "read",
		"inherit":       true,
	}, adminToken)

	// 验证 member2 有 read
	w := request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, member2Tkn)
	resp := parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "read" {
		t.Errorf("explicitly granted user should have read, got %s", perm)
	}
}

func TestCheckPermissionSuperAdminBypass(t *testing.T) {
	docID := createTestDoc(t, adminToken, "超级管理员绕过测试", dept1ID)

	w := request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, adminToken)
	resp := parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "admin" {
		t.Errorf("super_admin should always have admin, got %s", perm)
	}
}

func TestListPermissions(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限列表测试", dept1ID)

	// 设置权限
	request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member1ID,
		"permission":    "write",
		"inherit":       true,
	}, adminToken)

	w := request("GET", fmt.Sprintf("/api/permissions?resource_type=document&resource_id=%s", docID), nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list permissions failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have at least 1 permission")
	}
}

func TestRemovePermission(t *testing.T) {
	docID := createTestDoc(t, adminToken, "删除权限测试", dept1ID)

	// 设置权限
	w := request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "read",
		"inherit":       true,
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	permID := getString(data["id"])

	// member 不能删除权限
	w = request("DELETE", "/api/permissions/"+permID, nil, member1Tkn)
	if w.Code != 403 {
		t.Errorf("member should not remove permission, got %d", w.Code)
	}

	// admin 可以删除
	w = request("DELETE", "/api/permissions/"+permID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("admin should remove permission, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDepartmentLevelPermission(t *testing.T) {
	docID := createTestDoc(t, adminToken, "部门级权限测试", dept1ID)

	// 给 dept2 整个部门设置 read 权限
	w := request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "department",
		"target_id":     dept2ID,
		"permission":    "read",
		"inherit":       true,
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("set dept permission failed: %d %s", w.Code, w.Body.String())
	}

	// member2 在 dept2，应该有 read
	w = request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, member2Tkn)
	resp := parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "read" {
		t.Errorf("dept-level permission should grant read to member2, got %s", perm)
	}
}

func TestFolderPermissionInheritance(t *testing.T) {
	// 创建文件夹
	folderID := createTestFolder(t, adminToken, "权限继承测试文件夹", dept1ID)

	// 给 member2 文件夹 read 权限
	request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "folder",
		"resource_id":   folderID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "read",
		"inherit":       true,
	}, adminToken)

	// 在文件夹内创建文档
	w := request("POST", "/api/docs/documents", map[string]interface{}{
		"title":         "继承测试文档",
		"type":          "doc",
		"folder_id":     folderID,
		"department_id": dept1ID,
		"content":       "<p>继承</p>",
	}, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	docID := getString(data["id"])

	// member2 应该通过文件夹继承获得 read
	w = request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, member2Tkn)
	resp = parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "read" {
		t.Errorf("should inherit read from folder, got %s", perm)
	}
}

func TestPermissionLevelOrder(t *testing.T) {
	docID := createTestDoc(t, adminToken, "权限级别测试", dept1ID)

	// 先给 admin 权限
	request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "admin",
		"inherit":       true,
	}, adminToken)

	// 检查 admin 级别
	w := request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, member2Tkn)
	resp := parseJSON(t, w)
	perm := getString(resp["permission"])
	if perm != "admin" {
		t.Errorf("should have admin, got %s", perm)
	}

	// 降级为 read
	request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "read",
		"inherit":       true,
	}, adminToken)

	w = request("GET", fmt.Sprintf("/api/permissions/check?resource_type=document&resource_id=%s", docID), nil, member2Tkn)
	resp = parseJSON(t, w)
	perm = getString(resp["permission"])
	if perm != "read" {
		t.Errorf("should be downgraded to read, got %s", perm)
	}
}

func TestInvalidPermissionLevel(t *testing.T) {
	docID := createTestDoc(t, adminToken, "非法权限级别测试", dept1ID)

	w := request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member1ID,
		"permission":    "super_root_godmode",
		"inherit":       true,
	}, adminToken)
	if w.Code != 400 {
		t.Errorf("invalid permission level should return 400, got %d", w.Code)
	}
}

// ==================== 7. 文档锁测试 ====================

func TestLockUnlock(t *testing.T) {
	docID := createTestDoc(t, adminToken, "锁定测试文档", dept1ID)

	// admin 锁定
	w := request("POST", "/api/docs/documents/"+docID+"/lock", nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("lock failed: %d %s", w.Code, w.Body.String())
	}

	// member1 保存内容应该被拒绝（文档被 admin 锁定）
	w = request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>非法修改</p>",
	}, member1Tkn)
	if w.Code != 409 {
		t.Errorf("save while locked by other should be 409, got %d", w.Code)
	}

	// admin 自己可以保存
	w = request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>锁定者可修改</p>",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("lock owner should be able to save, got %d", w.Code)
	}

	// admin 解锁
	w = request("POST", "/api/docs/documents/"+docID+"/unlock", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("unlock failed: %d", w.Code)
	}

	// member1 现在可以保存了
	w = request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>解锁后可修改</p>",
	}, member1Tkn)
	// 因为 member1 是同部门默认 write，所以应该可以
	if w.Code != 200 {
		t.Errorf("after unlock, member should be able to save, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLockConflict(t *testing.T) {
	docID := createTestDoc(t, adminToken, "锁定冲突测试", dept1ID)

	// member1 锁定
	w := request("POST", "/api/docs/documents/"+docID+"/lock", nil, member1Tkn)
	if w.Code != 200 {
		t.Fatalf("member1 lock failed: %d", w.Code)
	}

	// member2 再锁定 → 冲突
	w = request("POST", "/api/docs/documents/"+docID+"/lock", nil, member2Tkn)
	if w.Code != 409 {
		t.Errorf("second lock should conflict, got %d", w.Code)
	}

	// member2 不能解锁别人的锁
	w = request("POST", "/api/docs/documents/"+docID+"/unlock", nil, member2Tkn)
	if w.Code != 403 {
		t.Errorf("member2 should not unlock member1's lock, got %d", w.Code)
	}
}

// ==================== 8. 文档分享测试 ====================

func TestCreateShare(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享测试文档", dept1ID)

	w := request("POST", "/api/docs/documents/"+docID+"/share", map[string]interface{}{
		"password":   "abc123",
		"expires_in": 24,
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create share failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if resp["token"] == nil {
		t.Error("should return share token")
	}
	if resp["share_url"] == nil {
		t.Error("should return share_url")
	}
}

func TestAccessShare(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享访问测试", dept1ID)

	// 创建分享
	w := request("POST", "/api/docs/documents/"+docID+"/share", map[string]interface{}{
		"password": "test123",
	}, adminToken)
	resp := parseJSON(t, w)
	token := getString(resp["token"])

	// 无密码访问 → 需要
	w = request("GET", "/api/s/"+token, nil, "")
	resp = parseJSON(t, w)
	needPwd, _ := resp["need_password"].(bool)
	if !needPwd {
		t.Errorf("should require password, got %v", resp)
	}

	// 正确密码访问
	w = request("GET", "/api/s/"+token+"?password=test123", nil, "")
	if w.Code != 200 {
		t.Errorf("access with correct password should succeed, got %d: %s", w.Code, w.Body.String())
	}

	// 错误密码
	w = request("GET", "/api/s/"+token+"?password=wrong", nil, "")
	if w.Code != 403 {
		t.Errorf("wrong password should return 403, got %d", w.Code)
	}
}

func TestShareInfo(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享信息测试", dept1ID)

	w := request("POST", "/api/docs/documents/"+docID+"/share", map[string]interface{}{
		"password": "infotest",
	}, adminToken)
	resp := parseJSON(t, w)
	token := getString(resp["token"])

	// 获取分享信息（不需要密码）
	w = request("GET", "/api/s/"+token+"/info", nil, "")
	if w.Code != 200 {
		t.Errorf("share info should work, got %d", w.Code)
	}
	resp = parseJSON(t, w)
	hasPwd, _ := resp["has_password"].(bool)
	if !hasPwd {
		t.Error("should report has_password=true")
	}
}

func TestListAndDeleteShare(t *testing.T) {
	docID := createTestDoc(t, adminToken, "分享列表测试", dept1ID)

	// 创建分享
	request("POST", "/api/docs/documents/"+docID+"/share", map[string]interface{}{}, adminToken)

	// 列出分享
	w := request("GET", "/api/docs/documents/"+docID+"/shares", nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list shares failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Fatal("should have at least 1 share")
	}

	// 删除分享
	share := data[0].(map[string]interface{})
	shareID := getString(share["id"])
	w = request("DELETE", "/api/docs/shares/"+shareID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete share failed: %d", w.Code)
	}
}

// ==================== 9. 收藏测试 ====================

func TestFavorite(t *testing.T) {
	docID := createTestDoc(t, adminToken, "收藏测试文档", dept1ID)

	// 添加收藏
	w := request("POST", "/api/docs/favorites/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("add favorite failed: %d %s", w.Code, w.Body.String())
	}

	// 查看收藏列表
	w = request("GET", "/api/docs/favorites", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list favorites failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have at least 1 favorite")
	}

	// 取消收藏
	w = request("DELETE", "/api/docs/favorites/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("remove favorite failed: %d", w.Code)
	}
}

// ==================== 10. 评论测试 ====================

func TestComments(t *testing.T) {
	docID := createTestDoc(t, adminToken, "评论测试文档", dept1ID)

	// 创建评论
	w := request("POST", "/api/docs/documents/"+docID+"/comments", map[string]string{
		"content": "这是一条测试评论",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create comment failed: %d %s", w.Code, w.Body.String())
	}

	// 列出评论
	w = request("GET", "/api/docs/documents/"+docID+"/comments", nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list comments failed: %d", w.Code)
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Fatal("should have at least 1 comment")
	}

	comment := data[0].(map[string]interface{})
	commentID := getString(comment["id"])

	// 更新评论
	w = request("PUT", "/api/docs/comments/"+commentID, map[string]string{
		"content": "更新后的评论",
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("update comment failed: %d", w.Code)
	}

	// 删除评论
	w = request("DELETE", "/api/docs/comments/"+commentID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete comment failed: %d", w.Code)
	}
}

// ==================== 11. 标签测试 ====================

func TestTags(t *testing.T) {
	// 创建标签
	w := request("POST", "/api/docs/tags", map[string]string{
		"name":  "测试标签",
		"color": "#ff0000",
	}, adminToken)
	if w.Code != 200 {
		t.Fatalf("create tag failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	tagID := getString(data["id"])

	// 列出标签
	w = request("GET", "/api/docs/tags", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list tags failed: %d", w.Code)
	}

	// 给文档打标签
	docID := createTestDoc(t, adminToken, "标签测试文档", dept1ID)
	w = request("PUT", "/api/docs/documents/"+docID+"/tags", map[string]interface{}{
		"tag_ids": []string{tagID},
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("set doc tags failed: %d %s", w.Code, w.Body.String())
	}

	// 获取文档标签
	w = request("GET", "/api/docs/documents/"+docID+"/tags", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("get doc tags failed: %d", w.Code)
	}

	// 删除标签
	w = request("DELETE", "/api/docs/tags/"+tagID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("delete tag failed: %d", w.Code)
	}
}

// ==================== 12. 回收站测试 ====================

func TestTrashAndRestore(t *testing.T) {
	docID := createTestDoc(t, adminToken, "回收站测试文档", dept1ID)

	// 删除 → 进回收站
	w := request("DELETE", "/api/docs/documents/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("delete failed: %d", w.Code)
	}

	// 查看回收站
	w = request("GET", "/api/docs/trash", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list trash failed: %d", w.Code)
	}

	// 恢复
	w = request("POST", "/api/docs/trash/restore/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("restore failed: %d %s", w.Code, w.Body.String())
	}

	// 验证恢复
	w = request("GET", "/api/docs/documents/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("restored doc should be accessible, got %d", w.Code)
	}
}

func TestTrashPurge(t *testing.T) {
	docID := createTestDoc(t, adminToken, "彻底删除测试文档", dept1ID)

	// 删除
	request("DELETE", "/api/docs/documents/"+docID, nil, adminToken)

	// 彻底删除
	w := request("DELETE", "/api/docs/trash/purge/"+docID, nil, adminToken)
	if w.Code != 200 {
		t.Errorf("purge failed: %d %s", w.Code, w.Body.String())
	}

	// 验证文件被删除
	docDir := filepath.Join(testStorageRoot, "_trash", docID)
	if _, err := os.Stat(docDir); !os.IsNotExist(err) {
		t.Error("purged doc dir should not exist")
	}
}

// ==================== 13. 版本管理测试 ====================

func TestVersionHistory(t *testing.T) {
	docID := createTestDoc(t, adminToken, "版本历史测试", dept1ID)

	// 保存多次
	for i := 0; i < 3; i++ {
		request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
			"content": fmt.Sprintf("<p>版本 %d</p>", i+2), // v1 是初始创建
		}, adminToken)
	}

	// 查看版本列表
	w := request("GET", "/api/docs/documents/"+docID+"/versions", nil, adminToken)
	if w.Code != 200 {
		t.Fatalf("list versions failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) < 3 {
		t.Errorf("should have at least 3 versions, got %d", len(data))
	}
}

func TestRestoreVersion(t *testing.T) {
	docID := createTestDoc(t, adminToken, "版本恢复测试", dept1ID)

	// 保存不同内容
	request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>第二个版本</p>",
	}, adminToken)
	request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>第三个版本</p>",
	}, adminToken)

	// 恢复到版本 1
	w := request("POST", "/api/docs/documents/"+docID+"/restore", map[string]interface{}{
		"version": 1,
	}, adminToken)
	if w.Code != 200 {
		t.Errorf("restore version failed: %d %s", w.Code, w.Body.String())
	}

	// 验证内容
	w = request("GET", "/api/docs/documents/"+docID+"/content", nil, adminToken)
	resp := parseJSON(t, w)
	data := resp["data"].(map[string]interface{})
	content := getString(data["content"])
	if content != "<p>测试内容</p>" {
		t.Errorf("restored content should be original, got %s", content)
	}
}

// ==================== 14. 审计日志测试 ====================

func TestListAudits(t *testing.T) {
	// 先触发一些操作产生审计日志
	docID := createTestDoc(t, adminToken, "审计测试文档", dept1ID)
	request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>审计内容</p>",
	}, adminToken)

	// 查看审计日志
	w := request("GET", "/api/audits", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("list audits failed: %d %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("should have audit logs")
	}
}

func TestAuditsForbiddenForMember(t *testing.T) {
	w := request("GET", "/api/audits", nil, member1Tkn)
	if w.Code != 403 {
		t.Errorf("member should not list audits, got %d", w.Code)
	}
}

// ==================== 15. Dashboard & 存储测试 ====================

func TestDashboard(t *testing.T) {
	w := request("GET", "/api/admin/dashboard", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("dashboard failed: %d %s", w.Code, w.Body.String())
	}
}

func TestStorageStatus(t *testing.T) {
	w := request("GET", "/api/storage/status", nil, adminToken)
	if w.Code != 200 {
		t.Errorf("storage status failed: %d %s", w.Code, w.Body.String())
	}
}

// ==================== 16. 权限对文档操作的影响 ====================

func TestContentReadPermissionEnforcement(t *testing.T) {
	// 在 dept1 创建文档
	docID := createTestDoc(t, adminToken, "内容读取权限测试", dept1ID)

	// member2（dept2）默认 none → 不能读内容
	w := request("GET", "/api/docs/documents/"+docID+"/content", nil, member2Tkn)
	if w.Code != 403 {
		t.Errorf("member2 should not read content without permission, got %d", w.Code)
	}

	// 授予 read 权限
	request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "read",
		"inherit":       true,
	}, adminToken)

	// 现在可以读
	w = request("GET", "/api/docs/documents/"+docID+"/content", nil, member2Tkn)
	if w.Code != 200 {
		t.Errorf("member2 with read permission should access content, got %d: %s", w.Code, w.Body.String())
	}

	// 但不能保存（write）
	w = request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>非法写入</p>",
	}, member2Tkn)
	if w.Code != 403 {
		t.Errorf("member2 with read-only should not save, got %d", w.Code)
	}
}

func TestContentWritePermissionEnforcement(t *testing.T) {
	docID := createTestDoc(t, adminToken, "写入权限测试", dept1ID)

	// 给 member2 write 权限
	request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     member2ID,
		"permission":    "write",
		"inherit":       true,
	}, adminToken)

	// member2 可以读
	w := request("GET", "/api/docs/documents/"+docID+"/content", nil, member2Tkn)
	if w.Code != 200 {
		t.Errorf("write permission should also allow read, got %d", w.Code)
	}

	// member2 可以保存
	w = request("PUT", "/api/docs/documents/"+docID+"/content", map[string]string{
		"content": "<p>member2 写入</p>",
	}, member2Tkn)
	if w.Code != 200 {
		t.Errorf("member2 with write should save content, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== 17. dept_admin 权限边界测试 ====================

func TestDeptAdminCannotManageOtherDept(t *testing.T) {
	// dept_admin1 属于 dept1，不能操作 dept2 的用户管理
	w := request("DELETE", "/api/users/"+member2ID, nil, deptAdmin1Tkn)
	if w.Code != 403 {
		t.Errorf("dept_admin should not delete user, got %d", w.Code)
	}

	// dept_admin 不能重置其他部门用户密码
	w = request("PUT", "/api/users/"+member2ID+"/reset-password", map[string]string{
		"password": "Hacked123!",
	}, deptAdmin1Tkn)
	if w.Code != 403 {
		t.Errorf("dept_admin should not reset other user password, got %d", w.Code)
	}
}

func TestDeptAdminCannotUpdateDept(t *testing.T) {
	// dept_admin 不能更新部门信息（只有 super_admin）
	w := request("PUT", "/api/departments/"+dept1ID, map[string]string{
		"name": "被篡改的名字",
	}, deptAdmin1Tkn)
	if w.Code != 403 {
		t.Errorf("dept_admin should not update department, got %d", w.Code)
	}
}

// ==================== 18. 无效输入测试 ====================

func TestCreateDocEmptyTitle(t *testing.T) {
	w := request("POST", "/api/docs/documents", map[string]string{
		"title": "",
		"type":  "doc",
	}, adminToken)
	if w.Code != 400 {
		t.Errorf("empty title should return 400, got %d", w.Code)
	}
}

func TestSetPermissionInvalidTarget(t *testing.T) {
	docID := createTestDoc(t, adminToken, "非法目标测试", dept1ID)

	w := request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "invalid_type",
		"target_id":     "nobody",
		"permission":    "read",
		"inherit":       true,
	}, adminToken)
	if w.Code != 400 {
		t.Errorf("invalid target_type should return 400, got %d", w.Code)
	}
}

func TestSetPermissionNonexistentUser(t *testing.T) {
	docID := createTestDoc(t, adminToken, "不存在用户测试", dept1ID)

	w := request("POST", "/api/permissions", map[string]interface{}{
		"resource_type": "document",
		"resource_id":   docID,
		"target_type":   "user",
		"target_id":     "nonexistent-user-id",
		"permission":    "read",
		"inherit":       true,
	}, adminToken)
	if w.Code != 400 {
		t.Errorf("nonexistent user should return 400, got %d", w.Code)
	}
}

func TestGetNonexistentDocument(t *testing.T) {
	w := request("GET", "/api/docs/documents/nonexistent-id", nil, adminToken)
	if w.Code != 404 {
		t.Errorf("nonexistent doc should return 404, got %d", w.Code)
	}
}

func TestAccessInvalidShare(t *testing.T) {
	w := request("GET", "/api/s/nonexistent-token", nil, "")
	if w.Code != 404 {
		t.Errorf("invalid share should return 404, got %d", w.Code)
	}
}
