package gateway

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresUsers struct{ pool *pgxpool.Pool }

func NewPostgresUsers(pool *pgxpool.Pool) *PostgresUsers { return &PostgresUsers{pool: pool} }

func (users *PostgresUsers) Exists(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := users.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1::uuid)`, userID).Scan(&exists)
	return exists, err
}
