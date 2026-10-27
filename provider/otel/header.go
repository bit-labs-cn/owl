package otel

import (
	"context"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// HeaderCarrier 在 map 形式的消息头上读写 W3C trace context。
// RabbitMQ 的 amqp.Table 可以与 map[string]interface{} 相互转换后使用。
type HeaderCarrier map[string]interface{}

var _ propagation.TextMapCarrier = HeaderCarrier(nil)

func (c HeaderCarrier) Get(key string) string {
	v, ok := c[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

func (c HeaderCarrier) Set(key, value string) {
	c[key] = value
}

func (c HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

// InjectHeaders 把 ctx 中的 trace context 写入 headers。headers 为 nil 时新建 map。
func InjectHeaders(ctx context.Context, headers map[string]interface{}) map[string]interface{} {
	if ctx == nil {
		ctx = context.Background()
	}
	if headers == nil {
		headers = make(map[string]interface{})
	}
	otelapi.GetTextMapPropagator().Inject(ctx, HeaderCarrier(headers))
	return headers
}

// ExtractHeaders 从 headers 提取 trace context，作为 ctx 的父级。
func ExtractHeaders(ctx context.Context, headers map[string]interface{}) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if headers == nil {
		return ctx
	}
	return otelapi.GetTextMapPropagator().Extract(ctx, HeaderCarrier(headers))
}
