package sql

import (
	"context"
	"database/sql"
)

type StmtCache struct {
	tx    *sql.Tx
	cache []*sql.Stmt
}

func NewPushStmtCache(ctx context.Context, tx *sql.Tx) *StmtCache {
	return &StmtCache{
		tx:    tx,
		cache: make([]*sql.Stmt, len(pushOpToSQL)),
	}
}

func NewPullStmtCache(ctx context.Context, tx *sql.Tx) *StmtCache {
	return &StmtCache{
		tx:    tx,
		cache: make([]*sql.Stmt, len(pullOpToSQL)),
	}
}

func (s *StmtCache) GetPush(ctx context.Context, op PushOp) (*sql.Stmt, error) {
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

func (s *StmtCache) GetPull(ctx context.Context, op PullOp) (*sql.Stmt, error) {
	stmt := s.cache[op]
	if stmt == nil {
		stmt, err := s.tx.PrepareContext(ctx, pullOpToSQL[op])
		if err != nil {
			return nil, err
		}
		s.cache[op] = stmt
	}
	return s.cache[op], nil
}

func (s *StmtCache) Close() {
	for _, stmt := range s.cache {
		if stmt != nil {
			stmt.Close()
		}
	}
}
