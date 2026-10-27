package middleware

import (
	"net/http"
	"runtime/debug"

	logContract "bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/otel"
	"github.com/gin-gonic/gin"
)

const LoggerContextKey = "__logger__"

func Recovery(logger logContract.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqCtx := c.Request.Context()
		reqLogger := logger
		if logger != nil {
			reqLogger = logger.WithContext(reqCtx)
		}
		c.Set(LoggerContextKey, reqLogger)

		defer func() {
			if recovered := recover(); recovered != nil {
				requestID, _ := c.Get("request_id")
				reqLogger.Emergency(append([]any{
					"HTTP panic",
					"requestId:", requestID,
					"method:", c.Request.Method,
					"path:", c.Request.URL.Path,
					"panic info:", recovered,
					"stack", string(debug.Stack()),
				}, otel.Fields(reqCtx)...)...)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"msg":     "服务器内部错误",
				})
			}
		}()
		c.Next()
	}
}
