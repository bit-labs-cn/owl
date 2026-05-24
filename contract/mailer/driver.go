package mailer

import "context"

// MailDriver 邮件发送驱动接口
type MailDriver interface {
	Send(ctx context.Context, msg *Message) error
}
