package handlers

import (
	"encoding/base64"
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

// A new user has joined the group
func NewUserJoinedHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		for _, member := range c.Message().UsersJoined {
			if isAdmin(bot, c.Chat().ID, member.ID) {
				log.Printf("Skipping admin user @%s (ID: %d)", member.Username, member.ID)
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

// Handler /start
func StartHandler(bot *telebot.Bot) func(c telebot.Context) error {
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

		jsonData, _ := auth.GenerateAuthRequest(userID)

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
