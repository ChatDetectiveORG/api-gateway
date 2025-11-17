package test

import (
	"app/internal/handlers"
	"time"
	e "app/pkg/errors"
	tele "gopkg.in/telebot.v4"
)

func TestChain() *handlers.HandlerChain {
	return handlers.HandlerChain{}.Init(10*time.Second, sendTestMessage1, sendTestMessage2)
}

func sendTestMessage1(c tele.Context, args *handlers.Arg) (*handlers.Arg, *e.ErrorInfo) {
	errorInfo := e.FromError(c.Reply("Test message 1"), "Failed to send test message 1")
	return args, errorInfo
}

func sendTestMessage2(c tele.Context, args *handlers.Arg) (*handlers.Arg, *e.ErrorInfo) {
	errorInfo := e.FromError(c.Reply("Test message 2"), "Failed to send test message 2")
	return args, errorInfo
}

