// handlers/pragmatic_endround_controller.go
package pragmatic

import (
	"strconv"
	"strings"
	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// POST /endRound.html
func EndRound(c *fiber.Ctx) error {
	ct := strings.ToLower(c.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		return c.JSON(fiber.Map{
			"cash":        0.0,
			"bonus":       0.0,
			"error":       1000,
			"description": "Invalid content type",
		})
	}

	providerId := c.FormValue("providerId")
	userId := c.FormValue("userId")
	gameId := c.FormValue("gameId")
	roundId := c.FormValue("roundId")
	hash := c.FormValue("hash")

	if providerId == "" || userId == "" || gameId == "" || roundId == "" || hash == "" {
		return c.JSON(fiber.Map{
			"cash":        0.0,
			"bonus":       0.0,
			"error":       1001,
			"description": "Missing required parameters",
		})
	}

	// === TX start ===
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Lock user
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", userId).First(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"cash":        0.0,
			"bonus":       0.0,
			"error":       2001,
			"description": "User not found",
		})
	}
	if !user.IsActive {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"cash":        0.0,
			"bonus":       0.0,
			"error":       2002,
			"description": "User inactive",
		})
	}

	// Cari transaksi bet yang masih Running dengan roundId
	var ugtx models.UserGameTransaction
	if err := tx.Where("ref_id = ? AND provider = ? AND status = ?", roundId, "PRAGMATIC", "Running").
		First(&ugtx).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"cash":        user.Balance,
			"bonus":       0.0,
			"error":       3003,
			"description": "Running bet not found",
		})
	}

	// Update status ke Ended
	ugtx.Status = "Ended"
	if err := tx.Save(&ugtx).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"cash":        user.Balance,
			"bonus":       0.0,
			"error":       5003,
			"description": "Failed to update UserGameTransaction",
		})
	}

	var prTx models.PragmaticTransaction
	if err := tx.Where("reference = ? AND provider_id = ?", roundId, "PRAGMATIC").
		First(&prTx).Error; err == nil {
		prTx.Cash = decimal.NewFromFloat(user.Balance)
		prTx.TotalBalance = decimal.NewFromFloat(user.Balance)
		prTx.ErrorCode = intPtr(0)
		prTx.Description = strPtr("EndRound Success")
		if err := tx.Save(&prTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(fiber.Map{
				"cash":        user.Balance,
				"bonus":       0.0,
				"error":       5004,
				"description": "Failed to update PragmaticTransaction",
			})
		}
	} else {
		prTx = models.PragmaticTransaction{
			UserID:        strconv.FormatUint(uint64(user.ID), 10),
			Currency:      user.Currency,
			Country:       &user.Country,
			Cash:          decimal.NewFromFloat(user.Balance),
			Amount:        decimal.NewFromFloat(0),
			TotalBalance:  decimal.NewFromFloat(user.Balance),
			GameID:        &gameId,
			Reference:     roundId,
			TransactionID: roundId,
			Token:         user.UserCode,
			ErrorCode:     intPtr(0),
			Description:   strPtr("EndRound Success"),
			ProviderID:    strPtr("PRAGMATIC"),
		}
		if err := tx.Create(&prTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(fiber.Map{
				"cash":        user.Balance,
				"bonus":       0.0,
				"error":       5005,
				"description": "Failed to create PragmaticTransaction",
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.JSON(fiber.Map{
			"cash":        user.Balance,
			"bonus":       0.0,
			"error":       5005,
			"description": "Commit failed",
		})
	}

	return c.JSON(fiber.Map{
		"transactionId": ugtx.ID,
		"currency":      user.Currency,
		"cash":          user.Balance,
		"bonus":         0.0,
		"error":         0,
		"description":   "Success",
	})
}
