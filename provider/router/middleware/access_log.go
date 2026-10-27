package middleware

import (
	"path"
	"strings"
	"time"

	logContract "bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/otel"
	"github.com/gin-gonic/gin"
)

// AccessLogConfig 访问日志配置
type AccessLogConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Format    string   `yaml:"format"`
	SkipPaths []string `yaml:"skip-paths"`
}

// AccessLog 访问日志中间件
// 记录HTTP请求的访问日志，支持跳过指定路径
func AccessLog(logger logContract.Logger, config AccessLogConfig) gin.HandlerFunc {
	skipExact := make(map[string]struct{}, len(config.SkipPaths))
	skipPrefixBases := make([]string, 0, len(config.SkipPaths))
	skipGlobs := make([]string, 0, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		if strings.HasSuffix(p, "/*") {
			skipPrefixBases = append(skipPrefixBases, strings.TrimSuffix(p, "/*"))
			continue
		}
		if strings.ContainsAny(p, "*?[") {
			skipGlobs = append(skipGlobs, p)
			continue
		}
		skipExact[p] = struct{}{}
	}

	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		reqPath := c.Request.URL.Path
		if _, ok := skipExact[reqPath]; ok {
			c.Next()
			return
		}
		for _, base := range skipPrefixBases {
			if reqPath == base || strings.HasPrefix(reqPath, base+"/") {
				c.Next()
				return
			}
		}
		for _, pattern := range skipGlobs {
			ok, err := path.Match(pattern, reqPath)
			if err == nil && ok {
				c.Next()
				return
			}
		}

		start := time.Now()
		requestPath := reqPath
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		requestID, _ := c.Get("request_id")
		if rawQuery != "" {
			requestPath = requestPath + "?" + rawQuery
		}
		reqCtx := c.Request.Context()
		args := append([]any{
			"HTTP请求",
			"requestId:", requestID,
			"status", status,
			"method", method,
			"path", requestPath,
			"latency", latency.String(),
			"clientIp", clientIP,
			"userAgent", userAgent,
		}, otel.Fields(reqCtx)...)
		reqLogger := logger
		if logger != nil {
			reqLogger = logger.WithContext(reqCtx)
		}

		if status >= 500 {
			reqLogger.Error(args...)
			return
		}
		args[0] = "HTTP请求 "
		reqLogger.Info(args...)
	}
}
