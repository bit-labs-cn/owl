package validator

import (
	"errors"

	validatorv10 "github.com/go-playground/validator/v10"
)

func TranslateValidationErrors(err error) string {
	var ves validatorv10.ValidationErrors
	if !errors.As(err, &ves) {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	if len(ves) == 0 {
		return "参数验证失败"
	}

	if trans != nil {
		return ves[0].Translate(trans)
	}

	return ves[0].Error()
}
