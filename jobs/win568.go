package jobs

import (
	"log"
	"os"
	"telo/services"
	"time"
)

func StartWin568Scheduler() {
	tickerFetch := time.NewTicker(30 * time.Second)
	go func() {
		for {
			<-tickerFetch.C
			err := services.FetchWin568BetListDaily(
				"SportsBook",
				os.Getenv("WIN568_COMPANY_KEY"),
				os.Getenv("WIN568_SERVER_ID"),
			)
			if err != nil {
				log.Printf("❌ error fetch win568: %v", err)
			}
		}
	}()

	tickerResend := time.NewTicker(2 * time.Minute)
	go func() {
		for {
			<-tickerResend.C
			err := services.ResendWin568Orders(
				"SportsBook",
				os.Getenv("WIN568_COMPANY_KEY"),
				os.Getenv("WIN568_SERVER_ID"),
			)
			if err != nil {
				log.Printf("❌ error resend win568: %v", err)
			}
		}
	}()
}
