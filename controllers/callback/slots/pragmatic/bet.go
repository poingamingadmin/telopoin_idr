package pragmatic

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"telo/database"
	"telo/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Bet(c *fiber.Ctx) error {
	start := time.Now()

	// Content-Type check
	ct := strings.ToLower(c.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		return c.JSON(fiber.Map{
			"currency":    "USD",
			"cash":        0.0,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       1000,
			"description": "Invalid content type",
		})
	}

	// Required fields
	providerId := c.FormValue("providerId")
	userId := c.FormValue("userId")
	gameId := c.FormValue("gameId")
	roundId := c.FormValue("roundId")
	amountStr := c.FormValue("amount")
	reference := c.FormValue("reference")
	hash := c.FormValue("hash")
	timestamp := c.FormValue("timestamp")

	if providerId == "" || userId == "" || gameId == "" || roundId == "" ||
		amountStr == "" || reference == "" || hash == "" || timestamp == "" {
		return c.JSON(fiber.Map{
			"currency":    "USD",
			"cash":        0.0,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       1001,
			"description": "Missing required parameters",
		})
	}

	// TODO: providerId + hash validation (optional)
	// if providerId != "pragmaticplay" { ... }
	// if !verifyHashPragmatic(c, "yoursecret") { ... }

	// Parse amount
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount < 0 {
		return c.JSON(fiber.Map{
			"currency":    "USD",
			"cash":        0.0,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       3002,
			"description": "Invalid amount",
		})
	}
	amountCents := int64(math.Round(amount * 100))

	// === Idempotency check ===
	var existing models.UserGameTransaction
	err = database.DB.Where("provider_tx = ? AND provider = ?", reference, "PRAGMATIC").
		First(&existing).Error
	if err == nil {
		var user models.User
		_ = database.DB.First(&user, existing.UserID).Error
		return c.JSON(fiber.Map{
			"transactionId": existing.ID,
			"currency":      user.Currency,
			"cash":          user.Balance,
			"bonus":         0.0,
			"usedPromo":     0,
			"error":         0,
			"description":   "Success",
		})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(fiber.Map{
			"currency":    "USD",
			"cash":        0.0,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       5001,
			"description": "DB error",
		})
	}

	// === Start TX ===
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", userId).First(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"currency":    "USD",
			"cash":        0.0,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       2001,
			"description": "User not found",
		})
	}
	if !user.IsActive {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"currency":    user.Currency,
			"cash":        0.0,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       2002,
			"description": "User inactive",
		})
	}
	if user.Balance < amount {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"currency":    user.Currency,
			"cash":        user.Balance,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       3001,
			"description": "Insufficient funds",
		})
	}

	before := user.Balance
	user.Balance -= amount
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"currency":    user.Currency,
			"cash":        before,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       5002,
			"description": "Failed to update balance",
		})
	}

	// Save UserGameTransaction
	ugtx := models.UserGameTransaction{
		UserID:        user.ID,
		UserCode:      user.UserCode,
		AgentCode:     user.AgentCode,
		GameID:        gameId,
		ProviderTx:    reference,
		Provider:      "PRAGMATIC",
		BetAmount:     amountCents,
		Currency:      user.Currency,
		BalanceBefore: before,
		BalanceAfter:  user.Balance,
		Status:        "Running",
		Note:          "Pragmatic Bet round " + roundId,
		RefID:         reference,
	}
	if err := tx.Create(&ugtx).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"currency":    user.Currency,
			"cash":        before,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       5003,
			"description": "Failed to create UserGameTransaction",
		})
	}

	// Save PragmaticTransaction
	prTx := models.PragmaticTransaction{
		UserID:        strconv.FormatUint(uint64(user.ID), 10),
		Currency:      user.Currency,
		Country:       &user.Country,
		Cash:          decimal.NewFromFloat(user.Balance),
		Amount:        decimal.NewFromFloat(amount),
		TotalBalance:  decimal.NewFromFloat(user.Balance),
		GameID:        &gameId,
		Reference:     reference,
		TransactionID: reference,
		Token:         user.UserCode,
		ErrorCode:     intPtr(0),
		Description:   strPtr("Success"),
	}
	if err := tx.Create(&prTx).Error; err != nil {
		tx.Rollback()
		return c.JSON(fiber.Map{
			"currency":    user.Currency,
			"cash":        before,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       5004,
			"description": "Failed to create PragmaticTransaction",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.JSON(fiber.Map{
			"currency":    user.Currency,
			"cash":        before,
			"bonus":       0.0,
			"usedPromo":   0,
			"error":       5005,
			"description": "Commit failed",
		})
	}

	return c.JSON(fiber.Map{
		"transactionId": ugtx.ID,
		"currency":      user.Currency,
		"cash":          user.Balance,
		"bonus":         0.0,
		"usedPromo":     0,
		"error":         0,
		"description":   "Success",
		"duration":      time.Since(start).String(),
	})
}
