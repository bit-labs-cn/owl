package captcha

import (
	"context"
	"errors"
	"strings"
	"time"

	"bit-labs.cn/owl/provider/captcha/cache_captcha"
	"github.com/golang/freetype/truetype"
	"github.com/google/uuid"
	"github.com/wenlng/go-captcha-assets/bindata/chars"
	"github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/shapes"
	"github.com/wenlng/go-captcha-assets/resources/thumbs"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
)

type ClickCaptchaResp struct {
	CaptchaId   string `json:"captchaId"`   // 验证码ID
	MasterImage string `json:"masterImage"` // 主图Base64
	ThumbImage  string `json:"thumbImage"`  // 缩略图Base64
}

type SlideCaptchaResp struct {
	CaptchaId   string `json:"captchaId"`   // 验证码ID
	MasterImage string `json:"masterImage"` // 主图Base64
	TileImage   string `json:"tileImage"`   // 滑块图Base64
	TileY       int    `json:"tileY"`       // 滑块Y坐标
	TileWidth   int    `json:"tileWidth"`   // 滑块宽度
	TileHeight  int    `json:"tileHeight"`  // 滑块高度
}

type RotateCaptchaResp struct {
	CaptchaId   string `json:"captchaId"`   // 验证码ID
	MasterImage string `json:"masterImage"` // 主图Base64
	ThumbImage  string `json:"thumbImage"`  // 缩略图Base64
}

type Options struct {
	Enabled         bool   `json:"enabled"`          // 是否启用
	TTL             int    `json:"ttl"`              // 过期时间(秒)
	Type            string `json:"type"`             // 默认验证码类型
	Mode            string `json:"mode"`             // 默认生成模式
	Padding         int    `json:"padding"`          // 校验容差
	Store           string `json:"store"`            // 存储驱动(memory|redis)
	CleanupInterval int    `json:"cleanup-interval"` // 清理间隔(秒)
}

type Service struct {
	opt           Options                    // 配置项
	clickBuilder  click.Builder              // 点选验证码构建器
	slideBuilder  slide.Builder              // 滑块验证码构建器
	rotateBuilder rotate.Builder             // 旋转验证码构建器
	storage       cache_captcha.CaptchaStore // 存储实现
}

// NewService 创建验证码服务实例
func NewService(opt Options, store cache_captcha.CaptchaStore) (*Service, error) {

	clickBuilder := click.NewBuilder()
	clickResources, err := loadClickResources()
	if err != nil {
		return nil, err
	}
	clickBuilder.SetResources(clickResources...)

	slideBuilder := slide.NewBuilder()
	slideResources, err := loadSlideResources()
	if err != nil {
		return nil, err
	}
	slideBuilder.SetResources(slideResources...)

	rotateBuilder := rotate.NewBuilder()
	rotateResources, err := loadRotateResources()
	if err != nil {
		return nil, err
	}
	rotateBuilder.SetResources(rotateResources...)

	return &Service{
		opt:           opt,
		clickBuilder:  clickBuilder,
		slideBuilder:  slideBuilder,
		rotateBuilder: rotateBuilder,
		storage:       store,
	}, nil
}

type GenerateReq struct {
	Type string `json:"type"` // 验证码类型
}

// Generate 生成指定类型的验证码
func (s *Service) Generate(ctx context.Context, typ string) (interface{}, error) {
	resolved := s.resolveType(typ)
	switch resolved {
	case "click":
		return s.GenerateClick(ctx, s.resolveMode(typ))
	case "slide":
		return s.GenerateSlide(ctx, s.resolveMode(typ))
	case "rotate":
		return s.GenerateRotate(ctx)
	default:
		return nil, errors.New("不支持的验证码类型")
	}
}

type ClickPoint struct {
	Index int `json:"index"` // 点序号
	X     int `json:"x"`     // X坐标
	Y     int `json:"y"`     // Y坐标
}

type VerifyReq struct {
	Type      string       `json:"type"`      // 验证码类型
	CaptchaId string       `json:"captchaId"` // 验证码ID
	Points    []ClickPoint `json:"points"`    // 点选坐标
	X         int          `json:"x"`         // 滑块X
	Y         int          `json:"y"`         // 滑块Y
	Angle     int          `json:"angle"`     // 旋转角度
}

// Verify 校验验证码
func (s *Service) Verify(ctx context.Context, req *VerifyReq) (bool, error) {
	resolved := s.resolveType(req.Type)
	switch resolved {
	case "click":
		return s.VerifyClick(ctx, &VerifyClickReq{CaptchaId: req.CaptchaId, Points: req.Points})
	case "slide":
		return s.VerifySlide(ctx, &VerifySlideReq{CaptchaId: req.CaptchaId, X: req.X, Y: req.Y})
	case "rotate":
		return s.VerifyRotate(ctx, &VerifyRotateReq{CaptchaId: req.CaptchaId, Angle: req.Angle})
	default:
		return false, errors.New("不支持的验证码类型")
	}
}

