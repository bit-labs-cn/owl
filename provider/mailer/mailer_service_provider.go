package mailer

import (
	_ "embed"

	"bit-labs.cn/owl"
	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/provider/conf"
)

var _ foundation.ServiceProvider = (*MailerServiceProvider)(nil)

// MailerServiceProvider 邮件服务提供者
type MailerServiceProvider struct {
	app foundation.Application
}

func (m *MailerServiceProvider) Description() string {
	return "邮件发送驱动管理"
}

// Register 注册服务
func (m *MailerServiceProvider) Register() {
	m.app.Register(func(c *conf.Configure) *MailerManager {
		var opt Options
		err := c.GetConfig("mailer", &opt)
		owl.PanicIf(err)

		manager := NewMailerManager()

		if opt.SMTP.Host != "" {
			manager.AddDriver("smtp", NewSMTPDriver(&opt.SMTP))
		}

		driver := opt.Default
		if driver == "" {
			driver = "smtp"
		}
		if err := manager.SetDefaultDriver(driver); err != nil {
			owl.PanicIf(err)
		}

		return manager
	})
}

// Boot 启动服务
func (m *MailerServiceProvider) Boot() {}

//go:embed mailer.yaml
var mailerYaml string

func (m *MailerServiceProvider) Conf() map[string]string {
	return map[string]string{
		"mailer.yaml": mailerYaml,
	}
}
