package otel

import (
	"bytes"
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestWrapPayload_roundTrip(t *testing.T) {
	setupTracer(t)
	ctx, span := Start(context.Background(), "pub")
	defer span.End()

	body := []byte("hello")
	wrapped := WrapPayload(ctx, body)
	if bytes.Equal(wrapped, body) {
		t.Fatal("WrapPayload() = body; want OWLTRACE prefix")
	}
	if !bytes.HasPrefix(wrapped, []byte(traceMagic)) {
		t.Fatalf("WrapPayload() = %q; want %s prefix", wrapped, traceMagic)
	}

	gotCtx, gotBody := UnwrapPayload(context.Background(), wrapped)
	if !bytes.Equal(gotBody, body) {
		t.Fatalf("body = %q; want %q", gotBody, body)
	}
	got := trace.SpanFromContext(gotCtx).SpanContext()
	if got.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("trace id = %s; want %s", got.TraceID(), span.SpanContext().TraceID())
	}
}

func TestWrapPayload_withoutSpan(t *testing.T) {
	setupTracer(t)
	body := []byte("hello")
	got := WrapPayload(context.Background(), body)
	if !bytes.Equal(got, body) {
		t.Fatalf("WrapPayload() = %q; want original body", got)
	}
}

func TestUnwrapPayload_keepsUnframed(t *testing.T) {
	raw := []byte("OWLTRACE\nnot-a-frame")
	_, body := UnwrapPayload(context.Background(), raw)
	if !bytes.Equal(body, raw) {
		t.Fatalf("body = %q; want original", body)
	}
}
