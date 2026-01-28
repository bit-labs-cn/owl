package cache_captcha

import (
	"context"
	"errors"
	"time"
)

var ErrCaptchaNotFound = errors.New("验证码不存在")

type CaptchaRecord struct {
	Type        string     `json:"type"`                  // 验证码类型
	ClickDots   []ClickDot `json:"clickDots,omitempty"`   // 点选坐标
	SlideDX     int        `json:"slideDx,omitempty"`     // 滑块X
	SlideDY     int        `json:"slideDy,omitempty"`     // 滑块Y
	RotateAngle int        `json:"rotateAngle,omitempty"` // 旋转角度
}

type ClickDot struct {
	Index  int `json:"index"`  // 点序号
	X      int `json:"x"`      // X坐标
	Y      int `json:"y"`      // Y坐标
	Width  int `json:"width"`  // 宽度
	Height int `json:"height"` // 高度
}

type CaptchaStore interface {
	// Save 保存验证码记录
	Save(ctx context.Context, key string, record *CaptchaRecord, ttl time.Duration) error
	// Load 加载验证码记录
	Load(ctx context.Context, key string) (*CaptchaRecord, error)
	// Remove 删除验证码记录
	Remove(ctx context.Context, key string) error
	// Start 启动存储清理或初始化逻辑
	Start()
}

const StoreKeyPrefix = "owlCaptcha:"
