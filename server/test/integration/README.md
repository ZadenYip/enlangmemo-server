# Integration 测试使用说明

这个目录/包主要是放集成测试，利用 Testcontainers 库启动 PG 和 Redis，然后去测试各个接口。

## 文件结构

- `suite_test.go`：集成测试的总入口/启动处。
- `env_test.go`：Testcontainers 的一些配置，设计 PG 和 Redis。
- `*_test.go`：具体场景对应的集成测试。

## 运行方式

本地要求能够使用 Docker。

```bash
go test ./test/integration -count=1
```

如果 Go build cache 目录不可写，可以临时指定到 `/tmp`：

```bash
GOCACHE=/tmp/enlangmemo-go-build go test ./test/integration -count=1
```

只跑测试函数 `TestRegister`：

```bash
go test ./test/integration -run TestRegister -count=1
```

跑整个 integration package 的测试：

```bash
go test ./test/integration -count=1
```

### suite_test.go

`suite_test.go` 会在整个 integration package 开始时初始化一套共享环境，然后跑各个测试用例。

Postgres 初始化完成后会创建一次 snapshot（快照）。每个测试用例开始时调用 `resetEnv(t)`，会把 PG 还原到这个 snapshot，并清空 Redis 当前 DB。

## 新增测试

新增测试文件放在 `test/integration` 目录，package 使用 `integration`。

每个测试用例开头先调用：

```go
resetEnv(t)
```

然后通过全局的 `testServer.URL` 和 `testClient` 发请求：

```go
func TestSomething(t *testing.T) {
	resetEnv(t)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		testServer.URL+"/v1/example",
		bytes.NewBufferString(`{"key":"value"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## 直接访问数据库或 Redis

需要检查数据库状态时，可以使用：

```go
env.dbPool
```

需要检查 Redis 状态时，可以使用：

    env.rdsClient

不要在测试里长期保存这些 client。`resetEnv(t)` 会关闭旧连接并创建新的连接池和 Redis client。

## 注意事项

- 不要手动拼 Postgres 或 Redis 地址，使用容器提供的 `ConnectionString()`。
- 不要在单个测试里调用 `env.reset`，统一用 `resetEnv(t)`。
- 修改数据库 schema 时，更新 `docker/pg-init-scripts/000_schema.sql`，snapshot 会自动基于新的初始化结果创建。
