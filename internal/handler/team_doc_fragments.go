package handler

import (
	"net/http"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 文档 ↔ 团队片段联动（方向一：终端进、知识出）
// fragment 数据存储在 mist-team-server 的 fragments 表（同一 mist_team 库）。
// 此处仅存关联关系（md_doc_fragments），并通过 join fragments 取实时标题/命令。

// TeamListDocFragments 返回某文档已关联的团队片段（按 position 排序，join 实时数据）。
func TeamListDocFragments(c *gin.Context) {
	teamID := getTeamID(c)
	docID := c.Param("id")

	// 校验文档属于当前团队
	var exists int
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents WHERE id=? AND team_id=? AND status=1`,
		docID, teamID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	rows, err := database.DB.Query(
		`SELECT l.id, l.fragment_id, l.position, l.created_at,
		        IFNULL(f.title, ''), IFNULL(f.command, ''), IFNULL(f.category, ''), f.status
		 FROM md_doc_fragments l
		 LEFT JOIN fragments f ON f.id = l.fragment_id
		 WHERE l.document_id = ? AND l.team_id = ?
		 ORDER BY l.position ASC, l.created_at ASC`, docID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var linkID, fragID, title, command, category, fstatus string
		var position int
		var createdAt string
		rows.Scan(&linkID, &fragID, &position, &createdAt, &title, &command, &category, &fstatus)

		item := map[string]interface{}{
			"link_id":     linkID,
			"fragment_id": fragID,
			"title":       title,
			"command":     command,
			"category":    category,
			"position":    position,
			"created_at":  createdAt,
		}
		if fstatus != "published" {
			item["deleted"] = true // 片段已删除/归档，前端展示占位
		}
		list = append(list, item)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// TeamAttachDocFragment 往文档附加一个团队片段。
func TeamAttachDocFragment(c *gin.Context) {
	teamID := getTeamID(c)
	docID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		FragmentID string `json:"fragment_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.FragmentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 文档属于当前团队
	var exists int
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM md_documents WHERE id=? AND team_id=? AND status=1`,
		docID, teamID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	// 片段属于当前团队且未删除
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM fragments WHERE id=? AND team_id=? AND deleted=0 AND status='published'`,
		req.FragmentID, teamID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "片段不存在或不可用"})
		return
	}

	// 幂等：已存在则直接返回
	var linkID string
	database.DB.QueryRow(
		`SELECT id FROM md_doc_fragments WHERE document_id=? AND fragment_id=?`,
		docID, req.FragmentID).Scan(&linkID)
	if linkID != "" {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"link_id": linkID, "attached": true}, "message": "已存在"})
		return
	}

	// 取当前最大 position
	var maxPos int
	database.DB.QueryRow(
		`SELECT COALESCE(MAX(position), -1) FROM md_doc_fragments WHERE document_id=?`, docID).Scan(&maxPos)

	newID := uuid.New().String()
	_, err := database.DB.Exec(
		`INSERT INTO md_doc_fragments (id, team_id, document_id, fragment_id, position, created_by)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		newID, teamID, docID, req.FragmentID, maxPos+1, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"link_id": newID, "attached": true}, "message": "已关联"})
}

// TeamDetachDocFragment 从文档移除某个片段关联（不删除片段本身）。
func TeamDetachDocFragment(c *gin.Context) {
	teamID := getTeamID(c)
	docID := c.Param("id")
	fragID := c.Param("fragment_id")

	res, err := database.DB.Exec(
		`DELETE FROM md_doc_fragments WHERE document_id=? AND fragment_id=? AND team_id=?`,
		docID, fragID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	c.JSON(http.StatusOK, gin.H{"message": "已移除", "removed": n > 0})
}

// TeamSearchTeamFragments 供"插入片段"选择器使用：按关键词搜索当前团队可用片段。
func TeamSearchTeamFragments(c *gin.Context) {
	teamID := getTeamID(c)
	q := c.Query("q")

	args := []interface{}{teamID}
	where := `WHERE team_id = ? AND deleted = 0 AND status = 'published'`
	if q != "" {
		where += ` AND (title LIKE ? OR command LIKE ? OR category LIKE ?)`
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	where += ` ORDER BY title ASC LIMIT 50`

	rows, err := database.DB.Query(
		`SELECT id, title, LEFT(command, 200), IFNULL(category,'') FROM fragments `+where, args...)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id, title, command, category string
		rows.Scan(&id, &title, &command, &category)
		list = append(list, map[string]interface{}{
			"fragment_id": id, "title": title, "command": command, "category": category,
		})
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}
