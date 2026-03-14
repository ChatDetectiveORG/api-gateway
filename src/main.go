package main

import (
	app "app/src/application"
	"app/src/infrastructure/api"
	"app/src/infrastructure/config"
	"os"
	"os/signal"
	"syscall"

	// "app/src/infrastructure/postgresql"
	"app/src/infrastructure/rabbitmq"
	"context"
	"log"
	"sync"
	"time"
)

func main() {
	config, _ := config.FetchConfig()

	// err := postgresql.InitPostgresql()
	// if !err.IsNil() {
	// 	log.Fatal(err.JSON())
	// }

	err := rabbitmq.InitRabbitMQ(config, rabbitmq.RequiredModels)
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	client, err := api.SetupWebhook(config)
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	app.InitUpdateChannel(config, ctx, &wg)

	api.LoadHandlers(client)

	log.Println("Starting client...")

	// Канал для ловли системных сигналов
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		client.Start()
	}()

	// Ожидание сигнала завершения
	sig := <-sigCh
	log.Printf("Received signal %s, shutting down gracefully...", sig)
	cancel()

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
		// Successfully waited for WaitGroup
	case <-time.After(30 * time.Second):
		log.Println("Timeout reached while waiting for WaitGroup, exiting forcefully")
	}
	log.Println("client stopped")
}
