package logging

import (
	"context"
	"log/slog"
	"os"
)

type Logger interface {
	Info() *slog.Logger
	Error() *slog.Logger
	InfoCtx(ctx context.Context, msg string, args ...any)
	WarnCtx(ctx context.Context, msg string, args ...any)
	ErrorCtx(ctx context.Context, msg string, args ...any)
}

type ServerLog struct {
	infoLog *slog.Logger
	errLog  *slog.Logger
}

type TraceKey struct{}

func NewServerLog() *ServerLog {
	return &ServerLog{
		infoLog: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
		errLog: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     slog.LevelError,
			AddSource: true,
		})),
	}
}

func (l *ServerLog) Info() *slog.Logger {
	return l.infoLog
}

func (l *ServerLog) Error() *slog.Logger {
	return l.errLog
}

func (l *ServerLog) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.infoLog.InfoContext(ctx, msg, argsWithTrace(ctx, args)...)
}

func (l *ServerLog) WarnCtx(ctx context.Context, msg string, args ...any) {
	l.infoLog.WarnContext(ctx, msg, argsWithTrace(ctx, args)...)
}

func (l *ServerLog) ErrorCtx(ctx context.Context, msg string, args ...any) {
	l.errLog.ErrorContext(ctx, msg, argsWithTrace(ctx, args)...)
}

func TraceIDFromCtx(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(TraceKey{}).(string)
	return traceID, ok
}

func argsWithTrace(ctx context.Context, args []any) []any {
	traceID, ok := TraceIDFromCtx(ctx)
	if !ok {
		return args
	}

	return append(args, "traceparent", traceID)
}

// MaskSecret 用于在日志中隐藏敏感信息，只展示前 4 个字符和后 4 个字符，中间用星号替代
//
// 返回的字符串格式为：前 4 个字符 + "***" + 后 4 个字符
func MaskSecret(value string) string {
	return MaskSecretWith(value, 4, 4)
}

// MaskSecretWith 用于在日志中隐藏敏感信息
//
// 如果 value 的长度小于等于 prefix + suffix，则直接返回 "***"
// 否则返回前 prefix 个字符 + "***" + 后 suffix 个字符
// 例如：MaskSecretWith("1234567890", 4, 4) => "1234***7890"
func MaskSecretWith(value string, prefix, suffix int) string {
	if len(value) <= prefix+suffix {
		return "***"
	}
	return value[:prefix] + "***" + value[len(value)-suffix:]
}
