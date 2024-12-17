package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	//"strconv"
	//"sync"
	"github.com/ArtemHvozdov/tg-auth-bot/auth"
	"github.com/ArtemHvozdov/tg-auth-bot/storage"

	"time"

	"gopkg.in/telebot.v3"
)

// isAdmin checks if the user is a group admin
func isAdmin(bot *telebot.Bot, chatID int64, userID int64) bool {
	member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
	if err != nil {
		log.Printf("Error fetching user role: %v", err)
		return false
	}
	return member.Role == "administrator" || member.Role == "creator"
}

// Handler for /start
func StartHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userName := c.Sender().FirstName
        if c.Sender().LastName != "" {
            userName += " " + c.Sender().Username
        }

        msg := fmt.Sprintf(
            "Hello, %s!\n\nIf you want to be verified, run the command /verify.\nIf you want to configure the bot to verify participants, run the command /setup.",
            userName,
        )
        return c.Send(msg)
    }
}
// /setup command - administrator initiates configuration
func SetupHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		// Шаг 1: Отправляем сообщение о необходимости добавить бота в группу с правами администратора
		msg := "To set me up for verification in your group, please add me to the group as an administrator and call the /check_admin command in the group."
		if err := c.Send(msg); err != nil {
			log.Printf("Error sending setup message: %v", err)
			return err
		}
		return nil
	}
}

// /check_admin command - check administrator rights in a group and verify verification parameters
func CheckAdminHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		chatID := c.Chat().ID
		userID := c.Sender().ID
		chatName := c.Chat().Title // Getting the name of the chat (group)
		userName := c.Sender().Username // Username

		log.Printf("User ID: %d, Chat ID: %d, Command received", userID, chatID)
		log.Printf("User's name: %s %s (@%s)", c.Sender().FirstName, c.Sender().LastName, c.Sender().Username)

		// Checking if the bot is an administrator in this group
		member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: bot.Me.ID})
		if err != nil {
			log.Printf("Error fetching bot's role in the group: %v", err)
			// Send a private message to the user
			msg := "I couldn't fetch my role in this group. Please make sure I am an administrator."
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Error sending bot admin check message: %v", err)
				return err
			}
			return nil
		}

		// Logging the bot's role
		log.Printf("Bot's role in the group '%s': %s", chatName, member.Role)

		// Checking if the bot is an administrator
		if member.Role != "administrator" && member.Role != "creator" {
			msg := fmt.Sprintf("I am not an administrator in the group '%s'. Please promote me to an administrator.", chatName)
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Error sending bot admin check message: %v", err)
				return err
			}
			return nil
		}

		// Checking if the user the bot is interacting with is an administrator
		memberUser, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
		if err != nil {
			log.Printf("Error fetching user's role: %v", err)
			// Send a private message to the user
			msg := "I couldn't fetch your role in this group."
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Error sending user admin check message: %v", err)
				return err
			}
			return nil
		}

		// Logging the user role
		log.Printf("User's role in the group '%s': %s", chatName, memberUser.Role)

		// Checking if the user is an administrator
		if memberUser.Role != "administrator" && memberUser.Role != "creator" {
			// We inform the user that he is not an administrator
			groupMsg := fmt.Sprintf("@%s, you are not an administrator in the group '%s'. You cannot configure me for this group.", userName, chatName)
			if _, err := bot.Send(&telebot.Chat{ID: chatID}, groupMsg); err != nil {
				log.Printf("Error sending message to group: %v", err)
				return err
			}
			return nil
		}

		// All checks were successful
		msg := fmt.Sprintf("I have confirmed your admin status and my role in the group '%s'. You can now proceed with the setup.", chatName)
		if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
			log.Printf("Error sending success message to user: %v", err)
			return err
		}

		// Notify admin to send verification parameters in a private chat
		bot.Send(&telebot.User{ID: userID}, "Send verification parameters in JSON format in this private chat. Example:\n\n"+
			"{\n"+
			"  \"circuitId\": \"AtomicQuerySigV2CircuitID\",\n"+
			"  \"id\": 1,\n"+
			"  \"query\": {\n"+
			"    \"allowedIssuers\": [\"*\"],\n"+
			"    \"context\": \"https://example.com/context\",\n"+
			"    \"type\": \"ExampleType\",\n"+
			"    \"credentialSubject\": {\n"+
			"      \"birthday\": {\"$lt\": 20000101}\n"+
			"    }\n"+
			"  }\n"+
			"}")

		// Set up handler for incoming private messages
		bot.Handle(telebot.OnText, func(c telebot.Context) error {
			// Ensure the message is from a private chat and from the correct user
			if c.Chat().Type != telebot.ChatPrivate || c.Sender().ID != userID {
				return nil
			}

			var params storage.VerificationParams

			// Parse JSON from the admin's message
			if err := json.Unmarshal([]byte(c.Text()), &params); err != nil {
				log.Printf("Failed to parse JSON: %v", err)
				bot.Send(c.Sender(), "Invalid JSON format. Please ensure your parameters match the expected structure.")
				return nil
			}

			// Validate required fields in parsed JSON
			if params.CircuitID == "" || params.ID == 0 || params.Query == nil {
				log.Println("JSON does not contain all required fields.")
				bot.Send(c.Sender(), "Missing required fields in JSON. Please include 'circuitId', 'id', and 'query'.")
				return nil
			}

			// Save parameters to storage
			storage.VerificationParamsMap[chatID] = params
			log.Printf("Verification parameters set for group '%s': %+v", chatName, params)

			// Notify admin about successful setup
			bot.Send(c.Sender(), fmt.Sprintf("Verification parameters have been successfully set for the group '%s'.", chatName))
			return nil
		})

		return nil
	}
}


