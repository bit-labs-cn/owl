package validator

import (
	"reflect"
	"strings"

	"bit-labs.cn/owl/contract/foundation"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
)

var _ foundation.ServiceProvider = (*ValidatorServiceProvider)(nil)

var trans ut.Translator

type ValidatorServiceProvider struct {
	app foundation.Application
}

func (i *ValidatorServiceProvider) Description() string {
	return "结构体验证器"
}

func (i *ValidatorServiceProvider) Register() {
	i.app.Register(func() *validator.Validate {
		v := validator.New(validator.WithRequiredStructEnabled())
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			if label := fld.Tag.Get("label"); label != "" {
				return label
			}
			name := fld.Tag.Get("json")
			if name == "" {
				return fld.Name
			}
			name = strings.Split(name, ",")[0]
			if name == "" || name == "-" {
				return fld.Name
			}
			return name
		})

		zhLocale := zh.New()
		uni := ut.New(zhLocale, zhLocale)
		trans, _ = uni.GetTranslator("zh")
		_ = zhTranslations.RegisterDefaultTranslations(v, trans)

		return v
	})
}

func (i *ValidatorServiceProvider) Boot() {}

func (i *ValidatorServiceProvider) Conf() map[string]string {
	return nil
}