// GenerateClick 生成点选验证码
func (s *Service) GenerateClick(ctx context.Context, mode string) (*ClickCaptchaResp, error) {
	var capt click.Captcha
	if mode == "shape" {
		capt = s.clickBuilder.MakeShape()
	} else {
		capt = s.clickBuilder.Make()
	}

	data, err := capt.Generate()
	if err != nil {
		return nil, err
	}

	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}
	thumb, err := data.GetThumbImage().ToBase64()
	if err != nil {
		return nil, err
	}

	dots := data.GetData()
	clickDots := make([]cache_captcha.ClickDot, 0, len(dots))
	for _, dot := range dots {
		clickDots = append(clickDots, cache_captcha.ClickDot{
			Index:  dot.Index,
			X:      dot.X,
			Y:      dot.Y,
			Width:  dot.Width,
			Height: dot.Height,
		})
	}

	id := uuid.NewString()
	record := cache_captcha.CaptchaRecord{Type: "click", ClickDots: clickDots}
	if err := s.store(ctx, id, &record); err != nil {
		return nil, err
	}

	return &ClickCaptchaResp{
		CaptchaId:   id,
		MasterImage: master,
		ThumbImage:  thumb,
	}, nil
}

// GenerateSlide 生成滑块验证码
func (s *Service) GenerateSlide(ctx context.Context, mode string) (*SlideCaptchaResp, error) {
	var capt slide.Captcha
	if mode == "drag" {
		capt = s.slideBuilder.MakeDragDrop()
	} else {
		capt = s.slideBuilder.Make()
	}
	data, err := capt.Generate()
	if err != nil {
		return nil, err
	}

	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}
	tile, err := data.GetTileImage().ToBase64()
	if err != nil {
		return nil, err
	}

	block := data.GetData()
	id := uuid.NewString()
	record := cache_captcha.CaptchaRecord{Type: "slide", SlideDX: block.DX, SlideDY: block.DY}
	if err := s.store(ctx, id, &record); err != nil {
		return nil, err
	}

	return &SlideCaptchaResp{
		CaptchaId:   id,
		MasterImage: master,
		TileImage:   tile,
		TileY:       block.DY,
		TileWidth:   block.Width,
		TileHeight:  block.Height,
	}, nil
}

// GenerateRotate 生成旋转验证码
func (s *Service) GenerateRotate(ctx context.Context) (*RotateCaptchaResp, error) {
	capt := s.rotateBuilder.Make()
	data, err := capt.Generate()
	if err != nil {
		return nil, err
	}

	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}
	thumb, err := data.GetThumbImage().ToBase64()
	if err != nil {
		return nil, err
	}

	block := data.GetData()
	id := uuid.NewString()
	record := cache_captcha.CaptchaRecord{Type: "rotate", RotateAngle: block.Angle}
	if err := s.store(ctx, id, &record); err != nil {
		return nil, err
	}

	return &RotateCaptchaResp{
		CaptchaId:   id,
		MasterImage: master,
		ThumbImage:  thumb,
	}, nil
}

type VerifyClickReq struct {
	CaptchaId string       `json:"captchaId"` // 验证码ID
	Points    []ClickPoint `json:"points"`    // 点选坐标
}

// VerifyClick 校验点选验证码
func (s *Service) VerifyClick(ctx context.Context, req *VerifyClickReq) (bool, error) {
	if req.CaptchaId == "" || len(req.Points) == 0 {
		return false, nil
	}
	record, err := s.load(ctx, req.CaptchaId)
	if err != nil {
		if errors.Is(err, cache_captcha.ErrCaptchaNotFound) {
			return false, nil
		}
		return false, err
	}
	if record.Type != "click" {
		return false, nil
	}

	dotMap := make(map[int]cache_captcha.ClickDot, len(record.ClickDots))
	for _, d := range record.ClickDots {
		dotMap[d.Index] = d
	}
	if len(req.Points) != len(dotMap) {
		return false, nil
	}
	for _, p := range req.Points {
		dot, ok := dotMap[p.Index]
		if !ok {
			return false, nil
		}
		if !click.Validate(p.X, p.Y, dot.X, dot.Y, dot.Width, dot.Height, s.opt.Padding) {
			return false, nil
		}
	}
	_ = s.remove(ctx, req.CaptchaId)
	return true, nil
}

type VerifySlideReq struct {
	CaptchaId string `json:"captchaId"` // 验证码ID
	X         int    `json:"x"`         // 滑块X
	Y         int    `json:"y"`         // 滑块Y
}

