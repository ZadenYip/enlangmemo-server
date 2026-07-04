# 注册新路由

## 1. 在业务包内定义 Handler

每个业务模块自己维护 handler 类型、构造函数和具体路由函数。

```go
type XxxHandler struct {
	store XxxStore
}

func NewXxxHandler(store XxxStore) *XxxHandler {
	return &XxxHandler{
		store: store,
	}
}
```

要求：

- handler 放在对应业务包内，不放到 `internal/server`。
- handler 依赖通过构造函数注入，不在路由函数里临时创建数据库、Redis 等基础设施客户端。
- 具体路由函数使用 `func (h *XxxHandler) action(w http.ResponseWriter, r *http.Request)` 的形式。
- 请求和响应结构体放在具体 action 文件附近，参考 `internal/auth/register.go`。

## 2. 注册路由

所有需要要被注册的路由要实现 RegisterRoutes：

```go
func (h *XxxHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/xxx", h.createXxx)
	mux.HandleFunc("GET /v1/xxx/{id}", h.getXxx)
}
```

`AuthHandler` 示例：

```go
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
}
```

## 3. 在 Server 中持有 Handler

在 `internal/server/server.go` 的 `Server` 结构体里增加字段：

```go
type Server struct {
	authHandler *auth.AuthHandler
	xxxHandler  *xxx.XxxHandler
}
```

在 `New` 中创建依赖和 handler：

```go
func New(dbPool *pgxpool.Pool, rdb *redis.Client) *Server {
	userStore := auth.NewPGUserStore(dbPool)
	ssoStore := &sso.RedisSSOStore{Rds: rdb}

	xxxStore := xxx.NewPGXxxStore(dbPool)

	return &Server{
		authHandler: auth.NewAuthHandler(userStore, ssoStore),
		xxxHandler:  xxx.NewXxxHandler(xxxStore),
	}
}
```

## 4. 在 server.go 下的 routes 调用前面写的 RegisterRoutes

在 `routes()` 中调用 handler 的 `RegisterRoutes`：

```go
func (srv *Server) routes() http.Handler {
	mux := http.NewServeMux()

	srv.authHandler.RegisterRoutes(mux)
	srv.xxxHandler.RegisterRoutes(mux)

	return mux
}
```
routes() 只负责注册路由，不做任何中间件处理，这里的 Hnadler 会返回给 `GetHandler()` 统一套 middleware，而 `GetHandler()` 是最终包装好的 `http.Handler` 被传给 http 服务器。

```go
func (srv *Server) GetHandler() http.Handler {
	handler := srv.routes()
	handler = middleware.Logging(handler)
	handler = middleware.PanicRecovery(handler)

	return handler
}
```