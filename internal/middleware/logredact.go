package middleware

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryParam matches query parameters whose values must never reach the
// request log (agent secret, login/refresh tokens, signed-URL signatures, codes...).
var sensitiveQueryParam = regexp.MustCompile(`(?i)([?&](?:secret|token|access_token|refresh_token|id_token|sig|signature|password|passwd|pwd|code|key|api_key|apikey|auth|otp|ticket|session)=)[^&]*`)

// RedactQuery replaces the values of sensitive query parameters with "***".
func RedactQuery(pathWithQuery string) string {
	return sensitiveQueryParam.ReplaceAllString(pathWithQuery, "${1}***")
}

// RequestLogger is gin's default request log with sensitive query values hidden.
// The line format is the same as gin.Logger().
func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if p.IsOutputColor() {
			statusColor = p.StatusCodeColor()
			methodColor = p.MethodColor()
			resetColor = p.ResetColor()
		}
		if p.Latency > time.Minute {
			p.Latency = p.Latency.Truncate(time.Second)
		}
		return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"),
			statusColor, p.StatusCode, resetColor,
			p.Latency,
			p.ClientIP,
			methodColor, p.Method, resetColor,
			RedactQuery(p.Path),
			p.ErrorMessage,
		)
	})
}
