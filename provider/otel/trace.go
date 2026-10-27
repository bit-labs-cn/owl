package otel

import (
	"context"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "bit-labs.cn/owl"

// Start 从 ctx 创建一个 span。ctx 为 nil 时从 Background 起根 span。
// 默认 SpanKind 为 Internal；opts 中的 SpanKind 会覆盖默认值。
func Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	all := make([]trace.SpanStartOption, 0, len(opts)+1)
	all = append(all, trace.WithSpanKind(trace.SpanKindInternal))
	all = append(all, opts...)
	return otelapi.Tracer(tracerName).Start(ctx, name, all...)
}

// Run 在入口创建一个 internal span，执行 fn，然后结束 span。fn 为 nil 时直接返回。
func Run(ctx context.Context, name string, fn func(context.Context)) {
	if fn == nil {
		return
	}
	ctx, span := Start(ctx, name)
	defer span.End()
	fn(ctx)
}

// Fields 返回可追加到日志参数中的 trace_id 与 span_id。没有有效 span 时返回 nil。
func Fields(ctx context.Context) []any {
	if ctx == nil {
		return nil
	}
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return nil
	}
	return []any{"trace_id:", sc.TraceID().String(), "span_id:", sc.SpanID().String()}
}
