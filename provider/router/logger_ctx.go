package router

import (
	logContract "bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/router/middleware"
	"github.com/gin-gonic/gin"
)

func GetLoggerFromCtx(ctx *gin.Context) logContract.Logger {
	v, ok := ctx.Get(middleware.LoggerContextKey)
	if !ok {
		return nil
	}
	l, _ := v.(logContract.Logger)
	return l
}
