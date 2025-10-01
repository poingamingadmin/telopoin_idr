// handlers/pragmatic_result_controller.go
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

// POST /result.html (x-www-form-urlencoded)
func Result(c *fiber.Ctx) error {
	db := database.DB // ✅ pakai global DB

	// Content-Type check
	ct := strings.ToLower(c.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		return c.JSON(errorResult("USD", 1000, "Invalid content type"))
	}

	// Required params
	providerId := c.FormValue("providerId")
	userId := c.FormValue("userId")
	gameId := c.FormValue("gameId")
	roundId := c.FormValue("roundId")
	amountStr := c.FormValue("amount")
	reference := c.FormValue("reference")
	hash := c.FormValue("hash")

	if providerId == "" || userId == "" || gameId == "" || roundId == "" ||
		amountStr == "" || reference == "" || hash == "" {
		return c.JSON(errorResult("USD", 1001, "Missing required parameters"))
	}

	// TODO: validasi providerId & hash kalau perlu

	// Parse win amount
	winAmt, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || winAmt < 0 {
		return c.JSON(errorResult("USD", 3002, "Invalid amount"))
	}
	// Tambahkan promoWinAmount kalau ada
	if promoStr := c.FormValue("promoWinAmount"); promoStr != "" {
		if v, err := strconv.ParseFloat(promoStr, 64); err == nil && v >= 0 {
			winAmt += v
		}
	}
	winCents := int64(math.Round(winAmt * 100))

	// Idempotent check → sudah ada Result untuk refId ini
	var existed models.UserGameTransaction
	if err := db.Where("ref_id = ? AND provider = ? AND status = ?", reference, "PRAGMATIC", "Settled").
		First(&existed).Error; err == nil {
		var user models.User
		_ = db.First(&user, existed.UserID).Error
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
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Ambil user
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_code = ?", userId).First(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorResult("USD", 2001, "User not found"))
	}
	if !user.IsActive {
		tx.Rollback()
		return c.JSON(errorResult(user.Currency, 2002, "User inactive"))
	}

	// Ambil bet asal yang masih Running
	var bet models.UserGameTransaction
	if err := tx.Where("ref_id = ? AND provider = ? AND status = ?", reference, "PRAGMATIC", "Running").
		First(&bet).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorResult(user.Currency, 2003, "Original bet not found or not Running"))
	}

	user.Balance += winAmt
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorResult(user.Currency, 5002, "Failed to update balance"))
	}

	// Update bet → Settled
	bet.Status = "Settled"
	bet.WinAmount = winCents
	bet.BalanceAfter = user.Balance
	if err := tx.Save(&bet).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorResult(user.Currency, 5003, "Failed to update UserGameTransaction"))
	}

	// Update/Insert PragmaticTransaction
	var prTx models.PragmaticTransaction
	if err := tx.Where("reference = ?", reference).First(&prTx).Error; err == nil {
		prTx.Cash = decimal.NewFromFloat(user.Balance)
		prTx.Amount = decimal.NewFromFloat(winAmt)
		prTx.TotalBalance = decimal.NewFromFloat(user.Balance)
		prTx.ErrorCode = intPtr(0)
		prTx.Description = strPtr("Result Success")
		if err := tx.Save(&prTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(errorResult(user.Currency, 5005, "Failed to update PragmaticTransaction"))
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.JSON(errorResult(user.Currency, 5006, "Commit failed"))
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"transactionId": bet.ID,
		"currency":      user.Currency,
		"cash":          user.Balance,
		"bonus":         0.0,
		"error":         0,
		"description":   "Success",
	})
}

// helper
func errorResult(currency string, code int, msg string) fiber.Map {
	return fiber.Map{
		"transactionId": "",
		"currency":      currency,
		"cash":          0.0,
		"bonus":         0.0,
		"error":         code,
		"description":   msg,
	}
}
