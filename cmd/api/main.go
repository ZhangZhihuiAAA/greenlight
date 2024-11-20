package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
	"greenlight.zzh.net/internal/config"
	"greenlight.zzh.net/internal/data"
	"greenlight.zzh.net/internal/mail"
)

// Declare a string containing the application version number. Later in the book we'll
// generate this automatically at build time, but for now we'll just store the version
// number as a hard-coded global constant.
const version = "1.0.0"

type appConfig struct {
    serverAddress string
    env           string
    dbConnString  string
    limiter       struct {
        rps     float64
        burst   int
        enabled bool
    }
    emailSender mail.EmailSender
}

// Define an application struct to hold the dependencies for our HTTP handlers, helpers,
// and middleware.
type application struct {
    config appConfig
    logger *slog.Logger
    models data.Models
}

func main() {
    var (
        configPath    string
        serverAddress string
        env           string
    )

    flag.StringVar(&configPath, "config-path", "internal/config", "The directory that contains configuration files.")
    flag.StringVar(&serverAddress, "server-address", ":4000", "The server address of this application.")
    flag.StringVar(&env, "env", "development", "Environment (development|staging|production)")
    flag.Parse()

    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    var cfgDynamic config.Config

    // Load dynamic configuration.
    viperDynamic := viper.New()
    err := config.LoadConfig(viperDynamic, configPath, "env", "dynamic", &cfgDynamic)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }

    // Load dynamic DB configuration.
    viperDynamicDB := viper.New()
    err = config.LoadConfig(viperDynamicDB, configPath, "env", "dynamic_db_secret", &cfgDynamic)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }

    // Load dynamic SMTP configuration.
    viperDynamicSMTP := viper.New()
    err = config.LoadConfig(viperDynamicSMTP, configPath, "env", "dynamic_smtp_secret", &cfgDynamic)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }

    // Create an appConfig instance.
    cfg := appConfig{
        serverAddress: serverAddress,
        env:           env,
        dbConnString: fmt.Sprintf(
            "postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d&pool_max_conn_idle_time=%s",
            cfgDynamic.DBUsername, cfgDynamic.DBPassword, cfgDynamic.DBServer, cfgDynamic.DBPort, cfgDynamic.DBName,
            cfgDynamic.DBSSLMode, cfgDynamic.DBPoolMaxConns, cfgDynamic.DBPoolMaxConnIdleTime,
        ),
        limiter: struct {
            rps     float64
            burst   int
            enabled bool
        }{
            cfgDynamic.LimiterRPS,
            cfgDynamic.LimiterBurst,
            cfgDynamic.LimiterEnabled,
        },
        emailSender: mail.EmailSender{
            Username:          cfgDynamic.SMTPUername,
            Password:          cfgDynamic.SMTPPassword,
            SMTPAuthAddress:   cfgDynamic.SMTPAuthAddress,
            SMTPServerAddress: cfgDynamic.SMTPServerAddress,
        },
    }

    // Create a DB connection pool.
    dbConnPool, err := createDBConnPool(cfg.dbConnString)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
    defer dbConnPool.Close()
    logger.Info("database connection pool established")

    // Watch and reload dynamic.env config file.
    go func() {
        viperDynamic.OnConfigChange(func(in fsnotify.Event) {
            // A change in the config file can cause two 'write' events.
            // Only need to respond once. We respond to the first one.
            if time.Since(cfgDynamic.LoadTime) > time.Duration(100*time.Millisecond) {
                // Reload the config file if any change is detected.
                err := config.LoadConfig(viperDynamic, configPath, "env", "dynamic", &cfgDynamic)
                if err != nil {
                    logger.Error(err.Error())
                    os.Exit(1)
                }

                cfg.limiter = struct {
                    rps     float64
                    burst   int
                    enabled bool
                }{
                    cfgDynamic.LimiterRPS,
                    cfgDynamic.LimiterBurst,
                    cfgDynamic.LimiterEnabled,
                }
            }
        })
        viperDynamic.WatchConfig()
    }()

    // Watch and reload dynamic_db_secret.env config file.
    go func() {
        viperDynamicDB.OnConfigChange(func(in fsnotify.Event) {
            if time.Since(cfgDynamic.LoadTime) > time.Duration(100*time.Millisecond) {
                err := config.LoadConfig(viperDynamicDB, configPath, "env", "dynamic_db_secret", &cfgDynamic)
                if err != nil {
                    logger.Error(err.Error())
                    os.Exit(1)
                }

                cfg.dbConnString = fmt.Sprintf(
                    "postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d&pool_max_conn_idle_time=%s",
                    cfgDynamic.DBUsername, cfgDynamic.DBPassword, cfgDynamic.DBServer, cfgDynamic.DBPort, cfgDynamic.DBName,
                    cfgDynamic.DBSSLMode, cfgDynamic.DBPoolMaxConns, cfgDynamic.DBPoolMaxConnIdleTime,
                )

                // Close and recreate the dbConnPool.
                dbConnPool.Close()
                dbConnPool, err = createDBConnPool(cfg.dbConnString)
                if err != nil {
                    logger.Error(err.Error())
                    os.Exit(1)
                }
            }
        })
        viperDynamicDB.WatchConfig()
    }()

    // Watch and reload dynamic_smtp_secret.env config file.
    go func() {
        viperDynamicSMTP.OnConfigChange(func(in fsnotify.Event) {
            if time.Since(cfgDynamic.LoadTime) > time.Duration(100*time.Millisecond) {
                err := config.LoadConfig(viperDynamicSMTP, configPath, "env", "dynamic_smtp_secret", &cfgDynamic)
                if err != nil {
                    logger.Error(err.Error())
                    os.Exit(1)
                }

                cfg.emailSender = mail.EmailSender{
                    Username:          cfgDynamic.SMTPUername,
                    Password:          cfgDynamic.SMTPPassword,
                    SMTPAuthAddress:   cfgDynamic.SMTPAuthAddress,
                    SMTPServerAddress: cfgDynamic.SMTPServerAddress,
                }
            }
        })
        viperDynamicSMTP.WatchConfig()
    }()

    app := &application{
        config: cfg,
        logger: logger,
        models: data.NewModels(dbConnPool),
    }

    err = app.serve()
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
}

func createDBConnPool(connString string) (*pgxpool.Pool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    p, err := pgxpool.New(ctx, connString)
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
