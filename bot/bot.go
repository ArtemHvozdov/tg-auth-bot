package bot

import (
	"fmt"
	"log"
	"time"

	"github.com/ArtemHvozdov/tg-auth-bot/bot/handlers"
	"github.com/ArtemHvozdov/tg-auth-bot/config"
	"github.com/ArtemHvozdov/tg-auth-bot/storage"

	//"github.com/ArtemHvozdov/tg-auth-bot/storage"

	"gopkg.in/telebot.v3"
)

var InstanceBot *telebot.Bot

// StartBot runs Telegram-бота
func StartBot(cfg config.Config) error {
	pref := telebot.Settings{
		Token:  cfg.TelegramToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	Bot, err := telebot.NewBot(pref)
	if err != nil {
		return fmt.Errorf("error creating bot: %v", err)
	}

	// Установка доступных команд
	err = Bot.SetCommands([]telebot.Command{
		{Text: "start", Description: "Launch bot"},
		{Text: "verify", Description: "Go through verification"},
		{Text: "setup", Description: "Configure verification settings"},
		{Text: "help", Description: "Get information about commands"},
	})
	if err != nil {
		log.Printf("Failed to set bot commands: %v", err)
	}

	InstanceBot = Bot

	handlers.ListenForStorageChanges(Bot)

	// Handlers
	Bot.Handle(telebot.OnUserJoined, handlers.NewUserJoinedHandler(Bot))
	Bot.Handle("/start", handlers.StartHandler(Bot))
	Bot.Handle("/verify", handlers.VerifyHandler(Bot))
	// bot.Handle(telebot.OnText, handlers.TextMessageHandler(bot))

	log.Println("Bot started...")
	Bot.Start()

	log.Println("Default user storage:", storage.UserStore)
	return nil
}
