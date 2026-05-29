package handler

import (
	"github.com/gin-gonic/gin"
)

// APIDocs returns OpenAPI 3.0 spec as JSON
func APIDocs(c *gin.Context) {
	c.JSON(200, gin.H{
		"openapi": "3.0.0",
		"info": gin.H{
			"title":   "MistDocs API",
			"version": "1.0.0",
			"description": "MistDocs 文档管理系统 RESTful API",
		},
		"servers": gin.H{
			"url": "/api",
		},
		"paths": buildPaths(),
		"components": gin.H{
			"securitySchemes": gin.H{
				"bearerAuth": gin.H{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
	})
}

func buildPaths() map[string]interface{} {
	return map[string]interface{}{
		"/auth/login": gin.H{
			"post": op("登录", "用户名密码登录", nil, gin.H{"username": "string", "password": "string"}, "token"),
		},
		"/auth/me": gin.H{
			"get": op("当前用户", "获取当前登录用户信息", nil, nil, "user"),
		},
		"/auth/password": gin.H{
			"put": op("修改密码", "修改当前用户密码", nil, gin.H{"old": "string", "new_": "string"}, "message"),
		},
		"/docs/tree": gin.H{
			"get": op("文件夹树", "获取文件夹树形结构", nil, nil, "tree"),
		},
		"/docs/folders": gin.H{
			"post":   op("创建文件夹", nil, nil, gin.H{"name": "string", "parent_id": "string"}, "folder"),
			"get":    op("文件夹列表", nil, nil, nil, "folders"),
			"put":    op("更新文件夹", nil, gin.H{"id": "path"}, gin.H{"name": "string"}, "message"),
			"delete": op("删除文件夹", nil, gin.H{"id": "path"}, nil, "message"),
		},
		"/docs/documents": gin.H{
			"get":  op("文档列表", "支持 folder_id/type/page/page_size 参数", nil, nil, "documents"),
			"post": op("创建文档", nil, nil, gin.H{"title": "string", "type": "doc|sheet", "folder_id": "string", "content": "string"}, "document"),
		},
		"/docs/documents/search": gin.H{
			"get": op("搜索文档", "搜索标题和内容，参数 q", nil, nil, "documents"),
		},
		"/docs/documents/recent": gin.H{
			"get": op("最近文档", nil, nil, nil, "documents"),
		},
		"/docs/documents/{id}": gin.H{
			"get":    op("获取文档", nil, nil, nil, "document"),
			"put":    op("更新文档", nil, nil, gin.H{"title": "string", "folder_id": "string"}, "message"),
			"delete": op("删除文档", "软删除到回收站", nil, nil, "message"),
		},
		"/docs/documents/{id}/content": gin.H{
			"get": op("获取内容", "返回解密后的HTML内容", nil, nil, "html"),
			"put": op("保存内容", nil, nil, gin.H{"content": "string"}, "message"),
		},
		"/docs/documents/{id}/versions": gin.H{
			"get": op("版本历史", nil, nil, nil, "versions"),
		},
		"/docs/documents/{id}/stats": gin.H{
			"get": op("文档统计", "字数/编辑次数/活跃时段等", nil, nil, "stats"),
		},
		"/docs/documents/{id}/lock": gin.H{
			"post": op("锁定文档", "防止他人编辑", nil, nil, "message"),
		},
		"/docs/documents/{id}/unlock": gin.H{
			"post": op("解锁文档", nil, nil, nil, "message"),
		},
		"/docs/documents/{id}/export": gin.H{
			"get": op("导出文档", "format 参数: html/markdown/txt/pdf/docx", nil, nil, "file"),
		},
		"/docs/documents/{id}/share": gin.H{
			"post": op("创建分享", nil, nil, gin.H{"password": "string", "expires_in": "int"}, "share"),
		},
		"/docs/documents/{id}/comments": gin.H{
			"get":  op("评论列表", nil, nil, nil, "comments"),
			"post": op("创建评论", nil, nil, gin.H{"content": "string", "parent_id": "string"}, "comment"),
		},
		"/docs/tags": gin.H{
			"get":    op("标签列表", nil, nil, nil, "tags"),
			"post":   op("创建标签", nil, nil, gin.H{"name": "string", "color": "string"}, "tag"),
			"delete": op("删除标签", nil, nil, nil, "message"),
		},
		"/docs/documents/{id}/tags": gin.H{
			"get": op("文档标签", nil, nil, nil, "tags"),
			"put": op("设置文档标签", "替换所有标签", nil, gin.H{"tag_ids": "string[]"}, "message"),
		},
		"/docs/import": gin.H{
			"post": op("批量导入", "multipart, 支持 .txt/.md/.html", nil, nil, "results"),
		},
		"/docs/favorites": gin.H{
			"get": op("收藏列表", nil, nil, nil, "favorites"),
		},
		"/docs/trash": gin.H{
			"get": op("回收站列表", nil, nil, nil, "documents"),
		},
		"/docs/trash/restore/{id}": gin.H{
			"post": op("恢复文档", nil, nil, nil, "message"),
		},
		"/docs/trash/purge/{id}": gin.H{
			"delete": op("永久删除", nil, nil, nil, "message"),
		},
		"/users": gin.H{
			"get":  op("用户列表", nil, nil, nil, "users"),
			"post": op("创建用户", nil, nil, gin.H{"username": "string", "password": "string", "name": "string", "role": "string"}, "user"),
		},
		"/departments": gin.H{
			"get":  op("部门列表", nil, nil, nil, "departments"),
			"post": op("创建部门", nil, nil, gin.H{"name": "string"}, "department"),
		},
		"/permissions": gin.H{
			"get":  op("权限列表", nil, nil, nil, "permissions"),
			"post": op("设置权限", nil, nil, gin.H{"resource_type": "string", "resource_id": "string", "target_type": "string", "target_id": "string", "permission": "read|write|admin"}, "permission"),
		},
		"/notifications": gin.H{
			"get": op("通知列表", nil, nil, nil, "notifications"),
		},
		"/audits": gin.H{
			"get": op("审计日志", nil, nil, nil, "audits"),
		},
		"/admin/dashboard": gin.H{
			"get": op("仪表盘统计", nil, nil, nil, "stats"),
		},
		"/admin/webhooks": gin.H{
			"get":    op("Webhook列表", nil, nil, nil, "webhooks"),
			"post":   op("创建Webhook", nil, nil, gin.H{"name": "string", "url": "string", "events": "string"}, "webhook"),
			"delete": op("删除Webhook", nil, nil, nil, "message"),
		},
	}
}

func op(summary string, description interface{}, pathParams, bodyFields, returnType interface{}) gin.H {
	descStr := ""
	if s, ok := description.(string); ok {
		descStr = s
	} else {
		descStr = summary
	}
	op := gin.H{
		"summary":     summary,
		"description": descStr,
		"responses": gin.H{
			"200": gin.H{"description": "success"},
		},
	}
	if bodyFields != nil {
		op["requestBody"] = gin.H{
			"content": gin.H{
				"application/json": gin.H{
					"schema": gin.H{"type": "object", "properties": bodyFields},
				},
			},
		}
	}
	return op
}
