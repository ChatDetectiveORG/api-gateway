package main

import (
	"app/internal/client"
	"app/internal/config"
	"app/pkg/database"
)

func main() {
	config, err := config.Load()
	if !err.IsNil() {
		err.Fatal()
	}
	
	err = database.InitDb()
	if !err.IsNil() {
		err.Fatal()
	}

	bot, err := client.SetupWebhook(config)
	if !err.IsNil() {
		err.Fatal()
	}

	err = client.LoadHandlers(bot)
	if !err.IsNil() {
		err.Fatal()
	}

	bot.Start()
}
