package evolutionlive

import (
	"log"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CancelRequest struct {
	SID         string      `json:"sid"`
	UserID      string      `json:"userId"`
	Currency    string      `json:"currency"`
	Game        GameWithID  `json:"game"`
	Transaction Transaction `json:"transaction"`
	UUID        string      `json:"uuid"`
}

func CancelHandler(c *fiber.Ctx) error {
	db := c.Locals("db").(*gorm.DB)

	var req CancelRequest

	if err := c.BodyParser(&req); err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Failed to parse cancel request: %v", req.UserID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "INVALID_PARAMETER",
			"message": "INVALID PARAMETER",
			"uuid":    req.UUID,
		})
	}

	var user models.User
	if err := db.Where("user_code = ?", req.UserID).First(&user).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Cancel: User not found", req.UserID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "INVALID_TOKEN_ID",
			"message": "INVALID TOKEN ID",
			"uuid":    req.UUID,
		})
	}

	if req.SID != "" {
		var session models.Session
		if err := db.Where("s_id = ? AND user_id = ?", req.SID, user.ID).First(&session).Error; err != nil {
			log.Printf("[EVOLUTIONLIVE] username=%s ⚠️ Cancel: SID not found or expired: %s", req.UserID, req.SID)
		}
	}

	var tx models.EvolutionTransaction
	if err := db.Where("ref_id = ? AND type = ?", req.Transaction.RefID, "DEBIT").First(&tx).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Cancel: RefID not found: %s", req.UserID, req.Transaction.RefID)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "BET_DOES_NOT_EXIST",
			"message": "Referenced bet not found",
			"uuid":    req.UUID,
		})
	}

	if tx.Status == "CANCEL" {
		log.Printf("[EVOLUTIONLIVE] username=%s ⚠️ Bet already cancelled for RefID=%s", req.UserID, req.Transaction.RefID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "BET_ALREADY_SETTLED",
			"balance": helpers.FormatFloat(user.Balance, 2),
			"uuid":    req.UUID,
		})
	}

	err := db.Transaction(func(txn *gorm.DB) error {
		user.Balance += req.Transaction.Amount
		if err := txn.Save(&user).Error; err != nil {
			return err
		}

		tx.Status = "CANCEL"
		if err := txn.Save(&tx).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ DB transaction error on cancel: %v", req.UserID, err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":  "TEMPORARY_ERROR",
			"message": "There is a temporary problem with the game server.",
			"uuid":    req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] username=%s ✅ Cancel success. RefID=%s Amount=%.2f NewBalance=%.2f",
		req.UserID, req.Transaction.RefID, req.Transaction.Amount, user.Balance)

	return c.JSON(fiber.Map{
		"status":  "OK",
		"balance": helpers.FormatFloat(user.Balance, 2),
		"uuid":    req.UUID,
	})
}
