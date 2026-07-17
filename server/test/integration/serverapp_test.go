package integration

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/serverapp"
)

func TestServerAppRunStartsWithRealDependencies(t *testing.T) {
	resetEnv(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- serverapp.Run(ctx, addr)
	}()

	client := &http.Client{Timeout: time.Second}

	healthURL := "http://" + addr + "/healthz"
	require.Eventually(t, func() bool {
		resp, err := client.Get(healthURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 100*time.Millisecond)

	// 停止服务器，这里是通过取消上下文来实现的
	cancel()
	require.NoError(t, <-errCh)
}
