package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStorage MinIO 存储实现
type MinIOStorage struct {
	client *minio.Client
	config *MinIOConfig
}

// NewMinIOStorage 创建 MinIO 存储实例
func NewMinIOStorage(config *MinIOConfig) (*MinIOStorage, error) {
	// 创建 MinIO 客户端
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// 检查存储桶是否存在，如果不存在则创建
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{
			Region: config.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &MinIOStorage{
		client: client,
		config: config,
	}, nil
}

// Put 一次性保存文件；path 为调用方决定的完整对象键
func (m *MinIOStorage) Put(ctx context.Context, path string, reader io.Reader, size int64) (*FileInfo, error) {
	return m.putObject(ctx, NormalizeObjectKey(path), reader, size)
}

// PutStream 流式保存文件；读至 EOF，无需预先知道 size
func (m *MinIOStorage) PutStream(ctx context.Context, path string, reader io.Reader) (*FileInfo, error) {
	return m.putObject(ctx, NormalizeObjectKey(path), reader, -1)
}

func (m *MinIOStorage) putObject(ctx context.Context, objectName string, reader io.Reader, size int64) (*FileInfo, error) {
	if objectName == "" {
		return nil, fmt.Errorf("object path is empty")
	}
	info, err := m.client.PutObject(ctx, m.config.Bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: MimeType(objectName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	fileInfo := &FileInfo{
		Name:        filepath.Base(objectName),
		Path:        objectName,
		Size:        info.Size,
		ContentType: MimeType(objectName),
		Extension:   filepath.Ext(objectName),
		URL:         m.buildURL(objectName),
		Hash:        info.ETag,
		UploadTime:  time.Now(),
		Metadata: map[string]string{
			"bucket":     m.config.Bucket,
			"object":     objectName,
			"etag":       info.ETag,
			"version_id": info.VersionID,
		},
	}

	return fileInfo, nil
}

// PutFile 上传本地文件
func (m *MinIOStorage) PutFile(ctx context.Context, path string, localPath string) (*FileInfo, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}

	return m.Put(ctx, path, file, stat.Size())
}

// Get 获取文件
func (m *MinIOStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	objectName := NormalizeObjectKey(path)

	object, err := m.client.GetObject(ctx, m.config.Bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return object, nil
}

// Delete 删除文件
func (m *MinIOStorage) Delete(ctx context.Context, path string) error {
	objectName := NormalizeObjectKey(path)

	err := m.client.RemoveObject(ctx, m.config.Bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists 检查文件是否存在
func (m *MinIOStorage) Exists(ctx context.Context, path string) (bool, error) {
	objectName := NormalizeObjectKey(path)

	_, err := m.client.StatObject(ctx, m.config.Bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// Size 获取文件大小
func (m *MinIOStorage) Size(ctx context.Context, path string) (int64, error) {
	objectName := NormalizeObjectKey(path)

	stat, err := m.client.StatObject(ctx, m.config.Bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}

	return stat.Size, nil
}

// URL 获取文件访问 URL
func (m *MinIOStorage) URL(ctx context.Context, path string) (string, error) {
	objectName := NormalizeObjectKey(path)

	// 如果配置了 URL 前缀，直接使用
	if m.config.URLPrefix != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(m.config.URLPrefix, "/"), objectName), nil
	}

	// 生成预签名 URL（15分钟有效期）
	presignedURL, err := m.client.PresignedGetObject(ctx, m.config.Bucket, objectName, 15*time.Minute, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// List 列出文件
func (m *MinIOStorage) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	objectPrefix := NormalizeObjectKey(prefix)

	var files []*FileInfo

	for object := range m.client.ListObjects(ctx, m.config.Bucket, minio.ListObjectsOptions{
		Prefix:    objectPrefix,
		Recursive: true,
	}) {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", object.Err)
		}

		fileInfo := &FileInfo{
			Name:        filepath.Base(object.Key),
			Path:        object.Key,
			Size:        object.Size,
			ContentType: MimeType(object.Key),
			Extension:   filepath.Ext(object.Key),
			URL:         m.buildURL(object.Key),
			Hash:        object.ETag,
			UploadTime:  object.LastModified,
			Metadata: map[string]string{
				"bucket": m.config.Bucket,
				"object": object.Key,
				"etag":   object.ETag,
			},
		}

		files = append(files, fileInfo)
	}

	return files, nil
}

// Copy 复制文件
func (m *MinIOStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	srcObjectName := NormalizeObjectKey(srcPath)
	dstObjectName := NormalizeObjectKey(dstPath)

	// 创建复制源
	src := minio.CopySrcOptions{
		Bucket: m.config.Bucket,
		Object: srcObjectName,
	}

	// 创建复制目标
	dst := minio.CopyDestOptions{
		Bucket: m.config.Bucket,
		Object: dstObjectName,
	}

	_, err := m.client.CopyObject(ctx, dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Move 移动文件
func (m *MinIOStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	// 先复制文件
	err := m.Copy(ctx, srcPath, dstPath)
	if err != nil {
		return fmt.Errorf("failed to copy file during move: %w", err)
	}

	// 删除源文件
	err = m.Delete(ctx, srcPath)
	if err != nil {
		return fmt.Errorf("failed to delete source file during move: %w", err)
	}

	return nil
}

// buildURL 构建文件 URL（objectName 已是完整键）
func (m *MinIOStorage) buildURL(objectName string) string {
	objectName = NormalizeObjectKey(objectName)
	if m.config.URLPrefix != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(m.config.URLPrefix, "/"), objectName)
	}

	// 构建默认 URL
	scheme := "http"
	if m.config.UseSSL {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", scheme, m.config.Endpoint, m.config.Bucket, objectName)
}
