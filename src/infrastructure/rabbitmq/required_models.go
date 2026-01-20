package rabbitmq

import amqp "github.com/rabbitmq/amqp091-go"

// RequiredModels is a starter template for RabbitMQ topology.
// Customize it for your app (names, durability, DLX, routing keys, etc).
//
// Tip: model declarations are idempotent, so calling InitRabbitMQ on every boot is fine.
var RequiredModels = []Model{
	// Exchange template
	ExchangeModel{
		Exchange:   "chatdetective.events",
		Kind:       "topic",
		Durable:    true,
		AutoDelete: false,
		Internal:   false,
		NoWait:     false,
		Args:       amqp.Table{}, // e.g. {"alternate-exchange": "ae.name"}
	},

	// Queue template
	QueueModel{
		Queue:      "chatdetective.events.router",
		Durable:    true,
		AutoDelete: false,
		Exclusive:  false,
		NoWait:     false,
		Args:       amqp.Table{
			// Templates you may want:
			// "x-dead-letter-exchange":    "chatdetective.dlx",
			// "x-dead-letter-routing-key": "router.dlq",
		},
	},

	// Binding template
	BindingModel{
		Queue:      "chatdetective.events.router",
		Exchange:   "chatdetective.events",
		RoutingKey: "router.*",
		NoWait:     false,
		Args:       amqp.Table{},
	},
}
