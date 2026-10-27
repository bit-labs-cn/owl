package otel

import (
	_ "embed"
	"strings"

	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/provider/conf"
)

// Options 链路追踪配置，对应 conf/otel.yaml。
type Options struct {
	Enabled     bool    `json:"enabled"`
	ServiceName string  `json:"service-name"`
	Protocol    string  `json:"protocol"`
	Endpoint    string  `json:"endpoint"`
	Insecure    bool    `json:"insecure"`
	SampleRatio float64 `json:"sample-ratio"`
}

func normalizeEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return strings.TrimRight(raw, "/")
}

func normalizedRatio(v float64) float64 {
	if v <= 0 || v > 1 {
		return 1
	}
	return v
}

func resolveServiceName(c *conf.Configure, cfg Options) string {
	if name := strings.TrimSpace(cfg.ServiceName); name != "" {
		return name
	}
	if c == nil {
		return defaultServiceName
	}
	var app struct {
		AppName string `json:"app-name"`
	}
	if err := c.GetConfig("app", &app); err == nil {
		if name := strings.TrimSpace(app.AppName); name != "" {
			return name
		}
	}
	return defaultServiceName
}

// TraceServiceProvider 注册链路追踪。
type TraceServiceProvider struct {
	app foundation.Application
}

var _ foundation.ServiceProvider = (*TraceServiceProvider)(nil)

func (s *TraceServiceProvider) Description() string {
	return "OpenTelemetry 链路追踪与 OTLP 导出"
}

func (s *TraceServiceProvider) Register() {
	s.app.Register(func(c *conf.Configure) *Provider {
		var cfg Options
		if err := c.GetConfig("otel", &cfg); err != nil {
			panic(err)
		}
		p, err := New(cfg, resolveServiceName(c, cfg))
		if err != nil {
			panic(err)
		}
		return p
	})
}

func (s *TraceServiceProvider) Boot() {}

//go:embed otel.yaml
var otelConf string

func (s *TraceServiceProvider) Conf() map[string]string {
	return map[string]string{
		"otel.yaml": otelConf,
	}
}
