// handlers/pragmatic_adjustment_controller.go
package pragmatic

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// POST /adjustment.html (x-www-form-urlencoded)
func Adjustment(c *fiber.Ctx) error {
	// Content-Type check
	ct := strings.ToLower(c.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		return c.JSON(errorAdjustment("USD", 1000, "Invalid content type"))
	}

	// Required params
	providerId := c.FormValue("providerId")
	userId := c.FormValue("userId")
	gameId := c.FormValue("gameId")
	roundId := c.FormValue("roundId")
	amountStr := c.FormValue("amount")
	reference := c.FormValue("reference")
	validBetAmount := c.FormValue("validBetAmount")
	hash := c.FormValue("hash")
	timestamp := c.FormValue("timestamp")

	if providerId == "" || userId == "" || gameId == "" || roundId == "" ||
		amountStr == "" || reference == "" || validBetAmount == "" || hash == "" || timestamp == "" {
		return c.JSON(errorAdjustment("USD", 1001, "Missing required parameters"))
	}

	// TODO: validasi providerId & hash kalau perlu

	// Parse amount
	adjAmt, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return c.JSON(errorAdjustment("USD", 3002, "Invalid amount"))
	}
	adjCents := int64(math.Round(adjAmt * 100))

	// Idempotent check (sudah pernah Adjusted?)
	var existed models.UserGameTransaction
	if err := database.DB.Where("ref_id = ? AND provider = ? AND status = ?", reference, "PRAGMATIC", "Adjusted").
		First(&existed).Error; err == nil {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"transactionId": existed.ID,
			"currency":      existed.Currency,
			"cash":          existed.BalanceAfter,
			"bonus":         0.0,
			"error":         0,
			"description":   "Success (idempotent)",
		})
	}

	// TX begin
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
		return c.JSON(errorAdjustment("USD", 2001, "User not found"))
	}
	if !user.IsActive {
		tx.Rollback()
		return c.JSON(errorAdjustment(user.Currency, 2002, "User inactive"))
	}

	before := user.Balance
	newBalance := before + adjAmt
	if adjAmt < 0 && before < -adjAmt {
		tx.Rollback()
		return c.JSON(errorAdjustment(user.Currency, 1, "Insufficient balance"))
	}

	user.Balance = newBalance
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorAdjustment(user.Currency, 5002, "Failed to update balance"))
	}

	// Update/create UserGameTransaction
	var gameTx models.UserGameTransaction
	if err := tx.Where("ref_id = ? AND provider = ?", reference, "PRAGMATIC").First(&gameTx).Error; err == nil {
		// Update existing
		gameTx.Status = "Adjusted"
		gameTx.BalanceAfter = user.Balance
		gameTx.Note = "Pragmatic Adjustment " + gameId + " round " + roundId
		if err := tx.Save(&gameTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(errorAdjustment(user.Currency, 5003, "Failed to update UserGameTransaction"))
		}
	} else {
		// Create baru
		gameTx = models.UserGameTransaction{
			UserID:        user.ID,
			UserCode:      user.UserCode,
			AgentCode:     user.AgentCode,
			GameID:        gameId,
			ProviderTx:    reference,
			Provider:      "PRAGMATIC",
			BetAmount:     0,
			WinAmount:     0,
			BonusAmount:   0,
			Currency:      user.Currency,
			BalanceBefore: before,
			BalanceAfter:  user.Balance,
			Status:        "Adjusted",
			Note:          "Pragmatic Adjustment " + gameId + " round " + roundId,
			RefID:         reference,
		}
		if adjAmt > 0 {
			gameTx.WinAmount = adjCents
		} else {
			gameTx.BetAmount = adjCents
		}
		if err := tx.Create(&gameTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(errorAdjustment(user.Currency, 5004, "Failed to create UserGameTransaction"))
		}
	}

	// Update/insert PragmaticTransaction
	var prTx models.PragmaticTransaction
	if err := tx.Where("reference = ?", reference).First(&prTx).Error; err == nil {
		prTx.Cash = decimal.NewFromFloat(user.Balance)
		prTx.Amount = decimal.NewFromFloat(adjAmt)
		prTx.TotalBalance = decimal.NewFromFloat(user.Balance)
		prTx.ErrorCode = intPtr(0)
		prTx.Description = strPtr("Adjustment Success")
		if err := tx.Save(&prTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(errorAdjustment(user.Currency, 5005, "Failed to update PragmaticTransaction"))
		}
	} else {
		newTx := models.PragmaticTransaction{
			UserID:        strconv.FormatUint(uint64(user.ID), 10),
			Currency:      user.Currency,
			Country:       &user.Country,
			Cash:          decimal.NewFromFloat(user.Balance),
			Amount:        decimal.NewFromFloat(adjAmt),
			TotalBalance:  decimal.NewFromFloat(user.Balance),
			GameID:        &gameId,
			RoundID:       nil,
			Reference:     reference,
			TransactionID: reference,
			Token:         user.UserCode,
			ErrorCode:     intPtr(0),
			Description:   strPtr("Adjustment Success"),
		}
		if err := tx.Create(&newTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(errorAdjustment(user.Currency, 5006, "Failed to create PragmaticTransaction"))
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.JSON(errorAdjustment(user.Currency, 5007, "Commit failed"))
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"transactionId": gameTx.ID,
		"currency":      user.Currency,
		"cash":          user.Balance,
		"bonus":         0.0,
		"error":         0,
		"description":   "Adjustment Success",
	})
}

// helper error response
func errorAdjustment(currency string, code int, msg string) fiber.Map {
	return fiber.Map{
		"transactionId": "",
		"currency":      currency,
		"cash":          0.0,
		"bonus":         0.0,
		"error":         code,
		"description":   msg,
	}
}
