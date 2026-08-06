package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

// Options 存储配置选项
type Options struct {
	Driver     string           `json:"driver"`
	Local      LocalConfig      `json:"local"`
	S3         S3Config         `json:"s3"`
	MinIO      MinIOConfig      `json:"minio"`
	OSS        OSSConfig        `json:"oss"`
	COS        COSConfig        `json:"cos"`
	Qiniu      QiniuConfig      `json:"qiniu"`
	Upload     UploadConfig     `json:"upload"`
	Image      ImageConfig      `json:"image"`
	Security   SecurityConfig   `json:"security"`
	Cache      CacheConfig      `json:"cache"`
	Logging    LoggingConfig    `json:"logging"`
	Monitoring MonitoringConfig `json:"monitoring"`
}

// LocalConfig 本地存储配置
type LocalConfig struct {
	Root       string `json:"root"`
	URLPrefix  string `json:"url-prefix"`
	CreateDirs bool   `json:"create-dirs"`
	DirMode    uint32 `json:"dir-mode"`
	FileMode   uint32 `json:"file-mode"`
}

// S3Config Amazon S3 配置
type S3Config struct {
	AccessKeyID     string `json:"access-key-id"`
	SecretAccessKey string `json:"secret-access-key"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Endpoint        string `json:"endpoint"`
	UseSSL          bool   `json:"use-ssl"`
	PathStyle       bool   `json:"path-style"`
	ACL             string `json:"acl"`
	URLPrefix       string `json:"url-prefix"`
}

// MinIOConfig MinIO 配置
type MinIOConfig struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access-key-id"`
	SecretAccessKey string `json:"secret-access-key"`
	UseSSL          bool   `json:"use-ssl"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	URLPrefix       string `json:"url-prefix"`
}

// OSSConfig 阿里云 OSS 配置
type OSSConfig struct {
	AccessKeyID     string `json:"access-key-id"`
	SecretAccessKey string `json:"secret-access-key"`
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	UseSSL          bool   `json:"use-ssl"`
	URLPrefix       string `json:"url-prefix"`
}

// COSConfig 腾讯云 COS 配置
type COSConfig struct {
	SecretID  string `json:"secret-id"`
	SecretKey string `json:"secret-key"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	UseSSL    bool   `json:"use-ssl"`
	URLPrefix string `json:"url-prefix"`
}

// QiniuConfig 七牛云配置
type QiniuConfig struct {
	AccessKey string `json:"access-key"`
	SecretKey string `json:"secret-key"`
	Bucket    string `json:"bucket"`
	Domain    string `json:"domain"`
	UseSSL    bool   `json:"use-ssl"`
	Zone      string `json:"zone"`
}

// UploadConfig 上传配置
type UploadConfig struct {
	MaxFileSize       int64    `json:"max-file-size"`
	AllowedTypes      []string `json:"allowed-types"`
	AllowedExtensions []string `json:"allowed-extensions"`
	CheckFileType     bool     `json:"check-file-type"`
	UniqueFilename    bool     `json:"unique-filename"`
	FilenameStrategy  string   `json:"filename-strategy"`
	KeepOriginalName  bool     `json:"keep-original-name"`
}

// ImageConfig 图片处理配置
type ImageConfig struct {
	Enabled    bool                       `json:"enabled"`
	Quality    int                        `json:"quality"`
	Formats    []string                   `json:"formats"`
	Thumbnails map[string]ThumbnailConfig `json:"thumbnails"`
	Watermark  WatermarkConfig            `json:"watermark"`
}

// ThumbnailConfig 缩略图配置
type ThumbnailConfig struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Mode   string `json:"mode"`
}

