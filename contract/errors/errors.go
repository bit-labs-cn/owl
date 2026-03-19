package errors

// BizError 表示可直接展示给用户的业务错误（非系统内部错误）。
type BizError struct {
	Code    string
	Message string
	Raw     error
}

func NewBizError(code string, msg string) *BizError {
	return &BizError{Code: code, Message: msg}
}

func WrapBizError(code string, msg string, err error) *BizError {
	return &BizError{Code: code, Message: msg, Raw: err}
}

func (e *BizError) Error() string { return e.Message }
func (e *BizError) Unwrap() error { return e.Raw }
