package api

import (
	e "app/pkg/errors"

	tele "gopkg.in/telebot.v4"
	"app/src/infrastructure/config"
)

func SetupWebhook(config *config.Config) (*tele.Bot, *e.ErrorInfo) {
	pref := tele.Settings{
		Token: config.TeleAPIWebhookConfig.Token,
		Poller: &tele.Webhook{
			Listen: ":" + config.TeleAPIWebhookConfig.Port,
			Endpoint: &tele.WebhookEndpoint{
				PublicURL: config.TeleAPIWebhookConfig.URL,
			},
			MaxConnections: 100,
		},
	}

	client, err := tele.NewBot(pref)
	if err != nil {
		return nil, e.FromError(err, "Failed to create bot").
			WithSeverity(e.Critical).
			WithData(map[string]any{
				"token":        config.TeleAPIWebhookConfig.Token,
				"webhook_url":  config.TeleAPIWebhookConfig.URL,
				"webhook_port": config.TeleAPIWebhookConfig.Port,
			})
	}

	return client, e.Nil()
}
