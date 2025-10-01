package evolutionlive

import (
	"log"
	"telo/helpers"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DebitRequest struct {
	SID         string      `json:"sid"`
	UserID      string      `json:"userId"`
	Currency    string      `json:"currency"`
	Game        GameWithID  `json:"game"`
	Transaction Transaction `json:"transaction"`
	UUID        string      `json:"uuid"`
}

type GameWithID struct {
	ID      string       `json:"id"`
	Type    string       `json:"type"`
	Details *GameDetails `json:"details"`
}

type Transaction struct {
	ID     string  `json:"id"`
	RefID  string  `json:"refId"`
	Amount float64 `json:"amount"`
}

func DebitHandler(c *fiber.Ctx) error {
	db := c.Locals("db").(*gorm.DB)

	var req DebitRequest
	log.Println("📌 Raw body:", string(c.Body()))

	if err := c.BodyParser(&req); err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Failed to parse debit request: %v", req.UserID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "INVALID_PARAMETER",
			"message": "INVALID PARAMETER",
			"uuid":    req.UUID,
		})
	}

	var user models.User
	if err := db.Where("user_code = ?", req.UserID).First(&user).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ User not found", req.UserID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "INVALID_TOKEN_ID",
			"message": "INVALID TOKEN ID",
			"uuid":    req.UUID,
		})
	}

	var session models.Session
	if err := db.Where("s_id = ? AND user_id = ?", req.SID, user.ID).First(&session).Error; err != nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Invalid or expired SID: %s", req.UserID, req.SID)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "INVALID_SID",
			"message": "Invalid or expired SID",
			"uuid":    req.UUID,
		})
	}

	var existingTx models.EvolutionTransaction
	if err := db.Where("tx_id = ?", req.Transaction.ID).First(&existingTx).Error; err == nil {
		log.Printf("[EVOLUTIONLIVE] username=%s ⚠️ Debit duplicate bet id=%s", req.UserID, req.Transaction.ID)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "BET_ALREADY_EXIST",
			"balance": helpers.FormatFloat(user.Balance, 2),
			"uuid":    req.UUID,
		})
	}

	if user.Balance < req.Transaction.Amount {
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ Insufficient balance", req.UserID)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "INSUFFICIENT_FUNDS",
			"message": "Insufficient balance",
			"uuid":    req.UUID,
		})
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		user.Balance -= req.Transaction.Amount
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
			Type:     "DEBIT",
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
		log.Printf("[EVOLUTIONLIVE] username=%s ❌ DB transaction error: %v", req.UserID, err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":  "TEMPORARY_ERROR",
			"message": "There is a temporary problem with the game server.",
			"uuid":    req.UUID,
		})
	}

	log.Printf("[EVOLUTIONLIVE] username=%s ✅ Debit success. Bet ID=%s Amount=%.2f NewBalance=%.2f",
		req.UserID, req.Transaction.ID, req.Transaction.Amount, user.Balance)

	return c.JSON(fiber.Map{
		"status":  "OK",
		"balance": helpers.FormatFloat(user.Balance, 2),
		"uuid":    req.UUID,
	})
}
