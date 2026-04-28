package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Keycloak KeycloakConfig `mapstructure:"keycloak"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Email    EmailConfig    `mapstructure:"email"`
}

type AppConfig struct {
	Name             string   `mapstructure:"name"`
	Version          string   `mapstructure:"version"`
	Environment      string   `mapstructure:"environment"`
	Debug            bool     `mapstructure:"debug"`
	Port             int      `mapstructure:"port"`
	Host             string   `mapstructure:"host"`
	SwaggerHost      string   `mapstructure:"swagger_host"`
	ShutdownTimeout  string   `mapstructure:"shutdown_timeout"`
	ReadTimeout      string   `mapstructure:"read_timeout"`
	WriteTimeout     string   `mapstructure:"write_timeout"`
	IdleTimeout      string   `mapstructure:"idle_timeout"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
}

// ListenAddr returns host:port for http.Server (defaults host 0.0.0.0).
func (a AppConfig) ListenAddr() string {
	host := strings.TrimSpace(a.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	port := a.Port
	if port <= 0 {
		port = 8085
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func (a AppConfig) GetShutdownTimeout() time.Duration {
	d, err := time.ParseDuration(a.ShutdownTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func (a AppConfig) GetReadTimeout() time.Duration {
	d, err := time.ParseDuration(a.ReadTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func (a AppConfig) GetWriteTimeout() time.Duration {
	d, err := time.ParseDuration(a.WriteTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func (a AppConfig) GetIdleTimeout() time.Duration {
	d, err := time.ParseDuration(a.IdleTimeout)
	if err != nil {
		return 120 * time.Second
	}
	return d
}

type PostgresConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Schema         string `mapstructure:"schema"`
	UserReader     string `mapstructure:"user_reader"`
	PasswordReader string `mapstructure:"password_reader"`
	UserWriter     string `mapstructure:"user_writer"`
	PasswordWriter string `mapstructure:"password_writer"`
	Database       string `mapstructure:"database"`
	SSLMode        string `mapstructure:"sslmode"`
	MaxIdleConns   int32  `mapstructure:"max_idle_conns"`
	MaxActiveConns int32  `mapstructure:"max_active_conns"`
	MaxConnTimeout string `mapstructure:"max_conn_timeout"`
	DebugLog       bool   `mapstructure:"debug_log"`
}

func (p PostgresConfig) WriterURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.UserWriter, p.PasswordWriter, p.Host, p.Port, p.Database, p.SSLMode)
}

func (p PostgresConfig) ReaderURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.UserReader, p.PasswordReader, p.Host, p.Port, p.Database, p.SSLMode)
}

type RedisConfig struct {
	Host              string         `mapstructure:"host"`
	Port              int            `mapstructure:"port"`
	Database          int            `mapstructure:"database"`
	RateLimitDatabase int            `mapstructure:"rate_limit_database"`
	TTL               string         `mapstructure:"ttl"`
	PoolSize          int            `mapstructure:"pool_size"`
	Pass              string         `mapstructure:"pass"`
	UserName          string         `mapstructure:"user_name"`
	WriteTimeout      string         `mapstructure:"write_timeout"`
	ReadTimeout       string         `mapstructure:"read_timeout"`
	DialTimeout       string         `mapstructure:"dial_timeout"`
	TLSConfig         RedisTLSConfig `mapstructure:"tls_config"`
}

type RedisTLSConfig struct {
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

type KafkaConfig struct {
	BrokerList             string `mapstructure:"broker_list"`
	Enable                 bool   `mapstructure:"enable"`
	TLSEnable              bool   `mapstructure:"tls_enable"`
	Partition              int    `mapstructure:"partition"`
	Partitioner            string `mapstructure:"partitioner"`
	SASLProducerUsername   string `mapstructure:"sasl_producer_username"`
	SASLProducerPassword   string `mapstructure:"sasl_producer_password"`
	SASLConsumerUsername   string `mapstructure:"sasl_consumer_username"`
	SASLConsumerPassword   string `mapstructure:"sasl_consumer_password"`
	UserActivatedTopicName string `mapstructure:"user_activated_topic_name"`
	ClientID               string `mapstructure:"client_id"`
	ConsumerGroupID        string `mapstructure:"consumer_group_id"`
}

func (k KafkaConfig) GetBrokers() []string {
	if k.BrokerList == "" {
		return nil
	}
	brokers := strings.Split(k.BrokerList, ",")
	result := make([]string, 0, len(brokers))
	for _, b := range brokers {
		if trimmed := strings.TrimSpace(b); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

type KeycloakConfig struct {
	BaseURL                string `mapstructure:"base_url"`
	Realm                  string `mapstructure:"realm"`
	ClientID               string `mapstructure:"client_id"`
	ClientSecret           string `mapstructure:"client_secret"`
	AdminUser              string `mapstructure:"admin_user"`
	AdminPass              string `mapstructure:"admin_pass"`
	RedirectURL            string `mapstructure:"redirect_url"`
	FrontendCallbackURL    string `mapstructure:"frontend_callback_url"`
}

type EmailConfig struct {
	SMTP      SMTPConfig      `mapstructure:"smtp"`
	Templates TemplatesConfig `mapstructure:"templates"`
}

type SMTPConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	FromEmail string `mapstructure:"from_email"`
	FromName  string `mapstructure:"from_name"`
}

type TemplatesConfig struct {
	Welcome       string `mapstructure:"welcome"`
	PasswordReset string `mapstructure:"password_reset"`
	Verification  string `mapstructure:"verification"`
}

type ObservabilityConfig struct {
	ServiceName    string `mapstructure:"service_name"`
	ServiceVersion string `mapstructure:"service_version"`
	Environment    string `mapstructure:"environment"`
	LogLevel       string `mapstructure:"log_level"`
	OTLPEndpoint   string `mapstructure:"otlp_endpoint"`
	OTLPInsecure   bool   `mapstructure:"otlp_insecure"`
}

func Load() (*Config, error) {
	v := viper.New()

	// YAML config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

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
