package handler

import (
	"github.com/gin-gonic/gin"
)

// APIDocs returns OpenAPI 3.0 spec as JSON
func APIDocs(c *gin.Context) {
	c.JSON(200, gin.H{
		"openapi": "3.0.0",
		"info": gin.H{
			"title":       "MistDocs API",
			"version":     "1.0.0",
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
	paths := map[string]interface{}{}
	add := func(path, method, summary, desc string) {
		entry, _ := paths[path].(gin.H)
		if entry == nil {
			entry = gin.H{}
		}
		entry[method] = op(summary, desc, nil, nil, nil)
		paths[path] = entry
	}

	add("/auth/login", "post", "已废弃", "此登录接口已废弃，请通过 mistlab.dev Portal 登录。调用返回 410。")
	add("/auth/logout", "post", "退出", "需要 Portal JWT。")
	add("/auth/me", "get", "当前用户", "读取 Portal JWT，返回共享 users 表中的用户和团队。")
	add("/auth/password", "put", "修改密码（不要用于 Portal 账号）", "Portal 用户请在 mistlab.dev 修改密码。本接口查的是旧用户表。")

	add("/s/{token}", "get", "公开分享", "按分享 token 读取文档。不要求登录。")
	add("/s/{token}/info", "get", "分享信息", "返回标题、是否过期、是否需要密码。")

	const team = "/teams/{team_id}"
	add(team+"/folders/tree", "get", "文件夹树", "当前团队的文件夹。")
	add(team+"/folders", "post", "创建文件夹", "")
	add(team+"/folders/{id}", "put", "更新文件夹", "")
	add(team+"/folders/{id}", "delete", "删除文件夹", "")

	add(team+"/documents", "get", "文档列表", "类型只有 doc 和 sheet。")
	add(team+"/documents", "post", "创建文档", "type 为 doc 或 sheet。计费打开且套餐缺失时会检查文档篇数。")
	add(team+"/documents/search", "get", "搜索文档", "参数 q。")
	add(team+"/documents/recent", "get", "最近文档", "")
	add(team+"/docs/search", "get", "段落检索", "")
	add(team+"/documents/{id}", "get", "获取文档", "")
	add(team+"/documents/{id}", "put", "更新文档", "")
	add(team+"/documents/{id}", "delete", "删除文档", "软删除，留在回收站直到恢复或清空。")
	add(team+"/documents/{id}/content", "get", "获取内容", "")
	add(team+"/documents/{id}/content", "put", "保存内容", "保存记审计 edit_doc，并投递 Webhook document.updated。")
	add(team+"/documents/{id}/stats", "get", "文档统计", "")
	add(team+"/documents/{id}/versions", "get", "版本列表", "")
	add(team+"/documents/{id}/versions/{ver}/content", "get", "版本内容", "")
	add(team+"/documents/{id}/restore", "post", "恢复版本", "")
	add(team+"/documents/{id}/lock", "post", "锁定文档", "")
	add(team+"/documents/{id}/unlock", "post", "解锁文档", "")
	add(team+"/documents/{id}/share", "post", "创建分享", "请求字段 password、expiresIn（小时，0 表示不过期）。响应 data.share_url 为 /s/{token}。")
	add(team+"/documents/{id}/shares", "get", "分享列表", "")
	add(team+"/documents/{id}/collaborators", "get", "协作者", "角色为 viewer、editor、admin。")
	add(team+"/documents/{id}/collaborators", "post", "添加协作者", "")
	add(team+"/documents/{id}/comments", "get", "评论列表", "")
	add(team+"/documents/{id}/comments", "post", "创建评论", "")
	add(team+"/documents/{id}/export", "get", "导出文档", "format：markdown、html、txt、pdf。下载扩展名与内容一致。不提供 docx。编辑器里的 PDF 由浏览器生成，不走此接口。服务端 pdf 在计费打开且套餐不含 pdf_export 时返回 402。")
	add(team+"/documents/{id}/fragments", "get", "文档关联片段", "")
	add(team+"/documents/{id}/fragments", "post", "关联片段", "")
	add(team+"/documents/{id}/fragments/{fragment_id}", "delete", "取消关联片段", "")
	add(team+"/fragments-search", "get", "搜索团队片段", "")

	add(team+"/trash", "get", "回收站", "没有按天数自动清理。")
	add(team+"/trash/restore/{id}", "post", "从回收站恢复", "")
	add(team+"/trash/purge/{id}", "delete", "永久删除", "")
	add(team+"/trash/empty", "delete", "清空回收站", "")

	add(team+"/tags", "get", "标签列表", "")
	add(team+"/tags", "post", "创建标签", "")
	add(team+"/tags/{id}", "delete", "删除标签", "")
	add(team+"/tags/{id}/documents", "get", "标签下的文档", "")
	add(team+"/documents/{id}/tags", "get", "文档标签", "")
	add(team+"/documents/{id}/tags", "put", "设置文档标签", "")

	add(team+"/templates", "get", "模板列表", "")
	add(team+"/templates", "post", "创建模板", "")
	add(team+"/templates/{id}", "get", "获取模板", "")
	add(team+"/templates/{id}", "put", "更新模板", "")
	add(team+"/templates/{id}", "delete", "删除模板", "")

	add(team+"/permissions", "get", "权限列表", "")
	add(team+"/permissions", "post", "设置权限", "")
	add(team+"/permissions/{id}", "delete", "删除权限", "")
	add(team+"/permissions/check", "get", "检查权限", "")

	add(team+"/audits", "get", "审计日志", "团队管理员。计费打开且套餐不含 audit 时返回 402。")
	add(team+"/audits/export", "get", "导出审计", "")
	add(team+"/audits/stats", "get", "审计统计", "")

	add(team+"/favorites", "get", "收藏", "")
	add(team+"/favorites/{id}", "post", "添加收藏", "")
	add(team+"/favorites/{id}", "delete", "取消收藏", "")

	add(team+"/storage/status", "get", "存储用量", "展示配额。媒体上传会按 MaxStorageMB 拦截；保存文档正文不会。团队数和成员数不在本服务检查。")

	add(team+"/webhooks", "get", "Webhook 列表", "")
	add(team+"/webhooks", "post", "创建 Webhook", "默认事件 document.created 与 document.updated。创建文档投递 document.created，保存文档投递 document.updated。events 可以是 JSON 数组或逗号分隔，create_doc / edit_doc 会当作别名匹配。")
	add(team+"/webhooks/{id}", "delete", "删除 Webhook", "")
	add(team+"/webhooks/{id}/toggle", "put", "开关 Webhook", "")
	add(team+"/webhooks/{id}/logs", "get", "Webhook 投递日志", "")

	add(team+"/upload", "post", "上传媒体", "计费打开且设了存储上限时，超出返回 402 plan_limit。")
	add(team+"/media", "get", "媒体列表", "")
	add(team+"/media/{filename}", "get", "读取媒体", "")
	add(team+"/media/{filename}", "delete", "删除媒体", "")

	add(team+"/shares/{id}", "delete", "删除分享", "")
	add(team+"/collaborators/{id}", "put", "更新协作者", "")
	add(team+"/collaborators/{id}", "delete", "移除协作者", "")
	add(team+"/comments/{id}", "put", "更新评论", "")
	add(team+"/comments/{id}", "delete", "删除评论", "")
	add(team+"/search-targets", "get", "搜索分享对象", "")
	add(team+"/import", "post", "导入", "支持 txt、md、html、docx、xlsx。Markdown 导入后仍是 doc。")
	add(team+"/dashboard", "get", "概览", "")
	add(team+"/system-info", "get", "系统信息", "仅团队管理员。")
	add(team+"/notifications", "get", "通知", "")
	add(team+"/notifications/{id}/read", "put", "标为已读", "")
	add(team+"/notifications/read-all", "put", "全部已读", "")
	add(team+"/notifications/{id}", "delete", "删除通知", "")
	add(team+"/notifications/unread-count", "get", "未读数", "")
	add(team+"/deadlines", "get", "交期列表", "")
	add(team+"/deadlines/board", "get", "交期看板", "")
	add(team+"/deadlines", "post", "创建交期", "")
	add(team+"/deadlines/{id}", "get", "交期详情", "")
	add(team+"/deadlines/{id}", "put", "更新交期", "")
	add(team+"/deadlines/{id}", "delete", "删除交期", "")
	add(team+"/deadlines/{id}/events", "get", "交期变更", "")
	add(team+"/reminder-rules", "get", "提醒规则", "")
	add(team+"/reminder-rules", "post", "创建提醒规则", "")
	add(team+"/reminder-rules/seed", "post", "写入默认提醒规则", "")
	add(team+"/reminder-rules/{id}", "put", "更新提醒规则", "")
	add(team+"/reminder-rules/{id}", "delete", "删除提醒规则", "")
	add(team+"/reminder-log", "get", "提醒记录", "")

	return paths
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
