package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	otelprov "bit-labs.cn/owl/provider/otel"
	"github.com/gin-gonic/gin"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const requestIDContextKey = "request_id"

// RequestID 为每个请求保留 X-Request-ID，并创建 server span。
// 入站 traceparent 会成为该 span 的父级。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)

		ctx := otelapi.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := otelprov.Start(ctx, c.Request.Method+" "+c.Request.URL.Path, trace.WithSpanKind(trace.SpanKindServer))
		defer func() {
			route := c.FullPath()
			if route != "" {
				span.SetName(c.Request.Method + " " + route)
			}
			status := c.Writer.Status()
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("url.path", c.Request.URL.Path),
				attribute.Int("http.response.status_code", status),
				attribute.String("client.address", c.ClientIP()),
				attribute.String("user_agent.original", c.Request.UserAgent()),
				attribute.String("request_id", requestID),
			}
			if query := c.Request.URL.RawQuery; query != "" {
				attrs = append(attrs, attribute.String("url.query", query))
			}
			if route != "" {
				attrs = append(attrs, attribute.String("http.route", route))
			}
			span.SetAttributes(attrs...)
			if status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			span.End()
		}()

		c.Set(requestIDContextKey, requestID)
		c.Request = c.Request.WithContext(context.WithValue(ctx, requestIDContextKey, requestID))
		c.Next()
	}
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
