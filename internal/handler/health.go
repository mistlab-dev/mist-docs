package handler

import (
	"context"
	"net/http"
	"syscall"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/c-wind/mist-docs/internal/ws"
	"github.com/gin-gonic/gin"
)

// Healthz 进程/存活探活接口，供负载均衡、systemd、监控调用。
// 返回 overall: ok | degraded：
//   - DB ping 失败 → degraded
//   - 磁盘使用率 > 95% → degraded
// 不做文件统计等重活，保持轻量，适合高频探活。
func Healthz(c *gin.Context) {
	overall := "ok"
	var dbErr string

	// 1. DB 连通性
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if database.DB == nil {
		dbErr = "db not initialized"
		overall = "degraded"
	} else if err := database.DB.PingContext(ctx); err != nil {
		dbErr = err.Error()
		overall = "degraded"
	}

	// 2. 磁盘使用率
	root := store.RootPath()
	var diskUsagePercent float64
	var diskErr string
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err != nil {
		diskErr = err.Error()
		overall = "degraded"
	} else {
		total := stat.Blocks * uint64(stat.Bsize)
		avail := stat.Bavail * uint64(stat.Bsize)
		if total > 0 {
			diskUsagePercent = float64(total-avail) / float64(total) * 100
		}
		if diskUsagePercent > 95 {
			overall = "degraded"
		}
	}

	// 3. WebSocket 连接数
	wsConns := ws.ActiveConnections()

	c.JSON(http.StatusOK, gin.H{
		"overall":       overall,
		"db":            map[string]string{"status": boolString(dbErr == ""), "error": dbErr},
		"disk":          map[string]interface{}{"usage_percent": diskUsagePercent, "error": diskErr},
		"ws_connections": wsConns,
		"time":          time.Now().UTC().Format(time.RFC3339),
	})
}

func boolString(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}
