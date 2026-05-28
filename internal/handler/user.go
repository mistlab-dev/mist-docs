package handler

import (
	"encoding/csv"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/c-wind/mist-docs/internal/middleware"
	"github.com/c-wind/mist-docs/internal/model"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ==================== 认证 ====================

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	user, err := service.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		log.Printf("[Login] query user error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role, user.DepartmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}

	service.UpdateLastLogin(c.Request.Context(), user.ID)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"name":          user.Name,
			"role":          user.Role,
			"department_id": user.DepartmentID,
		},
	})
}

func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

func Me(c *gin.Context) {
	userID := c.GetString("user_id")
	user, err := service.GetUserByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"name":          user.Name,
		"email":         user.Email,
		"phone":         user.Phone,
		"role":          user.Role,
		"department_id": user.DepartmentID,
		"status":        user.Status,
	}})
}

type ChangePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func ChangePassword(c *gin.Context) {
	var req ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	userID := c.GetString("user_id")
	user, err := service.GetUserByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
		return
	}

	if err := service.ResetPassword(c.Request.Context(), userID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "密码已修改"})
}

// ==================== 部门 ====================

func ListDepartments(c *gin.Context) {
	tree, err := service.GetDepartmentTree(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tree})
}

func CreateDepartment(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	var d model.Department
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if d.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "部门名称不能为空"})
		return
	}

	if err := service.CreateDepartment(c.Request.Context(), &d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}

func UpdateDepartment(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	var d model.Department
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	d.ID = c.Param("id")

	if err := service.UpdateDepartment(c.Request.Context(), &d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func DeleteDepartment(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	if err := service.DeleteDepartment(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ==================== 用户 ====================

func ListUsers(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	deptID := c.Query("department_id")
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// dept_admin 只能看本部门
	if role == "dept_admin" {
		deptID = c.GetString("department_id")
	}

	users, total, err := service.ListUsers(c.Request.Context(), deptID, keyword, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users, "total": total, "page": page, "page_size": pageSize})
}

type CreateUserReq struct {
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password" binding:"required,min=6"`
	Name         string `json:"name" binding:"required"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	DepartmentID string `json:"department_id"`
	Role         string `json:"role"`
}

func CreateUser(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	u := &model.User{
		Username:     req.Username,
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		DepartmentID: req.DepartmentID,
		Role:         req.Role,
	}

	if err := service.CreateUser(c.Request.Context(), u, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": u.ID}})
}

func UpdateUser(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	var u model.User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	u.ID = c.Param("id")

	if err := service.UpdateUser(c.Request.Context(), &u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

func DeleteUser(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	// soft delete
	user, err := service.GetUserByID(c.Request.Context(), c.Param("id"))
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	user.Status = 0
	service.UpdateUser(c.Request.Context(), user)
	c.JSON(http.StatusOK, gin.H{"message": "已禁用"})
}

func ResetPassword(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少6位"})
		return
	}

	if err := service.ResetPassword(c.Request.Context(), c.Param("id"), req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

// ==================== CSV 导入 ====================

func ImportDepartments(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// skip header
	if _, err := reader.Read(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV 格式错误"})
		return
	}

	success, fail := 0, 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			fail++
			continue
		}

		name := record[0]
		parentName := record[1]

		d := &model.Department{Name: name}
		if parentName != "" {
			parentID, err := service.FindDeptIDByName(c.Request.Context(), parentName)
			if err != nil || parentID == "" {
				fail++
				continue
			}
			d.ParentID = parentID
		}

		if len(record) > 2 {
			d.SortOrder, _ = strconv.Atoi(record[2])
		}

		if err := service.CreateDepartment(c.Request.Context(), d); err != nil {
			fail++
			continue
		}
		success++
	}

	c.JSON(http.StatusOK, gin.H{"success": success, "fail": fail})
}

func ImportUsers(c *gin.Context) {
	role := c.GetString("role")
	if role != "super_admin" && role != "dept_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if _, err := reader.Read(); err != nil { // skip header
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV 格式错误"})
		return
	}

	success, fail := 0, 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 4 {
			fail++
			continue
		}

		name := record[0]
		username := record[1]
		password := record[2]
		deptName := record[3]
		userRole := "member"
		email := ""
		phone := ""

		if len(record) > 4 && record[4] != "" {
			userRole = record[4]
		}
		if len(record) > 5 {
			email = record[5]
		}
		if len(record) > 6 {
			phone = record[6]
		}

		deptID := ""
		if deptName != "" {
			deptID, err = service.FindDeptIDByName(c.Request.Context(), deptName)
			if err != nil || deptID == "" {
				fail++
				continue
			}
		}

		u := &model.User{
			Username:     username,
			Name:         name,
			Email:        email,
			Phone:        phone,
			DepartmentID: deptID,
			Role:         userRole,
		}

		if err := service.CreateUser(c.Request.Context(), u, password); err != nil {
			fail++
			continue
		}
		success++
	}

	c.JSON(http.StatusOK, gin.H{"success": success, "fail": fail})
}
