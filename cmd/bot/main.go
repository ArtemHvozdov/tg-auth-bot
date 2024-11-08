package main

import (
    "fmt"
    "log"
    "math/rand"
    "os"
    "time"
    "gopkg.in/telebot.v3"
    "github.com/joho/godotenv"
)

type SessionInfo struct {
    currentUserId string
    currentUserName string
    currentChanelId string
    currentUserAdmin bool
    currentChanelBotAdmin bool
    typeVerification string
}

var sessionStorage = make(map[string]SessionInfo)

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

    var userName string

    bot.Handle("/start", func(c telebot.Context) error {
        userName = ""
        return c.Send("What's your name?")
    })

    bot.Handle(telebot.OnText, func(c telebot.Context) error {
        if userName == "" {
            userName = c.Text()

            sessionID := fmt.Sprintf("%08d", rand.Intn(100000000))

            session := SessionInfo{
                currentUserId:   fmt.Sprintf("%d", c.Sender().ID),
                currentUserName: userName,
                currentChanelId: "", // Можно обновить позже, если требуется
                currentUserAdmin: false, // Примерные значения, можно обновить при необходимости
                currentChanelBotAdmin: false,
            }

            sessionStorage[sessionID] = session

            log.Printf("New session created: %v\n", sessionStorage[sessionID])

            reply := fmt.Sprintf("Hello, %s!", userName)
            return c.Send(reply)
        }
        return nil
    })

    bot.Handle("/regchat", func(c telebot.Context) error {
        // Убедимся, что сессия активна
        if currentSessionID == "" {
            return c.Send("Please start a session by typing /start.")
        }

        // Запрашиваем ссылку на канал
        return c.Send("Please provide the link to your channel (e.g. https://t.me/your_channel).")
    })

    bot.Handle(telebot.OnText, func(c telebot.Context) error {
        // Проверяем, если это ссылка на канал
        if len(c.Text()) > 7 && c.Text()[0:7] == "https://" {
            // Извлекаем ID канала из URL
            channelUsername := c.Text()[8:] // Убираем https://t.me/

            // Получаем информацию о канале
            chat, err := bot.ChatByUsername(channelUsername)
            if err != nil {
                log.Printf("Error fetching channel info: %v", err)
                return c.Send("Failed to fetch channel information. Please check the link.")
            }

            // Получаем информацию о боте в этом канале
            botMember, err := bot.ChatMemberOf(chat, bot.Me)
            if err != nil {
                log.Printf("Error fetching bot role in channel: %v", err)
                return c.Send("Error fetching bot role in this channel.")
            }

            // Проверяем, является ли бот администратором
            isBotAdmin := botMember.Role == telebot.Administrator || botMember.Role == telebot.Creator

            // Получаем информацию о пользователе
            userMember, err := bot.ChatMemberOf(chat, c.Sender())
            if err != nil {
                log.Printf("Error fetching user role in channel: %v", err)
                return c.Send("Error fetching user role in this channel.")
            }

            // Проверяем, является ли пользователь администратором
            isUserAdmin := userMember.Role == telebot.Administrator || userMember.Role == telebot.Creator

            // Если и бот, и пользователь являются администраторами, обновляем сессию
            if isBotAdmin && isUserAdmin {
                // Обновляем данные сессии
                session := sessionStorage[currentSessionID]
                session.currentChanelId = fmt.Sprintf("%d", chat.ID)
                session.currentUserAdmin = true
                session.currentChanelBotAdmin = true
                sessionStorage[currentSessionID] = session

                // Выводим данные сессии в консоль
                currentSessionID := 
                log.Printf("Session updated: %v\n", sessionStorage[currentSessionID])

                return c.Send("Channel successfully registered!")
            }

            return c.Send("Both you and the bot must be admins in this channel to register it.")
        }

        return c.Send("Invalid channel link. Please provide a valid link in the format https://t.me/your_channel.")
    })
    
    

    bot.Start()
}
