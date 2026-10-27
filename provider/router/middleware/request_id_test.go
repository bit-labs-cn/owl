package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func setupTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
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

func TestRequestID_usesParentTrace(t *testing.T) {
	rec := setupTracer(t)
	tp := otelapi.GetTracerProvider()
	upstreamCtx, upstream := tp.Tracer("test").Start(context.Background(), "upstream")
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("X-Request-ID", "req-1")
	otelapi.GetTextMapPropagator().Inject(upstreamCtx, propagation.HeaderCarrier(req.Header))
	upstream.End()

	r := gin.New()
	r.Use(RequestID())
	r.GET("/orders", func(c *gin.Context) {
		if got := c.GetString("request_id"); got != "req-1" {
			t.Errorf("request_id = %q; want req-1", got)
		}
		c.Status(http.StatusNoContent)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "req-1" {
		t.Fatalf("X-Request-ID = %q; want req-1", got)
	}
	var server sdktrace.ReadOnlySpan
	for _, sp := range rec.Ended() {
		if sp.Name() == "GET /orders" {
			server = sp
		}
	}
	if server == nil {
		t.Fatalf("missing server span in %d spans", len(rec.Ended()))
	}
	if got, want := server.Parent().SpanID(), upstream.SpanContext().SpanID(); got != want {
		t.Fatalf("parent = %s; want %s", got, want)
	}
	if server.SpanKind() != trace.SpanKindServer {
		t.Fatalf("kind = %s; want server", server.SpanKind())
	}
	attrs := map[string]string{}
	for _, attr := range server.Attributes() {
		attrs[string(attr.Key)] = attr.Value.Emit()
	}
	if attrs["request_id"] != "req-1" {
		t.Fatalf("request_id = %q; want req-1", attrs["request_id"])
	}
	if attrs["url.path"] != "/orders" {
		t.Fatalf("url.path = %q; want /orders", attrs["url.path"])
	}
	if attrs["http.request.method"] != http.MethodGet {
		t.Fatalf("method = %q; want GET", attrs["http.request.method"])
	}
}

func TestRequestID_generatesRequestID(t *testing.T) {
	setupTracer(t)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/orders", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/orders", nil))
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID is empty")
	}
}

func TestRequestID_marksServerError(t *testing.T) {
	rec := setupTracer(t)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/orders", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/orders", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500", w.Code)
	}
	ended := rec.Ended()
	if len(ended) != 1 {
		t.Fatalf("len(spans) = %d; want 1", len(ended))
	}
	if ended[0].Status().Code != codes.Error {
		t.Fatalf("span status = %s; want error", ended[0].Status().Code)
	}
}
