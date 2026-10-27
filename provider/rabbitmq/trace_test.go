package rabbitmq

import (
	"context"
	"testing"

	"bit-labs.cn/owl/provider/otel"
	"github.com/streadway/amqp"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestDeliver_extractsTraceparent(t *testing.T) {
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

	ctx, span := otel.Start(context.Background(), "publish")
	headers := amqp.Table(otel.InjectHeaders(ctx, nil))
	span.End()

	var got trace.TraceID
	client := &RabbitMQClient{}
	client.deliver("orders", amqp.Delivery{Headers: headers}, func(ctx context.Context, msg amqp.Delivery) {
		got = trace.SpanFromContext(ctx).SpanContext().TraceID()
		if msg.Headers["traceparent"] == nil {
			t.Fatal("delivery lost traceparent")
		}
	})
	if got != span.SpanContext().TraceID() {
		t.Fatalf("trace id = %s; want %s", got, span.SpanContext().TraceID())
	}
	var consumer sdktrace.ReadOnlySpan
	for _, sp := range rec.Ended() {
		if sp.Name() == "orders process" {
			consumer = sp
		}
	}
	if consumer == nil {
		t.Fatal("missing consumer span")
	}
	if consumer.Parent().SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("parent = %s; want %s", consumer.Parent().SpanID(), span.SpanContext().SpanID())
	}
	if consumer.SpanKind() != trace.SpanKindConsumer {
		t.Fatalf("kind = %s; want consumer", consumer.SpanKind())
	}
}
