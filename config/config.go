package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
}

func LoadConfig() Config {


	if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }
    token := os.Getenv("TELEGRAM_TOKEN")
    if token == "" {
        panic("TELEGRAM_BOT_TOKEN is not set")
    }
    return Config{
        TelegramToken: token,
    }
}
