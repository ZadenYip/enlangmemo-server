package serverapp

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	// 这里的地址已经被第一行的 listener 占用，所以 Run 应该返回一个错误
	err = Run(context.Background(), listener.Addr().String())

	require.Error(t, err)
}
