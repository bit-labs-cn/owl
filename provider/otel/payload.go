package otel

import (
	"bytes"
	"context"
	"sort"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// traceMagic 是 MQTT 3.1.1 载荷前的追踪帧。协议本身没有属性字段。
const traceMagic = "OWLTRACE\n"

// WrapPayload 在 body 前写入 W3C trace context。ctx 中没有可传播的 span 时返回原 body。
func WrapPayload(ctx context.Context, body []byte) []byte {
	if ctx == nil {
		ctx = context.Background()
	}
	carrier := propagation.MapCarrier{}
	otelapi.GetTextMapPropagator().Inject(ctx, carrier)
	if len(carrier) == 0 {
		return body
	}

	var buf bytes.Buffer
	buf.WriteString(traceMagic)
	writeHeader(&buf, carrier, "traceparent")
	writeHeader(&buf, carrier, "tracestate")
	keys := make([]string, 0, len(carrier))
	for key := range carrier {
		if key == "traceparent" || key == "tracestate" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		writeHeader(&buf, carrier, key)
	}
	buf.WriteByte('\n')
	buf.Write(body)
	return buf.Bytes()
}

func writeHeader(buf *bytes.Buffer, carrier propagation.MapCarrier, key string) {
	value, ok := carrier[key]
	if !ok || value == "" {
		return
	}
	buf.WriteString(key)
	buf.WriteString(": ")
	buf.WriteString(value)
	buf.WriteByte('\n')
}

// UnwrapPayload 去掉 OWLTRACE 前缀并提取父 span。没有完整前缀时原样返回 body。
func UnwrapPayload(ctx context.Context, body []byte) (context.Context, []byte) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !bytes.HasPrefix(body, []byte(traceMagic)) {
		return ctx, body
	}
	rest := body[len(traceMagic):]
	idx := bytes.Index(rest, []byte("\n\n"))
	if idx < 0 {
		return ctx, body
	}
	carrier := propagation.MapCarrier{}
	if idx > 0 {
		for _, line := range bytes.Split(rest[:idx], []byte("\n")) {
			key, value, ok := bytes.Cut(line, []byte(": "))
			if !ok || len(key) == 0 {
				continue
			}
			carrier[string(key)] = string(value)
		}
	}
	ctx = otelapi.GetTextMapPropagator().Extract(ctx, carrier)
	return ctx, rest[idx+2:]
}
