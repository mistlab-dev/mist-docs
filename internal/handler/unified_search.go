package handler

// Unified cross-product search endpoint (#78 T3 MVP).
// GET /teams/:team_id/search/all?q=&limit=
//
// Sections: docs (per-doc ACL) + term (ingested events) always; audits
// omitted for non-admins or plans without the audit feature — the section
// is absent from the response ("无权限的源不返回"), not a 402: docs/term
// search must keep working for everyone. Source failures degrade to an
// "error" field on that section instead of failing the request.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

func TeamUnifiedSearch(c *gin.Context) {
	teamID := getTeamID(c)
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		c.JSON(http.StatusOK, gin.H{
			"query": "",
			"docs":  gin.H{"items": []any{}},
			"term":  gin.H{"items": []any{}},
		})
		return
	}
	// Truncate by runes, not bytes: CJK keywords would otherwise be cut
	// mid-character (review 应改 2).
	if runes := []rune(keyword); len(runes) > 200 {
		keyword = string(runes[:200])
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 20 {
		limit = 10
	}

	// Audit section visibility mirrors TeamListAudits (admin + audit plan
	// feature), but as section-omission rather than a whole-request 402.
	canSeeAudits := false
	switch getTeamRole(c) {
	case "admin", "owner":
		if limits := service.GetPlanLimits(c.GetString("user_id")); limits.AuditEnabled {
			canSeeAudits = true
		}
	}

	res := service.UnifiedSearch(c.Request.Context(), teamID, c.GetString("user_id"), getTeamRole(c), keyword, limit, canSeeAudits)
	c.JSON(http.StatusOK, res)
}
