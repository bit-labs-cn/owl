package storage

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/go-sdk/v7/storagev2/credentials"
	httpclient "github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/region"
	"github.com/qiniu/go-sdk/v7/storagev2/uploader"
	"github.com/qiniu/go-sdk/v7/storagev2/uptoken"
)

// QiniuStorage 七牛云存储实现（上传走 storagev2；列举/元数据/删除等走 v1 BucketManager，避免 storagev2/objects 在部分 Go 版本下无法编译）
type QiniuStorage struct {
	credProv      credentials.CredentialsProvider
	config        *QiniuConfig
	regions       region.RegionsProvider
	uploadManager *uploader.UploadManager
	mac           *qbox.Mac
	bucketManager *storage.BucketManager
}

type qiniuStaticCreds struct{ v *credentials.Credentials }

func (q qiniuStaticCreds) Get(ctx context.Context) (*credentials.Credentials, error) {
	return q.v, nil
}

func qiniuRegionsProvider(zone string, useHTTPS bool) region.RegionsProvider {
	z := strings.TrimSpace(zone)
	var id string
	switch z {
	case "", "z0", "华东":
		id = "z0"
	case "z1", "华北":
		id = "z1"
	case "z2", "华南":
		id = "z2"
	case "na0", "北美":
		id = "na0"
	case "as0", "新加坡":
		id = "as0"
	case "华东浙江2区", "cn-east-2":
		id = "cn-east-2"
	default:
		if z != "" {
			id = z
		} else {
			id = "z0"
		}
	}
	return region.GetRegionByID(id, useHTTPS)
}

func qiniuV1Zone(zone string) *storage.Zone {
	if zone == "" {
		return &storage.ZoneHuadong
	}
	switch zone {
	case "z0", "华东":
		return &storage.ZoneHuadong
	case "z1", "华北":
		return &storage.ZoneHuabei
	case "z2", "华南":
		return &storage.ZoneHuanan
	case "na0", "北美":
		return &storage.ZoneBeimei
	case "as0", "新加坡":
		return &storage.ZoneXinjiapo
	case "华东浙江2区", "cn-east-2":
		return &storage.ZoneHuadongZheJiang2
	default:
		return &storage.ZoneHuadong
	}
}

type qiniuUploadRet struct {
	Hash string `json:"hash"`
}

// NewQiniuStorage 创建七牛云存储实例
func NewQiniuStorage(config *QiniuConfig) (*QiniuStorage, error) {
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	cred := credentials.NewCredentials(config.AccessKey, config.SecretKey)
	credProv := qiniuStaticCreds{v: cred}
	useHTTPS := config.UseSSL
	reg := qiniuRegionsProvider(config.Zone, useHTTPS)

	httpOpts := httpclient.Options{
		Credentials:         credProv,
		UseInsecureProtocol: !useHTTPS,
		Regions:             reg,
	}

	cfg := &storage.Config{
		UseHTTPS:      config.UseSSL,
		UseCdnDomains: false,
		Zone:          qiniuV1Zone(strings.TrimSpace(config.Zone)),
	}

	return &QiniuStorage{
		credProv: credProv,
		config:   config,
		regions:  reg,
		uploadManager: uploader.NewUploadManager(&uploader.UploadManagerOptions{
			Options: httpOpts,
		}),
		mac:           mac,
		bucketManager: storage.NewBucketManager(mac, cfg),
	}, nil
}

// Put 上传文件
func (q *QiniuStorage) Put(ctx context.Context, path string, reader io.Reader, _ int64) (*FileInfo, error) {
	key := q.buildPath(path)

	buf := new(bytes.Buffer)
	hash := md5.New()
	tee := io.TeeReader(reader, hash)

	if _, err := buf.ReadFrom(tee); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	putPolicy, err := uptoken.NewPutPolicy(q.config.Bucket, time.Now().Add(time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to build put policy: %w", err)
	}

	objectName := key
	fileName := filepath.Base(path)
	var ret qiniuUploadRet
	err = q.uploadManager.UploadReader(ctx, bytes.NewReader(buf.Bytes()), &uploader.ObjectOptions{
		BucketName:      q.config.Bucket,
		ObjectName:      &objectName,
		FileName:        fileName,
		ContentType:     MimeType(path),
		RegionsProvider: q.regions,
		UpToken:         uptoken.NewSigner(putPolicy, q.credProv),
	}, &ret)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &FileInfo{
		Name:        filepath.Base(path),
		Path:        path,
		Size:        int64(buf.Len()),
		ContentType: MimeType(path),
		Extension:   filepath.Ext(path),
		URL:         q.buildURL(key),
		Hash:        fmt.Sprintf("%x", hash.Sum(nil)),
		UploadTime:  time.Now(),
		Metadata: map[string]string{
			"bucket": q.config.Bucket,
			"key":    key,
			"hash":   ret.Hash,
		},
	}, nil
}

// PutFile 上传本地文件
func (q *QiniuStorage) PutFile(ctx context.Context, path string, localPath string) (*FileInfo, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}

	return q.Put(ctx, path, file, stat.Size())
}

