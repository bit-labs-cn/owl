package mailer

// Attachment 邮件附件
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

// Message 邮件内容
type Message struct {
	From        string       `json:"from"`
	FromName    string       `json:"from_name"`
	To          []string     `json:"to"`
	Cc          []string     `json:"cc"`
	Bcc         []string     `json:"bcc"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body"`
	HTML        string       `json:"html"`
	Attachments []Attachment `json:"attachments"`
}

// SendResult 发送结果
type SendResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
