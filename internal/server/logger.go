package server

import (
	"log/slog"
	"os"
)

type Logger interface {
	Info() *slog.Logger
	Error() *slog.Logger
}

type ServerLog struct {
	infoLog *slog.Logger
	errLog  *slog.Logger
}

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
