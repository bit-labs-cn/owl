// Package otel 提供进程内链路追踪。
// HTTP、定时器和消息队列把 span 放进 context，日志从中读取 trace_id 与 span_id，并通过 OTLP 上报。
package otel
