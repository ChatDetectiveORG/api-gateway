package config

import (
	"strings"
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	sharedTelegram "github.com/ChatDetectiveORG/shared/telegram"

	"github.com/spf13/viper"
)

type Config struct {
	RuntimeConfig        *RuntimeConfig
	RabbitMQConfig       *RabbitMQConfig
	PostgresConfig       *PostgresConfig
	RedisConfig          *RedisConfig
	TeleAPIWebhookConfig *TeleAPIWebhookConfig
}

type RuntimeConfig struct {
	NumRoutingGorutines int
}

type RabbitMQConfig struct {
	URL string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	Database int

	MaxIdle     int
	MaxActive   int
	IdleTimeout time.Duration
	Wait        bool

	ConnectionTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
}

type TeleAPIWebhookConfig struct {
	URL    string
	Port   string
	Token  string
	Secret string
}

// Fetches config from environment variables
func FetchConfig() (*Config, *e.ErrorInfo) {
	viper.AutomaticEnv()
	viper.SetDefault("TELEGRAM_BOT_WEBHOOK_PORT", "6002")
	viper.SetDefault("NUM_ROUTING_GOROUTINES", 4)

	config := &Config{
		TeleAPIWebhookConfig: &TeleAPIWebhookConfig{
			URL:    viper.GetString("TELEGRAM_BOT_PUBLIC_URL"),
			Port:   viper.GetString("TELEGRAM_BOT_WEBHOOK_PORT"),
			Token:  viper.GetString("TELEGRAM_BOT_TOKEN"),
			Secret: viper.GetString(sharedTelegram.WebhookSecretEnv),
		},
		PostgresConfig: &PostgresConfig{
			Host:     viper.GetString("POSTGRES_HOST"),
			Port:     viper.GetString("POSTGRES_PORT"),
			User:     viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			Database: viper.GetString("POSTGRES_DB"),
		},
		RedisConfig: &RedisConfig{
			Host:              viper.GetString("REDIS_HOST"),
			Port:              viper.GetString("REDIS_PORT"),
			Password:          viper.GetString("REDIS_PASSWORD"),
			Database:          viper.GetInt("REDIS_DB"),
			MaxIdle:           viper.GetInt("REDIS_MAX_IDLE"),
			MaxActive:         viper.GetInt("REDIS_MAX_ACTIVE"),
			IdleTimeout:       viper.GetDuration("REDIS_IDLE_TIMEOUT"),
			Wait:              viper.GetBool("REDIS_WAIT"),
			ConnectionTimeout: viper.GetDuration("REDIS_CONNECTION_TIMEOUT"),
			ReadTimeout:       viper.GetDuration("REDIS_READ_TIMEOUT"),
			WriteTimeout:      viper.GetDuration("REDIS_WRITE_TIMEOUT"),
		},
		RabbitMQConfig: &RabbitMQConfig{
			URL: viper.GetString("RABBITMQ_URL"),
		},
		RuntimeConfig: &RuntimeConfig{
			NumRoutingGorutines: viper.GetInt("NUM_ROUTING_GOROUTINES"),
		},
	}

	if err := validateConfig(config); e.IsNonNil(err) {
		return nil, err
	}

	return config, e.Nil()
}

// validateConfig fails fast on missing required configuration so the gateway never
// starts half-configured (e.g. webhook without authentication).
func validateConfig(config *Config) *e.ErrorInfo {
	var missing []string

	if config.TeleAPIWebhookConfig.Token == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if config.TeleAPIWebhookConfig.URL == "" {
		missing = append(missing, "TELEGRAM_BOT_PUBLIC_URL")
	}
	if config.TeleAPIWebhookConfig.Secret == "" {
		missing = append(missing, sharedTelegram.WebhookSecretEnv)
	}
	if config.RabbitMQConfig.URL == "" {
		missing = append(missing, "RABBITMQ_URL")
	}
	if config.PostgresConfig.Host == "" {
		missing = append(missing, "POSTGRES_HOST")
	}

	if len(missing) > 0 {
		return e.NewError(
			"missing required environment variables: "+strings.Join(missing, ", "),
			"invalid api-gateway configuration",
		).WithSeverity(e.Critical)
	}
	if config.RuntimeConfig.NumRoutingGorutines <= 0 {
		return e.NewError(
			"NUM_ROUTING_GOROUTINES must be positive",
			"invalid api-gateway configuration",
		).WithSeverity(e.Critical)
	}
	return e.Nil()
}
