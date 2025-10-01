// handlers/pragmatic_refund_controller.go
package pragmatic

import (
	"net/http"
	"strconv"
	"strings"
	"telo/database"
	"telo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// POST /refund.html (x-www-form-urlencoded)
func Refund(c *fiber.Ctx) error {
	db := database.DB // ✅ ambil global DB

	// Content-Type check
	ct := strings.ToLower(c.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/x-www-form-urlencoded") {
		return c.JSON(errorRefund("USD", 1000, "Invalid content type"))
	}

	// Required params
	providerId := c.FormValue("providerId")
	userId := c.FormValue("userId")
	reference := c.FormValue("reference")
	hash := c.FormValue("hash")
	amountStr := c.FormValue("amount")

	if providerId == "" || userId == "" || reference == "" || hash == "" {
		return c.JSON(errorRefund("USD", 1001, "Missing required parameters"))
	}

	// --- idempotent check ---
	var existed models.UserGameTransaction
	if err := db.Where("ref_id = ? AND provider = ? AND status = ?", reference, "PRAGMATIC", "Refund").
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

	// cari bet asal
	var bet models.UserGameTransaction
	if err := db.Where("ref_id = ? AND provider = ?", reference, "PRAGMATIC").
		First(&bet).Error; err != nil {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"transactionId": "",
			"currency":      "USD",
			"cash":          0.0,
			"bonus":         0.0,
			"error":         0,
			"description":   "Success (no bet found)",
		})
	}

	// hitung refund
	refundAmt := float64(bet.BetAmount) / 100.0
	if amountStr != "" {
		if v, err := strconv.ParseFloat(amountStr, 64); err == nil && v >= 0 && v < refundAmt {
			refundAmt = v
		}
	}

	// TX begin
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_code = ?", bet.UserID, userId).First(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorRefund(bet.Currency, 2001, "User not found"))
	}
	if !user.IsActive {
		tx.Rollback()
		return c.JSON(errorRefund(user.Currency, 2002, "User inactive"))
	}

	user.Balance += refundAmt
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorRefund(user.Currency, 5002, "Failed to update balance"))
	}

	// update bet jadi Refund
	bet.Status = "Refund"
	bet.WinAmount = 0
	bet.BonusAmount = 0
	bet.BalanceAfter = user.Balance
	if err := tx.Save(&bet).Error; err != nil {
		tx.Rollback()
		return c.JSON(errorRefund(user.Currency, 5003, "Failed to update UserGameTransaction"))
	}

	// update PragmaticTransaction
	var prTx models.PragmaticTransaction
	if err := tx.Where("reference = ?", reference).First(&prTx).Error; err == nil {
		prTx.Cash = decimal.NewFromFloat(user.Balance)
		prTx.Amount = decimal.NewFromFloat(refundAmt)
		prTx.TotalBalance = decimal.NewFromFloat(user.Balance)
		prTx.ErrorCode = intPtr(0)
		prTx.Description = strPtr("Refund Success")
		if err := tx.Save(&prTx).Error; err != nil {
			tx.Rollback()
			return c.JSON(errorRefund(user.Currency, 5004, "Failed to update PragmaticTransaction"))
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.JSON(errorRefund(user.Currency, 5005, "Commit failed"))
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"transactionId": bet.ID,
		"currency":      user.Currency,
		"cash":          user.Balance,
		"bonus":         0.0,
		"error":         0,
		"description":   "Refund Success",
	})
}

// helper
func errorRefund(currency string, code int, msg string) fiber.Map {
	return fiber.Map{
		"transactionId": "",
		"currency":      currency,
		"cash":          0.0,
		"bonus":         0.0,
		"error":         code,
		"description":   msg,
	}
}
