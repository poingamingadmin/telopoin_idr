package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"telo/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, pass, name, port, sslmode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("❌ Failed to connect to database:", err)
	}

	DB = db
	log.Println("✅ Connected to database")

	autoMigrateEnv := os.Getenv("DB_AUTO_MIGRATE")
	autoMigrate, err := strconv.ParseBool(autoMigrateEnv)
	if err != nil {
		log.Printf("⚠️  Invalid value for DB_AUTO_MIGRATE: %s\n", autoMigrateEnv)
	}

	if autoMigrate {
		log.Println("🟡 Starting auto-migration...")

		if err := DB.AutoMigrate(
			&models.Agent{},
			&models.User{},
			&models.AgentTransaction{},
			&models.UserTransaction{},
			&models.TeloSlotTransaction{},
			&models.X568WinTransaction{},
			&models.Session{},
			&models.EvolutionTransaction{},
			&models.PragmaticTransaction{},
			&models.WmSubBet{},
			&models.FastSpinTransaction{},
			&models.SpadeGamingTransaction{},
			&models.Win568Bet{},
			&models.Win568SubBet{},
			&models.UserGameTransaction{},
		); err != nil {
			log.Fatal("❌ Failed to auto-migrate database:", err)
		}

		log.Println("✅ Auto migration completed")
	}
}
