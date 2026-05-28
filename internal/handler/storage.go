package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/crypto"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
)

// StorageStatus 存储状态总览
func StorageStatus(c *gin.Context) {
	root := store.RootPath()

	// 磁盘信息
	var disk DiskInfo
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err == nil {
		disk.Total = stat.Blocks * uint64(stat.Bsize)
		disk.Available = stat.Bavail * uint64(stat.Bsize)
		disk.Used = disk.Total - disk.Available
		disk.UsagePercent = float64(disk.Used) / float64(disk.Total) * 100
		disk.TotalHuman = formatBytes(int64(disk.Total))
		disk.UsedHuman = formatBytes(int64(disk.Used))
		disk.AvailableHuman = formatBytes(int64(disk.Available))
	}

	// 文件统计
	fileStats := scanStorage(root)

	// 加密状态
	encryption := EncryptionInfo{
		Enabled: crypto.IsMasterKeyLoaded(),
	}

	// 健康检查
	health := checkStorageHealth(root, disk, fileStats)

	c.JSON(200, gin.H{
		"storage_root": root,
		"disk":         disk,
		"files":        fileStats,
		"encryption":   encryption,
		"health":       health,
	})
}

type DiskInfo struct {
	Total           uint64  `json:"total"`
	Used            uint64  `json:"used"`
	Available       uint64  `json:"available"`
	UsagePercent    float64 `json:"usage_percent"`
	TotalHuman      string  `json:"total_human"`
	UsedHuman       string  `json:"used_human"`
	AvailableHuman  string  `json:"available_human"`
}

type FileStats struct {
	TotalFiles      int              `json:"total_files"`
	TotalSize       int64            `json:"total_size"`
	TotalSizeHuman  string           `json:"total_size_human"`
	VersionFiles    int              `json:"version_files"`
	VersionSize     int64            `json:"version_size"`
	CurrentFiles    int              `json:"current_files"`
	YjsStateFiles   int              `json:"yjs_state_files"`
	TrashFiles      int              `json:"trash_files"`
	TrashSize       int64            `json:"trash_size"`
	TrashSizeHuman  string           `json:"trash_size_human"`
	Departments     []DeptStorage    `json:"departments"`
}

type DeptStorage struct {
	DepartmentID   string `json:"department_id"`
	FileCount      int    `json:"file_count"`
	TotalSize      int64  `json:"total_size"`
	TotalSizeHuman string `json:"total_size_human"`
	DocumentCount  int    `json:"document_count"`
}

type EncryptionInfo struct {
	Enabled    bool   `json:"enabled"`
	Algorithm  string `json:"algorithm,omitempty"`
	DEKActive  bool   `json:"dek_active,omitempty"`
	KeyCount   int    `json:"key_count,omitempty"`
}

type StorageHealth struct {
	Status     string   `json:"status"`            // healthy / warning / critical
	Warnings   []string `json:"warnings,omitempty"`
	Checks     []CheckResult `json:"checks"`
}

type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok / warn / error
	Detail string `json:"detail"`
}

func scanStorage(root string) FileStats {
	stats := FileStats{}

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		name := info.Name()
		size := info.Size()
		stats.TotalFiles++
		stats.TotalSize += size

		switch {
		case name == "current.dat":
			stats.CurrentFiles++
		case len(name) > 5 && name[:2] == "v" && name[len(name)-4:] == ".dat":
			stats.VersionFiles++
			stats.VersionSize += size
		case name == "yjs.state.dat":
			stats.YjsStateFiles++
		}

		// Trash
		rel, _ := filepath.Rel(root, path)
		if len(rel) > 6 && rel[:6] == "_trash" {
			stats.TrashFiles++
			stats.TrashSize += size
		}

		return nil
	})

	stats.TotalSizeHuman = formatBytes(stats.TotalSize)
	stats.VersionSize = stats.TotalSize - int64(stats.CurrentFiles)*0 // version size tracked above
	stats.TrashSizeHuman = formatBytes(stats.TrashSize)

	// Per-department stats
	entries, err := os.ReadDir(root)
	if err != nil {
		return stats
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "_trash" {
			continue
		}
		dept := DeptStorage{DepartmentID: entry.Name()}
		deptPath := filepath.Join(root, entry.Name())
		scanDir(deptPath, &dept)
		dept.TotalSizeHuman = formatBytes(dept.TotalSize)
		stats.Departments = append(stats.Departments, dept)
	}

	return stats
}

