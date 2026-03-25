package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	DB       DBConfig       `mapstructure:"db"`
	Keycloak KeycloakConfig `mapstructure:"keycloak"`
	Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
	Port            int    `mapstructure:"port"`
	ShutdownTimeout string `mapstructure:"shutdown_timeout"`
}

func (s ServerConfig) GetShutdownTimeout() time.Duration {
	d, err := time.ParseDuration(s.ShutdownTimeout)
	if err != nil {
		return 15 * time.Second
	}
	return d
}

type DBConfig struct {
	URL      string `mapstructure:"url"`
	MaxConns int32  `mapstructure:"max_conns"`
	MinConns int32  `mapstructure:"min_conns"`
}

type KeycloakConfig struct {
	BaseURL      string `mapstructure:"base_url"`
	Realm        string `mapstructure:"realm"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	AdminUser    string `mapstructure:"admin_user"`
	AdminPass    string `mapstructure:"admin_pass"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load() (*Config, error) {
	v := viper.New()

	// YAML config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// Defaults
	v.SetDefault("server.port", 8085)
	v.SetDefault("server.shutdown_timeout", "15s")
	v.SetDefault("db.max_conns", 25)
	v.SetDefault("db.min_conns", 5)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Read config.yml
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// Env vars override YAML (SERVER_PORT overrides server.port)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
