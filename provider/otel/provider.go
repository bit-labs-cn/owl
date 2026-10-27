package otel

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const defaultServiceName = "owl"

// Provider 持有进程内的 TracerProvider。未启用时 Shutdown 为空操作。
type Provider struct {
	mu sync.Mutex
	tp *sdktrace.TracerProvider
}

var (
	installMu sync.Mutex
	installed *Provider
)

func setInstalled(p *Provider) {
	installMu.Lock()
	installed = p
	installMu.Unlock()
}

// Shutdown 刷出并关闭当前进程安装的 TracerProvider。未安装时返回 nil。
func Shutdown(ctx context.Context) error {
	installMu.Lock()
	p := installed
	installMu.Unlock()
	if p == nil {
		return nil
	}
	return p.Shutdown(ctx)
}

// New 按配置安装全局 TracerProvider 与 W3C 传播器。
// enabled 为 false 时不创建导出器。enabled 为 true 且 endpoint 为空时返回错误。
func New(cfg Options, serviceName string) (*Provider, error) {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		name = strings.TrimSpace(cfg.ServiceName)
	}
	if name == "" {
		name = defaultServiceName
	}

	otelapi.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	p := &Provider{}
	if !cfg.Enabled {
		setInstalled(p)
		return p, nil
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("otel endpoint is empty")
	}

	exp, err := newExporter(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	res, err := processResource(context.Background(), name)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}
	p.tp = sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exp)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(normalizedRatio(cfg.SampleRatio)))),
	)
	otelapi.SetTracerProvider(p.tp)
	setInstalled(p)
	return p, nil
}

// processResource 构造进程资源。service.name 标识服务，主机、系统和进程属性供后端展示。
func processResource(ctx context.Context, serviceName string) (*sdkresource.Resource, error) {
	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(attribute.String("service.name", serviceName)),
		sdkresource.WithHost(),
		sdkresource.WithOS(),
		sdkresource.WithProcessPID(),
		sdkresource.WithProcessExecutableName(),
		sdkresource.WithProcessRuntimeName(),
		sdkresource.WithProcessRuntimeVersion(),
		sdkresource.WithTelemetrySDK(),
	)
	if err != nil && !errors.Is(err, sdkresource.ErrPartialResource) {
		return nil, err
	}
	return res, nil
}

func newExporter(ctx context.Context, cfg Options) (sdktrace.SpanExporter, error) {
	endpoint := normalizeEndpoint(cfg.Endpoint)
	switch strings.ToLower(strings.TrimSpace(cfg.Protocol)) {
	case "", "http":
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exp, err := otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("otel http exporter: %w", err)
		}
		return exp, nil
	case "grpc":
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exp, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("otel grpc exporter: %w", err)
		}
		return exp, nil
	default:
		return nil, fmt.Errorf("otel protocol %q", cfg.Protocol)
	}
}

// Shutdown 刷出剩余 span。重复调用返回 nil。
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	p.mu.Lock()
	tp := p.tp
	p.tp = nil
	p.mu.Unlock()
	if tp == nil {
		return nil
	}
	return tp.Shutdown(ctx)
}
