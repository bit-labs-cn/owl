package router

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"unicode"

	errContract "bit-labs.cn/owl/contract/errors"
	owlvalidator "bit-labs.cn/owl/provider/validator"
	"github.com/gin-gonic/gin"
	validatorv10 "github.com/go-playground/validator/v10"
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

func Fail(ctx *gin.Context, err error) {
	var ves validatorv10.ValidationErrors
	var bizErr *errContract.BizError
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	switch {
	case stderrors.As(err, &ves):
		BadRequest(ctx, owlvalidator.TranslateValidationErrors(err))
	case stderrors.As(err, &typeErr):
		BadRequest(ctx, formatUnmarshalTypeError(typeErr))
	case stderrors.As(err, &syntaxErr):
		BadRequest(ctx, formatJSONSyntaxError(syntaxErr))
	case stderrors.As(err, &bizErr):
		ctx.JSON(http.StatusOK, Resp{Code: bizErr.Code, Success: false, Msg: bizErr.Message})
	default:
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		if isSafeErrorMessage(msg) {
			ctx.JSON(http.StatusOK, Resp{Success: false, Msg: msg})
			return
		}
		InternalError(ctx, err)
	}
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
	requestID, _ := ctx.Get("request_id")
	logger := GetLoggerFromCtx(ctx)
	if logger != nil && err != nil {
		logger.Error(
			"internal error",
			" requestId:", requestID,
			" method:", ctx.Request.Method,
			" path:", ctx.Request.URL.Path,
			" error:", err.Error(),
		)
	}
	ctx.JSON(http.StatusInternalServerError, Resp{Success: false, Msg: "服务器内部错误，请稍后重试"})
}

func PageSuccess(ctx *gin.Context, total int, currentPage int, pageSize int, data any) {
	ctx.JSON(http.StatusOK, PageResp{
		Resp:        Resp{Success: true, Msg: "操作成功", Data: gin.H{"list": data}},
		Total:       total,
		CurrentPage: currentPage,
		PageSize:    pageSize,
	})
}

// formatUnmarshalTypeError 生成类型错误提示：字段、期望类型、实际传入值
func formatUnmarshalTypeError(e *json.UnmarshalTypeError) string {
	field := e.Field
	if e.Struct != "" {
		field = e.Struct + "." + field
	}
	return fmt.Sprintf("字段 %s 需要 %s 类型，实际传入: %s", field, e.Type.String(), e.Value)
}

// formatJSONSyntaxError 生成 JSON 语法错误提示
func formatJSONSyntaxError(e *json.SyntaxError) string {
	return fmt.Sprintf("请求体 JSON 语法错误（offset %d）: %s", e.Offset, e.Error())
}

func isSafeErrorMessage(msg string) bool {
	if msg == "" {
		return false
	}
	for _, r := range msg {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
