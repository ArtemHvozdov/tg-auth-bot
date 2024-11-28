package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/telebot.v3"
)

type SessionInfo struct {
    currentUserId        string
    currentUserName      string
    currentChanelId      string
    currentUserAdmin     bool
    currentChanelBotAdmin bool
}

type NewMember struct {
	UserID          int64
	ChatID          int64
	VerificationType    string // Тип верификации
	VerificationStatus bool
	ContactedBot    bool   // Написал ли участник боту
	Verified        bool   // Пройдено ли верификация
}

var sessionStorage = make(map[string]SessionInfo)

var newMembers = make(map[int64]*NewMember)

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

    var currentSessionID string
    var awaitingChannelLink bool

    // Кнопки для выбора схемы
    menu := &telebot.ReplyMarkup{}
    btnScheme1 := menu.Data("Scheme 1", "scheme1")
    btnScheme2 := menu.Data("Scheme 2", "scheme2")
    btnScheme3 := menu.Data("Scheme 3", "scheme3")
    btnScheme4 := menu.Data("Scheme 4", "scheme4")
    btnCustom := menu.Data("Add a custom scheme", "custom_scheme")

    // Ряд кнопок
    menu.Inline(
        menu.Row(btnScheme1, btnScheme2),
        menu.Row(btnScheme3, btnScheme4),
        menu.Row(btnCustom),
    )

    bot.Handle("/start", func(c telebot.Context) error {
        currentSessionID = ""
        awaitingChannelLink = false
        return c.Send("What's your name?")
    })

    bot.Handle(telebot.OnText, func(c telebot.Context) error {
        if currentSessionID == "" {
            userName := c.Text()

            sessionID := fmt.Sprintf("%08d", rand.Intn(100000000))
            currentSessionID = sessionID

            session := SessionInfo{
                currentUserId:        fmt.Sprintf("%d", c.Sender().ID),
                currentUserName:      userName,
                currentChanelId:      "",
                currentUserAdmin:     false,
                currentChanelBotAdmin: false,
            }

            sessionStorage[sessionID] = session

            log.Printf("New session created: %v\n", sessionStorage[sessionID])

            reply := fmt.Sprintf("Hello, %s! To get started, you need to add your channel. To do this, use the /regchat command.", userName)
            return c.Send(reply)
        }

        // Checking if the bot is expecting a channel link
        if awaitingChannelLink {
            awaitingChannelLink = false
            channelLink := c.Text()
            if strings.HasPrefix(channelLink, "https://t.me/") {
                channelUsername := strings.TrimPrefix(channelLink, "https://t.me/")
                log.Println("Channel username:", channelUsername)
                channelUsername = "@" + channelUsername

                //Getting information about the channel
                chat, err := bot.ChatByUsername(channelUsername)
                if err != nil {
                    log.Printf("Error getting channel information:: %v", err)
                    return c.Send("Failed to get channel information. Check the link.")
                }

                // Checking if the bot is an administrator
                botMember, err := bot.ChatMemberOf(chat, bot.Me)
                if err != nil {
                    log.Printf("Error checking bot role in channel: %v", err)
                    return c.Send("Failed to check the bot's role in the channel. Make sure you have added the bot to the channel as an administrator")
                }

                isBotAdmin := botMember.Role == telebot.Administrator || botMember.Role == telebot.Creator

                if !isBotAdmin {
                    return c.Send("The bot must be the administrator of this channel. Add the bot as an administrator and try again.")
                }

                // Checking if the user is an administrator
                userMember, err := bot.ChatMemberOf(chat, c.Sender())
                if err != nil {
                    log.Printf("Error checking user role in channel: %v", err)
                    return c.Send("The bot cannot verify your role in this channel. Make sure he is an administrator.")
                }

                isUserAdmin := userMember.Role == telebot.Administrator || userMember.Role == telebot.Creator

                if !isUserAdmin {
                    return c.Send("You must be an administrator of this channel to register it.")
                }

                // Updating session data
                session := sessionStorage[currentSessionID]
                session.currentChanelId = fmt.Sprintf("%d", chat.ID)
                session.currentUserAdmin = true
                session.currentChanelBotAdmin = true
                sessionStorage[currentSessionID] = session

                log.Printf("Session updated: %v\n", sessionStorage[currentSessionID])

                // Уведомление о регистрации
                c.Send("The channel has been successfully registered! The bot also has administrator rights.")

                // Отправка сообщения через одну секунду
                time.AfterFunc(1*time.Second, func() {
                    bot.Send(c.Sender(), "The following schemes are available to verify new subscribers:", menu)
                })

                return nil
            }

            return c.Send("Invalid link format. Please provide the link in the format https://t.me/your_channel.")
        }

        return nil
    })

    bot.Handle("/regchat", func(c telebot.Context) error {
        if currentSessionID == "" {
            return c.Send("Please start your session using the command /start.")
        }

        awaitingChannelLink = true
        return c.Send("Send me a link to your channel (for example, https://t.me/your_channel). Before doing this, add the bot as an administrator to your channel.")
    })

    // Обработчик добавления нового участника
	bot.Handle(telebot.OnUserJoined, func(c telebot.Context) error {
		newMember := c.Sender() // Новый участник
		if newMember == nil {
			return nil
		}

		chat := c.Chat()

		
		// Проверка: если пользователь является администратором, пропускаем
		admins, err := bot.AdminsOf(c.Chat())
		if err != nil {
			log.Printf("Ошибка получения списка администраторов: %v", err)
			return nil
		}
		for _, admin := range admins {
			if admin.User.ID == newMember.ID {
				log.Printf("Игнорируем событие для администратора: %s (ID: %d)", newMember.Username, newMember.ID)
				return nil
			}
		}

		// Добавляем участника в карту
		memberData := &NewMember{
			UserID:       newMember.ID,
			ChatID:       chat.ID,
			VerificationType: "age", // Тип верификации
			ContactedBot: false,
			Verified:     false,
		}
		newMembers[newMember.ID] = memberData

		// Личное сообщение новому участнику
		privateMessage := fmt.Sprintf("Hello, %s! You have joined a group. To be in it you need to pass verification..", newMember.FirstName)
		_, err = bot.Send(newMember, privateMessage)
		if err != nil {
			log.Printf("Не удалось отправить личное сообщение пользователю %s (ID: %d): %v", newMember.Username, newMember.ID, err)

			// Сообщение в группу
			mentionMessage := fmt.Sprintf(
				"@%s, I couldn't send you a private message. Please write me a private message to pass verification, otherwise you will be removed from the group in 5 minutes.",
				newMember.Username,
			)
			if err := c.Send(mentionMessage); err != nil {
				log.Printf("Ошибка отправки сообщения в группу: %v", err)
			} else {
				log.Printf("Сообщение в группу отправлено для пользователя %s (ID: %d).", newMember.Username, newMember.ID)
			}
		} else {
			log.Printf("Личное сообщение отправлено пользователю %s (ID: %d).", newMember.Username, newMember.ID)

			// === Добавлено: сообщение о возрасте ===
			// Спросить возраст через 1 секунду
			time.AfterFunc(4*time.Second, func() {
				_, ageAskErr := bot.Send(newMember, "How old are you?")
				if ageAskErr != nil {
					log.Printf("Ошибка отправки запроса возраста пользователю %s (ID: %d): %v", newMember.Username, newMember.ID, ageAskErr)
				}
			})

			// Таймер на 5 минут для удаления (если пользователь не ответит)
			time.AfterFunc(40*time.Second, func() {
				member, exists := newMembers[newMember.ID]
				log.Println("Exists:", exists)
				if exists && !member.Verified {
					// Удаляем участника из группы
					err := bot.Ban(chat, &telebot.ChatMember{User: newMember})
					if err != nil {
						log.Printf("Не удалось удалить пользователя %s (ID: %d): %v", newMember.Username, newMember.ID, err)
					} else {
						log.Printf("Пользователь %s (ID: %d) был удален из группы. Пользователь не дал ответ боту", newMember.Username, newMember.ID)

						time.AfterFunc(2*time.Second, func() {
							_, err = bot.Send(newMember, "You have been removed from the group. You did not respond to the bot to pass verification.")
						})

						// Автоматическое снятие бана
						time.AfterFunc(1*time.Second, func() {
							unbanErr := bot.Unban(chat, newMember)
							if unbanErr != nil {
								log.Printf("Ошибка снятия бана для пользователя %s (ID: %d): %v", newMember.Username, newMember.ID, unbanErr)
							} else {
								log.Printf("Бан снят для пользователя %s (ID: %d).", newMember.Username, newMember.ID)
							}
						})
					}
					delete(newMembers, newMember.ID)
				}
			})

			// Обработчик ответа
			bot.Handle(telebot.OnText, func(c telebot.Context) error {
				userID := c.Sender().ID
				member, exists := newMembers[userID]
				if !exists {
					return nil // Игнорируем сообщения от пользователей, которых нет в карте
				}

				// Обновляем статус "Контактировал с ботом"
				member.ContactedBot = true

				age, convErr := strconv.Atoi(c.Text())
				if convErr != nil || age < 18 {
					// Сообщение о том, что пользователь не прошёл верификацию
					_, msgErr := bot.Send(c.Sender(), "Sorry, you are under 18, you cannot be in this group.")
					if msgErr != nil {
						log.Printf("Ошибка отправки сообщения о верификации пользователю %s (ID: %d): %v", c.Sender().Username, c.Sender().ID, msgErr)
					}

					// Удаление из группы через 5 секунд
					time.AfterFunc(5*time.Second, func() {
						err := bot.Ban(chat, &telebot.ChatMember{User: c.Sender()})
						if err != nil {
							log.Printf("Не удалось удалить пользователя %s (ID: %d): %v.", c.Sender().Username, c.Sender().ID, err)
						} else {
							log.Printf("Пользователь %s (ID: %d) был удален из группы. Пользовавтель не прошёл верификацию", c.Sender().Username, chat.ID)

							// Автоматическое снятие бана
							time.AfterFunc(1*time.Second, func() {
								unbanErr := bot.Unban(chat, c.Sender())
								if unbanErr != nil {
									log.Printf("Ошибка снятия бана для пользователя %s (ID: %d): %v", c.Sender().Username, c.Sender().ID, unbanErr)
								} else {
									log.Printf("Бан снят для пользователя %s (ID: %d).", c.Sender().Username, c.Sender().ID)
								}
							})
						}

						time.AfterFunc(2*time.Second, func() {
							_, err = bot.Send(newMember, "You have been removed from the group. You have not passed verification.")
						})
						
						delete(newMembers, userID)
					})
				} else {
					// Сообщение о прохождении верификации
					member.Verified = true // Обновляем статус верификации
					_, successMsgErr := bot.Send(c.Sender(), "You have passed verification. You can be in this group.")
					if successMsgErr != nil {
						log.Printf("Ошибка отправки сообщения о прохождении верификации пользователю %s (ID: %d): %v", c.Sender().Username, c.Sender().ID, successMsgErr)
					}
				}

				return nil
			})
		
		}

		return nil
	})
    
    log.Println("Bot is running...")

    bot.Start()
}
