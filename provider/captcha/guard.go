package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	errContract "bit-labs.cn/owl/contract/errors"
	"bit-labs.cn/owl/provider/router"
	"github.com/gin-gonic/gin"
)

const CodeCaptchaInvalid = "CAPTCHA_INVALID"

type ctxKey int

const captchaVerifiedKey ctxKey = 1

// NeedCaptchaFunc 由业务侧注入，返回当前请求是否需要图形验证码。
type NeedCaptchaFunc func(c *gin.Context, payload GuardPayload) (need bool, err error)

// GuardPayload 中间件从请求体解析出的公共字段。
type GuardPayload struct {
	Username string
	DeviceID string
	Captcha  *Answer
}

// Answer 请求体中的验证码答案（不含 type，校验时使用配置默认类型）。
type Answer struct {
	CaptchaId string       `json:"captchaId"`
	X         int          `json:"x"`
	Y         int          `json:"y"`
	Angle     int          `json:"angle"`
	Points    []ClickPoint `json:"points"`
}

func (a *Answer) toVerifyReq(captchaType string) *VerifyReq {
	if a == nil {
		return nil
	}
	return &VerifyReq{
		Type:      captchaType,
		CaptchaId: a.CaptchaId,
		X:         a.X,
		Y:         a.Y,
		Angle:     a.Angle,
		Points:    a.Points,
	}
}

type guardBody struct {
	Username string  `json:"username"`
	DeviceID string  `json:"deviceId"`
	Captcha  *Answer `json:"captcha"`
}

// RequiredResp 需要验证码时的统一响应体。
type RequiredResp struct {
	NeedCaptcha bool   `json:"needCaptcha"`
	CaptchaType string `json:"captchaType"`
}

func CaptchaInvalid() *errContract.BizError {
	return errContract.NewBizError(CodeCaptchaInvalid, "验证码错误或已过期")
}

// VerifiedFromContext 请求是否已通过图形验证码校验。
func VerifiedFromContext(ctx context.Context) bool {
	v, ok := ctx.Value(captchaVerifiedKey).(bool)
	return ok && v
}

func setVerified(c *gin.Context) {
	c.Set("captchaVerified", true)
	reqCtx := context.WithValue(c.Request.Context(), captchaVerifiedKey, true)
	c.Request = c.Request.WithContext(reqCtx)
}

// Guard 通用验证码守卫中间件。
func Guard(svc *Service, need NeedCaptchaFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !svc.Enabled() {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			router.Fail(c, err)
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		var parsed guardBody
		if len(body) > 0 {
			if err := json.Unmarshal(body, &parsed); err != nil {
				router.Fail(c, err)
				c.Abort()
				return
			}
		}

		payload := GuardPayload{
			Username: parsed.Username,
			DeviceID: parsed.DeviceID,
			Captcha:  parsed.Captcha,
		}

		require, err := need(c, payload)
		if err != nil {
			router.Fail(c, err)
			c.Abort()
			return
		}
		if !require {
			c.Next()
			return
		}

		captchaType := svc.DefaultType()
		if payload.Captcha == nil || payload.Captcha.CaptchaId == "" {
			router.Success(c, RequiredResp{
				NeedCaptcha: true,
				CaptchaType: captchaType,
			})
			c.Abort()
			return
		}

		ok, verifyErr := svc.Verify(c.Request.Context(), payload.Captcha.toVerifyReq(captchaType))
		if verifyErr != nil {
			router.Fail(c, verifyErr)
			c.Abort()
			return
		}
		if !ok {
			router.Fail(c, CaptchaInvalid())
			c.Abort()
			return
		}

		setVerified(c)
		c.Next()
	}
}

// GuardAlways captcha 启用时始终要求图形验证码（注册等公开接口）。
func GuardAlways(svc *Service) gin.HandlerFunc {
	return Guard(svc, func(_ *gin.Context, _ GuardPayload) (bool, error) {
		return true, nil
	})
}
