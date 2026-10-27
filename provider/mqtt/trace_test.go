package mqtt

import (
	"bytes"
	"context"
	"testing"

	"bit-labs.cn/owl/provider/otel"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type fakeMessage struct {
	topic   string
	payload []byte
}

func (m fakeMessage) Duplicate() bool   { return false }
func (m fakeMessage) Qos() byte         { return 0 }
func (m fakeMessage) Retained() bool    { return false }
func (m fakeMessage) Topic() string     { return m.topic }
func (m fakeMessage) MessageID() uint16 { return 1 }
func (m fakeMessage) Payload() []byte   { return m.payload }
func (m fakeMessage) Ack()              {}

func TestPublishContext_subscribeContextRoundTrip(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
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

	fake := &fakeMQTTClient{}
	opt := &Options{}
	setDefaults(opt)
	client := &MQTTClient{options: opt, client: fake, subs: map[string]subscriptionEntry{}}

	ctx, span := otel.Start(context.Background(), "job")
	err := client.PublishContext(ctx, "orders", []byte("hello"))
	span.End()
	if err != nil {
		t.Fatalf("PublishContext() error = %v", err)
	}
	raw, ok := fake.published.([]byte)
	if !ok {
		t.Fatalf("published type = %T; want []byte", fake.published)
	}
	if bytes.Equal(raw, []byte("hello")) {
		t.Fatal("PublishContext() left payload unwrapped")
	}

	var gotBody []byte
	var gotTrace trace.TraceID
	err = client.SubscribeContext("orders", func(ctx context.Context, _ mqtt.Client, msg mqtt.Message) {
		gotBody = append([]byte(nil), msg.Payload()...)
		gotTrace = trace.SpanFromContext(ctx).SpanContext().TraceID()
	})
	if err != nil {
		t.Fatalf("SubscribeContext() error = %v", err)
	}
	if fake.handler == nil {
		t.Fatal("SubscribeContext() did not register a handler")
	}
	fake.handler(nil, fakeMessage{topic: "orders", payload: raw})
	if !bytes.Equal(gotBody, []byte("hello")) {
		t.Fatalf("payload = %q; want hello", gotBody)
	}
	if gotTrace != span.SpanContext().TraceID() {
		t.Fatalf("trace id = %s; want %s", gotTrace, span.SpanContext().TraceID())
	}
}

func TestPublishContext_rejectsNonBytes(t *testing.T) {
	opt := &Options{}
	setDefaults(opt)
	client := &MQTTClient{options: opt, client: &fakeMQTTClient{}, subs: map[string]subscriptionEntry{}}
	err := client.PublishContext(context.Background(), "orders", 1)
	if err == nil {
		t.Fatal("PublishContext() error = nil; want payload type error")
	}
}
