package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/zadenyip/enlangmemo-server/internal/serverapp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := serverapp.Run(ctx, ":8080"); err != nil {
		os.Exit(1)
	}
}
