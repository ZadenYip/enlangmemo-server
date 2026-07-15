package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	postgresImage = "postgres:18.4-alpine"
	redisImage    = "redis:8.8-alpine"

	testDBName     = "enlangmemo"
	testDBUser     = "postgres"
	testDBPassword = "enlangmemo"
)

type testEnv struct {
	pgContainer  *postgres.PostgresContainer
	rdsContainer *tcredis.RedisContainer

	dbURL  string
	rdsURL string

	dbPool    *pgxpool.Pool
	rdsClient *redis.Client
}

// 初始化测试环境，启动 Testcontainers、创建数据库快照、连接池等。
func initTestEnv(ctx context.Context) (*testEnv, error) {
	env := &testEnv{}

	pg, err := postgres.Run(
		ctx,
		postgresImage,
		postgres.WithDatabase(testDBName),
		postgres.WithUsername(testDBUser),
		postgres.WithPassword(testDBPassword),
		// init 建表
		postgres.WithOrderedInitScripts(filepath.Join("..", "..", "..", "docker", "pg-init-scripts", "000_schema.sql")),
		postgres.BasicWaitStrategies(),
		postgres.WithSQLDriver("pgx"),
	)
	if err != nil {
		env.close(ctx)
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}
	env.pgContainer = pg

	redisContainer, err := tcredis.Run(
		ctx,
		redisImage,
		// 集成测试不依赖 Redis 持久化，只保留普通运行日志即可。
		tcredis.WithLogLevel(tcredis.LogLevelNotice),
	)
	if err != nil {
		env.close(ctx)
		return nil, fmt.Errorf("failed to start redis container: %w", err)
	}
	env.rdsContainer = redisContainer

	if err := env.configure(ctx); err != nil {
		env.close(ctx)
		return nil, err
	}

	// 快照记录的是执行完 00_schema.sql 后、测试写入数据前的干净数据库。
	if err := env.pgContainer.Snapshot(ctx); err != nil {
		env.close(ctx)
		return nil, err
	}

	// 连接池放在 Snapshot 之后创建，避免 Restore 时还有旧连接占用目标库。
	if err := env.openClients(ctx); err != nil {
		env.close(ctx)
		return nil, err
	}

	return env, nil
}

// 启动完 Testcontainers 后，把相关连接配置写进 TestEnv 和环境变量
func (e *testEnv) configure(ctx context.Context) error {
	databaseURL, err := e.pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return err
	}
	e.dbURL = databaseURL

	redisURL, err := e.rdsContainer.ConnectionString(ctx)
	if err != nil {
		return err
	}
	e.rdsURL = redisURL

	// 应用配置默认从环境变量读取，这里让被测代码拿到 Testcontainers 的动态端口。
	if err := os.Setenv("DATABASE_URL", databaseURL); err != nil {
		return err
	}
	if err := os.Setenv("REDIS_URL", redisURL); err != nil {
		return err
	}

	return nil
}

// 创建连接 PG 和 Redis 的客户端，供测试用例使用。
func (e *testEnv) openClients(ctx context.Context) error {
	dbPool, err := pgxpool.New(ctx, e.dbURL)
	if err != nil {
		return err
	}
	e.dbPool = dbPool

	opts, err := redis.ParseURL(e.rdsURL)
	if err != nil {
		e.dbPool.Close()
		e.dbPool = nil
		return err
	}
	e.rdsClient = redis.NewClient(opts)

	return nil
}

func (e *testEnv) reset(ctx context.Context) error {
	if e.dbPool != nil {
		// Restore 会 drop/recreate 数据库，必须先释放连接池里的数据库连接。
		e.dbPool.Close()
		e.dbPool = nil
	}

	if e.rdsClient != nil {
		_ = e.rdsClient.Close()
		e.rdsClient = nil
	}

	// 还原到 initTestEnv 里创建的 snapshot，也就是只有 schema、没有测试数据。
	if err := e.pgContainer.Restore(ctx); err != nil {
		return err
	}

	// Restore 后旧连接不可复用，重新创建应用侧客户端。
	if err := e.openClients(ctx); err != nil {
		return err
	}

	if e.rdsClient != nil {
		// Redis module 没有类似 Postgres snapshot 的能力，测试间直接清空当前 DB。
		if err := e.rdsClient.FlushDB(ctx).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (e *testEnv) close(_ context.Context) {
	if e == nil {
		return
	}

	if e.dbPool != nil {
		e.dbPool.Close()
	}

	if e.rdsClient != nil {
		_ = e.rdsClient.Close()
	}

	if e.rdsContainer != nil {
		_ = testcontainers.TerminateContainer(e.rdsContainer)
	}

	if e.pgContainer != nil {
		_ = testcontainers.TerminateContainer(e.pgContainer)
	}
}
