package main


import (
	//"fmt"
	"log"
	//"strconv"
	//"sync"
	//"time"

	//"gopkg.in/telebot.v3"

	"github.com/ArtemHvozdov/tg-auth-bot/bot"
	"github.com/ArtemHvozdov/tg-auth-bot/web"
	//"test-bot/auth"
	//"test-bot/web"
	"github.com/ArtemHvozdov/tg-auth-bot/config"
)

func main() {

	cfg := config.LoadConfig() // Loading the configuration from a file or environment variables

	// We launch the Telegram bot in a separate goroutine
	go func() {
		err := bot.StartBot(cfg)
		if err != nil {
			log.Fatalf("Failed to start bot: %v", err)
		}
	}()

	// Run webserver
	web.StartServer()
}