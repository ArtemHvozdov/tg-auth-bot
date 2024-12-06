package web

import (
	"log"
	"net/http"
	"github.com/ArtemHvozdov/tg-auth-bot/auth"
)

// StartServer run webserver
func StartServer() {
	//http.HandleFunc("/api/sign-in", auth.GetAuthRequest)
	http.HandleFunc("/api/callback", auth.Callback)
	http.HandleFunc("/home", auth.Home)
	log.Println("Web server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal(err)
    }
}
