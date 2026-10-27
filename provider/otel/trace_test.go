package otel

import (
	"context"
	"testing"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func setupTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prevTP := otelapi.GetTracerProvider()
	prevProp := otelapi.GetTextMapPropagator()
	otelapi.SetTracerProvider(tp)
	otelapi.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		otelapi.SetTracerProvider(prevTP)
		otelapi.SetTextMapPropagator(prevProp)
	})
	return rec
}

func TestStart_backgroundIsRoot(t *testing.T) {
	rec := setupTracer(t)
	ctx, span := Start(nil, "cron.cleanup")
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		t.Fatal("Start() ctx has invalid span")
	}
	span.End()

	ended := rec.Ended()
	if len(ended) != 1 {
		t.Fatalf("len(spans) = %d; want 1", len(ended))
	}
	if ended[0].Name() != "cron.cleanup" {
		t.Fatalf("name = %s; want cron.cleanup", ended[0].Name())
	}
	if ended[0].Parent().IsValid() {
		t.Fatalf("parent = %s; want root", ended[0].Parent().SpanID())
	}
	if ended[0].SpanKind() != trace.SpanKindInternal {
		t.Fatalf("kind = %s; want internal", ended[0].SpanKind())
	}
}

func TestRun_callsFnInsideSpan(t *testing.T) {
	rec := setupTracer(t)
	var called bool
	Run(context.Background(), "job", func(ctx context.Context) {
		called = true
		if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
			t.Fatal("Run() ctx has invalid span")
		}
	})
	if !called {
		t.Fatal("Run() did not call fn")
	}
	if len(rec.Ended()) != 1 {
		t.Fatalf("len(spans) = %d; want 1", len(rec.Ended()))
	}
}

func TestFields_withoutSpan(t *testing.T) {
	if got := Fields(context.Background()); got != nil {
		t.Fatalf("Fields() = %#v; want nil", got)
	}
	if got := Fields(nil); got != nil {
		t.Fatalf("Fields(nil) = %#v; want nil", got)
	}
}

func TestFields_withSpan(t *testing.T) {
	setupTracer(t)
	ctx, span := Start(context.Background(), "work")
	defer span.End()
	got := Fields(ctx)
	if len(got) != 4 {
		t.Fatalf("Fields() = %#v; want 4 items", got)
	}
	if got[0] != "trace_id:" || got[1] != span.SpanContext().TraceID().String() {
		t.Fatalf("trace = %#v; want %s", got[:2], span.SpanContext().TraceID())
	}
	if got[2] != "span_id:" || got[3] != span.SpanContext().SpanID().String() {
		t.Fatalf("span = %#v; want %s", got[2:], span.SpanContext().SpanID())
	}
}
