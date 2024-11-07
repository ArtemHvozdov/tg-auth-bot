package main

import (
    "fmt"
    "log"
    "os"
    "time"
    "gopkg.in/telebot.v3"
    "github.com/joho/godotenv"
)

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
            reply := fmt.Sprintf("Hello, %s!", userName)
            return c.Send(reply)
        }
        return nil
    })

    bot.Start()
}
