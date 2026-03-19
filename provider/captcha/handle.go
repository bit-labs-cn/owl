package captcha

import (
	"bit-labs.cn/owl/provider/router"
	"github.com/gin-gonic/gin"
)

// @Summary		生成验证码
// @Description	根据类型生成验证码
// @Tags			验证码
// @Accept		json
// @Produce		json
// @Param			request	body		GenerateReq	true	"验证码生成请求"
// @Success		200		{object}	router.Resp{data=ClickCaptchaResp}	"操作成功"
// @Success		200		{object}	router.Resp{data=SlideCaptchaResp}	"操作成功"
// @Success		200		{object}	router.Resp{data=RotateCaptchaResp}	"操作成功"
// @Failure		400		{object}	router.Resp	"参数错误"
// @Failure		500		{object}	router.Resp	"服务器内部错误"
// @Router			/captcha/generate [POST]
// handleGenerate 生成验证码
func (s *Service) handleGenerate(ctx *gin.Context) {
	var req GenerateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		router.Fail(ctx, err)
		return
	}
	res, err := s.Generate(ctx.Request.Context(), req.Type)
	if err != nil {
		router.InternalError(ctx, err)
		return
	}
	router.Success(ctx, res)
}

// @Summary		校验验证码
// @Description	根据类型校验验证码
// @Tags			验证码
// @Accept		json
// @Produce		json
// @Param			request	body		VerifyReq	true	"验证码校验请求"
// @Success		200		{object}	router.Resp{data=map[string]bool}	"操作成功"
// @Failure		400		{object}	router.Resp	"参数错误"
// @Failure		500		{object}	router.Resp	"服务器内部错误"
// @Router			/captcha/verify [POST]
// handleVerify 校验验证码
func (s *Service) handleVerify(ctx *gin.Context) {
	var req VerifyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		router.Fail(ctx, err)
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
