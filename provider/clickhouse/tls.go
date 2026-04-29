package clickhouse

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
)

func buildTLS(cfg TLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
	}

	switch strings.ToLower(strings.TrimSpace(cfg.MinVersion)) {
	case "":
		// 使用 Go 默认最低版本
	case "1.2":
		tlsCfg.MinVersion = tls.VersionTLS12
	case "1.3":
		tlsCfg.MinVersion = tls.VersionTLS13
	default:
		return nil, fmt.Errorf("clickhouse: tls.min-version 只能是 1.2 或 1.3，当前 %q", cfg.MinVersion)
	}

	if cfg.CAFile != "" {
		b, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("clickhouse: 读取 tls.ca-file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("clickhouse: tls.ca-file 中无有效 PEM 证书")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.CertFile != "" || cfg.KeyFile != "" {
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return nil, errors.New("clickhouse: tls.cert-file 与 tls.key-file 必须同时配置")
		}
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("clickhouse: 加载客户端证书: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
