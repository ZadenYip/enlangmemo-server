package serverapp

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/config"
	"github.com/zadenyip/enlangmemo-server/internal/infra/mysql"
	"github.com/zadenyip/enlangmemo-server/internal/infra/redisclient"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/server"
)

func Run(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	return Serve(ctx, listener)
}

func Serve(ctx context.Context, listener net.Listener) error {
	logger := logging.NewServerLog()
	logger.Info().Info("starting server")

	config := config.Load()

	db := mysql.NewClient(config.DatabaseURL)
	logger.Info().Info("connected to mysql")
	defer db.Close()

	rdb := redisclient.NewClient(config.RedisURL)
	logger.Info().Info("connected to redis")

	storeDeps := server.StoreDeps{
		DB:  db,
		Rdb: rdb,
	}

	server := server.New(storeDeps, logger)
	handler := server.GetHandler()

	httpServer := &http.Server{Handler: handler}
	errCh := make(chan error, 1)

	go func() {
		errCh <- httpServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		if err := httpServer.Shutdown(context.Background()); err != nil {
			logger.Error().Error("server shutdown failed", "err", err)
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		logger.Error().Error("server stopped", "err", err)
		return err
	}
}
