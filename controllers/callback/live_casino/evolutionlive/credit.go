package evolutionlive

import (
	"log"
	"strings"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CreditRequest struct {
	SID         string      `json:"sid"`
	UserID      string      `json:"userId"`
	Currency    string      `json:"currency"`
	Game        GameWithID  `json:"game"`
	Transaction Transaction `json:"transaction"`
	UUID        string      `json:"uuid"`
}

func CreditHandler(c *fiber.Ctx) error {
	db := c.Locals("db").(*gorm.DB)

	var req CreditRequest
	log.Println("📌 Raw body:", string(c.Body()))

	if err := c.BodyParser(&req); err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Failed to parse credit request: %v", req.UserID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "INVALID_PARAMETER",
			"message": "INVALID PARAMETER",
			"uuid":    req.UUID,
		})
	}

	var user models.User
	if err := db.Where("user_code = ?", req.UserID).First(&user).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Credit: User not found", req.UserID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "INVALID_TOKEN_ID",
			"message": "INVALID TOKEN ID",
			"uuid":    req.UUID,
		})
	}

	var existingTx models.EvolutionTransaction
	if err := db.Where("tx_id = ?", req.Transaction.ID).First(&existingTx).Error; err == nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ⚠️ Credit duplicate transaction id=%s", req.UserID, req.Transaction.ID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "BET_ALREADY_EXIST",
			"balance": helpers.FormatFloat(user.Balance, 2),
			"uuid":    req.UUID,
		})
	}

	var refTx models.EvolutionTransaction
	if err := db.Where("ref_id = ? AND type = ?", req.Transaction.RefID, "DEBIT").First(&refTx).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Credit: RefID not found: %s", req.UserID, strings.TrimPrefix(req.Transaction.ID, "C"))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "BET_DOES_NOT_EXIST",
			"message": "Referenced bet not found",
			"uuid":    req.UUID,
		})
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		user.Balance += req.Transaction.Amount
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		evoTx := models.EvolutionTransaction{
			UserID:   user.ID,
			SID:      req.SID,
			TxID:     req.Transaction.ID,
			RefID:    req.Transaction.RefID,
			Amount:   req.Transaction.Amount,
			Currency: req.Currency,
			Type:     "CREDIT",
			GameID:   req.Game.ID,
			GameType: req.Game.Type,
			TableID:  req.Game.Details.Table.ID,
			TableVID: req.Game.Details.Table.VID,
			UUID:     req.UUID,
			Status:   "SUCCESS",
			Provider: "Evolution Live",
		}

		return tx.Create(&evoTx).Error
	})

	if err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ DB transaction error on credit: %v", req.UserID, err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":  "TEMPORARY_ERROR",
			"message": "There is a temporary problem with the game server.",
			"uuid":    req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] username=%s ✅ Credit success. Bet ID=%s RefID=%s Amount=%.2f NewBalance=%.2f",
		req.UserID, req.Transaction.ID, req.Transaction.RefID, req.Transaction.Amount, user.Balance)

	return c.JSON(fiber.Map{
		"status":  "OK",
		"balance": helpers.FormatFloat(user.Balance, 2),
		"uuid":    req.UUID,
	})
}
