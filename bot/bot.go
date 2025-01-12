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

	bot, err := telebot.NewBot(pref)
	if err != nil {
		return fmt.Errorf("error creating bot: %v", err)
	}

	bot.Use(AdminOnlyMiddleware(bot))

	// Setting up commands
	err = bot.SetCommands([]telebot.Command{
		{Text: "start", Description: "Launch bot"},
		{Text: "verify", Description: "Go through verification"},
		{Text: "setup", Description: "Configure verification settings"},
		{Text: "check_admin", Description: "Check if the user and bot are an administrators"},
		{Text: "test_verification", Description: "Test verification for admin"},
		{Text: "verified_users_list", Description: "Get list of verified users"},
		{Text: "help", Description: "Get information about commands"},
		{Text: "add_verification_params", Description: "Add verification parameters"},
		{Text: "list_verification_params", Description: "List verification parameters"},
		{Text: "set_active_verification_params", Description: "Set active verification parameters"},
		{Text: "add_type_restriction", Description: "Add type restriction"},
		{Text: "set_type_restriction", Description: "Set type restriction"},
	})
	if err != nil {
		log.Printf("Failed to set bot commands: %v", err)
	}

	handlers.ListenForStorageChanges(bot)

	// Handlers
	bot.Handle(telebot.OnUserJoined, handlers.NewUserJoinedHandler(bot))
	bot.Handle("/start", handlers.StartHandler(bot))
	bot.Handle("/setup", handlers.SetupHandler(bot))
	bot.Handle("/verify", handlers.VerifyHandler(bot))
	bot.Handle("/check_admin", handlers.CheckAdminHandler(bot))
	bot.Handle("/test_verification", handlers.TestVerificationHandler(bot))
	bot.Handle("/verified_users_list", handlers.VerifiedUsersListHeandler(bot))
	bot.Handle("/add_verification_params", handlers.AddVerificationParamsHandler(bot))
	bot.Handle("/list_verification_params", handlers.ListVerificationParamsHandler(bot))
	bot.Handle("/set_active_verification_params", handlers.SetActiveVerificationParamsHandler(bot))
	bot.Handle("/add_type_restriction", handlers.AddTypeRestrictionHandler(bot))
	bot.Handle("/set_type_restriction", handlers.SetTypeRestrictionHandler(bot))
		
	messageTypes := []string{
		telebot.OnText,
		telebot.OnPhoto,
		telebot.OnAudio,
		telebot.OnDocument,
		telebot.OnSticker,
		telebot.OnVideo,
		telebot.OnVoice,
	}

	for _, messageType := range messageTypes {
		bot.Handle(messageType, handlers.UnifiedHandler(bot))
	}

	log.Println("Bot started...")
	bot.Start()

	log.Println("Default user storage:", storage.UserStore)
	return nil
}

// AdminOnlyMiddleware checks the user's role and allows access only to administrators
func AdminOnlyMiddleware(bot *telebot.Bot) telebot.MiddlewareFunc {
    return func(next telebot.HandlerFunc) telebot.HandlerFunc {
        return func(c telebot.Context) error {
            // Ignore events that are not associated with text commands
            if c.Message() == nil || c.Message().Text == "" || c.Message().Text[0] != '/' {
                return next(c)
            }

            // Checking that the command is executed in the group
            if c.Chat().Type == telebot.ChatGroup || c.Chat().Type == telebot.ChatSuperGroup {
                userID := c.Sender().ID
                chatID := c.Chat().ID

                // Checking the user role
                member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
                if err != nil {
                    log.Printf("Error fetching user's role: %v", err)
                    return nil // Ignore the error and do not execute the command
                }

                // If the user is not an administrator
                if member.Role != "administrator" && member.Role != "creator" {
                    // Delete the user's message with the command after 1 second
                    time.AfterFunc(1*time.Second, func() {
                        err := bot.Delete(c.Message())
                        if err != nil {
                            log.Printf("Error deleting message: %v", err)
                        }
                    })
                    return nil // Ignore the command
                }

				// Deleta bot command after 1 minute from creator or administrator
				if member.Role == "administrator" || member.Role == "creator" {
					time.AfterFunc(1*time.Minute, func() {
						err := bot.Delete(c.Message())
						if err != nil {
							log.Printf("Error deleting message: %v", err)
						}
					})
				}
            }

            // Pass control to the next handler
            return next(c)
        }
    }
}