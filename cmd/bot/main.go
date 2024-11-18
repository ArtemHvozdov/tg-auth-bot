package main

import (
    "fmt"
    "log"
    "math/rand"
    "os"
    "time"
    "strings"
    "gopkg.in/telebot.v3"
    "github.com/joho/godotenv"
)

type SessionInfo struct {
    currentUserId        string
    currentUserName      string
    currentChanelId      string
    currentUserAdmin     bool
    currentChanelBotAdmin bool
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

    var currentSessionID string
    var awaitingChannelLink bool

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
                currentUserId:   fmt.Sprintf("%d", c.Sender().ID),
                currentUserName: userName,
                currentChanelId: "",
                currentUserAdmin: false,
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
    
                return c.Send("The channel has been successfully registered! The bot also has administrator rights.")
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

    bot.Start()
}
