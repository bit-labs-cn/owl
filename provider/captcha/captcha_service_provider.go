package captcha

import (
	_ "embed"
	"errors"
	"strings"
	"time"

	"bit-labs.cn/owl"
	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/provider/captcha/cache_captcha"
	"bit-labs.cn/owl/provider/conf"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
)

type CaptchaServiceProvider struct {
	app foundation.Application
}

var _ foundation.ServiceProvider = (*CaptchaServiceProvider)(nil)

// Register 注册验证码服务到容器
func (i *CaptchaServiceProvider) Register() {
	type deps struct {
		dig.In
		Client redis.UniversalClient `optional:"true"`
	}

	i.app.Register(func(c *conf.Configure, d deps) *Service {
		var opt Options
		err := c.GetConfig("captcha", &opt)
		owl.PanicIf(err)

		store, err := newStore(opt, d.Client)
		owl.PanicIf(err)
		store.Start()

		svc, err := NewService(opt, store)
		owl.PanicIf(err)
		return svc
	})
}

// Boot 启动时挂载验证码路由
func (i *CaptchaServiceProvider) Boot() {
	err := i.app.Invoke(func(engine *gin.Engine, svc *Service) {
		svc.RegisterRoutes(engine)
	})
	owl.PanicIf(err)
}

//go:embed captcha.yaml
var captchaYaml string

// GenerateConf 生成验证码默认配置
func (i *CaptchaServiceProvider) GenerateConf() map[string]string {
	return map[string]string{
		"captcha.yaml": captchaYaml,
	}
}

// newStore 创建验证码存储实现
func newStore(opt Options, client redis.UniversalClient) (cache_captcha.CaptchaStore, error) {
	store := strings.TrimSpace(strings.ToLower(opt.Store))
	if store == "redis" {
		if client == nil {
			return nil, errors.New("redis存储未配置")
		}
		return cache_captcha.NewRedisStore(client), nil
	}
	interval := time.Duration(opt.CleanupInterval) * time.Second
	return cache_captcha.NewMemoryStore(interval), nil
}
