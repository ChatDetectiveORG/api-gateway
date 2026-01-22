package api

import (
	tele "gopkg.in/telebot.v4"
	e "app/pkg/errors"
)

func LoadHandlers(client *tele.Bot) *e.ErrorInfo {
	handledUpdates := []string{
		// Сообщения боту, коамнды
		tele.OnText,

		// Кнопки в боте
		tele.OnCallback,

		// Платежи внутри ботв
		tele.OnShipping,
		tele.OnCheckout,

		// Пользовательские чаты
		tele.OnBusinessConnection,
		tele.OnBusinessMessage,
		tele.OnEditedBusinessMessage,
		tele.OnDeletedBusinessMessages,
	}

	for _, event := range handledUpdates {
		client.Handle(
			event,
			func(_ tele.Context) error {
				return nil
			},
		)
	}

	return e.Nil()
}