package captcha

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册验证码路由
func (s *Service) RegisterRoutes(engine *gin.Engine) {
	if !s.opt.Enabled {
		return
	}

	group := engine.Group("/api/v1")
	group.POST("/generate", s.handleGenerate)
	group.POST("/verify", s.handleVerify)
}
