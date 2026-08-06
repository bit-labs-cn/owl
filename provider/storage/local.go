package storage

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage 本地存储实现
type LocalStorage struct {
	config *LocalConfig
}

// NewLocalStorage 创建本地存储实例
func NewLocalStorage(config *LocalConfig) *LocalStorage {
	return &LocalStorage{
		config: config,
	}
}

// Put 一次性保存文件；path 为调用方决定的完整对象键
func (ls *LocalStorage) Put(ctx context.Context, path string, reader io.Reader, size int64) (*FileInfo, error) {
	_ = size
	return ls.putObject(NormalizeObjectKey(path), reader)
}

// PutStream 流式保存文件；读至 EOF，无需预先知道 size
func (ls *LocalStorage) PutStream(ctx context.Context, path string, reader io.Reader) (*FileInfo, error) {
	return ls.putObject(NormalizeObjectKey(path), reader)
}

func (ls *LocalStorage) putObject(objectKey string, reader io.Reader) (*FileInfo, error) {
	if objectKey == "" {
		return nil, fmt.Errorf("object path is empty")
	}
	fullPath := ls.fullPath(objectKey)

	if ls.config.CreateDirs {
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, os.FileMode(ls.config.DirMode)); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(ls.config.FileMode))
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	multiWriter := io.MultiWriter(file, hash)

	written, err := io.Copy(multiWriter, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}

	return &FileInfo{
		Name:       filepath.Base(objectKey),
		Path:       objectKey,
		Size:       written,
		Extension:  filepath.Ext(objectKey),
		URL:        ls.buildURL(objectKey),
		Hash:       fmt.Sprintf("%x", hash.Sum(nil)),
		UploadTime: stat.ModTime(),
		Metadata:   make(map[string]string),
	}, nil
}

// PutFile 上传本地文件到指定对象键
func (ls *LocalStorage) PutFile(ctx context.Context, path string, localPath string) (*FileInfo, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}

	return ls.Put(ctx, path, file, stat.Size())
}

// Get 获取文件
func (ls *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := ls.fullPath(NormalizeObjectKey(path))

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete 删除文件
func (ls *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := ls.fullPath(NormalizeObjectKey(path))

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists 检查文件是否存在
func (ls *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := ls.fullPath(NormalizeObjectKey(path))

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Size 获取文件大小
func (ls *LocalStorage) Size(ctx context.Context, path string) (int64, error) {
	fullPath := ls.fullPath(NormalizeObjectKey(path))

	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file not found: %s", path)
		}
		return 0, fmt.Errorf("failed to get file stat: %w", err)
	}

	return stat.Size(), nil
}

// URL 获取文件访问 URL
func (ls *LocalStorage) URL(ctx context.Context, path string) (string, error) {
	return ls.buildURL(NormalizeObjectKey(path)), nil
}

// List 列出文件
func (ls *LocalStorage) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	var files []*FileInfo

	prefixPath := ls.fullPath(NormalizeObjectKey(prefix))

	err := filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(ls.config.Root, path)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		fileInfo := &FileInfo{
			Name:       info.Name(),
			Path:       relPath,
			Size:       info.Size(),
			Extension:  filepath.Ext(info.Name()),
			URL:        ls.buildURL(relPath),
			UploadTime: info.ModTime(),
			Metadata:   make(map[string]string),
		}

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// Copy 复制文件
func (ls *LocalStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath := ls.fullPath(NormalizeObjectKey(srcPath))
	dstFullPath := ls.fullPath(NormalizeObjectKey(dstPath))

	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	if ls.config.CreateDirs {
		dir := filepath.Dir(dstFullPath)
		if err := os.MkdirAll(dir, os.FileMode(ls.config.DirMode)); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	dstFile, err := os.OpenFile(dstFullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(ls.config.FileMode))
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Move 移动文件
func (ls *LocalStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath := ls.fullPath(NormalizeObjectKey(srcPath))
	dstFullPath := ls.fullPath(NormalizeObjectKey(dstPath))

	if ls.config.CreateDirs {
		dir := filepath.Dir(dstFullPath)
		if err := os.MkdirAll(dir, os.FileMode(ls.config.DirMode)); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if err := os.Rename(srcFullPath, dstFullPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

func (ls *LocalStorage) fullPath(objectKey string) string {
	objectKey = NormalizeObjectKey(objectKey)
	return filepath.Join(ls.config.Root, filepath.FromSlash(objectKey))
}

func (ls *LocalStorage) buildURL(objectKey string) string {
	objectKey = NormalizeObjectKey(objectKey)
	urlPrefix := strings.TrimSuffix(ls.config.URLPrefix, "/")
	if objectKey == "" {
		return urlPrefix
	}
	return urlPrefix + "/" + objectKey
}
