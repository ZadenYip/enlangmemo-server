// 所有 integration 测试的入口
package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/server"
)

var (
	env        *testEnv
	testServer *httptest.Server
	testClient *http.Client
)

// TestMain 测试入口
func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	env, err = initTestEnv(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "integration test setup failed: %v\n", err)
		os.Exit(1)
	}

	// 启动 HTTP，供测试用例发起请求。
	startHTTPServer()

	code := m.Run()

	// 关闭并清理资源
	testServer.Close()
	env.close(ctx)
	os.Exit(code)
}

func resetEnv(t *testing.T) {
	// 标记当前函数是 Helper，避免堆栈信息有这里。
	t.Helper()

	// env.reset 替换 dbPool/redisClient，server 也要绑定到新的客户端。
	require.NoError(t, env.reset(t.Context()))
	// 重建 HTTP server
	startHTTPServer()
}

func startHTTPServer() {
	if testServer != nil {
		testServer.Close()
	}

	storeDeps := server.StoreDeps{
		PGPool: env.dbPool,
		Rdb:    env.rdsClient,
	}

	srv := server.New(storeDeps, server.NewServerLog())
	testServer = httptest.NewServer(srv.GetHandler())

	testClient = testServer.Client()
	// 添加 tracepoint header 到测试客户端的请求中
	testClient.Transport = &traceparentTransport{base: testClient.Transport}
}

type traceparentTransport struct {
	base http.RoundTripper
}

func (t *traceparentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 在请求中添加 traceparent header
	req.Header.Set("traceparent", "integration-test")
	return t.base.RoundTrip(req)
}
