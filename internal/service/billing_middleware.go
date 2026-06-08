package service

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequirePlanFeature checks if the user's plan allows the given feature.
// Returns true if allowed, false if blocked (and writes 402 response).
func RequirePlanFeature(c *gin.Context, feature string) bool {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return false
	}

	limits := GetPlanLimits(userID)

	switch feature {
	case "pdf_export":
		if !limits.PDFExport {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "PDF export requires a Team plan",
				"code":  "plan_limit",
			})
			return false
		}
	case "version_history":
		if !limits.VersionHistory {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "Version history requires a Team plan",
				"code":  "plan_limit",
			})
			return false
		}
	case "external_share":
		if !limits.ExternalShare {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "External sharing requires a Team plan",
				"code":  "plan_limit",
			})
			return false
		}
	case "doc_lock":
		if !limits.DocLock {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "Document lock requires a Team plan",
				"code":  "plan_limit",
			})
			return false
		}
	case "audit":
		if !limits.AuditEnabled {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "Audit logs require a Team plan",
				"code":  "plan_limit",
			})
			return false
		}
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("unknown feature: %s", feature)})
		return false
	}

	return true
}

// CheckDocumentLimit checks if the user can create more documents.
// Returns true if allowed, false if blocked.
func CheckDocumentLimit(c *gin.Context, currentCount int) bool {
	userID := c.GetString("user_id")
	limits := GetPlanLimits(userID)

	// MaxDocuments: 0 = unlimited
	if limits.MaxDocuments == 0 {
		return true
	}
	if currentCount >= limits.MaxDocuments {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("Document limit reached (%d). Upgrade to Team for unlimited documents", limits.MaxDocuments),
			"code":  "plan_limit",
		})
		return false
	}
	return true
}

// CheckStorageLimit checks if the user has exceeded storage limit.
// currentBytes is the current storage usage in bytes.
func CheckStorageLimit(c *gin.Context, currentBytes int64, addBytes int64) bool {
	userID := c.GetString("user_id")
	limits := GetPlanLimits(userID)

	// MaxStorageMB: 0 = unlimited
	if limits.MaxStorageMB == 0 {
		return true
	}

	maxBytes := int64(limits.MaxStorageMB) * 1024 * 1024
	if currentBytes+addBytes > maxBytes {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": fmt.Sprintf("Storage limit reached (%dMB). Upgrade to Team for more storage", limits.MaxStorageMB),
			"code":  "plan_limit",
		})
		return false
	}
	return true
}
