package config

import "testing"

func validTestConfig() *Config {
	return &Config{
		TeleAPIWebhookConfig: &TeleAPIWebhookConfig{
			URL:    "https://bot.example.com/botTOKEN",
			Port:   "6002",
			Token:  "123:abc",
			Secret: "webhook-secret",
		},
		PostgresConfig: &PostgresConfig{Host: "db"},
		RedisConfig:    &RedisConfig{},
		RabbitMQConfig: &RabbitMQConfig{URL: "amqp://user:pass@mq:5672"},
		RuntimeConfig:  &RuntimeConfig{NumRoutingGorutines: 4},
	}
}

func TestValidateConfigAcceptsCompleteConfig(t *testing.T) {
	if err := validateConfig(validTestConfig()); !err.IsNil() {
		t.Fatalf("unexpected error: %s", err.JSON())
	}
}

func TestValidateConfigRejectsMissingRequiredValues(t *testing.T) {
	mutations := map[string]func(*Config){
		"token":          func(c *Config) { c.TeleAPIWebhookConfig.Token = "" },
		"public url":     func(c *Config) { c.TeleAPIWebhookConfig.URL = "" },
		"webhook secret": func(c *Config) { c.TeleAPIWebhookConfig.Secret = "" },
		"rabbitmq url":   func(c *Config) { c.RabbitMQConfig.URL = "" },
		"postgres host":  func(c *Config) { c.PostgresConfig.Host = "" },
		"goroutines":     func(c *Config) { c.RuntimeConfig.NumRoutingGorutines = 0 },
	}
	for name, mutate := range mutations {
		cfg := validTestConfig()
		mutate(cfg)
		if err := validateConfig(cfg); err.IsNil() {
			t.Fatalf("expected validation error for missing %s", name)
		}
	}
}
