package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/telebot.v3"
)

type UserVerification struct {
	UserID    int64  // User ID
	Username  string // User username
	GroupID   int64  // Group ID
	GroupName string // Group name
	IsPending bool   // Is the user awaiting verification
	verified  bool   // Verification status
}

var (
	verificationData = make(map[int64]*UserVerification)
	dataMutex        = sync.Mutex{} // For thread-safe access
)

func isAdmin(bot *telebot.Bot, chatID int64, userID int64) bool {
	member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
	if err != nil {
		log.Printf("Error fetching user role: %v", err)
		return false
	}
	return member.Role == "administrator" || member.Role == "creator"
}

func main() {
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }

    token := os.Getenv("TELEGRAM_TOKEN")
    if token == "" {
        log.Fatal("TELEGRAM_TOKEN is not set")
    }

    pref := telebot.Settings{
        Token:  token,
        Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
    }

    bot, err := telebot.NewBot(pref)
    if err != nil {
        log.Fatal(err)
    }

    bot.Handle(telebot.OnUserJoined, func(c telebot.Context) error {
		for _, member := range c.Message().UsersJoined {
			if isAdmin(bot, c.Chat().ID, member.ID) {
				log.Printf("Skipping admin user @%s (ID: %d)", member.Username, member.ID)
				continue
			}

			dataMutex.Lock()
			verificationData[member.ID] = &UserVerification{
				UserID:    member.ID,
				Username:  member.Username,
				GroupID:   c.Chat().ID,
				GroupName: c.Chat().Title,
				IsPending: true,
				verified:  false,
			}
			dataMutex.Unlock()

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

			go func(userID int64, groupID int64) {
				time.Sleep(30 * time.Second)

				dataMutex.Lock()
				userData, exists := verificationData[userID]
				if exists && userData.IsPending && userData.verified == false {
					log.Printf("User @%s (ID: %d) failed verification on time. Removing from group.", userData.Username, userID)
					bot.Ban(&telebot.Chat{ID: groupID}, &telebot.ChatMember{User: &telebot.User{ID: userID}})
					time.Sleep(1 * time.Second)
					bot.Unban(&telebot.Chat{ID: groupID}, &telebot.User{ID: userID})
					bot.Send(&telebot.User{ID: userID}, "You did not complete the verification on time and were removed from the group.")
					delete(verificationData, userID)
				}
				dataMutex.Unlock()
			}(member.ID, c.Chat().ID)
		}
		return nil
	})

	bot.Handle("/start", func(c telebot.Context) error {
		userID := c.Sender().ID

		dataMutex.Lock()
		userData, exists := verificationData[userID]
		dataMutex.Unlock()

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

		time.Sleep(2 * time.Second)
		return c.Send("How old are you?")
	})

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		userID := c.Sender().ID

		dataMutex.Lock()
		userData, exists := verificationData[userID]
		dataMutex.Unlock()

		if !exists || !userData.IsPending {
			log.Printf("User @%s (ID: %d) attempted age verification without being in the queue.", c.Sender().Username, userID)
			return c.Send("You are not awaiting verification.")
		}

		age, err := strconv.Atoi(c.Text())
		if err != nil {
			return c.Send("Please enter a valid age in numbers.")
		}

		if age < 18 {
			log.Printf("User @%s (ID: %d) is under 18. Banned from the group.", userData.Username, userID)
			verificationData[userID].verified = false
			verificationData[userID].IsPending = false
			bot.Ban(&telebot.Chat{ID: userData.GroupID}, &telebot.ChatMember{User: &telebot.User{ID: userID}})
			c.Send("Sorry, access denied. You must be 18 or older.")
			bot.Unban(&telebot.Chat{ID: userData.GroupID}, &telebot.User{ID: userID})
			c.Send("You were removed from the group.")
			delete(verificationData, userID)
		} else {
			log.Printf("User @%s (ID: %d) successfully verified.", userData.Username, userID)
			verificationData[userID].verified = true
			dataMutex.Lock()
			delete(verificationData, userID)
			dataMutex.Unlock()
			c.Send("Thank you! You have successfully passed the verification.")
		}

		return nil
	})

	log.Println("Bot started...")
	bot.Start()
}
