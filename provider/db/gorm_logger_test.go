package db

import (
	"context"
	"testing"
	"time"

	"bit-labs.cn/owl/provider/otel"
	otelapi "go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm/logger"
)

func TestTrace_recordsSQLSpan(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otelapi.GetTracerProvider()
	otelapi.SetTracerProvider(tp)
	t.Cleanup(func() { otelapi.SetTracerProvider(prev) })

	ctx, parent := otel.Start(context.Background(), "GET /api/v1/rollouts")
	begin := time.Now().Add(-20 * time.Millisecond)
	l := &OwlGormLogger{level: logger.Silent, ignoreRecordNotFoundErr: true}
	const statement = "SELECT * FROM rollouts WHERE id = 1"
	l.Trace(ctx, begin, func() (string, int64) {
		return statement, 1
	}, nil)
	parent.End()

	var query sdktrace.ReadOnlySpan
	for _, sp := range rec.Ended() {
		if sp.Name() == "SELECT" {
			query = sp
		}
	}
	if query == nil {
		t.Fatalf("missing SQL span in %d spans", len(rec.Ended()))
	}
	if query.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatalf("parent = %s; want %s", query.Parent().SpanID(), parent.SpanContext().SpanID())
	}
	if query.SpanKind() != trace.SpanKindClient {
		t.Fatalf("kind = %s; want client", query.SpanKind())
	}
	var gotSQL string
	for _, attr := range query.Attributes() {
		if attr.Key == "db.statement" {
			gotSQL = attr.Value.AsString()
		}
	}
	if gotSQL != statement {
		t.Fatalf("db.statement = %q; want %q", gotSQL, statement)
	}
	if query.EndTime().Sub(query.StartTime()) < 15*time.Millisecond {
		t.Fatalf("duration = %s; want about 20ms", query.EndTime().Sub(query.StartTime()))
	}
}
