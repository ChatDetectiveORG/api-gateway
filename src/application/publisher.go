package app

import (
	"context"

	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/config"
	"github.com/ChatDetectiveORG/shared/amqputil"
	e "github.com/ChatDetectiveORG/shared/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

type routingPublisher struct {
	pub *amqputil.PublishChannel
}

func newRoutingPublisher(cfg *config.Config) (*routingPublisher, *e.ErrorInfo) {
	open := func() (*amqp.Channel, error) {
		ch, err := newRabbitmqChannel(cfg)
		if !err.IsNil() {
			return nil, err
		}
		return ch, nil
	}

	ch, err := newRabbitmqChannel(cfg)
	if !err.IsNil() {
		return nil, err
	}

	return &routingPublisher{
		pub: amqputil.NewPublishChannel(ch, open),
	}, e.Nil()
}

func (p *routingPublisher) close() {
	if p.pub != nil {
		p.pub.Close()
	}
}

func (p *routingPublisher) publish(ctx context.Context, exchange, routingKey string, msg amqp.Publishing) error {
	return p.pub.Publish(ctx, exchange, routingKey, msg)
}
