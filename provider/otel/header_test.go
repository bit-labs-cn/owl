package otel

import (
	"context"
	"testing"

	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel/trace"
)

func TestInjectHeaders_amqpRoundTrip(t *testing.T) {
	setupTracer(t)
	ctx, span := Start(context.Background(), "publish")
	defer span.End()

	table := amqp.Table{}
	headers := InjectHeaders(ctx, map[string]interface{}(table))
	table = amqp.Table(headers)
	raw, ok := table["traceparent"].(string)
	if !ok || raw == "" {
		t.Fatalf("traceparent = %#v; want non-empty string", table["traceparent"])
	}

	extracted := ExtractHeaders(context.Background(), map[string]interface{}(table))
	got := trace.SpanFromContext(extracted).SpanContext()
	if got.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("trace id = %s; want %s", got.TraceID(), span.SpanContext().TraceID())
	}
	if got.SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("span id = %s; want %s", got.SpanID(), span.SpanContext().SpanID())
	}
}

func TestExtractHeaders_nil(t *testing.T) {
	ctx := ExtractHeaders(nil, nil)
	if ctx == nil {
		t.Fatal("ExtractHeaders() = nil ctx")
	}
	if trace.SpanFromContext(ctx).SpanContext().IsValid() {
		t.Fatal("nil headers produced a span")
	}
}