func scanDir(dir string, dept *DeptStorage) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		dept.FileCount++
		dept.TotalSize += info.Size()
		if info.Name() == "current.dat" {
			dept.DocumentCount++
		}
		return nil
	})
}

func checkStorageHealth(root string, disk DiskInfo, files FileStats) StorageHealth {
	health := StorageHealth{
		Status:   "healthy",
		Checks:   []CheckResult{},
		Warnings: []string{},
	}

	// Check 1: disk usage
	if disk.UsagePercent > 90 {
		health.Status = "critical"
		health.Checks = append(health.Checks, CheckResult{"disk_usage", "error",
			fmt.Sprintf("磁盘使用 %.1f%%，超过 90%% 阈值", disk.UsagePercent)})
		health.Warnings = append(health.Warnings, fmt.Sprintf("磁盘使用 %.1f%%，即将耗尽！", disk.UsagePercent))
	} else if disk.UsagePercent > 75 {
		if health.Status == "healthy" {
			health.Status = "warning"
		}
		health.Checks = append(health.Checks, CheckResult{"disk_usage", "warn",
			fmt.Sprintf("磁盘使用 %.1f%%，超过 75%% 阈值", disk.UsagePercent)})
		health.Warnings = append(health.Warnings, fmt.Sprintf("磁盘使用 %.1f%%，注意清理", disk.UsagePercent))
	} else {
		health.Checks = append(health.Checks, CheckResult{"disk_usage", "ok",
			fmt.Sprintf("磁盘使用 %.1f%%", disk.UsagePercent)})
	}

	// Check 2: storage root exists
	if _, err := os.Stat(root); err != nil {
		health.Status = "critical"
		health.Checks = append(health.Checks, CheckResult{"storage_root", "error",
			fmt.Sprintf("存储目录不存在: %s", root)})
		health.Warnings = append(health.Warnings, "存储目录不存在")
	} else {
		health.Checks = append(health.Checks, CheckResult{"storage_root", "ok", "存储目录正常"})
	}

	// Check 3: write permission
	testFile := filepath.Join(root, ".healthcheck")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		health.Status = "critical"
		health.Checks = append(health.Checks, CheckResult{"write_permission", "error", "无法写入存储目录"})
		health.Warnings = append(health.Warnings, "存储目录无写入权限")
	} else {
		os.Remove(testFile)
		health.Checks = append(health.Checks, CheckResult{"write_permission", "ok", "写入权限正常"})
	}

	// Check 4: encryption
	if crypto.IsMasterKeyLoaded() {
		health.Checks = append(health.Checks, CheckResult{"encryption", "ok", "AES-256-GCM 已启用"})
	} else {
		if health.Status == "healthy" {
			health.Status = "warning"
		}
		health.Checks = append(health.Checks, CheckResult{"encryption", "warn", "加密未启用"})
		health.Warnings = append(health.Warnings, "文档加密未启用，建议运行 keygen")
	}

	// Check 5: trash size
	if files.TrashSize > 1024*1024*1024 { // > 1GB
		if health.Status == "healthy" {
			health.Status = "warning"
		}
		health.Checks = append(health.Checks, CheckResult{"trash", "warn",
			fmt.Sprintf("回收站占用 %s", files.TrashSizeHuman)})
		health.Warnings = append(health.Warnings, "回收站占用较大，建议清理")
	} else {
		health.Checks = append(health.Checks, CheckResult{"trash", "ok",
			fmt.Sprintf("回收站 %s", files.TrashSizeHuman)})
	}

	// Check 6: version count
	maxVersions := store.VersionKeep()
	if files.VersionFiles > 0 && files.CurrentFiles > 0 {
		avgVersions := files.VersionFiles / files.CurrentFiles
		if avgVersions > maxVersions-2 {
			health.Checks = append(health.Checks, CheckResult{"versions", "warn",
				fmt.Sprintf("平均版本数 %d（上限 %d），即将自动清理旧版本", avgVersions, maxVersions)})
		} else {
			health.Checks = append(health.Checks, CheckResult{"versions", "ok",
				fmt.Sprintf("平均版本数 %d，上限 %d", avgVersions, maxVersions)})
		}
	}

	// Check 7: config consistency
	cfg := config.C.Storage
	if cfg.MaxFileSize == 0 || cfg.VersionKeep == 0 || cfg.Root == "" {
		health.Checks = append(health.Checks, CheckResult{"config", "warn", "部分配置使用默认值"})
	} else {
		health.Checks = append(health.Checks, CheckResult{"config", "ok", "配置完整"})
	}

	return health
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
