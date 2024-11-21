package data

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolWrapper wraps a *pgxpool.Pool.
type PoolWrapper struct {
    Pool *pgxpool.Pool
}

// CreatePool creates a *pgxpool.Pool and assigns it to the wrapper's Pool field.
func (pw *PoolWrapper) CreatePool(connString string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    p, err := pgxpool.New(ctx, connString)
    if err != nil {
        return err
    }

    err = p.Ping(ctx)
    if err != nil {
        p.Close()
        return err
    }

    pw.Pool = p

    return nil
}
