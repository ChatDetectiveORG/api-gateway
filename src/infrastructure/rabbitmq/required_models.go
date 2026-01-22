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
}
