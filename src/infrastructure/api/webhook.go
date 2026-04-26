package api

import (
	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/config"
	e "github.com/ChatDetectiveORG/shared/errors"
	tele "gopkg.in/telebot.v4"
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
			AllowedUpdates: []string{
				"message",
				"callback_query",
				"shipping_query",
				"pre_checkout_query",
				"business_connection",
				"business_message",
				"edited_business_message",
				"deleted_business_messages",
			},
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
