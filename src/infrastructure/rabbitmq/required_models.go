package rabbitmq

import (
	"github.com/ChatDetectiveORG/shared/events"
	amqp "github.com/rabbitmq/amqp091-go"
)

var RequiredModels = buildRequiredModels()

func buildRequiredModels() []Model {
	models := []Model{
		ExchangeModel{
			Exchange:   events.EventsExchange,
			Kind:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Args:       amqp.Table{},
		},
	}

	// Ensure payment queues exist even if payment-service has not started yet.
	for i := 0; i < events.ShardCount; i++ {
		queue := events.ShardQueueName(events.PodTypeShipping, i)
		models = append(models,
			QueueModel{
				Queue:      queue,
				Durable:    true,
				AutoDelete: false,
				Exclusive:  false,
				NoWait:     false,
				Args: amqp.Table{
					"x-single-active-consumer": true,
				},
			},
			BindingModel{
				Queue:      queue,
				Exchange:   events.EventsExchange,
				RoutingKey: queue,
				NoWait:     false,
				Args:       amqp.Table{},
			},
		)
	}

	return models
}
