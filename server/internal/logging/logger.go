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