// WatermarkConfig 水印配置
type WatermarkConfig struct {
	Enabled  bool    `json:"enabled"`
	Image    string  `json:"image"`
	Position string  `json:"position"`
	Opacity  float64 `json:"opacity"`
	Margin   int     `json:"margin"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	VirusScan           bool     `json:"virus-scan"`
	ScanCommand         string   `json:"scan-command"`
	ContentCheck        bool     `json:"content-check"`
	ForbiddenTypes      []string `json:"forbidden-types"`
	ForbiddenExtensions []string `json:"forbidden-extensions"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled bool   `json:"enabled"`
	Driver  string `json:"driver"`
	TTL     int    `json:"ttl"`
	Prefix  string `json:"prefix"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Enabled     bool   `json:"enabled"`
	Level       string `json:"level"`
	LogUpload   bool   `json:"log-upload"`
	LogDownload bool   `json:"log-download"`
	LogDelete   bool   `json:"log-delete"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled      bool `json:"enabled"`
	Interval     int  `json:"interval"`
	MonitorSpace bool `json:"monitor-space"`
	MonitorFiles bool `json:"monitor-files"`
	MonitorSpeed bool `json:"monitor-speed"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"` // 相对存储根的对象键（由调用方指定；Get/Delete 必须用此值）
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type"`
	Extension   string            `json:"extension"`
	URL         string            `json:"url"`
	Hash        string            `json:"hash"`
	UploadTime  time.Time         `json:"upload_time"`
	Metadata    map[string]string `json:"metadata"`
}

// UploadResult 上传结果
type UploadResult struct {
	FileInfo *FileInfo `json:"file_info"`
	Success  bool      `json:"success"`
	Message  string    `json:"message"`
	Error    error     `json:"error,omitempty"`
}

// Storage 存储接口。
// 驱动只负责在配置的根目录/桶下按对象键读写；目录如何分类由调用方决定。
type Storage interface {
	// Put 一次性保存文件。path 为完整对象键；size 为内容字节数。
	Put(ctx context.Context, path string, reader io.Reader, size int64) (*FileInfo, error)

	// PutStream 流式保存文件。path 为完整对象键；读至 EOF，无需预先知道 size。
	PutStream(ctx context.Context, path string, reader io.Reader) (*FileInfo, error)

	// PutFile 上传本地文件到指定对象键
	PutFile(ctx context.Context, path string, localPath string) (*FileInfo, error)

	// Get 获取文件
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete 删除文件
	Delete(ctx context.Context, path string) error

	// Exists 检查文件是否存在
	Exists(ctx context.Context, path string) (bool, error)

	// Size 获取文件大小
	Size(ctx context.Context, path string) (int64, error)

	// URL 获取文件访问 URL
	URL(ctx context.Context, path string) (string, error)

	// List 列出文件
	List(ctx context.Context, prefix string) ([]*FileInfo, error)

	// Copy 复制文件
	Copy(ctx context.Context, srcPath, dstPath string) error

	// Move 移动文件
	Move(ctx context.Context, srcPath, dstPath string) error
}

// StorageManager 存储管理器
type StorageManager struct {
	drivers       map[string]Storage
	defaultDriver string
	options       *Options
}

// NewStorageManager 创建存储管理器
func NewStorageManager() *StorageManager {
	return &StorageManager{
		drivers: make(map[string]Storage),
	}
}

// SetOptions 保存配置
func (sm *StorageManager) SetOptions(opt *Options) {
	sm.options = opt
}

// Options 返回当前存储配置
func (sm *StorageManager) Options() *Options {
	return sm.options
}

// AddDriver 添加存储驱动
func (sm *StorageManager) AddDriver(name string, driver Storage) {
	sm.drivers[name] = driver
}

// SetDefaultDriver 设置默认驱动
func (sm *StorageManager) SetDefaultDriver(name string) error {
	if _, exists := sm.drivers[name]; !exists {
		return fmt.Errorf("storage driver '%s' not found", name)
	}
	sm.defaultDriver = name
	return nil
}

// GetDriver 获取指定驱动
func (sm *StorageManager) GetDriver(name string) (Storage, error) {
	if name == "" {
		name = sm.defaultDriver
	}

	driver, exists := sm.drivers[name]
	if !exists {
		return nil, fmt.Errorf("storage driver %s not found", name)
	}

	return driver, nil
}

