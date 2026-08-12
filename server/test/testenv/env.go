package testenv

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/server"
)

const (
	mysqlImage = "mysql:8.4"
	redisImage = "redis:8.8-alpine"

	testDBName     = "enlangmemo"
	testDBUser     = "enlangmemo"
	testDBPassword = "enlangmemo"
)

type Env struct {
	mysqlContainer *mysql.MySQLContainer
	rdsContainer   *tcredis.RedisContainer

	DBURL    string
	RedisURL string

	DB  *sql.DB
	RDB *redis.Client
}

type Suite struct {
	Env    *Env
	Server *httptest.Server
	Client *http.Client
}

func Run(m *testing.M, bind func(*Suite)) int {
	ctx := context.Background()

	suite, err := StartSuite(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "integration test setup failed: %v\n", err)
		return 1
	}
	bind(suite)

	code := m.Run()

	suite.Close(ctx)
	return code
}

func StartSuite(ctx context.Context) (*Suite, error) {
	env, err := Start(ctx)
	if err != nil {
		return nil, err
	}

	suite := &Suite{Env: env}
	suite.StartHTTPServer()

	return suite, nil
}

func (s *Suite) Reset(t *testing.T) {
	t.Helper()
	require.NoError(t, s.Env.Reset(t.Context()))
	s.StartHTTPServer()
}

func (s *Suite) Close(ctx context.Context) {
	if s == nil {
		return
	}
	if s.Server != nil {
		s.Server.Close()
	}
	if s.Env != nil {
		s.Env.Close(ctx)
	}
}

func (s *Suite) StartHTTPServer() {
	if s.Server != nil {
		s.Server.Close()
	}

	storeDeps := server.StoreDeps{
		DB:  s.Env.DB,
		Rdb: s.Env.RDB,
	}

	srv := server.New(storeDeps, logging.NewServerLog())
	s.Server = httptest.NewServer(srv.GetHandler())

	s.Client = s.Server.Client()
	s.Client.Transport = &TraceparentTransport{Base: s.Client.Transport}
}

type TraceparentTransport struct {
	Base http.RoundTripper
}

func (t *TraceparentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 添加 traceparent header 到测试客户端的请求中
	req.Header.Set("traceparent", "integration-test")
	return t.Base.RoundTrip(req)
}

func Start(ctx context.Context) (*Env, error) {
	env := &Env{}

	mysqlContainer, err := mysql.Run(
		ctx,
		mysqlImage,
		mysql.WithDatabase(testDBName),
		mysql.WithUsername(testDBUser),
		mysql.WithPassword(testDBPassword),
		mysql.WithScripts(schemaPath()),
	)
	if err != nil {
		env.Close(ctx)
		return nil, fmt.Errorf("failed to start mysql container: %w", err)
	}
	env.mysqlContainer = mysqlContainer

	redisContainer, err := tcredis.Run(
		ctx,
		redisImage,
		tcredis.WithLogLevel(tcredis.LogLevelNotice),
	)
	if err != nil {
		env.Close(ctx)
		return nil, fmt.Errorf("failed to start redis container: %w", err)
	}
	env.rdsContainer = redisContainer

	if err := env.configure(ctx); err != nil {
		env.Close(ctx)
		return nil, err
	}

	if err := env.openClients(ctx); err != nil {
		env.Close(ctx)
		return nil, err
	}

	return env, nil
}

func schemaPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("docker", "mysql-init-scripts", "000_schema.sql")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "docker", "mysql-init-scripts", "000_schema.sql")
}

func (e *Env) configure(ctx context.Context) error {
	databaseURL, err := e.mysqlContainer.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
	if err != nil {
		return err
	}
	e.DBURL = databaseURL

	redisURL, err := e.rdsContainer.ConnectionString(ctx)
	if err != nil {
		return err
	}
	e.RedisURL = redisURL

	if err := os.Setenv("DATABASE_URL", databaseURL); err != nil {
		return err
	}
	if err := os.Setenv("REDIS_URL", redisURL); err != nil {
		return err
	}

	return nil
}

func (e *Env) openClients(ctx context.Context) error {
	db, err := sql.Open("mysql", e.DBURL)
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return err
	}
	e.DB = db

	opts, err := redis.ParseURL(e.RedisURL)
	if err != nil {
		_ = e.DB.Close()
		e.DB = nil
		return err
	}
	e.RDB = redis.NewClient(opts)

	return nil
}

func (e *Env) Reset(ctx context.Context) error {
	if e.DB != nil {
		if _, err := e.DB.ExecContext(ctx, `
			SET FOREIGN_KEY_CHECKS = 0;
			TRUNCATE TABLE sync_units;
			TRUNCATE TABLE collections;
			TRUNCATE TABLE oauth_clients;
			TRUNCATE TABLE users;
			SET FOREIGN_KEY_CHECKS = 1;
		`); err != nil {
			return err
		}
	}

	if e.RDB != nil {
		if err := e.RDB.FlushDB(ctx).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Env) Close(_ context.Context) {
	if e == nil {
		return
	}

	if e.DB != nil {
		_ = e.DB.Close()
	}

	if e.RDB != nil {
		_ = e.RDB.Close()
	}

	if e.rdsContainer != nil {
		_ = testcontainers.TerminateContainer(e.rdsContainer)
	}

	if e.mysqlContainer != nil {
		_ = testcontainers.TerminateContainer(e.mysqlContainer)
	}
}
