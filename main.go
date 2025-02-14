package main

import (
	//"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	//"strconv"
	//"sync"
	//"time"

	//"gopkg.in/telebot.v3"

	"github.com/ArtemHvozdov/tg-auth-bot/bot"
	"github.com/ArtemHvozdov/tg-auth-bot/web"

	//"test-bot/auth"
	//"test-bot/web"
	"github.com/ArtemHvozdov/tg-auth-bot/config"
	"github.com/ArtemHvozdov/tg-auth-bot/storage_db"
)

func main() {

	cfg := config.LoadConfig() // Loading the configuration from a file or environment variables

	// Initialize the database
	dataDir := "./data"
	dbPath := dataDir + "/tg-bot.db"

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Ошибка при создании папки %s: %v", dataDir, err)
	}

	err := storage_db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer storage_db.CloseDB() // Ensure the database is closed on shutdown

	// Create a channel to handle OS signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// We launch the Telegram bot in a separate goroutine
	go func() {
		err := bot.StartBot(cfg)
		if err != nil {
			log.Fatalf("Failed to start bot: %v", err)
		}
	}()

	go func ()  {
		// Run webserver
		web.StartServer()
	}()

	// Wait for termination signal
	<-stop
	log.Println("Shutting down gracefully...")

	// // Run webserver
	// web.StartServer()
}