// Default 获取默认驱动
func (sm *StorageManager) Default() (Storage, error) {
	return sm.GetDriver(sm.defaultDriver)
}

// Put 使用默认驱动一次性保存文件（path 由调用方决定）
func (sm *StorageManager) Put(ctx context.Context, path string, reader io.Reader, size int64) (*FileInfo, error) {
	driver, err := sm.Default()
	if err != nil {
		return nil, err
	}

	return driver.Put(ctx, path, reader, size)
}

// PutStream 使用默认驱动流式保存文件（path 由调用方决定）
func (sm *StorageManager) PutStream(ctx context.Context, path string, reader io.Reader) (*FileInfo, error) {
	driver, err := sm.Default()
	if err != nil {
		return nil, err
	}

	return driver.PutStream(ctx, path, reader)
}

// PutFile 使用默认驱动上传本地文件
func (sm *StorageManager) PutFile(ctx context.Context, path string, localPath string) (*FileInfo, error) {
	driver, err := sm.Default()
	if err != nil {
		return nil, err
	}

	return driver.PutFile(ctx, path, localPath)
}

// Get 使用默认驱动获取文件
func (sm *StorageManager) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	driver, err := sm.Default()
	if err != nil {
		return nil, err
	}

	return driver.Get(ctx, path)
}

// Delete 使用默认驱动删除文件
func (sm *StorageManager) Delete(ctx context.Context, path string) error {
	driver, err := sm.Default()
	if err != nil {
		return err
	}

	return driver.Delete(ctx, path)
}

// Exists 使用默认驱动检查文件是否存在
func (sm *StorageManager) Exists(ctx context.Context, path string) (bool, error) {
	driver, err := sm.Default()
	if err != nil {
		return false, err
	}

	return driver.Exists(ctx, path)
}

// URL 使用默认驱动获取文件 URL
func (sm *StorageManager) URL(ctx context.Context, path string) (string, error) {
	driver, err := sm.Default()
	if err != nil {
		return "", err
	}

	return driver.URL(ctx, path)
}

func NormalizeObjectKey(path string) string {
	path = strings.TrimSpace(path)
	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return path
}

// setDefaults 设置默认值
func setDefaults(opt *Options) {
	if opt.Driver == "" {
		opt.Driver = "local"
	}

	if opt.Local.Root == "" {
		opt.Local.Root = "./storage"
	}
	if opt.Local.URLPrefix == "" {
		opt.Local.URLPrefix = "/storage"
	}
	if opt.Local.DirMode == 0 {
		opt.Local.DirMode = 0755
	}
	if opt.Local.FileMode == 0 {
		opt.Local.FileMode = 0644
	}

	if opt.Upload.MaxFileSize == 0 {
		opt.Upload.MaxFileSize = 10485760 // 10MB
	}
	if opt.Upload.FilenameStrategy == "" {
		opt.Upload.FilenameStrategy = "uuid"
	}

	if opt.Image.Quality == 0 {
		opt.Image.Quality = 85
	}

	if opt.Cache.TTL == 0 {
		opt.Cache.TTL = 3600
	}
	if opt.Cache.Prefix == "" {
		opt.Cache.Prefix = "storage:"
	}

	if opt.Logging.Level == "" {
		opt.Logging.Level = "info"
	}

	if opt.Monitoring.Interval == 0 {
		opt.Monitoring.Interval = 60
	}
}

// MimeType 根据文件扩展名检测 MIME 类型，统一各存储实现行为
func MimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	// 首选内置表，覆盖部分常见类型的标准库返回
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".pdf":  "application/pdf",
		".txt":  "text/plain",
		".html": "text/html",
		".htm":  "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".zip":  "application/zip",
		".rar":  "application/x-rar-compressed",
		".7z":   "application/x-7z-compressed",
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	}

	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}

	// 其次尝试标准库的扩展名映射
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}

	// 兜底
	return "application/octet-stream"
}
