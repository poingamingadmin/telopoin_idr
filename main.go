package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"telo/database"
	"telo/jobs"
	_ "telo/providers/casino"
	_ "telo/providers/slots"
	_ "telo/providers/sportsbook"
	"telo/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	database.Connect()

	host := os.Getenv("HOST")
	port := os.Getenv("PORT")

	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "3000"
	}

	app := fiber.New()
	routes.Setup(app)
	jobs.StartWin568Scheduler()

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Println("Server running at", addr)

	go func() {
		if err := app.Listen(addr); err != nil {
			log.Panicf("Failed to start server: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Gracefully shutting down...")
	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited cleanly")
}
