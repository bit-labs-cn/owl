package log

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap/zapcore"
)

func TestFileImpl_WithContext_traceID(t *testing.T) {
	dir := t.TempDir()
	l := NewFileImpl(&FileImplOptions{
		StorePath:  dir,
		FileName:   "app",
		Level:      zapcore.DebugLevel,
		MaxSize:    10,
		MaxBackups: 1,
		MaxAge:     1,
	})
	defer func() {
		if err := l.close(); err != nil {
			t.Error(err)
		}
	}()
	tp := sdktrace.NewTracerProvider()
	ctx, span := tp.Tracer("test").Start(context.Background(), "work")
	defer span.End()
	traceID := span.SpanContext().TraceID().String()

	traced := l.WithContext(ctx)
	plain := l.WithContext(context.Background())
	traced.Info("hello-traced")
	plain.Info("hello-plain")

	data := readLogs(t, dir)
	var sawTraced, sawPlain bool
	for _, line := range strings.Split(data, "\n") {
		if strings.Contains(line, "hello-traced") {
			sawTraced = true
			if !strings.Contains(line, "trace_id: "+traceID) || !strings.Contains(line, "span_id:") {
				t.Fatalf("traced line = %q; want trace_id %s", line, traceID)
			}
		}
		if strings.Contains(line, "hello-plain") {
			sawPlain = true
			if strings.Contains(line, traceID) {
				t.Fatalf("plain line = %q; contains trace id", line)
			}
		}
	}
	if !sawTraced || !sawPlain {
		t.Fatalf("log = %q; want hello-traced and hello-plain", data)
	}
}

func readLogs(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		b.Write(data)
	}
	return b.String()
}
