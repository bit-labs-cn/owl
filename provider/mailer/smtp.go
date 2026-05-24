package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"strings"

	mailerContract "bit-labs.cn/owl/contract/mailer"
)

var _ mailerContract.MailDriver = (*SMTPDriver)(nil)

// SMTPDriver SMTP 邮件驱动
type SMTPDriver struct {
	cfg SMTPConfig
}

// NewSMTPDriver 创建 SMTP 驱动
func NewSMTPDriver(cfg *SMTPConfig) *SMTPDriver {
	return &SMTPDriver{cfg: *cfg}
}

// Send 发送邮件
func (d *SMTPDriver) Send(ctx context.Context, msg *mailerContract.Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("mailer: recipient is required")
	}

	from := strings.TrimSpace(msg.From)
	if from == "" {
		from = d.cfg.From
	}
	if from == "" {
		return fmt.Errorf("mailer: from address is required")
	}

	body, err := buildMessageBody(d.cfg, from, msg)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", d.cfg.Host, d.cfg.Port)
	recipients := append(append([]string{}, msg.To...), msg.Cc...)
	recipients = append(recipients, msg.Bcc...)

	auth := smtpAuth(d.cfg)

	if d.cfg.TLS && !d.cfg.StartTLS {
		return d.sendWithTLS(addr, auth, from, recipients, body)
	}

	return smtp.SendMail(addr, auth, from, recipients, body)
}

func smtpAuth(cfg SMTPConfig) smtp.Auth {
	if cfg.Username == "" {
		return nil
	}
	return smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
}

func (d *SMTPDriver) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, body []byte) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err = client.Auth(auth); err != nil {
				return err
			}
		}
	}

	if err = client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err = client.Rcpt(rcpt); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(body); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func buildMessageBody(cfg SMTPConfig, from string, msg *mailerContract.Message) ([]byte, error) {
	fromName := strings.TrimSpace(msg.FromName)
	if fromName == "" {
		fromName = cfg.FromName
	}

	var buf bytes.Buffer
	if fromName != "" {
		buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", mime.QEncoding.Encode("utf-8", fromName), from))
	} else {
		buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	}
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))
	if len(msg.Cc) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(msg.Cc, ", ")))
	}
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject)))
	buf.WriteString("MIME-Version: 1.0\r\n")

	if msg.HTML != "" && msg.Body != "" {
		return writeMultipart(&buf, msg)
	}
	if msg.HTML != "" {
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		buf.WriteString(msg.HTML)
	} else {
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		buf.WriteString(msg.Body)
	}
	buf.WriteString("\r\n")
	return buf.Bytes(), nil
}

func writeMultipart(buf *bytes.Buffer, msg *mailerContract.Message) ([]byte, error) {
	var parts bytes.Buffer
	writer := multipart.NewWriter(&parts)

	if msg.Body != "" {
		part, err := writer.CreatePart(map[string][]string{
			"Content-Type": {"text/plain; charset=UTF-8"},
		})
		if err != nil {
			return nil, err
		}
		if _, err = part.Write([]byte(msg.Body)); err != nil {
			return nil, err
		}
	}

	part, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"text/html; charset=UTF-8"},
	})
	if err != nil {
		return nil, err
	}
	if _, err = part.Write([]byte(msg.HTML)); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}

	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n", writer.Boundary()))
	buf.WriteString("\r\n")
	buf.Write(parts.Bytes())
	buf.WriteString("\r\n")
	return buf.Bytes(), nil
}
