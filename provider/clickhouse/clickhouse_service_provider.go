package clickhouse

import (
	"database/sql"
	_ "embed"

	"bit-labs.cn/owl"
	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/conf"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

var _ foundation.ServiceProvider = (*ClickHouseServiceProvider)(nil)

// ClickHouseServiceProvider 注册 ClickHouse 原生客户端与 database/sql。
type ClickHouseServiceProvider struct {
	app foundation.Application
}

func (p *ClickHouseServiceProvider) Description() string {
	return "ClickHouse 客户端（原生协议 / database/sql）"
}

func (p *ClickHouseServiceProvider) Register() {
	p.app.Register(func(c *conf.Configure, l log.Logger) *Client {
		var cfg Config
		err := c.GetConfig("clickhouse", &cfg)
		owl.PanicIf(err)

		ApplyDefaults(&cfg)
		err = Validate(&cfg)
		owl.PanicIf(err)

		cli, err := NewClient(&cfg, l)
		owl.PanicIf(err)

		err = HealthCheck(cli, &cfg)
		owl.PanicIf(err)

		return cli
	})

	p.app.Register(func(cli *Client) ch.Conn {
		return cli.Native()
	})

	p.app.Register(func(cli *Client) *sql.DB {
		return cli.DB()
	})
}

func (p *ClickHouseServiceProvider) Boot() {}

//go:embed clickhouse.yaml
var clickhouseYaml string

func (p *ClickHouseServiceProvider) Conf() map[string]string {
	return map[string]string{
		"clickhouse.yaml": clickhouseYaml,
	}
}
