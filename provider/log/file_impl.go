package log

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/otel"
	"github.com/golang-module/carbon"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

/*
日志初始化步骤：
1. 设置 log 写入的文件 (writer)
2. 设置为日志编码的方法（encoder）
3. 创建日志核心
*/

var _ log.Logger = (*FileImpl)(nil)

type FileImpl struct {
	options *FileImplOptions
	state   *fileState
	ctx     context.Context
}

type fileState struct {
	mu         sync.Mutex
	l          *zap.SugaredLogger
	writer     *lumberjack.Logger
	preGetTime string
	closeErr   error
}

func (s *fileState) closeWriterLocked() error {
	if s.writer == nil {
		return nil
	}
	err := s.writer.Close()
	s.writer = nil
	return err
}

func (i *FileImpl) close() error {
	if i == nil || i.state == nil {
		return nil
	}
	i.state.mu.Lock()
	defer i.state.mu.Unlock()
	err := i.state.closeWriterLocked()
	i.state.l = nil
	return errors.Join(i.state.closeErr, err)
}

func (i *FileImpl) WithContext(ctx context.Context) log.Logger {
	clone := *i
	clone.ctx = ctx
	return &clone
}

type FileImplOptions struct {
	StorePath    string
	MaxSize      int
	MaxBackups   int
	MaxAge       int
	Compress     bool
	Level        zapcore.Level
	FileName     string
	dateFileName string // 计算出来，每天一个文件
}

var (
	defaultOptions = &FileImplOptions{
		StorePath:    "./storage",
		dateFileName: "0000-00-00",
		MaxSize:      50,
		MaxBackups:   100,
		MaxAge:       30,
		Compress:     true,
		Level:        zap.DebugLevel,
	}
)

var _ log.Logger = (*FileImpl)(nil)

func NewFileImpl(options *FileImplOptions) *FileImpl {
	if options == nil {
		options = defaultOptions
	}

	l := &FileImpl{
		options: options,
		state:   &fileState{},
	}
	return l
}

func (i *FileImpl) Emergency(content ...any) {
	i.setLogger()
	i.state.l.DPanic(joinArgs(i.withTrace(content)))
}

func (i *FileImpl) Alert(content ...any) {
	i.setLogger()
	i.state.l.Error(joinArgs(i.withTrace(content)))
}

func (i *FileImpl) Critical(content ...any) {
	i.setLogger()
}

func (i *FileImpl) Error(content ...any) {
	i.setLogger()
	i.state.l.Error(joinArgs(i.withTrace(content)))
}

func (i *FileImpl) Warning(content ...any) {
	i.setLogger()
	i.state.l.Warn(joinArgs(i.withTrace(content)))
}

func (i *FileImpl) Notice(content ...any) {
	i.setLogger()
	i.state.l.Warn(joinArgs(i.withTrace(content)))
}

func (i *FileImpl) Info(content ...any) {
	i.setLogger()
	i.state.l.Info(joinArgs(i.withTrace(content)))
}

func (i *FileImpl) Debug(content ...any) {
	i.setLogger()
	i.state.l.Debug(joinArgs(i.withTrace(content)))
}

// joinArgs 用空格分隔参数。zap 的 Info 对相邻字符串不会加空格，键和值会粘成 requestId1793。
func joinArgs(content []any) string {
	if len(content) == 0 {
		return ""
	}
	return strings.TrimSuffix(fmt.Sprintln(content...), "\n")
}

func (i *FileImpl) withTrace(content []any) []any {
	if hasTraceKey(content) {
		return content
	}
	extra := otel.Fields(i.ctx)
	if len(extra) == 0 {
		return content
	}
	out := make([]any, 0, len(content)+len(extra))
	out = append(out, content...)
	return append(out, extra...)
}

func hasTraceKey(content []any) bool {
	for _, v := range content {
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, ":")
		if s == "trace_id" {
			return true
		}
	}
	return false
}

// 根据时间轮换，因为随时都有可能发生时间变化，调用日志方法之前需要先调用这个方法
func (i *FileImpl) setLogger() {

	i.state.mu.Lock()
	defer i.state.mu.Unlock()

	now := carbon.Now()
	nowDate := now.ToDateString()

	// 如果文件夹不存在，则递归创建
	if _, err := os.Stat(i.options.StorePath); os.IsNotExist(err) {
		err = os.MkdirAll(i.options.StorePath, 755)
		if err != nil {
			return
		}
	}

	// 重新换一个文件来记录日志
	if i.state.preGetTime != "" {
		if nowDate != i.state.preGetTime {
			if err := i.state.closeWriterLocked(); err != nil {
				i.state.closeErr = err
			}
			i.state.l = nil
			i.state.preGetTime = nowDate
		}
	} else {
		i.state.preGetTime = nowDate
	}

	if i.state.l != nil {
		return
	}

	i.options.dateFileName = i.options.FileName + "-" + nowDate

	encoder := getEncoder()
	writeSyncer := getLogWriter(i.options)
	i.state.writer = writeSyncer
	// 打印
	write2file := zapcore.AddSync(writeSyncer)
	write2stdout := zapcore.AddSync(os.Stdout)
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoder),
		zapcore.NewMultiWriteSyncer(write2file, write2stdout),
		i.options.Level,
	)

	zap.AddCaller() //添加将调用函数信息记录到日志中的功能。
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	i.state.l = logger.Sugar()
}

func getLogWriter(options *FileImplOptions) *lumberjack.Logger {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   options.StorePath + "/" + options.dateFileName + ".log", // 日志文件路径
		MaxSize:    options.MaxSize,                                         // 最大尺寸, M
		MaxBackups: options.MaxBackups,                                      // 备份数 在进行切割之前，日志文件的最大大小（以MB为单位）
		MaxAge:     options.MaxAge,                                          // 保留旧文件的最大天数
		Compress:   options.Compress,                                        // 是否压缩/归档旧文件
	}

	return lumberJackLogger
}

func getEncoder() zapcore.EncoderConfig {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000") // 修改时间编码器

	// 在日志文件中使用大写字母记录日志级别
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return encoderConfig
}
