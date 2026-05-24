package mailer

import (
	"context"
	"fmt"

	mailerContract "bit-labs.cn/owl/contract/mailer"
)

// Options 邮件配置
type Options struct {
	Default string     `json:"default"`
	SMTP    SMTPConfig `json:"smtp"`
}

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	FromName string `json:"from-name"`
	TLS      bool   `json:"tls"`
	StartTLS bool   `json:"starttls"`
}

// MailerManager 邮件管理器
type MailerManager struct {
	drivers       map[string]mailerContract.MailDriver
	defaultDriver string
}

// NewMailerManager 创建邮件管理器
func NewMailerManager() *MailerManager {
	return &MailerManager{
		drivers: make(map[string]mailerContract.MailDriver),
	}
}

// AddDriver 添加驱动
func (m *MailerManager) AddDriver(name string, driver mailerContract.MailDriver) {
	m.drivers[name] = driver
}

// SetDefaultDriver 设置默认驱动
func (m *MailerManager) SetDefaultDriver(name string) error {
	if _, exists := m.drivers[name]; !exists {
		return fmt.Errorf("mailer driver '%s' not found", name)
	}
	m.defaultDriver = name
	return nil
}

// GetDriver 获取指定驱动
func (m *MailerManager) GetDriver(name string) (mailerContract.MailDriver, error) {
	if name == "" {
		name = m.defaultDriver
	}
	driver, exists := m.drivers[name]
	if !exists {
		return nil, fmt.Errorf("mailer driver %s not found", name)
	}
	return driver, nil
}

// Default 获取默认驱动
func (m *MailerManager) Default() (mailerContract.MailDriver, error) {
	return m.GetDriver(m.defaultDriver)
}

// Send 使用默认驱动发送邮件
func (m *MailerManager) Send(ctx context.Context, msg *mailerContract.Message) error {
	driver, err := m.Default()
	if err != nil {
		return err
	}
	return driver.Send(ctx, msg)
}

// Use 使用指定驱动发送邮件
func (m *MailerManager) Use(name string) (mailerContract.MailDriver, error) {
	return m.GetDriver(name)
}
