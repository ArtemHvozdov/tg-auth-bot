package main


import (
	//"fmt"
	"log"
	//"strconv"
	//"sync"
	//"time"

	//"gopkg.in/telebot.v3"

	"github.com/ArtemHvozdov/tg-auth-bot/bot"
	"github.com/ArtemHvozdov/tg-auth-bot/web"
	//"test-bot/auth"
	//"test-bot/web"
	"github.com/ArtemHvozdov/tg-auth-bot/config"
)

func main() {

	cfg := config.LoadConfig() // Загружаем конфигурацию из файла или переменных окружения

	// Запускаем Telegram-бота в отдельной горутине
	go func() {
		err := bot.StartBot(cfg)
		if err != nil {
			log.Fatalf("Failed to start bot: %v", err)
		}
	}()

	// Запускаем веб-сервер
	web.StartServer()
}