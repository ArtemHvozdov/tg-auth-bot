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

	Bot.Use(AdminOnlyMiddleware(Bot))

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
	Bot.Handle("/setup", handlers.SetupHandler(Bot))
	Bot.Handle("/verify", handlers.VerifyHandler(Bot))
	Bot.Handle("/check_admin", handlers.CheckAdminHandler(Bot))
	
	// bot.Handle(telebot.OnText, handlers.TextMessageHandler(bot))

	log.Println("Bot started...")
	Bot.Start()

	log.Println("Default user storage:", storage.UserStore)
	return nil
}


// AdminOnlyMiddleware проверяет роль пользователя
func AdminOnlyMiddleware(bot *telebot.Bot) telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			// Checking whether the command is called in the group
			if c.Chat().Type == telebot.ChatGroup || c.Chat().Type == telebot.ChatSuperGroup {
				// Getting information about the user
				userID := c.Sender().ID
				chatID := c.Chat().ID
				userName := c.Sender().Username

				// Checking if the user is an administrator
				member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
				if err != nil {
					log.Printf("Error fetching user's role: %v", err)
					return c.Reply("I couldn't verify your role. Please try again later.")
				}

				// Если пользователь не администратор
				if member.Role != "administrator" && member.Role != "creator" {
					msg := fmt.Sprintf("@%s, you are not an administrator of this group and cannot use bot commands.", userName)
					_, err := bot.Send(c.Chat(), msg)
					if err != nil {
						log.Printf("Error sending non-admin message: %v", err)
					}
					return nil // Finishing the command
				}
			}

			// If the check passes, call the following handler
			return next(c)
		}
	}
}