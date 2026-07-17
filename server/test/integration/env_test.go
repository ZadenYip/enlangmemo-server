package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	mysqlImage = "mysql:8.4"
	redisImage = "redis:8.8-alpine"

	testDBName     = "enlangmemo"
	testDBUser     = "enlangmemo"
	testDBPassword = "enlangmemo"
)

type testEnv struct {
	mysqlContainer *mysql.MySQLContainer
	rdsContainer   *tcredis.RedisContainer

	dbURL  string
	rdsURL string

	db        *sql.DB
	rdsClient *redis.Client
}

// 初始化测试环境，启动 Testcontainers、创建连接池等。
func initTestEnv(ctx context.Context) (*testEnv, error) {
	env := &testEnv{}

	mysqlContainer, err := mysql.Run(
		ctx,
		mysqlImage,
		mysql.WithDatabase(testDBName),
		mysql.WithUsername(testDBUser),
		mysql.WithPassword(testDBPassword),
		// init 建表
		mysql.WithScripts(filepath.Join("..", "..", "..", "docker", "mysql-init-scripts", "000_schema.sql")),
	)
	if err != nil {
		env.close(ctx)
		return nil, fmt.Errorf("failed to start mysql container: %w", err)
	}
	env.mysqlContainer = mysqlContainer

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

	if err := env.openClients(ctx); err != nil {
		env.close(ctx)
		return nil, err
	}

	return env, nil
}

// 启动完 Testcontainers 后，把相关连接配置写进 TestEnv 和环境变量。
func (e *testEnv) configure(ctx context.Context) error {
	databaseURL, err := e.mysqlContainer.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
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

// 创建连接 MySQL 和 Redis 的客户端，供测试用例使用。
func (e *testEnv) openClients(ctx context.Context) error {
	db, err := sql.Open("mysql", e.dbURL)
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return err
	}
	e.db = db

	opts, err := redis.ParseURL(e.rdsURL)
	if err != nil {
		_ = e.db.Close()
		e.db = nil
		return err
	}
	e.rdsClient = redis.NewClient(opts)

	return nil
}

func (e *testEnv) reset(ctx context.Context) error {
	if e.db != nil {
		// MySQL module 没有类似 Postgres snapshot 的能力，测试间还原业务表即可。
		if _, err := e.db.ExecContext(ctx, `
			SET FOREIGN_KEY_CHECKS = 0;
			TRUNCATE TABLE oauth_clients;
			TRUNCATE TABLE users;
			SET FOREIGN_KEY_CHECKS = 1;
		`); err != nil {
			return err
		}
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

	if e.db != nil {
		_ = e.db.Close()
	}

	if e.rdsClient != nil {
		_ = e.rdsClient.Close()
	}

	if e.rdsContainer != nil {
		_ = testcontainers.TerminateContainer(e.rdsContainer)
	}

	if e.mysqlContainer != nil {
		_ = testcontainers.TerminateContainer(e.mysqlContainer)
	}
}
