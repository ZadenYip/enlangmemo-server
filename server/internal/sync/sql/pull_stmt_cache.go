package sql

import (
	"context"
	"database/sql"
	"fmt"
)

type PullStmtCache struct {
	cache []*sql.Stmt
}

// NewPullStmtCache 创建一个全局用的 StmtCache，用于缓存 Pull SQL 语句的预编译结果
func NewPullStmtCache(ctx context.Context, db *sql.DB) (*PullStmtCache, error) {
	cache := make([]*sql.Stmt, len(pullOpToSQL))
	for i, op := range pullOpToSQL {
		stmt, err := db.PrepareContext(ctx, op)
		if err != nil {
			for _, prepared := range cache {
				if prepared != nil {
					prepared.Close()
				}
			}
			return nil, err
		}
		cache[i] = stmt
	}

	return &PullStmtCache{
		cache: cache,
	}, nil
}

func (s *PullStmtCache) GetPull(ctx context.Context, op PullOp) *sql.Stmt {
	return s.cache[op]
}

func (s *PullStmtCache) Close() error {
	var err error = nil
	for _, stmt := range s.cache {
		if stmt != nil {
			if closeErr := stmt.Close(); closeErr != nil {
				err = fmt.Errorf("failed to close statement: %w", closeErr)
			}
		}
	}
	return err
}
