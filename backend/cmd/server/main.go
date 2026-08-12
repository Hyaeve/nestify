package main

import (
	"log"
	"time"

	"nestify/backend/internal/app"
)

func main() {
	if location, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		time.Local = location
	}
	log.SetFlags(log.Ldate | log.Ltime)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
