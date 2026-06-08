package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
)

// PlanLimits mirrors the Portal PlanLimits structure.
type PlanLimits struct {
	MaxTeams           int  `json:"max_teams"`
	MaxFragments       int  `json:"max_fragments"`
	MaxMembers         int  `json:"max_members"`
	AuditEnabled       bool `json:"audit_enabled"`
	AuditRetentionDays int  `json:"audit_retention_days"`
	PDFExport          bool `json:"pdf_export"`
	VersionHistory     bool `json:"version_history"`
	ExternalShare      bool `json:"external_share"`
	DocLock            bool `json:"doc_lock"`
	APIKeys            bool `json:"api_keys"`
	Webhooks           bool `json:"webhooks"`
	MaxStorageMB       int  `json:"max_storage_mb"`
	MaxDocuments       int  `json:"max_documents"`
	IsTrial            bool `json:"is_trial"`
	TrialDaysLeft      int  `json:"trial_days_left"`
}

type planResponse struct {
	Subscription interface{} `json:"subscription"`
	Limits       PlanLimits  `json:"limits"`
}

var (
	planCache   = make(map[string]*planCacheEntry)
	planCacheMu sync.RWMutex
)

type planCacheEntry struct {
	limits   PlanLimits
	cachedAt time.Time
}

const planCacheTTL = 5 * time.Minute

// GetPlanLimits fetches the user's plan limits from the Portal API.
// If billing is disabled (private deployment), returns Team (unlimited) limits.
func GetPlanLimits(userID string) PlanLimits {
	if !config.C.Billing.Enabled {
		// Private deployment: everything unlocked
		return PlanLimits{
			MaxTeams: -1, MaxFragments: -1, MaxMembers: -1,
			AuditEnabled: true, AuditRetentionDays: 365,
			PDFExport: true, VersionHistory: true, ExternalShare: true,
			DocLock: true, APIKeys: true, Webhooks: true,
			MaxStorageMB: 0, MaxDocuments: 0,
		}
	}

	// Check cache
	planCacheMu.RLock()
	entry, ok := planCache[userID]
	planCacheMu.RUnlock()
	if ok && time.Since(entry.cachedAt) < planCacheTTL {
		return entry.limits
	}

	// Fetch from Portal
	limits := fetchPlanFromPortal(userID)

	// Cache it
	planCacheMu.Lock()
	planCache[userID] = &planCacheEntry{limits: limits, cachedAt: time.Now()}
	planCacheMu.Unlock()

	return limits
}

func fetchPlanFromPortal(userID string) PlanLimits {
	portalURL := config.C.Billing.PortalURL
	if portalURL == "" {
		portalURL = "https://api.mistlab.dev"
	}
	url := fmt.Sprintf("%s/v1/internal/plan?user_id=%s", portalURL, userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return defaultLimits()
	}
	// Internal service call: use admin secret for auth
	if config.C.Billing.AdminSecret != "" {
		req.Header.Set("X-Admin-Secret", config.C.Billing.AdminSecret)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return defaultLimits()
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return defaultLimits()
	}

	var result planResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return defaultLimits()
	}

	return result.Limits
}

// defaultLimits returns Free plan limits as fallback.
func defaultLimits() PlanLimits {
	return PlanLimits{
		MaxTeams: 1, MaxFragments: -1, MaxMembers: 3,
		AuditEnabled: false, PDFExport: false, VersionHistory: false,
		ExternalShare: false, DocLock: false, APIKeys: false, Webhooks: false,
		MaxStorageMB: 500, MaxDocuments: 10,
	}
}
