package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
    InfuraKey string
    NgrokURL string
}

func LoadConfig() Config {
    // Downloading environment variables from .env file
	if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }

    // Getting the Telegram token
    token := os.Getenv("TELEGRAM_TOKEN")
    if token == "" {
        panic("TELEGRAM_BOT_TOKEN is not set")
    }

    // Getting the Infura key
    infuraKey := os.Getenv("INFURA_KEY")
    if infuraKey == "" {
        panic("INFURA_KEY is not set")
    }

    // Getting the Ngrok URL
    ngrokURL := os.Getenv("NGROK_URL")
    if ngrokURL == "" {
        panic("NGROK_URL is not set")
    }

    return Config{
        TelegramToken: token,
        InfuraKey: infuraKey,
        NgrokURL: ngrokURL,
    }
}
