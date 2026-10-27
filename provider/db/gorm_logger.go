package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	logContract "bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type OwlGormLogger struct {
	log                     logContract.Logger
	level                   logger.LogLevel
	slowThreshold           time.Duration
	ignoreRecordNotFoundErr bool
}

var _ logger.Interface = (*OwlGormLogger)(nil)

func NewOwlGormLogger(l logContract.Logger) *OwlGormLogger {
	return &OwlGormLogger{
		log:                     l,
		level:                   logger.Info,
		slowThreshold:           200 * time.Millisecond,
		ignoreRecordNotFoundErr: true,
	}
}

func (i *OwlGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	n := *i
	n.level = level
	return &n
}

func (i *OwlGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if i.level < logger.Info || i.log == nil {
		return
	}
	requestID := getRequestID(ctx)
	if requestID != "" {
		i.log.WithContext(ctx).Info(traced(ctx, []any{"GORM", "requestId:", requestID, "msg", fmt.Sprintf(msg, data...)})...)
		return
	}
	i.log.WithContext(ctx).Info(traced(ctx, []any{fmt.Sprintf(msg, data...)})...)
}

func (i *OwlGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if i.level < logger.Warn || i.log == nil {
		return
	}
	requestID := getRequestID(ctx)
	if requestID != "" {
		i.log.WithContext(ctx).Warning(traced(ctx, []any{"GORM", "requestId:", requestID, "msg", fmt.Sprintf(msg, data...)})...)
		return
	}
	i.log.WithContext(ctx).Warning(traced(ctx, []any{fmt.Sprintf(msg, data...)})...)
}

func (i *OwlGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if i.level < logger.Error || i.log == nil {
		return
	}
	requestID := getRequestID(ctx)
	if requestID != "" {
		i.log.WithContext(ctx).Error(traced(ctx, []any{"GORM", "requestId:", requestID, "msg", fmt.Sprintf(msg, data...)})...)
		return
	}
	i.log.WithContext(ctx).Error(traced(ctx, []any{fmt.Sprintf(msg, data...)})...)
}

func (i *OwlGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	queryErr := err
	if i.ignoreRecordNotFoundErr && errors.Is(err, gorm.ErrRecordNotFound) {
		queryErr = nil
	}
	i.recordQuery(ctx, begin, begin.Add(elapsed), sql, rows, queryErr)

	if i.level <= logger.Silent || i.log == nil {
		return
	}
	if err != nil && i.ignoreRecordNotFoundErr && errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}

	requestID := getRequestID(ctx)

	if err != nil {
		if i.level >= logger.Error {
			i.log.WithContext(ctx).Error(traced(ctx, []any{"执行 SQL 失败", "requestId:", requestID, "耗时", elapsed, "影响行数", rows, "sql:", sql, "错误:", err})...)
		}
		return
	}

	if i.slowThreshold > 0 && elapsed > i.slowThreshold {
		if i.level >= logger.Warn {
			i.log.WithContext(ctx).Warning(traced(ctx, []any{"慢查询", "requestId:", requestID, "耗时", elapsed, "影响行数", rows, "sql:", sql})...)
		}
		return
	}

	if i.level >= logger.Info {
		i.log.WithContext(ctx).Info(traced(ctx, []any{"执行 SQL", "requestId:", requestID, "耗时", elapsed, "影响行数", rows, "sql:", sql})...)
	}
}

func (i *OwlGormLogger) recordQuery(ctx context.Context, begin, end time.Time, sql string, rows int64, err error) {
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		return
	}
	_, span := otel.Start(ctx, sqlVerb(sql), trace.WithTimestamp(begin), trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(
		attribute.String("db.statement", sql),
		attribute.Int64("db.rows_affected", rows),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End(trace.WithTimestamp(end))
}

func sqlVerb(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return "db"
	}
	verb, _, _ := strings.Cut(sql, " ")
	return verb
}

func getRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value("request_id")
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func traced(ctx context.Context, args []any) []any {
	return append(args, otel.Fields(ctx)...)
}
