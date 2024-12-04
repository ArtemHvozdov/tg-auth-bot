package bot

import (
	"fmt"
	"log"
	"time"
	"github.com/ArtemHvozdov/tg-auth-bot/config"
	"github.com/ArtemHvozdov/tg-auth-bot/bot/handlers"

	"gopkg.in/telebot.v3"
)

type UserVerification struct {
	UserID    int64
	Username  string
	GroupID   int64
	GroupName string
	IsPending bool
	verified  bool
}

var (
	verificationData = make(map[int64]*UserVerification)
)

// StartBot запускает Telegram-бота
func StartBot(cfg config.Config) error {
	pref := telebot.Settings{
		Token:  cfg.TelegramToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		return fmt.Errorf("error creating bot: %v", err)
	}

	// Обработчики
	bot.Handle(telebot.OnUserJoined, handlers.NewUserJoinedHandler(bot))
	bot.Handle("/start", handlers.StartHandler(bot))
	bot.Handle(telebot.OnText, handlers.TextMessageHandler(bot))

	log.Println("Bot started...")
	bot.Start()
	return nil
}