// VerifySlide 校验滑块验证码
func (s *Service) VerifySlide(ctx context.Context, req *VerifySlideReq) (bool, error) {
	if req.CaptchaId == "" {
		return false, nil
	}
	record, err := s.load(ctx, req.CaptchaId)
	if err != nil {
		if errors.Is(err, cache_captcha.ErrCaptchaNotFound) {
			return false, nil
		}
		return false, err
	}
	if record.Type != "slide" {
		return false, nil
	}
	if !slide.Validate(req.X, req.Y, record.SlideDX, record.SlideDY, s.opt.Padding) {
		return false, nil
	}
	_ = s.remove(ctx, req.CaptchaId)
	return true, nil
}

type VerifyRotateReq struct {
	CaptchaId string `json:"captchaId"` // 验证码ID
	Angle     int    `json:"angle"`     // 旋转角度
}

// VerifyRotate 校验旋转验证码
func (s *Service) VerifyRotate(ctx context.Context, req *VerifyRotateReq) (bool, error) {
	if req.CaptchaId == "" {
		return false, nil
	}
	record, err := s.load(ctx, req.CaptchaId)
	if err != nil {
		if errors.Is(err, cache_captcha.ErrCaptchaNotFound) {
			return false, nil
		}
		return false, err
	}
	if record.Type != "rotate" {
		return false, nil
	}
	if !rotate.Validate(req.Angle, record.RotateAngle, s.opt.Padding) {
		return false, nil
	}
	_ = s.remove(ctx, req.CaptchaId)
	return true, nil
}

// store 保存验证码记录
func (s *Service) store(ctx context.Context, id string, record *cache_captcha.CaptchaRecord) error {
	key := s.key(id)
	return s.storage.Save(ctx, key, record, time.Duration(s.opt.TTL)*time.Second)
}

// load 加载验证码记录
func (s *Service) load(ctx context.Context, id string) (*cache_captcha.CaptchaRecord, error) {
	key := s.key(id)
	return s.storage.Load(ctx, key)
}

// remove 删除验证码记录
func (s *Service) remove(ctx context.Context, id string) error {
	key := s.key(id)
	return s.storage.Remove(ctx, key)
}

// key 生成存储键
func (s *Service) key(id string) string {
	return cache_captcha.StoreKeyPrefix + id
}

// resolveType 解析验证码类型
func (s *Service) resolveType(input string) string {
	value := strings.TrimSpace(strings.ToLower(input))
	if value != "" {
		return value
	}
	return strings.TrimSpace(strings.ToLower(s.opt.Type))
}

// resolveMode 解析验证码模式
func (s *Service) resolveMode(input string) string {
	value := strings.TrimSpace(strings.ToLower(input))
	if value != "" {
		return value
	}
	return strings.TrimSpace(strings.ToLower(s.opt.Mode))
}

// loadClickResources 加载点选验证码资源
func loadClickResources() ([]click.Resource, error) {
	shapesMap, err := shapes.GetShapes()
	if err != nil {
		return nil, err
	}
	images, err := imagesv2.GetImages()
	if err != nil {
		return nil, err
	}
	thumbImages, err := thumbs.GetThumbs()
	if err != nil {
		return nil, err
	}
	font, err := fzshengsksjw.GetFont()
	if err != nil {
		return nil, err
	}
	return []click.Resource{
		click.WithChars(chars.GetAlphaChars()),
		click.WithShapes(shapesMap),
		click.WithFonts([]*truetype.Font{font}),
		click.WithBackgrounds(images),
		click.WithThumbBackgrounds(thumbImages),
	}, nil
}

// loadSlideResources 加载滑块验证码资源
func loadSlideResources() ([]slide.Resource, error) {
	images, err := imagesv2.GetImages()
	if err != nil {
		return nil, err
	}
	graphTiles, err := tiles.GetTiles()
	if err != nil {
		return nil, err
	}
	graphs := make([]*slide.GraphImage, 0, len(graphTiles))
	for _, g := range graphTiles {
		graphs = append(graphs, &slide.GraphImage{
			OverlayImage: g.OverlayImage,
			ShadowImage:  g.ShadowImage,
			MaskImage:    g.MaskImage,
		})
	}
	return []slide.Resource{
		slide.WithBackgrounds(images),
		slide.WithGraphImages(graphs),
	}, nil
}

// loadRotateResources 加载旋转验证码资源
func loadRotateResources() ([]rotate.Resource, error) {
	images, err := imagesv2.GetImages()
	if err != nil {
		return nil, err
	}
	return []rotate.Resource{
		rotate.WithImages(images),
	}, nil
}
