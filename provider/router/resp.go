package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Resp struct {
	Code    string `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
}

type PageResp struct {
	Resp
	Total       int `json:"total"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
}

type PageReq struct {
	Page     int `json:"page" form:"page"`
	PageSize int `json:"pageSize" form:"pageSize"`
}

func Success(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Resp{Success: true, Msg: "操作成功", Data: data})
}

func Fail(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Resp{Success: false, Msg: "操作失败", Data: data})
}

func FailWithMsg(ctx *gin.Context, msg string, data any) {
	ctx.JSON(http.StatusOK, Resp{Success: false, Msg: msg, Data: data})
}

func SuccessWithMsg(ctx *gin.Context, msg string, data any) {
	ctx.JSON(http.StatusOK, Resp{Success: true, Msg: msg, Data: data})
}

func BadRequest(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusBadRequest, Resp{Success: false, Msg: msg})
}

func Forbidden(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusForbidden, Resp{Success: false, Msg: msg})
}

func InternalError(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusInternalServerError, Resp{Success: false, Msg: err.Error()})
}

func PageSuccess(ctx *gin.Context, total int, currentPage int, pageSize int, data any) {
	ctx.JSON(http.StatusOK, PageResp{
		Resp:        Resp{Success: true, Msg: "操作成功", Data: gin.H{"list": data}},
		Total:       total,
		CurrentPage: currentPage,
		PageSize:    pageSize,
	})
}
