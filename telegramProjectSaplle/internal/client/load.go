package client

import (
	"app/internal/handlers/test"
	e "app/pkg/errors"

	tele "gopkg.in/telebot.v4"
)

func LoadHandlers(bot *tele.Bot) *e.ErrorInfo {
	testChain := test.TestChain()

	bot.Handle("/start", testChain.Run)

	return e.Nil()
}
