package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"greenlight.zzh.net/internal/data"
)

// Declare a string containing the application version number. Later in the book we'll
// generate this automatically at build time, but for now we'll just store the version
// number as a hard-coded global constant.
const version = "1.0.0"

// Define a config struct to hold all the configuration settings for our application.
type config struct {
    port int
    env  string
    db   struct {
        dsn          string
        maxOpenConns int
        maxIdleTime  time.Duration
    }
    limiter struct {
        rps     float64
        burst   int
        enabled bool
    }
}

// Define an application struct to hold the dependencies for our HTTP handlers, helpers,
// and middleware.
type application struct {
    config config
    logger *slog.Logger
    models data.Models
}

func main() {
    var cfg config

    flag.IntVar(&cfg.port, "port", 4000, "API server port")
    flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

    flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")

    flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
    flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

    flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
    flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
    flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

    flag.Parse()

    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    dcp, err := createDBConnPool(cfg)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }

    defer dcp.Close()

    logger.Info("database connection pool established")

    app := &application{
        config: cfg,
        logger: logger,
        models: data.NewModels(dcp),
    }

    err = app.serve()
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
}

func createDBConnPool(cfg config) (*pgxpool.Pool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    config, err := pgxpool.ParseConfig(cfg.db.dsn)
    if err != nil {
        return nil, err
    }

    config.MaxConns = int32(cfg.db.maxOpenConns)
    config.MaxConnIdleTime = cfg.db.maxIdleTime

    p, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, err
    }

    err = p.Ping(ctx)
    if err != nil {
        p.Close()
        return nil, err
    }

    return p, nil
}
