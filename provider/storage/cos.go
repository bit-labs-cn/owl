package storage

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// COSStorage 腾讯云 COS 存储实现
type COSStorage struct {
	client *cos.Client
	config *COSConfig
}

// NewCOSStorage 创建 COS 存储实例
func NewCOSStorage(config *COSConfig) (*COSStorage, error) {
	scheme := "https"
	if !config.UseSSL {
		scheme = "http"
	}

	// 形如: https://<bucket>.cos.<region>.myqcloud.com
	bucketURLStr := fmt.Sprintf("%s://%s.cos.%s.myqcloud.com", scheme, config.Bucket, config.Region)
	bucketURL, err := url.Parse(bucketURLStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bucket url: %w", err)
	}

	baseURL := &cos.BaseURL{BucketURL: bucketURL}
	client := cos.NewClient(baseURL, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.SecretID,
			SecretKey: config.SecretKey,
		},
	})

	return &COSStorage{
		client: client,
		config: config,
	}, nil
}

// Put 一次性保存文件；path 为调用方决定的完整对象键
func (c *COSStorage) Put(ctx context.Context, path string, reader io.Reader, size int64) (*FileInfo, error) {
	_ = size
	return c.putObject(ctx, NormalizeObjectKey(path), reader)
}

// PutStream 流式保存文件；读至 EOF，无需预先知道 size
func (c *COSStorage) PutStream(ctx context.Context, path string, reader io.Reader) (*FileInfo, error) {
	return c.putObject(ctx, NormalizeObjectKey(path), reader)
}

func (c *COSStorage) putObject(ctx context.Context, key string, reader io.Reader) (*FileInfo, error) {
	if key == "" {
		return nil, fmt.Errorf("object path is empty")
	}
	buf := new(bytes.Buffer)
	hash := md5.New()
	tee := io.TeeReader(reader, hash)

	if _, err := buf.ReadFrom(tee); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	_, err := c.client.Object.Put(ctx, key, bytes.NewReader(buf.Bytes()), &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: MimeType(key),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	fileInfo := &FileInfo{
		Name:        filepath.Base(key),
		Path:        key,
		Size:        int64(buf.Len()),
		ContentType: MimeType(key),
		Extension:   filepath.Ext(key),
		URL:         c.buildURL(key),
		Hash:        fmt.Sprintf("%x", hash.Sum(nil)),
		UploadTime:  time.Now(),
		Metadata: map[string]string{
			"bucket": c.config.Bucket,
			"key":    key,
			"region": c.config.Region,
		},
	}

	return fileInfo, nil
}

// PutFile 上传本地文件
func (c *COSStorage) PutFile(ctx context.Context, path string, localPath string) (*FileInfo, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}

	return c.Put(ctx, path, file, stat.Size())
}

// Get 获取文件
func (c *COSStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	key := NormalizeObjectKey(path)
	resp, err := c.client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return resp.Body, nil
}

// Delete 删除文件
func (c *COSStorage) Delete(ctx context.Context, path string) error {
	key := NormalizeObjectKey(path)
	_, err := c.client.Object.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// Exists 检查文件是否存在
func (c *COSStorage) Exists(ctx context.Context, path string) (bool, error) {
	key := NormalizeObjectKey(path)
	resp, err := c.client.Object.Head(ctx, key, nil)
	if err != nil {
		if e, ok := err.(*cos.ErrorResponse); ok && e.Response != nil && e.Response.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	if resp != nil && resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}

// Size 获取文件大小
func (c *COSStorage) Size(ctx context.Context, path string) (int64, error) {
	key := NormalizeObjectKey(path)
	resp, err := c.client.Object.Head(ctx, key, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}
	if resp.Header != nil {
		// Content-Length
		cl := resp.Header.Get("Content-Length")
		if cl != "" {
			// parse int64
			var size int64
			_, scanErr := fmt.Sscanf(cl, "%d", &size)
			if scanErr == nil {
				return size, nil
			}
		}
	}
	return 0, nil
}

// URL 获取文件访问 URL
func (c *COSStorage) URL(ctx context.Context, path string) (string, error) {
	key := NormalizeObjectKey(path)
	// 如果配置了 URL 前缀，直接使用
	if c.config.URLPrefix != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(c.config.URLPrefix, "/"), key), nil
	}
	// 默认 URL
	return c.buildURL(key), nil
}

// List 列出文件
func (c *COSStorage) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	objectPrefix := NormalizeObjectKey(prefix)
	var files []*FileInfo

	marker := ""
	for {
		v, _, err := c.client.Bucket.Get(ctx, &cos.BucketGetOptions{
			Prefix: objectPrefix,
			Marker: marker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range v.Contents {
			files = append(files, &FileInfo{
				Name:        filepath.Base(obj.Key),
				Path:        obj.Key,
				Size:        obj.Size,
				ContentType: MimeType(obj.Key),
				Extension:   filepath.Ext(obj.Key),
				URL:         c.buildURL(obj.Key),
				UploadTime:  time.Now(),
				Metadata: map[string]string{
					"bucket": c.config.Bucket,
					"key":    obj.Key,
					"etag":   obj.ETag,
				},
			})
		}

		if v.IsTruncated {
			marker = v.NextMarker
		} else {
			break
		}
	}

	return files, nil
}

// Copy 复制文件
func (c *COSStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	srcKey := NormalizeObjectKey(srcPath)
	dstKey := NormalizeObjectKey(dstPath)

	// 源 URL 必须为 COS 端点 URL
	scheme := "https"
	if !c.config.UseSSL {
		scheme = "http"
	}
	srcURL := fmt.Sprintf("%s://%s.cos.%s.myqcloud.com/%s", scheme, c.config.Bucket, c.config.Region, srcKey)

	_, _, err := c.client.Object.Copy(ctx, dstKey, srcURL, nil)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}
	return nil
}

// Move 移动文件
func (c *COSStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	if err := c.Copy(ctx, srcPath, dstPath); err != nil {
		return err
	}
	return c.Delete(ctx, srcPath)
}

// buildURL 构建文件 URL（key 已是完整键）
func (c *COSStorage) buildURL(key string) string {
	key = NormalizeObjectKey(key)
	if c.config.URLPrefix != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(c.config.URLPrefix, "/"), key)
	}
	scheme := "https"
	if !c.config.UseSSL {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s.cos.%s.myqcloud.com/%s", scheme, c.config.Bucket, c.config.Region, key)
}
