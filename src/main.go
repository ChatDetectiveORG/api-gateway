package main

import (
	"app/src/infrastructure/config"
	"app/src/infrastructure/api"
	"app/src/infrastructure/postgresql"
	"app/src/infrastructure/rabbitmq"
	"app/src/infrastructure/redis"
	"log"
)

func main() {
	config, _ := config.FetchConfig()

	err := postgresql.InitPostgresql()
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	err = rabbitmq.InitRabbitMQ(config, rabbitmq.RequiredModels)
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	err = redis.InitRedis(config)
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	client, err := api.SetupWebhook(config)
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	api.LoadHandlers(client)

	wait := make(chan struct{}, 1)
	
	go func() {
		client.Start()
		wait <- struct{}{}
	}()

	<-wait
	log.Println("Bot started")
}
