package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolWrapper wraps a *pgxpool.Pool.
type PoolWrapper struct {
    Pool *pgxpool.Pool `json:"-"`
    Stat struct {
        PoolSerialNumber        int32         `json:"pool_serial_number"`      // serial number of the pool in use
        AcquireCount            int64         `json:"AcquireCount"`            // cumulative count of successful acquires from the pool
        AcquireDuration         time.Duration `json:"AcquireDuration"`         // total duration of all successful acquires from the pool
        AcquiredConns           int32         `json:"AcquiredConns"`           // number of currently acquired connections in the pool
        CanceledAcquireCount    int64         `json:"CanceledAcquireCount"`    // cumulative count of acquires from the pool that were canceled by a context
        EmptyAcquireCount       int64         `json:"EmptyAcquireCount"`       // cumulative count of successful acquires from the pool that waited for a resource to be released or constructed because the pool was empty
        IdleConns               int32         `json:"IdleConns"`               // number of currently idle conns in the pool
        MaxConns                int32         `json:"MaxConns"`                // maximum size of the pool
        TotalConns              int32         `json:"TotalConns"`              // total number of resources currently in the pool, the sum of ConstructingConns, AcquiredConns, and IdleConns
        NewConnsCount           int64         `json:"NewConnsCount"`           // cumulative count of new connections opened
        MaxLifetimeDestroyCount int64         `json:"MaxLifetimeDestroyCount"` // cumulative count of connections destroyed because they exceeded MaxConnLifetime
        MaxIdleDestroyCount     int64         `json:"MaxIdleDestroyCount"`     // cumulative count of connections destroyed because they exceeded MaxConnIdleTime
    }
}

// Implement the MarshalJSON method on PoolWrapper struct so that it satisfies the jons.Marshaler interface.
func (pw *PoolWrapper) MarshalJSON() ([]byte, error) {
    return json.Marshal(pw.Stat)
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
    pw.Stat.PoolSerialNumber = pw.Stat.PoolSerialNumber + 1
    pw.Stat.AcquireCount = p.Stat().AcquireCount()
    pw.Stat.AcquireDuration = p.Stat().AcquireDuration()
    pw.Stat.AcquiredConns = p.Stat().AcquiredConns()
    pw.Stat.CanceledAcquireCount = p.Stat().CanceledAcquireCount()
    pw.Stat.EmptyAcquireCount = p.Stat().EmptyAcquireCount()
    pw.Stat.IdleConns = p.Stat().IdleConns()
    pw.Stat.MaxConns = p.Stat().MaxConns()
    pw.Stat.TotalConns = p.Stat().TotalConns()
    pw.Stat.NewConnsCount = p.Stat().NewConnsCount()
    pw.Stat.MaxLifetimeDestroyCount = p.Stat().MaxLifetimeDestroyCount()
    pw.Stat.MaxIdleDestroyCount = p.Stat().MaxIdleDestroyCount()

    return nil
}
