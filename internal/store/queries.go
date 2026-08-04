package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("not found")

// Lax struct mapping: a SELECT may omit columns the struct declares (e.g. Score).
func selectAll[T any](ctx context.Context, s *Store, sql string, args ...any) ([]T, error) {
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
}

func selectOne[T any](ctx context.Context, s *Store, sql string, args ...any) (*T, error) {
	list, err := selectAll[T](ctx, s, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}