// Get 获取文件
func (q *QiniuStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	key := q.buildPath(path)
	url := q.buildURL(key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to get file, status: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// Delete 删除文件
func (q *QiniuStorage) Delete(ctx context.Context, path string) error {
	key := q.buildPath(path)
	err := q.bucketManager.Delete(q.config.Bucket, key)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// Exists 检查文件是否存在
func (q *QiniuStorage) Exists(ctx context.Context, path string) (bool, error) {
	key := q.buildPath(path)
	_, err := q.bucketManager.Stat(q.config.Bucket, key)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") ||
			strings.Contains(err.Error(), "612") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	return true, nil
}

// Size 获取文件大小
func (q *QiniuStorage) Size(ctx context.Context, path string) (int64, error) {
	key := q.buildPath(path)
	fileInfo, err := q.bucketManager.Stat(q.config.Bucket, key)
	if err != nil {
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}
	return fileInfo.Fsize, nil
}

// URL 获取文件访问 URL
func (q *QiniuStorage) URL(ctx context.Context, path string) (string, error) {
	key := q.buildPath(path)
	return q.buildURL(key), nil
}

// List 列出文件
func (q *QiniuStorage) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	keyPrefix := q.buildPath(prefix)

	var files []*FileInfo
	marker := ""
	limit := 1000

	for {
		entries, _, nextMarker, hasNext, err := q.bucketManager.ListFiles(q.config.Bucket, keyPrefix, "", marker, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to list files: %w", err)
		}

		for _, entry := range entries {
			relativePath := strings.TrimPrefix(entry.Key, keyPrefix)
			if relativePath == "" {
				relativePath = entry.Key
			}

			files = append(files, &FileInfo{
				Name:        filepath.Base(entry.Key),
				Path:        relativePath,
				Size:        entry.Fsize,
				ContentType: entry.MimeType,
				Extension:   filepath.Ext(entry.Key),
				URL:         q.buildURL(entry.Key),
				Hash:        entry.Hash,
				UploadTime:  time.Unix(entry.PutTime/10000000, 0),
				Metadata: map[string]string{
					"bucket":    q.config.Bucket,
					"key":       entry.Key,
					"hash":      entry.Hash,
					"mime_type": entry.MimeType,
					"end_user":  entry.EndUser,
				},
			})
		}

		if !hasNext {
			break
		}
		marker = nextMarker
	}

	return files, nil
}

// Copy 复制文件
func (q *QiniuStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	srcKey := q.buildPath(srcPath)
	dstKey := q.buildPath(dstPath)
	err := q.bucketManager.Copy(q.config.Bucket, srcKey, q.config.Bucket, dstKey, true)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	return nil
}

// Move 移动文件
func (q *QiniuStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	srcKey := q.buildPath(srcPath)
	dstKey := q.buildPath(dstPath)
	err := q.bucketManager.Move(q.config.Bucket, srcKey, q.config.Bucket, dstKey, true)
	if err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}
	return nil
}

// buildPath 构建对象路径
func (q *QiniuStorage) buildPath(path string) string {
	path = strings.TrimPrefix(path, "/")

	dateFormat := strings.TrimSpace(q.config.DateFormat)
	if dateFormat != "" {
		datePath := time.Now().Format(normalizeDateFormat(dateFormat))
		path = datePath + "/" + path
	}

	return path
}

// buildURL 构建文件 URL
func (q *QiniuStorage) buildURL(key string) string {
	scheme := "http"
	if q.config.UseSSL {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/%s", scheme, q.config.Domain, key)
}
