package sql

import (
	"context"
	"database/sql"
)

type PushStmtCache struct {
	tx    *sql.Tx
	cache []*sql.Stmt
}

// NewPushStmtCache 创建单个事务用的 StmtCache，用于缓存 Push SQL 语句的预编译结果
func NewPushStmtCache(ctx context.Context, tx *sql.Tx) *PushStmtCache {
	return &PushStmtCache{
		tx:    tx,
		cache: make([]*sql.Stmt, len(pushOpToSQL)),
	}
}

func (s *PushStmtCache) GetPush(ctx context.Context, op PushOp) (*sql.Stmt, error) {
	stmt := s.cache[op]
	if stmt == nil {
		stmt, err := s.tx.PrepareContext(ctx, pushOpToSQL[op])
		if err != nil {
			return nil, err
		}
		s.cache[op] = stmt
	}
	return s.cache[op], nil
}

func (s *PushStmtCache) Close() {
	for _, stmt := range s.cache {
		if stmt != nil {
			stmt.Close()
		}
	}
}