// A new user has joined the group
func NewUserJoinedHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		for _, member := range c.Message().UsersJoined {
			if isAdmin(bot, c.Chat().ID, member.ID) {
				log.Printf("Skipping admin user @%s (ID: %d)", member.Username, member.ID)
				continue
			}

			// Ограничиваем права нового участника
			// // restrictedUntil := time.Now().Add(10 * time.Minute).Unix() // Ограничения на 10 минут
			err := bot.Restrict(c.Chat(), &telebot.ChatMember{
				User: &telebot.User{ID: member.ID},
				Rights: telebot.Rights{
					CanSendMessages: false, // Полный запрет на отправку сообщений
				},
				//UntilDate: restrictedUntil, // Опционально: ограничение по времени
			})
			if err != nil {
				log.Printf("Failed to restrict user @%s (ID: %d): %s", member.Username, member.ID, err)
				continue
			}

			// Adding a new user to the repository
			newUser := &storage.UserVerification{
				UserID:    member.ID,
				Username:  member.Username,
				GroupID:   c.Chat().ID,
				GroupName: c.Chat().Title,
				IsPending: true,
				Verified:  false,
				SessionID: 0,
			}
			storage.AddOrUpdateUser(member.ID, newUser)

			log.Println("Bot Logs: new member -", newUser)

			btn := telebot.InlineButton{
				Text: "Verify your age",
				URL:  fmt.Sprintf("https://t.me/%s", bot.Me.Username),
			}

			inlineKeys := [][]telebot.InlineButton{{btn}}
			log.Printf("New member @%s added to verification queue.", member.Username)
			c.Send(
				fmt.Sprintf("Hi, @%s! Please verify your age by clicking the button below.", member.Username),
				&telebot.ReplyMarkup{InlineKeyboard: inlineKeys},
			)

			go handleVerificationTimeout(bot, member.ID, c.Chat().ID)
		}
		return nil
	}
}

// Handler /verify
func VerifyHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		userData, exists := storage.GetUser(userID)
		if !exists || !userData.IsPending {
			log.Printf("User @%s (ID: %d) is not awaiting verification.", c.Sender().Username, userID)
			return c.Send("You are not awaiting verification in any group.")
		}

		msg := fmt.Sprintf(
			"Hi, @%s! To remain in the group \"%s\", you need to complete the verification process.",
			userData.Username,
			userData.GroupName,
		)
		if err := c.Send(msg); err != nil {
			return err
		}

		userGroupID := storage.UserStore[userID].GroupID

		jsonData, _ := auth.GenerateAuthRequest(userID, storage.VerificationParamsMap[userGroupID])

		base64Data := base64.StdEncoding.EncodeToString(jsonData)

		// Create deeplink
		deepLink := fmt.Sprintf("https://wallet.privado.id/#i_m=%s", base64Data)

		// logs deeplinl
		log.Println("Deep Link:", deepLink)

		btn := telebot.InlineButton{
			Text: "Verify with Privado ID", // Text button
			URL:  deepLink,                // URL for redirect
		}

		// Creating markup with a button
		inlineKeyboard := &telebot.ReplyMarkup{}
		inlineKeyboard.InlineKeyboard = [][]telebot.InlineButton{{btn}}

		time.Sleep(2 * time.Second)
		// Send a message with a button
		return c.Send("Please click the button below to verify your age:", inlineKeyboard)
	}
}

// Handling verification timeout
func handleVerificationTimeout(bot *telebot.Bot, userID, groupID int64) {
	time.Sleep(10 * time.Minute)

	userData, exists := storage.GetUser(userID)
	if exists && userData.IsPending && !userData.Verified {
		log.Printf("User @%s (ID: %d) failed verification on time. Removing from group.", userData.Username, userID)
		bot.Ban(&telebot.Chat{ID: groupID}, &telebot.ChatMember{User: &telebot.User{ID: userID}})
		time.Sleep(1 * time.Second)
		bot.Unban(&telebot.Chat{ID: groupID}, &telebot.User{ID: userID})
		bot.Send(&telebot.User{ID: userID}, "You did not complete the verification on time and were removed from the group.")
		storage.DeleteUser(userID)
	}
}

// Store change listener
func ListenForStorageChanges(bot *telebot.Bot) {
	go func() {
		for event := range storage.DataChanges {
			userID := event.UserID
			data := event.Data

			if data == nil {
				// User was delete
				log.Printf("User ID: %d was removed from the store.", userID)
				continue
			}

			if !data.IsPending {
				if data.Verified {
					// Successful verification
					log.Printf("User @%s (ID: %d) passed verification.", data.Username, userID)
					
					// New logic code

					chat := &telebot.Chat{ID: storage.UserStore[userID].GroupID}

					err := bot.Restrict(chat, &telebot.ChatMember{
						User: &telebot.User{ID: userID},
						Rights: telebot.Rights{
							CanSendMessages: true, // Полный разрашение на отправку сообщений
						},
						//UntilDate: restrictedUntil, // Опционально: ограничение по времени
					})
					if err != nil {
						log.Printf("Failed to restrict user @%s (ID: %d): %s", storage.UserStore[userID].Username, userID, err)
						continue
					}

					bot.Send(&telebot.User{ID: userID}, "You have successfully passed verification and can stay in the group.")
				} else {
					// Verification failed
					log.Printf("User @%s (ID: %d) failed verification. Removing from group.", data.Username, userID)
					group := &telebot.Chat{ID: data.GroupID}
					user := &telebot.User{ID: userID}
					bot.Ban(group, &telebot.ChatMember{User: user})
					time.Sleep(1 * time.Second)
					bot.Unban(group, user)
					bot.Send(user, "You failed verification and were removed from the group.")
				}
			}
		}
	}()
}
