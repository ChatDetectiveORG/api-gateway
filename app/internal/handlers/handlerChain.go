package handlers

import (
	"context"
	"time"

	e "app/pkg/errors"
	tele "gopkg.in/telebot.v4"
)

type Arg map[string]any

type chainHandler func(c tele.Context, args *Arg) (*Arg, *e.ErrorInfo)

type HandlerChain struct {
	Handlers      []chainHandler
	Args          *Arg
	ErrorInfo     *e.ErrorInfo
	timeout       time.Duration
}

func (hc HandlerChain) Init(timeout time.Duration, handlers ...chainHandler) *HandlerChain {
	new := HandlerChain{
		Handlers: handlers,
		timeout:  timeout,
		Args:     &Arg{},
	}

	return &new
}

func (hc *HandlerChain) Run(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), hc.timeout)
	defer cancel()

	done := make(chan *e.ErrorInfo, 1)

	go func() {
		for _, handler := range hc.Handlers {
			select {
			case <-ctx.Done():
				hc.ErrorInfo = e.FromError(ctx.Err(), "Context cancelled")
				done <- hc.ErrorInfo
				return
			default:
			}

			newArgs, errInfo := handler(c, hc.Args)
			if !errInfo.IsNil() {
				hc.ErrorInfo = errInfo
				done <- hc.ErrorInfo
				return
			}
			*hc.Args = *newArgs
		}

		done <- e.Nil()
	}()

	select {
	case err := <-done:
		if err.IsNil() {
			return nil
		}
		return err
	case <-ctx.Done():
		hc.ErrorInfo = e.FromError(ctx.Err(), "Context timeout")
		return hc.ErrorInfo
	}
}
