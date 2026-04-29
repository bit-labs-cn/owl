package clickhouse

import (
	"fmt"
	"net/url"
	"strings"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

// BuildDriverOptions 将 YAML 配置转换为官方驱动的 Options。
func BuildDriverOptions(c *Config) (*ch.Options, error) {
	addrs, err := resolvedAddrs(c)
	if err != nil {
		return nil, err
	}

	tlsCfg, err := buildTLS(c.TLS)
	if err != nil {
		return nil, err
	}

	comp, err := compressionFromConfig(c.Compression)
	if err != nil {
		return nil, err
	}

	var proto ch.Protocol
	switch strings.ToLower(strings.TrimSpace(c.Protocol)) {
	case "http":
		proto = ch.HTTP
	default:
		proto = ch.Native
	}

	var strat ch.ConnOpenStrategy
	switch strings.ToLower(strings.TrimSpace(c.ConnOpenStrategy)) {
	case "round_robin":
		strat = ch.ConnOpenRoundRobin
	case "random":
		strat = ch.ConnOpenRandom
	default:
		strat = ch.ConnOpenInOrder
	}

	settings := ch.Settings{}
	for k, v := range c.Settings {
		settings[k] = v
	}

	o := &ch.Options{
		Protocol: proto,
		Addr:     addrs,
		Auth: ch.Auth{
			Database: c.Auth.Database,
			Username: c.Auth.Username,
			Password: c.Auth.Password,
		},
		TLS:                  tlsCfg,
		Settings:             settings,
		Compression:          comp,
		DialTimeout:          dialDuration(c),
		ReadTimeout:          readDuration(c),
		ConnOpenStrategy:     strat,
		FreeBufOnConnRelease: c.FreeBufOnConnRelease,
		HttpUrlPath:          c.HttpUrlPath,
		HttpHeaders:          c.HttpHeaders,
	}
	if c.HttpMaxConnsPerHost > 0 {
		o.HttpMaxConnsPerHost = c.HttpMaxConnsPerHost
	}
	if c.HTTPProxyURL != "" {
		u, perr := url.Parse(c.HTTPProxyURL)
		if perr != nil {
			return nil, fmt.Errorf("clickhouse: 解析 http-proxy-url: %w", perr)
		}
		o.HTTPProxyURL = u
	}
	if c.MaxOpenConns > 0 {
		o.MaxOpenConns = c.MaxOpenConns
	}
	if c.MaxIdleConns > 0 {
		o.MaxIdleConns = c.MaxIdleConns
	}
	if c.ConnMaxLifetimeSeconds > 0 {
		o.ConnMaxLifetime = connMaxLifetime(c)
	}
	if c.BlockBufferSize > 0 {
		if c.BlockBufferSize > 255 {
			return nil, fmt.Errorf("clickhouse: block-buffer-size 过大（最大 255）")
		}
		o.BlockBufferSize = uint8(c.BlockBufferSize) //nolint:gosec // 已上限校验
	}
	if c.MaxCompressionBuffer > 0 {
		o.MaxCompressionBuffer = c.MaxCompressionBuffer
	}

	ci := ch.ClientInfo{}
	for _, p := range c.ClientInfo.Products {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		ci.Products = append(ci.Products, struct {
			Name    string
			Version string
		}{Name: p.Name, Version: p.Version})
	}
	if len(c.ClientInfo.Comment) > 0 {
		ci.Comment = append(ci.Comment, c.ClientInfo.Comment...)
	}
	if len(ci.Products) > 0 || len(ci.Comment) > 0 {
		o.ClientInfo = ci
	}

	return o, nil
}
