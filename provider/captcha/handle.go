package captcha

import (
	"bit-labs.cn/owl/provider/router"
	"github.com/gin-gonic/gin"
)

// handleGenerate 生成验证码
func (s *Service) handleGenerate(ctx *gin.Context) {
	var req GenerateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		router.BadRequest(ctx, err.Error())
		return
	}
	res, err := s.Generate(ctx.Request.Context(), req.Type)
	if err != nil {
		router.InternalError(ctx, err)
		return
	}
	router.Success(ctx, res)
}

// handleVerify 校验验证码
func (s *Service) handleVerify(ctx *gin.Context) {
	var req VerifyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		router.BadRequest(ctx, err.Error())
		return
	}
	ok, err := s.Verify(ctx.Request.Context(), &req)
	if err != nil {
		router.InternalError(ctx, err)
		return
	}
	if !ok {
		router.BadRequest(ctx, "验证码校验失败")
		return
	}
	router.Success(ctx, gin.H{"valid": true})
}
