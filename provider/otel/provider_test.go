package otel

import (
	"context"
	"strings"
	"testing"

	otelapi "go.opentelemetry.io/otel"
)

func TestNew_disabled(t *testing.T) {
	installMu.Lock()
	prev := installed
	installMu.Unlock()
	prevTP := otelapi.GetTracerProvider()
	prevProp := otelapi.GetTextMapPropagator()
	t.Cleanup(func() {
		setInstalled(prev)
		otelapi.SetTracerProvider(prevTP)
		otelapi.SetTextMapPropagator(prevProp)
	})

	p, err := New(Options{Enabled: false}, "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("package Shutdown() error = %v", err)
	}
}

func TestNew_requiresEndpoint(t *testing.T) {
	prevTP := otelapi.GetTracerProvider()
	prevProp := otelapi.GetTextMapPropagator()
	t.Cleanup(func() {
		otelapi.SetTracerProvider(prevTP)
		otelapi.SetTextMapPropagator(prevProp)
	})

	_, err := New(Options{Enabled: true, Endpoint: "  "}, "owl")
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("New() error = %v; want endpoint", err)
	}
}

func TestNew_rejectsProtocol(t *testing.T) {
	prevTP := otelapi.GetTracerProvider()
	prevProp := otelapi.GetTextMapPropagator()
	t.Cleanup(func() {
		otelapi.SetTracerProvider(prevTP)
		otelapi.SetTextMapPropagator(prevProp)
	})

	_, err := New(Options{Enabled: true, Endpoint: "127.0.0.1:4318", Protocol: "nope"}, "owl")
	if err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("New() error = %v; want protocol", err)
	}
}

func TestProcessResource(t *testing.T) {
	res, err := processResource(context.Background(), "owl")
	if err != nil {
		t.Fatalf("processResource() error = %v", err)
	}
	got := map[string]string{}
	for _, attr := range res.Attributes() {
		got[string(attr.Key)] = attr.Value.Emit()
	}
	if got["service.name"] != "owl" {
		t.Fatalf("service.name = %q; want owl", got["service.name"])
	}
	for _, key := range []string{"host.name", "process.pid", "process.executable.name", "os.type", "telemetry.sdk.language"} {
		if got[key] == "" {
			t.Fatalf("missing %s in %v", key, got)
		}
	}
}

func TestResolveServiceName(t *testing.T) {
	if got := resolveServiceName(nil, Options{}); got != defaultServiceName {
		t.Fatalf("resolveServiceName() = %q; want %s", got, defaultServiceName)
	}
	if got := resolveServiceName(nil, Options{ServiceName: " api "}); got != "api" {
		t.Fatalf("resolveServiceName() = %q; want api", got)
	}
}
