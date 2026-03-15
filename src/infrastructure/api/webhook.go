package api

import (
	e "github.com/ChatDetectiveORG/shared/errors"

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
		// TODO: При добавлении зеркал продумать отправку данных о боте-источнике
		return nil, e.FromError(err, "Failed to create bot").
			WithSeverity(e.Critical)
	}

	return client, e.Nil()
}
