package router

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"bit-labs.cn/owl/contract/foundation"
	logContract "bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/conf"
	"bit-labs.cn/owl/provider/otel"
	"bit-labs.cn/owl/provider/router/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var _ foundation.ServiceProvider = (*RouterServiceProvider)(nil)

type RouterServiceProvider struct {
	app       foundation.Application
	opt       RouterOptions
	engine    *gin.Engine
	logger    logContract.Logger
	configure *conf.Configure
	serverCfg ServerConfig
	srv       *http.Server
}

func (i *RouterServiceProvider) Description() string {
	return "HTTP 路由与中间件"
}

func (i *RouterServiceProvider) Register() {
	i.app.Register(func(c *conf.Configure, l logContract.Logger) (*RouterServiceProvider, *gin.Engine) {
		i.configure = c
		i.logger = l
		err := c.GetConfig("router", &i.opt)
		if err != nil {
			panic(err)
		}
		i.BuildEngine()
		return i, i.engine
	})
}

func (i *RouterServiceProvider) Boot() {
	// 路由服务启动时的初始化逻辑
}

func (i *RouterServiceProvider) Run() {

	err := i.configure.GetConfig("router.server", &i.serverCfg)
	if err != nil {
		panic("读取配置失败，请检查 router.yaml 配置文件")
	}

	if i.engine == nil {
		panic("engine is nil")
	}

	addr := fmt.Sprintf("%s:%d", i.serverCfg.Host, i.serverCfg.Port)
	if i.serverCfg.TLS.Enabled {
		i.serveTLS(addr)
		return
	}

	i.srv = &http.Server{
		Addr:           addr,
		Handler:        i.engine,
		ReadTimeout:    time.Duration(i.serverCfg.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(i.serverCfg.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(i.serverCfg.IdleTimeout) * time.Second,
		MaxHeaderBytes: i.serverCfg.MaxHeaderBytes,
	}
	i.logger.Info("启动 HTTP 服务器", "监听", addr)
	if err := i.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func (i *RouterServiceProvider) serveTLS(addr string) {
	if i.serverCfg.TLS.CertFile == "" || i.serverCfg.TLS.KeyFile == "" {
		panic(fmt.Errorf("router.server.tls.cert-file 与 router.server.tls.key-file 不能为空"))
	}

	tlsCfg, err := i.buildTLSConfig()
	if err != nil {
		panic(err)
	}

	i.srv = &http.Server{
		Addr:           addr,
		Handler:        i.engine,
		ReadTimeout:    time.Duration(i.serverCfg.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(i.serverCfg.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(i.serverCfg.IdleTimeout) * time.Second,
		MaxHeaderBytes: i.serverCfg.MaxHeaderBytes,
		TLSConfig:      tlsCfg,
	}

	i.logger.Info("启动 HTTPS 服务器", "addr", addr)
	if err := i.srv.ListenAndServeTLS(i.serverCfg.TLS.CertFile, i.serverCfg.TLS.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

// Shutdown 优雅关闭 HTTP server，并刷出链路追踪。
func (i *RouterServiceProvider) Shutdown(ctx context.Context) error {
	var httpErr error
	if i.srv != nil {
		i.logger.Info("开始关闭 HTTP 服务器")
		httpErr = i.srv.Shutdown(ctx)
	}
	traceErr := otel.Shutdown(ctx)
	return errors.Join(httpErr, traceErr)
}

func (i *RouterServiceProvider) buildTLSConfig() (*tls.Config, error) {
	minVersion, err := i.parseTLSMinVersion(i.serverCfg.TLS.MinVersion)
	if err != nil {
		return nil, err
	}

	tlsCfg := &tls.Config{MinVersion: minVersion}
	if i.serverCfg.TLS.RequireClientCert {
		if i.serverCfg.TLS.ClientCAFile == "" {
			return nil, fmt.Errorf("router.server.tls.client-ca-file 不能为空")
		}
		pool, err := i.loadCertPoolFromPEM(i.serverCfg.TLS.ClientCAFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsCfg, nil
}

func (i *RouterServiceProvider) loadCertPoolFromPEM(path string) (*x509.CertPool, error) {
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(pemData); !ok {
		return nil, fmt.Errorf("router.server.tls.client-ca-file 证书解析失败")
	}
	return pool, nil
}

func (i *RouterServiceProvider) parseTLSMinVersion(v string) (uint16, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return tls.VersionTLS12, nil
	}
	if strings.HasPrefix(v, "TLS") || strings.HasPrefix(v, "tls") {
		v = strings.TrimSpace(v[3:])
	}
	switch v {
	case "1.0", "1":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("router.server.tls.min-version 不合法: %s", v)
	}
}

// BuildEngine 初始化路由引擎
func (i *RouterServiceProvider) BuildEngine() *gin.Engine {
	// 设置 Gin 模式
	gin.SetMode(i.opt.Mode)

	// 创建 Gin 引擎
	i.engine = gin.New()

	i.setupMiddleware()
	i.setupStatic()
	i.setupTemplate()
	i.setupHealth()
	i.setupMetrics()

	return i.engine
}

// setupMiddleware 设置中间件

func (i *RouterServiceProvider) setupMiddleware() {
	i.engine.Use(middleware.RequestID())
	i.engine.Use(middleware.Recovery(i.logger))

	if i.opt.Middleware.Logger {
		i.engine.Use(middleware.AccessLog(i.logger, middleware.AccessLogConfig{
			Enabled:   i.opt.Log.AccessLog,
			Format:    i.opt.Log.AccessLogFormat,
			SkipPaths: i.opt.Log.SkipPaths,
		}))
	}

	if i.opt.Security.HttpsRedirect {
		i.engine.Use(middleware.HttpsRedirect())
	}

	// CORS 中间件
	if i.opt.Middleware.Cors {
		config := cors.Config{
			AllowOrigins:     i.opt.Cors.AllowedOrigins,
			AllowMethods:     i.opt.Cors.AllowedMethods,
			AllowHeaders:     i.opt.Cors.AllowedHeaders,
			ExposeHeaders:    i.opt.Cors.ExposedHeaders,
			AllowCredentials: i.opt.Cors.AllowCredentials,
			MaxAge:           time.Duration(i.opt.Cors.MaxAge) * time.Second,
		}
		if len(i.opt.Cors.AllowedOrigins) == 0 {
			config.AllowAllOrigins = true
		}
		i.engine.Use(cors.New(config))
	}

	// 安全头中间件
	securityConfig := middleware.SecurityConfig{
		ContentTypeNoSniff: i.opt.Security.ContentTypeNoSniff,
		XssProtection:      i.opt.Security.XssProtection,
		FrameDeny:          i.opt.Security.FrameDeny,
		Hsts:               i.opt.Security.Hsts,
		HstsMaxAge:         i.opt.Security.HstsMaxAge,
	}
	i.engine.Use(middleware.Security(securityConfig))
}

func (i *RouterServiceProvider) setupStatic() {
	if !i.opt.Static.Enabled {
		return
	}
	if i.opt.Static.ListDirectory {
		i.engine.StaticFS(i.opt.Static.Path, http.Dir(i.opt.Static.Root))
		return
	}
	i.engine.Static(i.opt.Static.Path, i.opt.Static.Root)
}

func (i *RouterServiceProvider) setupTemplate() {
	if !i.opt.Template.Enabled {
		return
	}
	i.engine.LoadHTMLGlob(i.opt.Template.Pattern)
	if i.opt.Template.Delims.Left != "" && i.opt.Template.Delims.Right != "" {
		i.engine.Delims(i.opt.Template.Delims.Left, i.opt.Template.Delims.Right)
	}
}

func (i *RouterServiceProvider) setupHealth() {
	if !i.opt.Health.Enabled {
		return
	}
	i.engine.GET(i.opt.Health.Path, func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
}

func (i *RouterServiceProvider) setupMetrics() {
	if !i.opt.Metrics.Enabled {
		return
	}
	i.engine.GET(i.opt.Metrics.Path, func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
}

//go:embed router.yaml
var routerYaml string

func (i *RouterServiceProvider) Conf() map[string]string {
	return map[string]string{
		"router.yaml": routerYaml,
	}
}
