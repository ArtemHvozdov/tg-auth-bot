package handlers

import (
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"sync"
	"github.com/ArtemHvozdov/tg-auth-bot/auth"
	"time"

	"gopkg.in/telebot.v3"
)

// Вспомогательные структуры
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
	dataMutex        = sync.Mutex{}
)

// isAdmin проверяет, является ли пользователь администратором группы
func isAdmin(bot *telebot.Bot, chatID int64, userID int64) bool {
	member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
	if err != nil {
		log.Printf("Error fetching user role: %v", err)
		return false
	}
	return member.Role == "administrator" || member.Role == "creator"
}

// Новый пользователь присоединился к группе
func NewUserJoinedHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
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

			go handleVerificationTimeout(bot, member.ID, c.Chat().ID)
		}
		return nil
	}
}

// Обработчик команды /start
func StartHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
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

		jsonData, _ := auth.GenerateAuthRequest()

		base64Data := base64.StdEncoding.EncodeToString(jsonData)

		// Создание диплинка
		deepLink := fmt.Sprintf("https://wallet.privado.id/#i_m=%s", base64Data)

		// Вывод диплинка
		log.Println("Deep Link:", deepLink)

		time.Sleep(2 * time.Second)
		return c.Send(deepLink)

		// time.Sleep(2 * time.Second)
		// return c.Send("How old are you?")
	}
}

// Обработчик текстовых сообщений
func TextMessageHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
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

		handleAgeVerification(bot, userID, userData, age, c)

		return nil
	}
}

// Обработка истечения времени на верификацию
func handleVerificationTimeout(bot *telebot.Bot, userID, groupID int64) {
	time.Sleep(5 * time.Minute)

	dataMutex.Lock()
	userData, exists := verificationData[userID]
	if exists && userData.IsPending && !userData.verified {
		log.Printf("User @%s (ID: %d) failed verification on time. Removing from group.", userData.Username, userID)
		bot.Ban(&telebot.Chat{ID: groupID}, &telebot.ChatMember{User: &telebot.User{ID: userID}})
		time.Sleep(1 * time.Second)
		bot.Unban(&telebot.Chat{ID: groupID}, &telebot.User{ID: userID})
		bot.Send(&telebot.User{ID: userID}, "You did not complete the verification on time and were removed from the group.")
		delete(verificationData, userID)
	}
	dataMutex.Unlock()
}

// Обработка верификации возраста
func handleAgeVerification(bot *telebot.Bot, userID int64, userData *UserVerification, age int, c telebot.Context) {
	if age < 18 {
		log.Printf("User @%s (ID: %d) is under 18. Banned from the group.", userData.Username, userID)
		userData.verified = false
		userData.IsPending = false
		bot.Ban(&telebot.Chat{ID: userData.GroupID}, &telebot.ChatMember{User: &telebot.User{ID: userID}})
		c.Send("Sorry, access denied. You must be 18 or older.")
		bot.Unban(&telebot.Chat{ID: userData.GroupID}, &telebot.User{ID: userID})
		c.Send("You were removed from the group.")
		delete(verificationData, userID)
	} else {
		log.Printf("User @%s (ID: %d) successfully verified.", userData.Username, userID)
		userData.verified = true
		dataMutex.Lock()
		delete(verificationData, userID)
		dataMutex.Unlock()
		c.Send("Thank you! You have successfully passed the verification.")
	}
}
