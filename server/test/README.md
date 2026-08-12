# Integration 测试使用说明

这个目录主要放集成测试，利用 Testcontainers 启动 MySQL 和 Redis，然后按测试层级/领域拆成多个 package。

## 文件结构

- `authintegration/`：认证系统测试
- `oauthintegration/`：OAuth 系统测试
- `syncintegration/`：同步功能测试
- `miscintegration/`：一些杂项集成测试
- `testenv/`：共用的测试脚手架

## 运行方式

本地要求能够使用 Docker。

```bash
go test ./test/... -count=1
```

下面是跑某个特定测试用例的例子（`TestRegister`）：

```bash
go test ./test/authintegration -run TestRegister -count=1
```

跑某个包（oauthintegraion）下的集成测试例子：

```bash
go test ./test/oauthintegration -count=1
```

### suite_test.go

每个 integration package 都有自己的 `suite_test.go`，通过 `test/testenv` 初始化一套共享环境和 HTTP server，然后跑该 package 下的测试用例。

MySQL 初始化完成后会执行一次 schema。每个测试用例开始时调用 `resetEnv(t)`，会清空 MySQL 业务表，并清空 Redis 当前 DB。

## 新增测试

新增测试文件按领域放到对应目录，package 使用对应包名，例如 `authintegration`、`oauthintegration`、`syncintegration` 或 `miscintegration`。

每个测试用例开头先调用：

```go
resetEnv(t)
```

HTTP 级测试可以通过全局的 `suite.Server.URL` 和 `suite.Client` 发请求：

```go
func TestSomething(t *testing.T) {
	resetEnv(t)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		suite.Server.URL+"/v1/example",
		bytes.NewBufferString(`{"key":"value"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.Client.Do(req)
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
suite.Env.DB
```

需要检查 Redis 状态时，可以使用：

```go
suite.Env.RDB
```

不要在测试里自己保存这些 client。`resetEnv(t)` 会清理业务表和 Redis 当前 DB。

## 注意事项

- 不要手动拼 MySQL 或 Redis 地址，使用测试环境里的 `suite.Env.DBURL` / `suite.Env.RedisURL`。
- 不要在单个测试里调用 `suite.Env.Reset`，统一用 `resetEnv(t)`。
- 修改数据库 schema 时，更新 `docker/mysql/mysql-init-scripts/000_schema.sql`。
