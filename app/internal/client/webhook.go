package client

import (
	"app/internal/config"
	e "app/pkg/errors"

	tele "gopkg.in/telebot.v4"
)

func SetupWebhook(config *config.Config) (*tele.Bot, *e.ErrorInfo) {
	pref := tele.Settings{
		Token: config.TelegramClient.Token,
		Poller: &tele.Webhook{
			Listen: ":" + config.TelegramClient.WebhookPort,
			Endpoint: &tele.WebhookEndpoint{
				PublicURL: config.TelegramClient.WebhookURL,
			},
			MaxConnections: 100,
		},
	}

	client, err := tele.NewBot(pref)
	if err != nil {
		return nil, e.FromError(err, "Failed to create bot").
			WithSeverity(e.Critical).
			WithData(map[string]any{
				"token": config.TelegramClient.Token,
				"webhook_url": config.TelegramClient.WebhookURL,
				"webhook_port": config.TelegramClient.WebhookPort,
			})
	}
	
	return client, e.Nil()
}
