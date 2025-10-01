package tasks

import (
	"log"
	"telo/database"
	"telo/models"
	"time"
)

func CleanupOldSlotTransactions() {
	sixHoursAgo := time.Now().Add(-6 * time.Hour)
	result := database.DB.
		Where("created_at < ?", sixHoursAgo).
		Delete(&models.TeloSlotTransaction{})

	if result.Error != nil {
		log.Println("❌ Failed to delete old transactions:", result.Error)
	} else {
		log.Printf("✅ Deleted %d old transactions older than 6 hours\n", result.RowsAffected)
	}
}
