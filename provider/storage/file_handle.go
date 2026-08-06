package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"bit-labs.cn/owl/provider/router"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var _ router.Handler = (*FileHandle)(nil)

type FileHandle struct {
	storage *StorageManager
}

func NewFileHandle(storage *StorageManager) *FileHandle {
	return &FileHandle{storage: storage}
}

func (i *FileHandle) ModuleName() (en string, zh string) {
	return "file", "文件"
}

func (i *FileHandle) Upload(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		router.BadRequest(ctx, "缺少文件")
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	// 目录分类由调用方决定：通用上传按日期拆分
	objectPath := fmt.Sprintf("%s/%s%s", time.Now().Format("2006/01/02"), uuid.NewString(), ext)

	f, err := fileHeader.Open()
	if err != nil {
		router.InternalError(ctx, err)
		return
	}
	defer f.Close()

	fileInfo, err := i.storage.Put(ctx.Request.Context(), objectPath, f, fileHeader.Size)
	if err != nil {
		router.InternalError(ctx, err)
		return
	}

	fileInfo.ContentType = fileHeader.Header.Get("Content-Type")
	if fileInfo.ContentType == "" {
		fileInfo.ContentType = MimeType(fileInfo.Path)
	}

	router.Success(ctx, gin.H{
		"url":          fileInfo.URL,
		"originalName": fileHeader.Filename,
	})
}
