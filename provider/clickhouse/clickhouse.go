package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bit-labs.cn/owl/contract/log"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

// Client 同时持有原生 Conn 与 database/sql 连接池；两者共享同一套 Options，会产生两套池，请按场景择一为主。
type Client struct {
	cfg    *Config
	native ch.Conn
	db     *sql.DB
}

// NewClient 根据配置创建客户端（原生 + sql.DB）。
func NewClient(cfg *Config, l log.Logger) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("clickhouse: 配置不能为空")
	}
	opts, err := BuildDriverOptions(cfg)
	if err != nil {
		return nil, err
	}

	native, err := ch.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: Open 失败: %w", err)
	}

	db := ch.OpenDB(opts)

	if l != nil {
		l.Debug("ClickHouse 已初始化",
			"protocol", opts.Protocol.String(),
			"addrs", opts.Addr,
			"database", cfg.Auth.Database,
			"user", cfg.Auth.Username,
			"strategy", cfg.ConnOpenStrategy,
			"tls", cfg.TLS.Enabled,
		)
	}

	return &Client{
		cfg:    cfg,
		native: native,
		db:     db,
	}, nil
}

// Native 返回高性能原生连接（含连接池语义）。
func (c *Client) Native() ch.Conn {
	if c == nil {
		return nil
	}
	return c.native
}

// DB 返回 database/sql 句柄。
func (c *Client) DB() *sql.DB {
	if c == nil {
		return nil
	}
	return c.db
}

// Config 返回初始化时的配置副本引用（只读使用）。
func (c *Client) Config() *Config {
	if c == nil {
		return nil
	}
	return c.cfg
}

// Ping 对原生连接执行 Ping。
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.native == nil {
		return errors.New("clickhouse: 客户端未初始化")
	}
	return c.native.Ping(ctx)
}

// PingSQL 对 sql.DB 执行 Ping。
func (c *Client) PingSQL(ctx context.Context) error {
	if c == nil || c.db == nil {
		return errors.New("clickhouse: sql.DB 未初始化")
	}
	return c.db.PingContext(ctx)
}

// Close 关闭原生连接与 sql.DB。
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var errs []error
	if c.native != nil {
		if err := c.native.Close(); err != nil {
			errs = append(errs, fmt.Errorf("native close: %w", err))
		}
		c.native = nil
	}
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("sql.db close: %w", err))
		}
		c.db = nil
	}
	return errors.Join(errs...)
}

// HealthCheck 按配置执行启动健康检查（超时来自 health-check.timeout-seconds）。
func HealthCheck(cli *Client, cfg *Config) error {
	if cli == nil || cfg == nil || !cfg.HealthCheck.Enabled {
		return nil
	}
	sec := cfg.HealthCheck.TimeoutSeconds
	if sec <= 0 {
		sec = 5
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
	defer cancel()
	if err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("clickhouse: 健康检查失败（原生 Ping）: %w", err)
	}
	return nil
}
