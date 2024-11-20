package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config stores configuration that can be dynamically reloaded at runtime.
type Config struct {
    DBUsername            string        `mapstructure:"DB_USERNAME"`
    DBPassword            string        `mapstructure:"DB_PASSWORD"`
    DBServer              string        `mapstructure:"DB_SERVER"`
    DBPort                int           `mapstructure:"DB_PORT"`
    DBName                string        `mapstructure:"DB_NAME"`
    DBSSLMode             string        `mapstructure:"DB_SSLMODE"`
    DBPoolMaxConns        int           `mapstructure:"DB_POOL_MAX_CONNS"`
    DBPoolMaxConnIdleTime time.Duration `mapstructure:"DB_POOL_MAX_CONN_IDLE_TIME"`
    LimiterRPS            float64       `mapstructure:"LIMITER_RPS"`
    LimiterBurst          int           `mapstructure:"LIMITER_BURST"`
    LimiterEnabled        bool          `mapstructure:"LIMITER_ENABLED"`
    SMTPUername           string        `mapstructure:"SMTP_USERNAME"`
    SMTPPassword          string        `mapstructure:"SMTP_PASSWORD"`
    SMTPAuthAddress       string        `mapstructure:"SMTP_AUTH_ADDRESS"`
    SMTPServerAddress     string        `mapstructure:"SMTP_SERVER_ADDRESS"`
    LoadTime              time.Time
}

// LoadConfig loads configuration from a config file to a Config instance.
func LoadConfig(v *viper.Viper, cfgPath, cfgType, cfgName string, cfg *Config) error {
    v.AddConfigPath(cfgPath)
    v.SetConfigType(cfgType)
    v.SetConfigName(cfgName)

    err := v.ReadInConfig()
    if err != nil {
        return err
    }

    err = v.Unmarshal(cfg)
    if err != nil {
        return err
    }

    cfg.LoadTime = time.Now()

    return nil
}
