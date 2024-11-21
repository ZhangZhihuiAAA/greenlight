package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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
    // Field read from command line
    serverAddress string
    env           string

    // Fields loaded from dynamic.env
    limiter *config.LimiterConfig

    // Fields loaded from dynamic_db_secret.env
    dbConnString string

    // Fields loaded from dynamic_smtp_secret.env
    smtp *config.SMTPConfig
}

// application struct holds the dependencies for our HTTP handlers, helpers, and middleware.
type application struct {
    config      appConfig
    logger      *slog.Logger
    models      data.Models
    emailSender *mail.EmailSender
    wg          sync.WaitGroup
}

func main() {
    var cfg appConfig

    // Read static configuration from command line.
    flag.StringVar(&cfg.serverAddress, "server-address", ":4000", "The server address of this application.")
    flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

    var configPath string
    // Read the location of config files for dynamic configuration from command line.
    flag.StringVar(&configPath, "config-path", "internal/config", "The directory that contains configuration files.")

    // Parse command line parameters.
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

    cfg.limiter = &config.LimiterConfig{
        Rps:     cfgDynamic.LimiterRps,
        Burst:   cfgDynamic.LimiterBurst,
        Enabled: cfgDynamic.LimiterEnabled,
    }
    cfg.dbConnString = fmt.Sprintf(
        "postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d&pool_max_conn_idle_time=%s",
        cfgDynamic.DBUsername, cfgDynamic.DBPassword, cfgDynamic.DBServer, cfgDynamic.DBPort, cfgDynamic.DBName,
        cfgDynamic.DBSSLMode, cfgDynamic.DBPoolMaxConns, cfgDynamic.DBPoolMaxConnIdleTime,
    )
    cfg.smtp = &config.SMTPConfig{
        Username:      cfgDynamic.SMTPUsername,
        Password:      cfgDynamic.SMTPPassword,
        AuthAddress:   cfgDynamic.SMTPAuthAddress,
        ServerAddress: cfgDynamic.SMTPServerAddress,
    }

    // Create a database connection pool wrapper.
    var poolWrapper data.PoolWrapper
    err = poolWrapper.CreatePool(cfg.dbConnString)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
    defer poolWrapper.Pool.Close()
    logger.Info("database connection pool established")

    // Create the application instance.
    app := &application{
        config:      cfg,
        logger:      logger,
        models:      data.NewModels(&poolWrapper),
        emailSender: &mail.EmailSender{SMTPCfg: cfg.smtp},
    }

    // Watch and reload dynamic.env config file.
    go func() {
        viperDynamic.OnConfigChange(func(in fsnotify.Event) {
            // A change in the config file can cause two 'write' events.
            // Only need to respond once. We respond to the first one.
            if time.Since(cfgDynamic.LoadTime) > time.Duration(100*time.Millisecond) {
                logger.Info("configuration change detected", "filename", in.Name, "operation", in.Op)

                // Reload the config file if any change is detected.
                err := config.LoadConfig(viperDynamic, configPath, "env", "dynamic", &cfgDynamic)
                if err != nil {
                    logger.Error(err.Error())
                    os.Exit(1)
                }

                cfg.limiter.Rps = cfgDynamic.LimiterRps
                cfg.limiter.Burst = cfgDynamic.LimiterBurst
                cfg.limiter.Enabled = cfgDynamic.LimiterEnabled
            }
        })
        viperDynamic.WatchConfig()
    }()

    // Watch and reload dynamic_db_secret.env config file.
    go func() {
        viperDynamicDB.OnConfigChange(func(in fsnotify.Event) {
            if time.Since(cfgDynamic.LoadTime) > time.Duration(100*time.Millisecond) {
                logger.Info("configuration change detected", "filename", in.Name, "operation", in.Op)

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

                // Close the old database connection pool and create a new one.
                poolWrapper.Pool.Close()
                err = poolWrapper.CreatePool(cfg.dbConnString)
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
                logger.Info("configuration change detected", "filename", in.Name, "operation", in.Op)

                err := config.LoadConfig(viperDynamicSMTP, configPath, "env", "dynamic_smtp_secret", &cfgDynamic)
                if err != nil {
                    logger.Error(err.Error())
                    os.Exit(1)
                }

                cfg.smtp.Username = cfgDynamic.SMTPUsername
                cfg.smtp.Password = cfgDynamic.SMTPPassword
                cfg.smtp.AuthAddress = cfgDynamic.SMTPAuthAddress
                cfg.smtp.ServerAddress = cfgDynamic.SMTPServerAddress
            }
        })
        viperDynamicSMTP.WatchConfig()
    }()

    err = app.serve()
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
}
