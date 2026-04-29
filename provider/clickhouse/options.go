package clickhouse

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Config 对应 conf/clickhouse.yaml 根结构（与 GetConfig("clickhouse", ...) 对齐）。
type Config struct {
	Addrs    []string `json:"addrs"` // 形如 host:port；非空时优先于 host/port
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"` // native | http

	Auth AuthConfig `json:"auth"`

	ConnOpenStrategy string `json:"conn-open-strategy"` // in_order | round_robin | random

	TLS TLSConfig `json:"tls"`

	Compression CompressionConfig `json:"compression"`

	DialTimeoutSeconds     int               `json:"dial-timeout"`      // 秒，默认 30
	ReadTimeoutSeconds     int               `json:"read-timeout"`      // 秒，默认 300
	MaxOpenConns           int               `json:"max-open-conns"`    // 0 表示交给驱动默认
	MaxIdleConns           int               `json:"max-idle-conns"`    // 0 表示交给驱动默认
	ConnMaxLifetimeSeconds int               `json:"conn-max-lifetime"` // 秒，默认 3600
	BlockBufferSize        int               `json:"block-buffer-size"`
	MaxCompressionBuffer   int               `json:"max-compression-buffer"`
	FreeBufOnConnRelease   bool              `json:"free-buf-on-conn-release"`
	HttpUrlPath            string            `json:"http-url-path"`
	HttpHeaders            map[string]string `json:"http-headers"`
	HttpMaxConnsPerHost    int               `json:"http-max-conns-per-host"`
	HTTPProxyURL           string            `json:"http-proxy-url"`

	Settings map[string]any `json:"settings"`

	ClientInfo ClientInfoConfig `json:"client-info"`

	HealthCheck HealthCheckConfig `json:"health-check"`
}

type AuthConfig struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	InsecureSkipVerify bool   `json:"insecure-skip-verify"`
	ServerName         string `json:"server-name"`
	MinVersion         string `json:"min-version"` // 1.2 | 1.3
	CAFile             string `json:"ca-file"`
	CertFile           string `json:"cert-file"`
	KeyFile            string `json:"key-file"`
}

type CompressionConfig struct {
	Method string `json:"method"` // none | lz4 | lz4hc | zstd | gzip | deflate | br
	Level  int    `json:"level"`
}

type ClientInfoConfig struct {
	Products []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"products"`
	Comment []string `json:"comment"`
}

type HealthCheckConfig struct {
	Enabled        bool `json:"enabled"`
	TimeoutSeconds int  `json:"timeout-seconds"`
}

// ApplyDefaults 填充缺省项（可重复调用）。
func ApplyDefaults(c *Config) {
	if c == nil {
		return
	}
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 9000
	}
	if c.Protocol == "" {
		c.Protocol = "native"
	}
	if c.ConnOpenStrategy == "" {
		c.ConnOpenStrategy = "in_order"
	}
	if c.Auth.Database == "" {
		c.Auth.Database = "default"
	}
	if c.Auth.Username == "" {
		c.Auth.Username = "default"
	}
	if c.Compression.Method == "" {
		c.Compression.Method = "none"
	}
	if c.DialTimeoutSeconds == 0 {
		c.DialTimeoutSeconds = 30
	}
	if c.ReadTimeoutSeconds == 0 {
		c.ReadTimeoutSeconds = 300
	}
	if c.ConnMaxLifetimeSeconds == 0 {
		c.ConnMaxLifetimeSeconds = 3600
	}
	if c.HealthCheck.TimeoutSeconds == 0 {
		c.HealthCheck.TimeoutSeconds = 5
	}
}

// Validate 校验配置合法性。
func Validate(c *Config) error {
	if c == nil {
		return errors.New("clickhouse: 配置不能为空")
	}
	addrs, err := resolvedAddrs(c)
	if err != nil {
		return err
	}
	for _, a := range addrs {
		host, _, splitErr := net.SplitHostPort(a)
		if splitErr != nil {
			return fmt.Errorf("clickhouse: 无效地址 %q（期望 host:port）: %w", a, splitErr)
		}
		if host == "" {
			return fmt.Errorf("clickhouse: 地址 %q 缺少主机名", a)
		}
	}
	switch strings.ToLower(strings.TrimSpace(c.Protocol)) {
	case "native", "http":
	default:
		return fmt.Errorf("clickhouse: protocol 只能是 native 或 http，当前为 %q", c.Protocol)
	}
	switch strings.ToLower(strings.TrimSpace(c.ConnOpenStrategy)) {
	case "in_order", "round_robin", "random":
	default:
		return fmt.Errorf("clickhouse: conn-open-strategy 无效: %q", c.ConnOpenStrategy)
	}
	if c.DialTimeoutSeconds < 0 || c.ReadTimeoutSeconds < 0 {
		return errors.New("clickhouse: dial-timeout / read-timeout 不能为负数")
	}
	if c.MaxOpenConns < 0 || c.MaxIdleConns < 0 {
		return errors.New("clickhouse: max-open-conns / max-idle-conns 不能为负数")
	}
	if c.ConnMaxLifetimeSeconds < 0 {
		return errors.New("clickhouse: conn-max-lifetime 不能为负数")
	}
	if c.BlockBufferSize < 0 || c.MaxCompressionBuffer < 0 {
		return errors.New("clickhouse: block-buffer-size / max-compression-buffer 不能为负数")
	}
	if c.HttpMaxConnsPerHost < 0 {
		return errors.New("clickhouse: http-max-conns-per-host 不能为负数")
	}
	if c.HealthCheck.TimeoutSeconds < 0 {
		return errors.New("clickhouse: health-check.timeout-seconds 不能为负数")
	}
	return nil
}

func resolvedAddrs(c *Config) ([]string, error) {
	if len(c.Addrs) > 0 {
		out := make([]string, 0, len(c.Addrs))
		for _, a := range c.Addrs {
			s := strings.TrimSpace(a)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		if len(out) == 0 {
			return nil, errors.New("clickhouse: addrs 为空或仅含空字符串")
		}
		return out, nil
	}
	if c.Port <= 0 || c.Port > 65535 {
		return nil, fmt.Errorf("clickhouse: 端口无效: %d", c.Port)
	}
	h := strings.TrimSpace(c.Host)
	if h == "" {
		return nil, errors.New("clickhouse: host 不能为空（且未配置 addrs）")
	}
	return []string{net.JoinHostPort(h, fmt.Sprintf("%d", c.Port))}, nil
}

func dialDuration(c *Config) time.Duration {
	return time.Duration(c.DialTimeoutSeconds) * time.Second
}

func readDuration(c *Config) time.Duration {
	return time.Duration(c.ReadTimeoutSeconds) * time.Second
}

func connMaxLifetime(c *Config) time.Duration {
	return time.Duration(c.ConnMaxLifetimeSeconds) * time.Second
}
