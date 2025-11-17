package config

import (
	"os"

	e "app/pkg/errors"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	TelegramClient TelegramClientConfig
	PostgresClient PostgresClientConfig
}

type TelegramClientConfig struct {
	Token string
	WebhookURL string
	WebhookPort string
}

type PostgresClientConfig struct {
	Host string
	Port string
	User string
	Password string
	Database string
}

func Load() (*Config, *e.ErrorInfo) {
	if os.Getenv("DOCKER_TARGET") != "prod" {
		err := godotenv.Load()
		if err != nil {
			return &Config{}, e.FromError(err, "Env file is not present!").WithSeverity(e.Critical)
		}
	}

	viper.AutomaticEnv()

	config := &Config{
		TelegramClient: TelegramClientConfig{
			Token: viper.GetString("TELEGRAM_BOT_TOKEN"),
			WebhookURL: viper.GetString("TELEGRAM_BOT_PUBLIC_URL"),
			WebhookPort: viper.GetString("TELEGRAM_BOT_WEBHOOK_PORT"),
		},
		PostgresClient: PostgresClientConfig{
			Host: viper.GetString("POSTGRES_HOST"),
			Port: viper.GetString("POSTGRES_PORT"),
			User: viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			Database: viper.GetString("POSTGRES_DB"),
		},
	}

	return config, e.Nil()
}
